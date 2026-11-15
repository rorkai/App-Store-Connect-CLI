package reviews

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newReviewDetailsAttachmentsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("attachments-list", flag.ExitOnError)

	reviewDetailID := shared.BindResourceIDFlag(fs, "review-detail", "appStoreReviewDetails", "App Store review detail ID (required unless --next is provided)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(reviewAttachmentFieldList(), ", "))
	detailFields := fs.String("detail-fields", "", "Review detail fields to include: "+strings.Join(reviewDetailFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewAttachmentIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attachments-list",
		ShortUsage: "asc review attachments-list (--review-detail REVIEW_DETAIL_ID | --next URL) [flags]",
		ShortHelp:  "List review attachments for a review detail.",
		LongHelp: `List review attachments for a review detail.

Examples:
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID"
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID" --fields "fileName,fileSize" --limit 50
  asc review attachments-list --review-detail "REVIEW_DETAIL_ID" --paginate
  asc review attachments-list --next "<links.next>"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "review attachments-list", 200, "fields", "detail-fields", "include", "limit"); err != nil {
				return err
			}
			nextValue := strings.TrimSpace(*next)

			reviewDetailValue := strings.TrimSpace(*reviewDetailID)
			if reviewDetailValue == "" && nextValue == "" {
				return d.RequiredFlag("--review-detail")
			}

			fieldsValue, err := normalizeReviewAttachmentFields(*fields)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-list: %v", err), shared.DiagnosticInvalidInput, "--fields")
			}
			detailFieldsValue, err := normalizeReviewDetailFields(*detailFields)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-list: %v", err), shared.DiagnosticInvalidInput, "--detail-fields")
			}
			includeValue, err := normalizeReviewAttachmentInclude(*include)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-list: %v", err), shared.DiagnosticInvalidInput, "--include")
			}

			if nextValue != "" {
				reviewDetailValue = ""
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppStoreReviewAttachmentsOption{
				asc.WithAppStoreReviewAttachmentsFields(fieldsValue),
				asc.WithAppStoreReviewAttachmentReviewDetailFields(detailFieldsValue),
				asc.WithAppStoreReviewAttachmentsInclude(includeValue),
				asc.Limit[asc.AppStoreReviewAttachmentsOption](*limit),
				asc.NextURL[asc.AppStoreReviewAttachmentsOption](*next),
			}

			if *paginate {
				paginateOpts := opts
				if nextValue == "" {
					paginateOpts = append(paginateOpts, asc.Limit[asc.AppStoreReviewAttachmentsOption](200))
				}
				pages, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						resp, err := client.GetAppStoreReviewAttachmentsForReviewDetail(ctx, reviewDetailValue, paginateOpts...)
						if err != nil || *includeSensitive {
							return resp, err
						}
						return asc.RedactAppStoreReviewDetailIncludesInListResponse(resp)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						resp, err := client.GetAppStoreReviewAttachmentsForReviewDetail(ctx, "", asc.NextURL[asc.AppStoreReviewAttachmentsOption](nextURL))
						if err != nil || *includeSensitive {
							return resp, err
						}
						return asc.RedactAppStoreReviewDetailIncludesInListResponse(resp)
					},
				)
				if err != nil {
					return fmt.Errorf("review attachments-list: %w", err)
				}

				shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
				return d.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreReviewAttachmentsForReviewDetail(requestCtx, reviewDetailValue, opts...)
			if err != nil {
				return fmt.Errorf("review attachments-list: failed to fetch: %w", err)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			if *includeSensitive {
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}
			safe, err := asc.RedactAppStoreReviewDetailIncludesInListResponse(resp)
			if err != nil {
				return fmt.Errorf("review attachments-list: %w", err)
			}
			return d.PrintOutput(safe, *output.Output, *output.Pretty)
		},
	}
}

func newReviewDetailsAttachmentsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("attachments-get", flag.ExitOnError)

	attachmentID := shared.BindResourceIDFlag(fs, "id", "appStoreReviewAttachments", "Review attachment ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(reviewAttachmentFieldList(), ", "))
	detailFields := fs.String("detail-fields", "", "Review detail fields to include: "+strings.Join(reviewDetailFieldList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewAttachmentIncludeList(), ", "))
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}

			fieldsValue, err := normalizeReviewAttachmentFields(*fields)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-get: %v", err), shared.DiagnosticInvalidInput, "--fields")
			}
			detailFieldsValue, err := normalizeReviewDetailFields(*detailFields)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-get: %v", err), shared.DiagnosticInvalidInput, "--detail-fields")
			}
			includeValue, err := normalizeReviewAttachmentInclude(*include)
			if err != nil {
				return shared.WithDiagnostic(d.UsageErrorf("review attachments-get: %v", err), shared.DiagnosticInvalidInput, "--include")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreReviewAttachment(
				requestCtx, attachmentValue,
				asc.WithAppStoreReviewAttachmentsFields(fieldsValue),
				asc.WithAppStoreReviewAttachmentReviewDetailFields(detailFieldsValue),
				asc.WithAppStoreReviewAttachmentsInclude(includeValue),
			)
			if err != nil {
				return fmt.Errorf("review attachments-get: failed to fetch: %w", err)
			}

			shared.WarnIncludeSensitive(d.IO.ErrOut, *includeSensitive)
			if *includeSensitive {
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}
			safe, err := asc.RedactAppStoreReviewDetailIncludesInSingleResponse(resp)
			if err != nil {
				return fmt.Errorf("review attachments-get: %w", err)
			}
			return d.PrintOutput(safe, *output.Output, *output.Pretty)
		},
	}
}

func newReviewDetailsAttachmentsUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("attachments-upload", flag.ExitOnError)

	reviewDetailID := shared.BindResourceIDFlag(fs, "review-detail", "appStoreReviewDetails", "App Store review detail ID (required)")
	filePath := fs.String("file", "", "Path to attachment file (required)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--review-detail")
			}

			pathValue := strings.TrimSpace(*filePath)
			if pathValue == "" {
				return d.RequiredFlag("--file")
			}

			info, err := os.Lstat(pathValue)
			if err != nil {
				code := shared.DiagnosticFileInvalidFormat
				switch {
				case errors.Is(err, os.ErrNotExist):
					code = shared.DiagnosticFileNotFound
				case errors.Is(err, os.ErrPermission):
					code = shared.DiagnosticFilePermissionDenied
				}
				return shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("review attachments-upload: %w", err)), code, "--file")
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("review attachments-upload: refusing to read symlink %q", pathValue)), shared.DiagnosticFileInvalidFormat, "--file")
			}
			if info.IsDir() {
				return shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("review attachments-upload: %q is a directory", pathValue)), shared.DiagnosticFileInvalidFormat, "--file")
			}
			if info.Size() <= 0 {
				return shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("review attachments-upload: file size must be greater than 0")), shared.DiagnosticFileInvalidFormat, "--file")
			}
			source, err := os.Open(pathValue)
			if err != nil {
				code := shared.DiagnosticFileInvalidFormat
				switch {
				case errors.Is(err, os.ErrNotExist):
					code = shared.DiagnosticFileNotFound
				case errors.Is(err, os.ErrPermission):
					code = shared.DiagnosticFilePermissionDenied
				}
				return shared.WithDiagnostic(shared.NewValidationError(fmt.Errorf("review attachments-upload: upload failed: %w", err)), code, "--file")
			}
			_ = source.Close()

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("review attachments-upload: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreReviewAttachment(requestCtx, reviewDetailValue, filepath.Base(pathValue), info.Size())
			if err != nil {
				return fmt.Errorf("review attachments-upload: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("review attachments-upload: no upload operations returned")
			}

			uploadCtx, uploadCancel := d.UploadTimeout(ctx)
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

			commitCtx, commitCancel := d.UploadTimeout(ctx)
			commitResp, err := client.UpdateAppStoreReviewAttachment(commitCtx, resp.Data.ID, updateAttrs)
			commitCancel()
			if err != nil {
				return fmt.Errorf("review attachments-upload: failed to commit upload: %w", err)
			}

			return d.PrintOutput(commitResp, *output.Output, *output.Pretty)
		},
	}
}

func newReviewDetailsAttachmentsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "attachments-delete",
		ShortUsage: "asc review attachments-delete --id \"ATTACHMENT_ID\" --confirm",
		ShortHelp:  "Delete a review attachment.",
		LongHelp: `Delete a review attachment.

Examples:
  asc review attachments-delete --id "ATTACHMENT_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Review attachment ID (required)",
		IDType:             "appStoreReviewAttachments",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreReviewAttachment(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
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
