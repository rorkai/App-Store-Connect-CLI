package assets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
unreadable assets, and unsupported dimensions.

Examples:
  asc screenshots validate --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots validate --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots validate --path "./screenshots" --device-type "IPHONE_65" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("screenshots validate does not accept positional arguments")
			}

			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return shared.MissingRequiredUsageError()
			}
			deviceValue := strings.TrimSpace(*deviceType)
			if deviceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --device-type is required")
				return shared.MissingRequiredUsageError()
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
				return shared.NewReportedError(fmt.Errorf("screenshots validate: found %d error(s)", result.ErrorCount))
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
	result.TotalFiles = len(paths)
	if len(paths) == 0 {
		appendScreenshotValidateIssue(result, screenshotValidateIssue{
			Code:        "no_files",
			Severity:    screenshotValidateSeverityError,
			Message:     fmt.Sprintf("no files found in %q", rawPath),
			Remediation: "Add screenshot image files or point --path at a specific screenshot.",
		})
		return result, nil
	}

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

		dimensions, err := asc.ReadImageDimensions(filePath)
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

	return result, nil
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
