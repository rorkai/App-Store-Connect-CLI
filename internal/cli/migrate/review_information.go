package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type ReviewInformation struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

const reviewInformationDir = "review_information"

// presentableImportResult returns the result to render. Without an explicit
// opt-in the imported demo account password is replaced so JSON, table, and
// Markdown output cannot carry it, while the original result keeps the real
// value for request construction and comparison.
func presentableImportResult(result *MigrateImportResult, includeSensitive bool) *MigrateImportResult {
	if includeSensitive || result == nil || result.ReviewInformation == nil {
		return result
	}
	safe := *result
	safe.ReviewInformation = result.ReviewInformation.redactedCopy()
	return &safe
}

func (info *ReviewInformation) redactedCopy() *ReviewInformation {
	if info == nil {
		return nil
	}
	safe := *info
	if info.DemoAccountPassword != nil {
		redacted := asc.RedactSecret(*info.DemoAccountPassword)
		safe.DemoAccountPassword = &redacted
	}
	return &safe
}

func readFastlaneReviewInformation(d *shared.Deps, metadataDir string) (*ReviewInformation, error) {
	root, prefix, err := newMigrateContentRoot(d, metadataDir)
	if err != nil {
		return nil, err
	}
	if err := checkContentRootContained(root, prefix); err != nil {
		return nil, err
	}
	if exists, err := dirExists(filepath.Join(metadataDir, reviewInformationDir)); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	info := &ReviewInformation{}
	assigned := 0
	fields := []struct {
		file  string
		field **string
	}{
		{"first_name.txt", &info.ContactFirstName},
		{"last_name.txt", &info.ContactLastName},
		{"phone_number.txt", &info.ContactPhone},
		{"email_address.txt", &info.ContactEmail},
		{"demo_user.txt", &info.DemoAccountName},
		{"demo_password.txt", &info.DemoAccountPassword},
		{"notes.txt", &info.Notes},
	}
	for _, field := range fields {
		value, ok, err := readOptionalFile(root, filepath.Join(prefix, reviewInformationRelativePath(field.file)))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		stored := value
		*field.field = &stored
		assigned++
	}

	required, err := readOptionalReviewRequired(root, prefix)
	if err != nil {
		return nil, err
	}
	if required != nil {
		info.DemoAccountRequired = required
		assigned++
	}

	if assigned == 0 {
		return nil, nil
	}
	if err := asc.ValidateSecretMutationValue(info.DemoAccountPassword); err != nil {
		return nil, fmt.Errorf("review information: %w", err)
	}
	return info, nil
}

func buildReviewDetailCreateAttributes(info *ReviewInformation) *asc.AppStoreReviewDetailCreateAttributes {
	if info == nil {
		return nil
	}
	return &asc.AppStoreReviewDetailCreateAttributes{
		ContactFirstName:    info.ContactFirstName,
		ContactLastName:     info.ContactLastName,
		ContactPhone:        info.ContactPhone,
		ContactEmail:        info.ContactEmail,
		DemoAccountName:     info.DemoAccountName,
		DemoAccountPassword: info.DemoAccountPassword,
		DemoAccountRequired: info.DemoAccountRequired,
		Notes:               info.Notes,
	}
}

func buildReviewDetailUpdateAttributes(info *ReviewInformation) asc.AppStoreReviewDetailUpdateAttributes {
	if info == nil {
		return asc.AppStoreReviewDetailUpdateAttributes{}
	}
	return asc.AppStoreReviewDetailUpdateAttributes{
		ContactFirstName:    info.ContactFirstName,
		ContactLastName:     info.ContactLastName,
		ContactPhone:        info.ContactPhone,
		ContactEmail:        info.ContactEmail,
		DemoAccountName:     info.DemoAccountName,
		DemoAccountPassword: info.DemoAccountPassword,
		DemoAccountRequired: info.DemoAccountRequired,
		Notes:               info.Notes,
	}
}

func reviewInformationMatches(existing asc.AppStoreReviewDetailAttributes, info *ReviewInformation) bool {
	if info == nil {
		return true
	}
	if info.ContactFirstName != nil && existing.ContactFirstName != *info.ContactFirstName {
		return false
	}
	if info.ContactLastName != nil && existing.ContactLastName != *info.ContactLastName {
		return false
	}
	if info.ContactPhone != nil && existing.ContactPhone != *info.ContactPhone {
		return false
	}
	if info.ContactEmail != nil && existing.ContactEmail != *info.ContactEmail {
		return false
	}
	if info.DemoAccountName != nil && existing.DemoAccountName != *info.DemoAccountName {
		return false
	}
	if info.DemoAccountPassword != nil && existing.DemoAccountPassword != *info.DemoAccountPassword {
		return false
	}
	if info.DemoAccountRequired != nil && existing.DemoAccountRequired != *info.DemoAccountRequired {
		return false
	}
	if info.Notes != nil && existing.Notes != *info.Notes {
		return false
	}
	return true
}

func readOptionalReviewRequired(root rootfs.Root, prefix string) (*bool, error) {
	primary := filepath.Join(prefix, reviewInformationRelativePath("demo_account_required.txt"))
	secondary := filepath.Join(prefix, reviewInformationRelativePath("demo_required.txt"))

	primaryValue, primaryExists, err := readOptionalFile(root, primary)
	if err != nil {
		return nil, err
	}
	secondaryValue, secondaryExists, err := readOptionalFile(root, secondary)
	if err != nil {
		return nil, err
	}

	if !primaryExists && !secondaryExists {
		return nil, nil
	}

	primaryParsed, primaryErr := parseReviewRequiredValue(filepath.Join(root.Path(), primary), primaryValue, primaryExists)
	if primaryErr != nil {
		return nil, primaryErr
	}
	secondaryParsed, secondaryErr := parseReviewRequiredValue(filepath.Join(root.Path(), secondary), secondaryValue, secondaryExists)
	if secondaryErr != nil {
		return nil, secondaryErr
	}
	if primaryParsed != nil && secondaryParsed != nil && *primaryParsed != *secondaryParsed {
		return nil, fmt.Errorf("review_information contains conflicting demo required values")
	}
	if primaryParsed != nil {
		return primaryParsed, nil
	}
	return secondaryParsed, nil
}

func parseReviewRequiredValue(path, value string, exists bool) (*bool, error) {
	if !exists {
		return nil, nil
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("review_information %s must be true or false", path)
	}
	switch strings.ToLower(trimmed) {
	case "true":
		v := true
		return &v, nil
	case "false":
		v := false
		return &v, nil
	default:
		return nil, fmt.Errorf("review_information %s must be true or false", path)
	}
}

// fetchExportReviewDetail returns nil when the version has no App Review detail.
func fetchExportReviewDetail(ctx context.Context, d *shared.Deps, client *asc.Client, versionID string) (*asc.AppStoreReviewDetailAttributes, error) {
	requestCtx, cancel := migrateRequestContext(ctx, d)
	defer cancel()
	resp, err := client.GetAppStoreReviewDetailForVersion(requestCtx, versionID)
	if err != nil {
		if isNotFoundReviewDetail(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch review information: %w", err)
	}
	if resp == nil {
		return nil, nil
	}
	return &resp.Data.Attributes, nil
}

// writeExportReviewInformation writes the review information files that
// import reads, except the demo account password, which is never written in
// clear text.
func writeExportReviewInformation(root rootfs.Root, attrs *asc.AppStoreReviewDetailAttributes) (int, error) {
	dir := filepath.Join("metadata", reviewInformationDir)
	var detail asc.AppStoreReviewDetailAttributes
	required := ""
	if attrs != nil {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return 0, fmt.Errorf("failed to create review information directory: %w", err)
		}
		detail = *attrs
		required = strconv.FormatBool(attrs.DemoAccountRequired)
	}
	// Empty content removes a file left by an earlier export, including the
	// alternate demo required name that import rejects when it disagrees.
	files := []struct {
		name    string
		content string
	}{
		{"first_name.txt", detail.ContactFirstName},
		{"last_name.txt", detail.ContactLastName},
		{"phone_number.txt", detail.ContactPhone},
		{"email_address.txt", detail.ContactEmail},
		{"demo_user.txt", detail.DemoAccountName},
		{"demo_required.txt", required},
		{"demo_account_required.txt", ""},
		{"notes.txt", detail.Notes},
	}
	total := 0
	for _, file := range files {
		written, err := writeAndCount(root, filepath.Join(dir, file.name), file.content)
		if err != nil {
			return 0, err
		}
		total += written
	}
	return total, nil
}

func reviewInformationRelativePath(file string) string {
	return filepath.Join(reviewInformationDir, file)
}

func readOptionalFile(root rootfs.Root, name string) (string, bool, error) {
	data, found, err := root.ReadFileOptional(name)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return strings.TrimSpace(string(data)), true, nil
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}
