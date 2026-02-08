package reviews

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

// ReviewDetailsAttachmentsListCommand returns the review attachments list subcommand.
func ReviewDetailsAttachmentsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("attachments-list", flag.ExitOnError)

	reviewDetailID := fs.String("review-detail", "", "App Store review detail ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(reviewAttachmentFieldList(), ", "))
	detailFields := fs.String("detail-fields", "", "Review detail fields to include: "+strings.Join(reviewDetailFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewAttachmentIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "attachments-list",
		ShortUsage: "asc review attachments-list --review-detail \"REVIEW_DETAIL_ID\"",
		ShortHelp:  "List review attachments for a review detail.",
		LongHelp: `List review attachments for a review detail.

Examples:
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID"
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID" --fields "fileName,fileSize" --limit 50
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review attachments-list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}

			fieldsValue, err := normalizeReviewAttachmentFields(*fields)
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}
			detailFieldsValue, err := normalizeReviewDetailFields(*detailFields)
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}
			includeValue, err := normalizeReviewAttachmentInclude(*include)
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}

			reviewDetailValue := strings.TrimSpace(*reviewDetailID)
			if reviewDetailValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --review-detail is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreReviewAttachmentsOption{
				asc.WithAppStoreReviewAttachmentsFields(fieldsValue),
				asc.WithAppStoreReviewAttachmentReviewDetailFields(detailFieldsValue),
				asc.WithAppStoreReviewAttachmentsInclude(includeValue),
				asc.WithAppStoreReviewAttachmentsLimit(*limit),
				asc.WithAppStoreReviewAttachmentsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreReviewAttachmentsLimit(200))
				firstPage, err := client.GetAppStoreReviewAttachmentsForReviewDetail(requestCtx, reviewDetailValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("review attachments-list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreReviewAttachmentsForReviewDetail(ctx, reviewDetailValue, asc.WithAppStoreReviewAttachmentsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("review attachments-list: %w", err)
				}

				return shared.PrintOutput(pages, *output, *pretty)
			}

			resp, err := client.GetAppStoreReviewAttachmentsForReviewDetail(requestCtx, reviewDetailValue, opts...)
			if err != nil {
				return fmt.Errorf("review attachments-list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ReviewDetailsAttachmentsGetCommand returns the review attachments get subcommand.
func ReviewDetailsAttachmentsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("attachments-get", flag.ExitOnError)

	attachmentID := fs.String("id", "", "Review attachment ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(reviewAttachmentFieldList(), ", "))
	detailFields := fs.String("detail-fields", "", "Review detail fields to include: "+strings.Join(reviewDetailFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewAttachmentIncludeList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "attachments-get",
		ShortUsage: "asc review attachments-get --id \"ATTACHMENT_ID\"",
		ShortHelp:  "Get a review attachment by ID.",
		LongHelp: `Get a review attachment by ID.

Examples:
  asc review attachments-get --id "ATTACHMENT_ID"
  asc review attachments-get --id "ATTACHMENT_ID" --fields "fileName,fileSize"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			attachmentValue := strings.TrimSpace(*attachmentID)
			if attachmentValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeReviewAttachmentFields(*fields)
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}
			detailFieldsValue, err := normalizeReviewDetailFields(*detailFields)
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}
			includeValue, err := normalizeReviewAttachmentInclude(*include)
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreReviewAttachment(requestCtx, attachmentValue,
				asc.WithAppStoreReviewAttachmentsFields(fieldsValue),
				asc.WithAppStoreReviewAttachmentReviewDetailFields(detailFieldsValue),
				asc.WithAppStoreReviewAttachmentsInclude(includeValue),
			)
			if err != nil {
				return fmt.Errorf("review attachments-get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ReviewDetailsAttachmentsUploadCommand returns the review attachments upload subcommand.
func ReviewDetailsAttachmentsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("attachments-upload", flag.ExitOnError)

	reviewDetailID := fs.String("review-detail", "", "App Store review detail ID (required)")
	filePath := fs.String("file", "", "Path to attachment file (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "attachments-upload",
		ShortUsage: "asc review attachments-upload --review-detail \"REVIEW_DETAIL_ID\" --file ./attachment.pdf",
		ShortHelp:  "Upload a review attachment.",
		LongHelp: `Upload a review attachment.

Examples:
  asc review attachments-upload --review-detail "REVIEW_DETAIL_ID" --file ./review-doc.pdf`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			reviewDetailValue := strings.TrimSpace(*reviewDetailID)
			if reviewDetailValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --review-detail is required")
				return flag.ErrHelp
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			info, err := os.Lstat(pathValue)
			if err != nil {
				return fmt.Errorf("review attachments-upload: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("review attachments-upload: refusing to read symlink %q", pathValue)
			}
			if info.IsDir() {
				return fmt.Errorf("review attachments-upload: %q is a directory", pathValue)
			}
			if info.Size() <= 0 {
				return fmt.Errorf("review attachments-upload: file size must be greater than 0")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review attachments-upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreReviewAttachment(requestCtx, reviewDetailValue, filepath.Base(pathValue), info.Size())
			if err != nil {
				return fmt.Errorf("review attachments-upload: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("review attachments-upload: no upload operations returned")
			}

			uploadCtx, uploadCancel := shared.ContextWithUploadTimeout(ctx)
			err = asc.ExecuteUploadOperations(uploadCtx, pathValue, resp.Data.Attributes.UploadOperations)
			uploadCancel()
			if err != nil {
				return fmt.Errorf("review attachments-upload: upload failed: %w", err)
			}

			checksum, err := asc.ComputeFileChecksum(pathValue, asc.ChecksumAlgorithmMD5)
			if err != nil {
				return fmt.Errorf("review attachments-upload: checksum failed: %w", err)
			}

			uploaded := true
			updateAttrs := asc.AppStoreReviewAttachmentUpdateAttributes{
				SourceFileChecksum: &checksum.Hash,
				Uploaded:           &uploaded,
			}

			commitCtx, commitCancel := shared.ContextWithUploadTimeout(ctx)
			commitResp, err := client.UpdateAppStoreReviewAttachment(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("review attachments-upload: failed to commit upload: %w", err)
			}

			return shared.PrintOutput(commitResp, *output, *pretty)
		},
	}
}

// ReviewDetailsAttachmentsDeleteCommand returns the review attachments delete subcommand.
func ReviewDetailsAttachmentsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("attachments-delete", flag.ExitOnError)

	attachmentID := fs.String("id", "", "Review attachment ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "attachments-delete",
		ShortUsage: "asc review attachments-delete --id \"ATTACHMENT_ID\" --confirm",
		ShortHelp:  "Delete a review attachment.",
		LongHelp: `Delete a review attachment.

Examples:
  asc review attachments-delete --id "ATTACHMENT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			attachmentValue := strings.TrimSpace(*attachmentID)
			if attachmentValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review attachments-delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreReviewAttachment(requestCtx, attachmentValue); err != nil {
				return fmt.Errorf("review attachments-delete: failed to delete: %w", err)
			}

			result := &asc.AppStoreReviewAttachmentDeleteResult{
				ID:      attachmentValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func normalizeReviewAttachmentFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range reviewAttachmentFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(reviewAttachmentFieldList(), ", "))
		}
	}
	return fields, nil
}

func normalizeReviewDetailFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range reviewDetailFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--detail-fields must be one of: %s", strings.Join(reviewDetailFieldList(), ", "))
		}
	}
	return fields, nil
}

func normalizeReviewAttachmentInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, item := range reviewAttachmentIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(reviewAttachmentIncludeList(), ", "))
		}
	}
	return include, nil
}

func reviewAttachmentFieldList() []string {
	return []string{
		"fileSize",
		"fileName",
		"sourceFileChecksum",
		"uploadOperations",
		"assetDeliveryState",
		"appStoreReviewDetail",
	}
}

func reviewAttachmentIncludeList() []string {
	return []string{"appStoreReviewDetail"}
}

func reviewDetailFieldList() []string {
	return []string{
		"contactFirstName",
		"contactLastName",
		"contactPhone",
		"contactEmail",
		"demoAccountName",
		"demoAccountPassword",
		"demoAccountRequired",
		"notes",
		"appStoreVersion",
		"appStoreReviewAttachments",
	}
}
