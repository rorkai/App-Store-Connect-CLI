package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PrepareBuildUpload creates the build upload and file reservation records used
// by publish/build upload flows before the binary is transferred.
func PrepareBuildUpload(ctx context.Context, client *asc.Client, appID string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform, uti asc.UTI) (*asc.BuildUploadResponse, *asc.BuildUploadFileResponse, error) {
	uploadReq := asc.BuildUploadCreateRequest{
		Data: asc.BuildUploadCreateData{
			Type: asc.ResourceTypeBuildUploads,
			Attributes: asc.BuildUploadAttributes{
				CFBundleShortVersionString: version,
				CFBundleVersion:            buildNumber,
				Platform:                   platform,
			},
			Relationships: &asc.BuildUploadRelationships{
				App: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeApps, ID: appID},
				},
			},
		},
	}

	uploadResp, err := client.CreateBuildUpload(ctx, uploadReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upload record: %w", err)
	}

	fileReq := asc.BuildUploadFileCreateRequest{
		Data: asc.BuildUploadFileCreateData{
			Type: asc.ResourceTypeBuildUploadFiles,
			Attributes: asc.BuildUploadFileAttributes{
				FileName:  fileInfo.Name(),
				FileSize:  fileInfo.Size(),
				UTI:       uti,
				AssetType: asc.AssetTypeAsset,
			},
			Relationships: &asc.BuildUploadFileRelationships{
				BuildUpload: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeBuildUploads, ID: uploadResp.Data.ID},
				},
			},
		},
	}

	fileResp, err := client.CreateBuildUploadFile(ctx, fileReq)
	if err == nil {
		return uploadResp, fileResp, nil
	}

	uploadID := uploadResp.Data.ID
	if uploadID != "" && asc.IsResponseBodyReadError(err) && !errors.Is(err, context.Canceled) {
		// Read the new parent's files without replaying the reservation POST.
		lookupCtx, cancel := ContextWithTimeout(contextWithoutCurrentTimeout(ctx))
		defer cancel()
		files, lookupErr := client.GetBuildUploadFiles(lookupCtx, uploadID)
		if lookupErr == nil && files.Links.Next == "" && len(files.Data) == 1 {
			file := files.Data[0]
			attrs, expected := file.Attributes, fileReq.Data.Attributes
			if file.Type == asc.ResourceTypeBuildUploadFiles && file.ID != "" &&
				attrs.FileName == expected.FileName && attrs.FileSize == expected.FileSize &&
				attrs.UTI == expected.UTI && attrs.AssetType == expected.AssetType &&
				attrs.AssetDeliveryState != nil && attrs.AssetDeliveryState.State != nil && *attrs.AssetDeliveryState.State == "AWAITING_UPLOAD" &&
				len(attrs.UploadOperations) > 0 {
				return uploadResp, &asc.BuildUploadFileResponse{Data: file}, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("failed to create file reservation for build upload %q: %w", uploadID, err)
}

// CommitBuildUploadFile marks a reserved upload file as uploaded and optionally
// persists source-file checksums. When the mutation result is ambiguous, it
// reads the parent build upload once and accepts success only when App Store
// Connect reports that processing has started or completed. A reconciled
// success returns a nil response because no authoritative file payload exists.
func CommitBuildUploadFile(ctx context.Context, client *asc.Client, uploadID, fileID string, checksums *asc.Checksums) (*asc.BuildUploadFileResponse, error) {
	uploaded := true
	req := asc.BuildUploadFileUpdateRequest{
		Data: asc.BuildUploadFileUpdateData{
			Type: asc.ResourceTypeBuildUploadFiles,
			ID:   fileID,
			Attributes: &asc.BuildUploadFileUpdateAttributes{
				Uploaded:            &uploaded,
				SourceFileChecksums: checksums,
			},
		},
	}

	resp, err := client.UpdateBuildUploadFile(ctx, fileID, req)
	if err == nil {
		return resp, nil
	}

	if !isAmbiguousBuildUploadCommitError(err) {
		return nil, buildUploadCommitError(uploadID, err, "")
	}

	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return nil, buildUploadCommitError(uploadID, err, "build upload ID is unavailable for reconciliation")
	}

	// The mutation context may already have reached its deadline. Remove only
	// that request deadline so an outer operation deadline still bounds the
	// reconciliation readback.
	reconcileBase := contextWithoutCurrentTimeout(ctx)
	if parentErr := reconcileBase.Err(); parentErr != nil {
		return nil, buildUploadCommitError(uploadID, err, fmt.Sprintf("reconciliation skipped because the parent operation ended: %v", parentErr))
	}
	reconcileCtx, cancel := ContextWithTimeout(reconcileBase)
	defer cancel()

	uploadResp, lookupErr := client.GetBuildUpload(reconcileCtx, uploadID)
	if lookupErr != nil {
		return nil, buildUploadCommitError(uploadID, err, fmt.Sprintf("reconciliation lookup failed: %v", lookupErr))
	}

	state := buildUploadState(uploadResp)
	switch state {
	case "PROCESSING", "COMPLETE":
		return nil, nil
	case "":
		return nil, buildUploadCommitError(uploadID, err, "reconciliation returned no authoritative upload state")
	default:
		return nil, buildUploadCommitError(uploadID, err, fmt.Sprintf("reconciliation returned upload state %q", state))
	}
}

func isAmbiguousBuildUploadCommitError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if asc.IsBuildUploadFileCommitResponseError(err) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if !asc.IsRetryable(err) {
		return false
	}

	var statusErr interface{ HTTPStatusCode() int }
	if !errors.As(err, &statusErr) {
		return true
	}
	status := statusErr.HTTPStatusCode()
	return status == 0 || status == 408 || status >= 500
}

func buildUploadState(resp *asc.BuildUploadResponse) string {
	if resp == nil || resp.Data.Attributes.State == nil || resp.Data.Attributes.State.State == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(asc.SanitizeTerminalText(*resp.Data.Attributes.State.State)))
}

func buildUploadCommitError(uploadID string, mutationErr error, reconciliation string) error {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		if reconciliation == "" {
			return fmt.Errorf("commit upload file: %w", mutationErr)
		}
		return fmt.Errorf("commit upload file: %w; %s", mutationErr, reconciliation)
	}

	message := fmt.Sprintf("commit upload file for build upload %q", uploadID)
	if reconciliation != "" {
		message += "; " + reconciliation
	}
	return fmt.Errorf("%s; inspect with asc builds uploads view --id %q: %w", message, uploadID, mutationErr)
}
