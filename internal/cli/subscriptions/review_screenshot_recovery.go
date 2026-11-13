package subscriptions

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionReviewScreenshotAction string

const (
	subscriptionReviewScreenshotSkip   subscriptionReviewScreenshotAction = "skip"
	subscriptionReviewScreenshotPoll   subscriptionReviewScreenshotAction = "poll"
	subscriptionReviewScreenshotResume subscriptionReviewScreenshotAction = "resume"
)

type subscriptionReviewScreenshotTarget struct {
	FileName string
	FileSize int64
	Checksum string
}

type subscriptionReviewScreenshotConflictError struct {
	message string
}

func (e subscriptionReviewScreenshotConflictError) Error() string {
	return e.message
}

func (e subscriptionReviewScreenshotConflictError) Unwrap() error {
	return asc.ErrConflict
}

func newSubscriptionReviewScreenshotConflictError(format string, args ...any) error {
	return subscriptionReviewScreenshotConflictError{message: fmt.Sprintf(format, args...)}
}

var subscriptionReviewScreenshotFields = []string{
	"fileName",
	"fileSize",
	"sourceFileChecksum",
	"uploadOperations",
	"assetDeliveryState",
}

func createOrResumeSubscriptionReviewScreenshot(ctx context.Context, client *asc.Client, subscriptionID string, file *os.File, info os.FileInfo, checksum string) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
	target := subscriptionReviewScreenshotTarget{
		FileName: strings.TrimSpace(info.Name()),
		FileSize: info.Size(),
		Checksum: strings.TrimSpace(checksum),
	}

	reservation, found, err := readSubscriptionReviewScreenshot(ctx, client, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect existing screenshot: %w", err)
	}
	if !found {
		reservation, _, err = shared.RunReconciledMutation(
			ctx,
			func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
				return client.CreateSubscriptionAppStoreReviewScreenshot(requestCtx, subscriptionID, target.FileName, target.FileSize)
			},
			func(readbackCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, bool, error) {
				candidate, exists, readErr := readSubscriptionReviewScreenshot(readbackCtx, client, subscriptionID)
				if readErr != nil || !exists {
					return candidate, false, readErr
				}
				matches, matchErr := subscriptionReviewScreenshotReservationMatches(candidate, target)
				return candidate, matches, matchErr
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create upload reservation: %w", err)
		}
	}

	action, err := classifySubscriptionReviewScreenshot(reservation, target)
	if err != nil {
		return nil, err
	}
	switch action {
	case subscriptionReviewScreenshotSkip:
		return pollSubscriptionReviewScreenshot(ctx, client, reservation.Data.ID, target.Checksum, target.FileSize)
	case subscriptionReviewScreenshotPoll:
		return pollSubscriptionReviewScreenshot(ctx, client, reservation.Data.ID, target.Checksum, target.FileSize)
	case subscriptionReviewScreenshotResume:
	default:
		return nil, fmt.Errorf("unsupported screenshot recovery action %q", action)
	}

	err = asc.ExecuteUploadOperationsFromFile(ctx, file, reservation.Data.Attributes.UploadOperations)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	uploaded := true
	commitAttrs := asc.SubscriptionAppStoreReviewScreenshotUpdateAttributes{
		SourceFileChecksum: &target.Checksum,
		Uploaded:           &uploaded,
	}
	committed, _, err := shared.RunReconciledMutation(
		ctx,
		func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
			return client.UpdateSubscriptionAppStoreReviewScreenshot(requestCtx, reservation.Data.ID, commitAttrs)
		},
		func(readbackCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, bool, error) {
			candidate, exists, readErr := readSubscriptionReviewScreenshot(readbackCtx, client, subscriptionID)
			if readErr != nil || !exists {
				return candidate, false, readErr
			}
			if strings.TrimSpace(candidate.Data.ID) != strings.TrimSpace(reservation.Data.ID) {
				return candidate, false, newSubscriptionReviewScreenshotConflictError("subscription now references screenshot %q instead of upload reservation %q", candidate.Data.ID, reservation.Data.ID)
			}
			remoteChecksum := strings.TrimSpace(candidate.Data.Attributes.SourceFileChecksum)
			if remoteChecksum == "" {
				return candidate, false, nil
			}
			if !strings.EqualFold(remoteChecksum, target.Checksum) {
				return candidate, false, newSubscriptionReviewScreenshotConflictError("subscription review screenshot checksum changed while committing upload")
			}
			return candidate, true, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}
	if committed != nil && strings.TrimSpace(committed.Data.ID) != "" {
		if strings.TrimSpace(committed.Data.ID) != strings.TrimSpace(reservation.Data.ID) {
			return nil, newSubscriptionReviewScreenshotConflictError("upload commit returned screenshot %q instead of upload reservation %q", committed.Data.ID, reservation.Data.ID)
		}
		reservation = committed
	}

	return pollSubscriptionReviewScreenshot(ctx, client, reservation.Data.ID, target.Checksum, target.FileSize)
}

func readSubscriptionReviewScreenshot(ctx context.Context, client *asc.Client, subscriptionID string) (*asc.SubscriptionAppStoreReviewScreenshotResponse, bool, error) {
	resp, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
		return client.GetSubscriptionAppStoreReviewScreenshotForSubscription(
			requestCtx,
			subscriptionID,
			asc.WithSubscriptionAppStoreReviewScreenshotFields(subscriptionReviewScreenshotFields),
		)
	})
	if err != nil {
		if asc.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if resp == nil {
		return nil, false, fmt.Errorf("empty screenshot response")
	}
	if strings.TrimSpace(resp.Data.ID) == "" {
		return nil, false, nil
	}
	return resp, true, nil
}

func subscriptionReviewScreenshotReservationMatches(resp *asc.SubscriptionAppStoreReviewScreenshotResponse, target subscriptionReviewScreenshotTarget) (bool, error) {
	if resp == nil {
		return false, nil
	}
	attrs := resp.Data.Attributes
	remoteChecksum := strings.TrimSpace(attrs.SourceFileChecksum)
	if remoteChecksum != "" {
		if strings.EqualFold(remoteChecksum, target.Checksum) {
			return true, nil
		}
		return false, newSubscriptionReviewScreenshotConflictError("subscription already has a review screenshot with a different checksum")
	}
	if attrs.FileName == target.FileName && attrs.FileSize == target.FileSize {
		return true, nil
	}
	return false, newSubscriptionReviewScreenshotConflictError("subscription already has a different incomplete review screenshot reservation")
}

func classifySubscriptionReviewScreenshot(resp *asc.SubscriptionAppStoreReviewScreenshotResponse, target subscriptionReviewScreenshotTarget) (subscriptionReviewScreenshotAction, error) {
	if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
		return "", fmt.Errorf("empty screenshot response")
	}
	attrs := resp.Data.Attributes
	state := subscriptionReviewScreenshotDeliveryState(resp)
	if state == "FAILED" {
		return "", shared.WithDiagnostic(newSubscriptionReviewScreenshotConflictError("existing %s", subscriptionReviewScreenshotDeliveryError(resp)), shared.DiagnosticResourceConflict, "")
	}

	remoteChecksum := strings.TrimSpace(attrs.SourceFileChecksum)
	sameChecksum := remoteChecksum != "" && strings.EqualFold(remoteChecksum, target.Checksum)
	if state == "COMPLETE" {
		if sameChecksum {
			return subscriptionReviewScreenshotSkip, nil
		}
		return "", newSubscriptionReviewScreenshotConflictError("subscription already has a complete review screenshot with a different checksum")
	}
	if remoteChecksum != "" {
		if !sameChecksum {
			return "", newSubscriptionReviewScreenshotConflictError("subscription already has a review screenshot with a different checksum")
		}
		return subscriptionReviewScreenshotPoll, nil
	}
	if attrs.FileName != target.FileName || attrs.FileSize != target.FileSize {
		return "", newSubscriptionReviewScreenshotConflictError("subscription already has a different incomplete review screenshot reservation")
	}
	if len(attrs.UploadOperations) == 0 {
		return "", fmt.Errorf("matching incomplete review screenshot has no upload operations")
	}
	return subscriptionReviewScreenshotResume, nil
}

func subscriptionReviewScreenshotDeliveryState(resp *asc.SubscriptionAppStoreReviewScreenshotResponse) string {
	if resp == nil || resp.Data.Attributes.AssetDeliveryState == nil || resp.Data.Attributes.AssetDeliveryState.State == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*resp.Data.Attributes.AssetDeliveryState.State))
}

func subscriptionReviewScreenshotDeliveryError(resp *asc.SubscriptionAppStoreReviewScreenshotResponse) error {
	var details []asc.StateDetail
	if state := resp.Data.Attributes.AssetDeliveryState; state != nil {
		details = state.Errors
	}
	return shared.ReviewScreenshotDeliveryError("asc subscriptions review screenshots", strings.TrimSpace(resp.Data.ID), details)
}

func pollSubscriptionReviewScreenshot(ctx context.Context, client *asc.Client, screenshotID, expectedChecksum string, fileSize int64) (*asc.SubscriptionAppStoreReviewScreenshotResponse, error) {
	verifyCtx, verifyCancel := shared.ContextWithMediaUploadTimeout(ctx, fileSize)
	defer verifyCancel()
	return waitForSubscriptionReviewScreenshotDelivery(verifyCtx, client, screenshotID, expectedChecksum)
}
