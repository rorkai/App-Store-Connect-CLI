package xcodecloud

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	maxDoctorLogBundleBytes       = 64 << 20
	maxDoctorLogEntryBytes        = 8 << 20
	maxDoctorLogUncompressedBytes = 64 << 20
	maxDoctorLogEntries           = 2000
	maxDoctorDiagnostics          = 20
	maxDoctorDiagnosticLength     = 2048
)

var doctorITMSCodePattern = regexp.MustCompile(`(?i)\bITMS-[0-9]+\b`)

type doctorLogBundleAnalysis struct {
	ExportStatus         string
	Diagnostics          []asc.XcodeCloudDoctorLogDiagnostic
	DiagnosticsTruncated bool
}

func inspectXcodeCloudDoctorLogs(ctx context.Context, client *asc.Client, result *asc.XcodeCloudDoctorResult, options xcodeCloudDoctorOptions) error {
	failedActions := doctorFailedActionIDs(result)
	inspectAll := strings.TrimSpace(options.SaveLogs) != ""
	var saveRoot rootfs.Root
	if inspectAll {
		var err error
		saveRoot, err = rootfs.New(options.SaveLogs)
		if err != nil {
			return fmt.Errorf("prepare --save-logs directory: %w", err)
		}
		defer saveRoot.Close()
		if err := saveRoot.MkdirAll(".", 0o700); err != nil {
			return fmt.Errorf("prepare --save-logs directory: %w", err)
		}
	}

	for _, action := range result.Actions {
		if !inspectAll {
			if _, failed := failedActions[action.ID]; !failed {
				continue
			}
		}
		for _, artifact := range action.Artifacts {
			if !strings.EqualFold(strings.TrimSpace(artifact.FileType), "LOG_BUNDLE") {
				continue
			}
			var (
				bundleResult asc.XcodeCloudDoctorLogBundle
				inspectErr   error
			)
			if inspectAll {
				name := doctorSavedLogBundleName(artifact)
				bundleResult, inspectErr = downloadSaveAndAnalyzeDoctorLogBundle(ctx, client, action.ID, artifact, saveRoot, options.SaveLogs, name)
			} else {
				bundleResult, inspectErr = downloadAndAnalyzeDoctorLogBundle(ctx, client, action.ID, artifact)
			}
			if inspectErr != nil {
				if errors.Is(inspectErr, context.Canceled) || errors.Is(inspectErr, context.DeadlineExceeded) {
					return inspectErr
				}
				result.LogBundles = append(result.LogBundles, bundleResult)
				remediation := fmt.Sprintf("Download artifact %s with asc xcode-cloud artifacts download and inspect it locally.", artifact.ID)
				if bundleResult.SavedPath != "" {
					remediation = fmt.Sprintf("Inspect the saved bundle %s locally.", asc.SanitizeTerminalText(bundleResult.SavedPath))
				}
				result.CoverageWarnings = append(result.CoverageWarnings, asc.XcodeCloudDoctorCoverageWarning{
					ID:          "log_bundle_inspection_failed",
					Message:     fmt.Sprintf("Could not inspect log bundle %s: %s", artifact.ID, asc.SanitizeTerminalText(inspectErr.Error())),
					Remediation: remediation,
				})
				continue
			}
			result.LogBundles = append(result.LogBundles, bundleResult)
			if bundleResult.DiagnosticsTruncated {
				remediation := fmt.Sprintf("Re-run with --save-logs and inspect artifact %s locally for additional diagnostics.", artifact.ID)
				if bundleResult.SavedPath != "" {
					remediation = fmt.Sprintf("Inspect the saved bundle %s locally for additional diagnostics.", asc.SanitizeTerminalText(bundleResult.SavedPath))
				}
				result.CoverageWarnings = append(result.CoverageWarnings, asc.XcodeCloudDoctorCoverageWarning{
					ID:          "log_diagnostics_truncated",
					Message:     fmt.Sprintf("Log bundle %s contains more than %d distinct ITMS diagnostics; only the first %d are reported.", artifact.ID, maxDoctorDiagnostics, maxDoctorDiagnostics),
					Remediation: remediation,
				})
			}
			if bundleResult.Inspected {
				result.Summary.LogBundlesInspected++
			}
		}
	}
	return nil
}

func newDoctorLogBundleResult(actionID string, artifact asc.XcodeCloudDoctorArtifact) asc.XcodeCloudDoctorLogBundle {
	return asc.XcodeCloudDoctorLogBundle{
		ArtifactID:  artifact.ID,
		ActionID:    actionID,
		FileName:    artifact.FileName,
		FileSize:    artifact.FileSize,
		Diagnostics: make([]asc.XcodeCloudDoctorLogDiagnostic, 0),
	}
}

func downloadAndAnalyzeDoctorLogBundle(ctx context.Context, client *asc.Client, actionID string, artifact asc.XcodeCloudDoctorArtifact) (asc.XcodeCloudDoctorLogBundle, error) {
	result := newDoctorLogBundleResult(actionID, artifact)
	if artifact.FileSize > maxDoctorLogBundleBytes {
		return result, fmt.Errorf("bundle size %d exceeds the %d-byte inspection limit", artifact.FileSize, maxDoctorLogBundleBytes)
	}

	body, err := openDoctorLogBundle(ctx, client, artifact.ID)
	if err != nil {
		return result, err
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, maxDoctorLogBundleBytes+1))
	if err != nil {
		return result, fmt.Errorf("read artifact: %w", err)
	}
	if len(data) > maxDoctorLogBundleBytes {
		return result, fmt.Errorf("download exceeds the %d-byte inspection limit", maxDoctorLogBundleBytes)
	}
	return analyzeDoctorLogBundleInto(result, data)
}

func downloadSaveAndAnalyzeDoctorLogBundle(
	ctx context.Context,
	client *asc.Client,
	actionID string,
	artifact asc.XcodeCloudDoctorArtifact,
	saveRoot rootfs.Root,
	saveDirectory string,
	name string,
) (asc.XcodeCloudDoctorLogBundle, error) {
	result := newDoctorLogBundleResult(actionID, artifact)
	body, err := openDoctorLogBundle(ctx, client, artifact.ID)
	if err != nil {
		return result, err
	}
	defer body.Close()
	return saveAndAnalyzeDoctorLogBundle(saveRoot, saveDirectory, name, result, body, maxDoctorLogBundleBytes)
}

func saveAndAnalyzeDoctorLogBundle(
	saveRoot rootfs.Root,
	saveDirectory string,
	name string,
	result asc.XcodeCloudDoctorLogBundle,
	reader io.Reader,
	inspectionLimit int64,
) (asc.XcodeCloudDoctorLogBundle, error) {
	written, err := saveRoot.CreateNewFrom(name, reader, 0o600)
	if err != nil {
		return result, fmt.Errorf("save log bundle %q: %w", filepath.Join(saveDirectory, name), err)
	}
	result.SavedPath = filepath.Join(saveDirectory, name)
	if written > inspectionLimit {
		return result, fmt.Errorf("saved bundle size %d exceeds the %d-byte inspection limit", written, inspectionLimit)
	}
	data, err := saveRoot.ReadFileLimited(name, inspectionLimit)
	if err != nil {
		return result, fmt.Errorf("read saved log bundle: %w", err)
	}
	return analyzeDoctorLogBundleInto(result, data)
}

func analyzeDoctorLogBundleInto(result asc.XcodeCloudDoctorLogBundle, data []byte) (asc.XcodeCloudDoctorLogBundle, error) {
	analysis, err := analyzeDoctorLogBundle(data)
	if err != nil {
		return result, err
	}
	result.Inspected = true
	result.ExportStatus = analysis.ExportStatus
	result.Diagnostics = analysis.Diagnostics
	result.DiagnosticsTruncated = analysis.DiagnosticsTruncated
	return result, nil
}

func openDoctorLogBundle(ctx context.Context, client *asc.Client, artifactID string) (io.ReadCloser, error) {
	detail, err := client.GetCiArtifact(ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("fetch artifact details: %w", err)
	}
	downloadURL := strings.TrimSpace(detail.Data.Attributes.DownloadURL)
	if downloadURL == "" {
		return nil, fmt.Errorf("artifact has no download URL")
	}
	download, err := client.DownloadCiArtifact(ctx, downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download artifact: %w", err)
	}
	return download.Body, nil
}

func analyzeDoctorLogBundle(data []byte) (doctorLogBundleAnalysis, error) {
	if len(data) == 0 {
		return doctorLogBundleAnalysis{}, fmt.Errorf("log bundle is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if bytes.IndexByte(data, 0) >= 0 {
			return doctorLogBundleAnalysis{}, fmt.Errorf("log bundle is neither a ZIP archive nor plain text")
		}
		analysis := doctorLogBundleAnalysis{Diagnostics: make([]asc.XcodeCloudDoctorLogDiagnostic, 0)}
		analyzeDoctorLogText(&analysis, "", string(data))
		return analysis, nil
	}
	if len(reader.File) > maxDoctorLogEntries {
		return doctorLogBundleAnalysis{}, fmt.Errorf("log bundle contains %d entries; limit is %d", len(reader.File), maxDoctorLogEntries)
	}

	analysis := doctorLogBundleAnalysis{Diagnostics: make([]asc.XcodeCloudDoctorLogDiagnostic, 0)}
	var total int64
	readableEntries := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !isDoctorLogTextFile(file.Name) {
			continue
		}
		if file.UncompressedSize64 > maxDoctorLogEntryBytes {
			return doctorLogBundleAnalysis{}, fmt.Errorf("log entry %q exceeds the %d-byte inspection limit", file.Name, maxDoctorLogEntryBytes)
		}
		if file.UncompressedSize64 > uint64(maxDoctorLogUncompressedBytes-int(total)) {
			return doctorLogBundleAnalysis{}, fmt.Errorf("log bundle exceeds the %d-byte uncompressed inspection limit", maxDoctorLogUncompressedBytes)
		}
		entry, err := file.Open()
		if err != nil {
			return doctorLogBundleAnalysis{}, fmt.Errorf("open log entry %q: %w", file.Name, err)
		}
		contents, readErr := io.ReadAll(io.LimitReader(entry, maxDoctorLogEntryBytes+1))
		closeErr := entry.Close()
		if readErr != nil {
			return doctorLogBundleAnalysis{}, fmt.Errorf("read log entry %q: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return doctorLogBundleAnalysis{}, fmt.Errorf("close log entry %q: %w", file.Name, closeErr)
		}
		if len(contents) > maxDoctorLogEntryBytes {
			return doctorLogBundleAnalysis{}, fmt.Errorf("log entry %q exceeds the %d-byte inspection limit", file.Name, maxDoctorLogEntryBytes)
		}
		total += int64(len(contents))
		if len(contents) == 0 {
			continue
		}
		if bytes.IndexByte(contents, 0) >= 0 {
			continue
		}
		readableEntries++
		analyzeDoctorLogText(&analysis, file.Name, string(contents))
	}
	if readableEntries == 0 {
		return doctorLogBundleAnalysis{}, fmt.Errorf("log bundle contains no readable text entries")
	}
	return analysis, nil
}

func analyzeDoctorLogText(analysis *doctorLogBundleAnalysis, sourceFile, contents string) {
	upper := strings.ToUpper(contents)
	if strings.Contains(upper, "** EXPORT FAILED **") {
		analysis.ExportStatus = "FAILED"
	} else if analysis.ExportStatus == "" && strings.Contains(upper, "** EXPORT SUCCEEDED **") {
		analysis.ExportStatus = "SUCCEEDED"
	}

	seen := make(map[string]struct{}, len(analysis.Diagnostics))
	for _, diagnostic := range analysis.Diagnostics {
		seen[diagnostic.Code+"\x00"+diagnostic.Message] = struct{}{}
	}
	for _, line := range strings.Split(contents, "\n") {
		code := strings.ToUpper(doctorITMSCodePattern.FindString(line))
		if code == "" {
			continue
		}
		message := strings.TrimSpace(asc.SanitizeTerminalText(line))
		if len(message) > maxDoctorDiagnosticLength {
			truncated := message[:maxDoctorDiagnosticLength]
			for len(truncated) > 0 && !utf8.ValidString(truncated) {
				truncated = truncated[:len(truncated)-1]
			}
			message = truncated + "…"
		}
		key := code + "\x00" + message
		if _, exists := seen[key]; exists {
			continue
		}
		if len(analysis.Diagnostics) >= maxDoctorDiagnostics {
			analysis.DiagnosticsTruncated = true
			return
		}
		seen[key] = struct{}{}
		analysis.Diagnostics = append(analysis.Diagnostics, asc.XcodeCloudDoctorLogDiagnostic{
			Code:       code,
			Message:    message,
			SourceFile: sourceFile,
		})
	}
}

func isDoctorLogTextFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".log", ".txt", ".json", ".xml":
		return true
	default:
		return false
	}
}

func isFailedDoctorAction(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FAILED", "ERRORED":
		return true
	default:
		return false
	}
}

func doctorSavedLogBundleName(artifact asc.XcodeCloudDoctorArtifact) string {
	artifactID := sanitizeDoctorFileComponent(artifact.ID)
	fileName := sanitizeDoctorFileComponent(filepath.Base(strings.TrimSpace(artifact.FileName)))
	if fileName == "" {
		fileName = "log-bundle.zip"
	}
	if artifactID == "" {
		return fileName
	}
	return artifactID + "-" + fileName
}

func sanitizeDoctorFileComponent(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '.' || char == '-' || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "._")
}
