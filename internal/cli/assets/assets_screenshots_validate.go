package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	screenshotValidateSeverityInfo    = "info"
	screenshotValidateSeverityWarning = "warning"
	screenshotValidateSeverityError   = "error"
)

type screenshotValidateFile struct {
	Order    int    `json:"order"`
	FilePath string `json:"filePath"`
	FileName string `json:"fileName"`
	Hidden   bool   `json:"hidden,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Status   string `json:"status,omitempty"`
}

type screenshotValidateIssue struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	FilePath    string `json:"filePath,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	DuplicateOf string `json:"duplicateOf,omitempty"`
	Match       string `json:"match,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type screenshotValidateResult struct {
	Path           string                    `json:"path"`
	DisplayType    string                    `json:"displayType"`
	APIDisplayType string                    `json:"apiDisplayType,omitempty"`
	TotalFiles     int                       `json:"totalFiles"`
	ReadyFiles     int                       `json:"readyFiles"`
	ErrorCount     int                       `json:"errorCount"`
	WarningCount   int                       `json:"warningCount"`
	Files          []screenshotValidateFile  `json:"files,omitempty"`
	Issues         []screenshotValidateIssue `json:"issues,omitempty"`
}

// AssetsScreenshotsValidateCommand returns the screenshots validate subcommand.
func AssetsScreenshotsValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)

	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65 or IPAD_PRO_3GEN_129)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc screenshots validate --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Validate screenshot assets locally before upload.",
		LongHelp: `Validate screenshot assets locally before upload.

This preflight mirrors upload file ordering and reports common local problems
before any App Store Connect mutation happens, including hidden files,
unreadable assets, unsupported dimensions, images whose encoded format
contradicts their file extension, PNG alpha channels or transparency, and
duplicate decoded pixels.

Examples:
  asc screenshots validate --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots validate --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots validate --path "./screenshots" --device-type "IPHONE_65" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				return shared.RequiredFlag("--path")
			}
			deviceValue := strings.TrimSpace(*deviceType)
			if deviceValue == "" {
				return shared.RequiredFlag("--device-type")
			}

			displayType, err := normalizeScreenshotDisplayType(deviceValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			result, err := validateScreenshotAssets(pathValue, displayType)
			if err != nil {
				return fmt.Errorf("screenshots validate: %w", err)
			}

			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderScreenshotValidateResult(result, false) },
				func() error { return renderScreenshotValidateResult(result, true) },
			); err != nil {
				return err
			}

			if result.ErrorCount > 0 {
				return shared.NewValidationReportedError(fmt.Errorf("screenshots validate: found %d error(s)", result.ErrorCount))
			}

			return nil
		},
	}
}

func validateScreenshotAssets(pathValue, displayType string) (*screenshotValidateResult, error) {
	rawPath := strings.TrimSpace(pathValue)
	apiDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(displayType)
	result := &screenshotValidateResult{
		Path:        rawPath,
		DisplayType: displayType,
		Files:       make([]screenshotValidateFile, 0),
		Issues:      make([]screenshotValidateIssue, 0),
	}
	if apiDisplayType != "" && apiDisplayType != displayType {
		result.APIDisplayType = apiDisplayType
	}

	paths, err := collectAssetPaths(rawPath)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		appendScreenshotValidateIssue(result, screenshotValidateIssue{
			Code:        "no_files",
			Severity:    screenshotValidateSeverityError,
			Message:     fmt.Sprintf("no files found in %q", rawPath),
			Remediation: "Add screenshot image files or point --path at a specific screenshot.",
		})
		return result, nil
	}
	checkScreenshotFiles(result, paths, apiDisplayType)
	return result, nil
}

// checkScreenshotFiles runs every local screenshot check on paths in upload
// order and records the outcome in result. Uploads run it too, so validate
// and upload accept and reject the same files.
func checkScreenshotFiles(result *screenshotValidateResult, paths []string, apiDisplayType string) {
	result.TotalFiles = len(paths)
	seenPixelDigests := make(map[screenshotPixelDigest][]string, len(paths))
	for index, filePath := range paths {
		fileName := filepath.Base(filePath)
		fileResult := screenshotValidateFile{
			Order:    index + 1,
			FilePath: filePath,
			FileName: fileName,
		}

		hasError := false
		hasWarning := false
		if strings.HasPrefix(fileName, ".") {
			fileResult.Hidden = true
			hasWarning = true
			appendScreenshotValidateIssue(result, screenshotValidateIssue{
				Code:        "hidden_file",
				Severity:    screenshotValidateSeverityWarning,
				FilePath:    filePath,
				FileName:    fileName,
				Message:     fmt.Sprintf("hidden file %q will be included in upload ordering", fileName),
				Remediation: "Remove hidden files like .DS_Store from the upload directory before uploading screenshots.",
			})
		}

		if err := asc.ValidateImageFile(filePath); err != nil {
			hasError = true
			appendScreenshotValidateIssue(result, screenshotValidateIssue{
				Code:        "read_failure",
				Severity:    screenshotValidateSeverityError,
				FilePath:    filePath,
				FileName:    fileName,
				Message:     err.Error(),
				Remediation: "Keep only regular, readable screenshot image files in the upload directory.",
			})
			fileResult.Status = screenshotValidateSeverityError
			result.Files = append(result.Files, fileResult)
			continue
		}

		if issue, found := screenshotImageFormatIssue(filePath, fileName); found {
			hasError = true
			appendScreenshotValidateIssue(result, issue)
			if issue.Code != "alpha_channel" {
				fileResult.Status = screenshotValidateSeverityError
				result.Files = append(result.Files, fileResult)
				continue
			}
		}

		dimensions, format, err := asc.ReadImageDimensionsAndFormat(filePath)
		if err != nil {
			hasError = true
			appendScreenshotValidateIssue(result, screenshotValidateIssue{
				Code:        "read_failure",
				Severity:    screenshotValidateSeverityError,
				FilePath:    filePath,
				FileName:    fileName,
				Message:     err.Error(),
				Remediation: "Replace this file with a valid PNG or JPEG screenshot.",
			})
			fileResult.Status = screenshotValidateSeverityError
			result.Files = append(result.Files, fileResult)
			continue
		}
		fileResult.Width = dimensions.Width
		fileResult.Height = dimensions.Height

		if err := asc.ValidateImageFormatMatchesExtension(filePath, format); err != nil {
			hasError = true
			appendScreenshotValidateIssue(result, screenshotValidateIssue{
				Code:        "format_mismatch",
				Severity:    screenshotValidateSeverityError,
				FilePath:    filePath,
				FileName:    fileName,
				Message:     err.Error(),
				Remediation: screenshotFormatMismatchRemediation(filePath, format),
			})
		}

		if err := asc.ValidateScreenshotDimensionsForSize(filePath, dimensions.Width, dimensions.Height, apiDisplayType); err != nil {
			hasError = true
			appendScreenshotValidateIssue(result, screenshotValidateIssue{
				Code:        "dimension_mismatch",
				Severity:    screenshotValidateSeverityError,
				FilePath:    filePath,
				FileName:    fileName,
				Message:     err.Error(),
				Remediation: fmt.Sprintf("Resize or replace this file to match %s screenshot requirements.", apiDisplayType),
			})
		}

		if !hasError {
			hasError = checkDecodedPixelDuplicate(result, filePath, fileName, seenPixelDigests)
		}

		switch {
		case hasError:
			fileResult.Status = screenshotValidateSeverityError
		case hasWarning:
			fileResult.Status = screenshotValidateSeverityWarning
		default:
			fileResult.Status = "ok"
		}

		if !hasError {
			result.ReadyFiles++
		}
		result.Files = append(result.Files, fileResult)
	}
}

// screenshotFilesError turns the error issues from checkScreenshotFiles into
// the error an upload returns before reserving anything.
func screenshotFilesError(result *screenshotValidateResult) error {
	messages := make([]string, 0, result.ErrorCount)
	var diagnostic shared.DiagnosticCode
	for _, issue := range result.Issues {
		if issue.Severity != screenshotValidateSeverityError {
			continue
		}
		messages = append(messages, issue.Message)
		if diagnostic == "" {
			diagnostic = shared.DiagnosticFileInvalidFormat
			if issue.Code == "dimension_mismatch" {
				diagnostic = shared.DiagnosticInvalidInput
			}
		}
	}
	if len(messages) == 0 {
		return nil
	}
	if len(messages) == 1 {
		return markMediaInput(errors.New(messages[0]), diagnostic)
	}
	return markMediaInput(fmt.Errorf("%d screenshot problems; run asc screenshots validate for the full report:\n  %s", len(messages), strings.Join(messages, "\n  ")), diagnostic)
}

// screenshotImageFormatIssue runs the encoded-format and alpha check that
// screenshot uploads repeat on the opened file just before reserving an asset.
func screenshotImageFormatIssue(filePath, fileName string) (screenshotValidateIssue, bool) {
	issue := screenshotValidateIssue{
		Code:        "read_failure",
		Severity:    screenshotValidateSeverityError,
		FilePath:    filePath,
		FileName:    fileName,
		Remediation: "Replace this file with a valid PNG or JPEG screenshot.",
	}
	file, err := rootfs.OpenFile(filePath)
	if err != nil {
		issue.Message = err.Error()
		return issue, true
	}
	_, err = asc.ReadAppStoreImageFormatFrom(file)
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err == nil {
		return screenshotValidateIssue{}, false
	}
	issue.Message = fmt.Sprintf("%q: %v", filePath, err)
	if errors.Is(err, asc.ErrAppStoreImageAlpha) {
		issue.Code = "alpha_channel"
		issue.Remediation = "Flatten this image onto an opaque background and re-export it as an RGB PNG or a JPEG without an alpha channel; App Store Connect rejects an alpha channel even when every pixel is opaque."
	}
	return issue, true
}

// screenshotFormatMismatchRemediation keeps the structured advice in step with
// the message: a rename is only useful when screenshot uploads collect the
// real format's extension, which they do for PNG and JPEG but not for GIF.
func screenshotFormatMismatchRemediation(filePath, format string) string {
	if renamed, ok := asc.SuggestedImageFileName(filePath, format); ok {
		return fmt.Sprintf("Rename this file to %s, or re-export it in the format its extension names.", renamed)
	}
	return "Re-export this file in the format its extension names; screenshot uploads do not collect its real format."
}

func checkDecodedPixelDuplicate(result *screenshotValidateResult, filePath, fileName string, seenPixelDigests map[screenshotPixelDigest][]string) bool {
	digest, err := decodedScreenshotPixelDigest(filePath)
	if err != nil {
		appendScreenshotValidateIssue(result, screenshotValidateIssue{
			Code:        "read_failure",
			Severity:    screenshotValidateSeverityError,
			FilePath:    filePath,
			FileName:    fileName,
			Message:     err.Error(),
			Remediation: "Replace this file with a valid PNG or JPEG screenshot.",
		})
		return true
	}

	duplicateOf, err := findDecodedScreenshotDuplicate(filePath, seenPixelDigests[digest])
	if err != nil {
		appendScreenshotValidateIssue(result, screenshotValidateIssue{
			Code:        "read_failure",
			Severity:    screenshotValidateSeverityError,
			FilePath:    filePath,
			FileName:    fileName,
			Message:     err.Error(),
			Remediation: "Replace this file with a valid PNG or JPEG screenshot.",
		})
		return true
	}
	if duplicateOf == "" {
		seenPixelDigests[digest] = append(seenPixelDigests[digest], filePath)
		return false
	}

	duplicateName := filepath.Base(duplicateOf)
	appendScreenshotValidateIssue(result, screenshotValidateIssue{
		Code:        "duplicate_content",
		Severity:    screenshotValidateSeverityError,
		FilePath:    filePath,
		FileName:    fileName,
		DuplicateOf: duplicateName,
		Match:       "pixels",
		Message:     fmt.Sprintf("screenshot %q has decoded pixels identical to %q", fileName, duplicateName),
		Remediation: "Remove one duplicate or replace it with a distinct screenshot before uploading.",
	})
	return true
}

func findDecodedScreenshotDuplicate(path string, candidates []string) (string, error) {
	for _, candidate := range candidates {
		equal, err := decodedScreenshotPixelsEqual(candidate, path)
		if err != nil {
			return "", err
		}
		if equal {
			return candidate, nil
		}
	}
	return "", nil
}

func appendScreenshotValidateIssue(result *screenshotValidateResult, issue screenshotValidateIssue) {
	if result == nil {
		return
	}
	result.Issues = append(result.Issues, issue)
	switch issue.Severity {
	case screenshotValidateSeverityError:
		result.ErrorCount++
	case screenshotValidateSeverityWarning:
		result.WarningCount++
	}
}

func renderScreenshotValidateResult(result *screenshotValidateResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	apiDisplayType := result.DisplayType
	if strings.TrimSpace(result.APIDisplayType) != "" {
		apiDisplayType = result.APIDisplayType
	}

	summaryRows := [][]string{
		{"path", result.Path},
		{"displayType", result.DisplayType},
		{"totalFiles", fmt.Sprintf("%d", result.TotalFiles)},
		{"readyFiles", fmt.Sprintf("%d", result.ReadyFiles)},
		{"errorCount", fmt.Sprintf("%d", result.ErrorCount)},
		{"warningCount", fmt.Sprintf("%d", result.WarningCount)},
	}
	if strings.TrimSpace(result.APIDisplayType) != "" && result.APIDisplayType != result.DisplayType {
		summaryRows = append(summaryRows[0:2], append([][]string{{"apiDisplayType", apiDisplayType}}, summaryRows[2:]...)...)
	}
	shared.RenderSection("Summary", []string{"field", "value"}, summaryRows, markdown)

	if len(result.Files) > 0 {
		rows := make([][]string, 0, len(result.Files))
		for _, file := range result.Files {
			dimensions := "n/a"
			if file.Width > 0 && file.Height > 0 {
				dimensions = fmt.Sprintf("%dx%d", file.Width, file.Height)
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", file.Order),
				file.FileName,
				dimensions,
				file.Status,
				fmt.Sprintf("%t", file.Hidden),
				file.FilePath,
			})
		}
		shared.RenderSection("Files", []string{"order", "file", "dimensions", "status", "hidden", "path"}, rows, markdown)
	}

	issueRows := make([][]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issueRows = append(issueRows, []string{
			issue.Severity,
			issue.Code,
			shared.OrNA(issue.FileName),
			issue.Message,
			shared.OrNA(issue.Remediation),
		})
	}
	if len(issueRows) == 0 {
		issueRows = append(issueRows, []string{screenshotValidateSeverityInfo, "ok", "n/a", "No issues found", "n/a"})
	}
	shared.RenderSection("Issues", []string{"severity", "code", "file", "message", "remediation"}, issueRows, markdown)
	return nil
}
