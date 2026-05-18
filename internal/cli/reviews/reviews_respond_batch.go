package reviews

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	reviewBatchStatusCreated = "created"
	reviewBatchStatusFailed  = "failed"
	reviewBatchStatusPlanned = "planned"
	reviewBatchStatusSkipped = "skipped"
)

type reviewBatchInput struct {
	Replies []reviewBatchReplyInput `json:"replies"`
}

type reviewBatchReplyInput struct {
	Response  string   `json:"response"`
	ReviewIDs []string `json:"reviewIds"`
}

type reviewBatchTarget struct {
	ReviewID string
	Response string
}

type reviewBatchResult struct {
	AppID   string                    `json:"appId"`
	DryRun  bool                      `json:"dryRun"`
	Summary reviewBatchSummary        `json:"summary"`
	Results []reviewBatchReviewResult `json:"results"`
}

type reviewBatchSummary struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Planned int `json:"planned"`
}

type reviewBatchReviewResult struct {
	ReviewID           string `json:"reviewId"`
	Status             string `json:"status"`
	ResponseID         string `json:"responseId,omitempty"`
	ExistingResponseID string `json:"existingResponseId,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Error              string `json:"error,omitempty"`
}

type reviewBatchReviewInfo struct {
	ReviewID           string
	ExistingResponseID string
}

// ReviewsRespondBatchCommand returns the reviews respond-batch subcommand.
func ReviewsRespondBatchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("respond-batch", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	filePath := fs.String("file", "", "Path to grouped JSON replies file (required)")
	dryRun := fs.Bool("dry-run", false, "Preview responses without creating them")
	skipExisting := fs.Bool("skip-existing", false, "Skip reviews that already have a published response")
	responseState := fs.String("response-state", reviewResponseStateAny, "Filter by response state: any, unresponded/unreplied, responded/replied")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "respond-batch",
		ShortUsage: "asc reviews respond-batch [flags]",
		ShortHelp:  "Create responses for multiple customer reviews. Experimental.",
		LongHelp: `Create responses for multiple customer reviews from a grouped JSON file.

This command is experimental.

The input file must contain a top-level replies array. Each reply has one
response body and one or more reviewIds.

Example input:
  {
    "replies": [
      {
        "response": "Thanks for the feedback.",
        "reviewIds": ["REVIEW_ID_1", "REVIEW_ID_2"]
      }
    ]
  }

Examples:
  asc reviews respond-batch --app "123456789" --file replies.json --dry-run
  asc reviews respond-batch --app "123456789" --file replies.json --skip-existing --output json
  asc reviews respond-batch --app "123456789" --file replies.json --response-state unresponded`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if strings.TrimSpace(*filePath) == "" {
				return shared.UsageError("--file is required")
			}

			normalizedResponseState, err := normalizeReviewResponseState(*responseState)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			targets, err := loadReviewBatchTargets(*filePath)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews respond-batch: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := executeReviewsRespondBatch(
				requestCtx,
				client,
				resolvedAppID,
				targets,
				*dryRun,
				*skipExisting,
				normalizedResponseState,
			)
			if err != nil {
				return err
			}

			if err := printReviewBatchResult(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.Summary.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("reviews respond-batch: %d review(s) failed", result.Summary.Failed))
			}
			return nil
		},
	}
}

func loadReviewBatchTargets(path string) ([]reviewBatchTarget, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read --file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("--file must not be empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var input reviewBatchInput
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to parse --file: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("failed to parse --file: multiple JSON values are not allowed")
		}
		return nil, fmt.Errorf("failed to parse --file: %w", err)
	}
	if len(input.Replies) == 0 {
		return nil, fmt.Errorf("replies must contain at least one item")
	}

	seen := map[string]struct{}{}
	targets := make([]reviewBatchTarget, 0)
	for replyIndex, reply := range input.Replies {
		response := strings.TrimSpace(reply.Response)
		if response == "" {
			return nil, fmt.Errorf("replies[%d].response is required", replyIndex)
		}
		if len(reply.ReviewIDs) == 0 {
			return nil, fmt.Errorf("replies[%d].reviewIds must contain at least one review id", replyIndex)
		}
		for reviewIndex, reviewID := range reply.ReviewIDs {
			trimmedReviewID := strings.TrimSpace(reviewID)
			if trimmedReviewID == "" {
				return nil, fmt.Errorf("replies[%d].reviewIds[%d] is required", replyIndex, reviewIndex)
			}
			if _, ok := seen[trimmedReviewID]; ok {
				return nil, fmt.Errorf("duplicate review id %q", trimmedReviewID)
			}
			seen[trimmedReviewID] = struct{}{}
			targets = append(targets, reviewBatchTarget{
				ReviewID: trimmedReviewID,
				Response: response,
			})
		}
	}

	return targets, nil
}

func executeReviewsRespondBatch(ctx context.Context, client *asc.Client, appID string, targets []reviewBatchTarget, dryRun bool, skipExisting bool, responseState string) (reviewBatchResult, error) {
	result := reviewBatchResult{
		AppID:  appID,
		DryRun: dryRun,
		Summary: reviewBatchSummary{
			Total: len(targets),
		},
		Results: make([]reviewBatchReviewResult, 0, len(targets)),
	}

	reviews, err := fetchReviewBatchReviewInfo(ctx, client, appID, targets)
	if err != nil {
		return result, fmt.Errorf("reviews respond-batch: failed to fetch reviews: %w", err)
	}

	for _, target := range targets {
		info, ok := reviews[target.ReviewID]
		if !ok {
			result.append(reviewBatchReviewResult{
				ReviewID: target.ReviewID,
				Status:   reviewBatchStatusFailed,
				Error:    "review not found for app",
			})
			continue
		}

		if skip, reason := shouldSkipForResponseState(responseState, info.ExistingResponseID != ""); skip {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusSkipped,
				ExistingResponseID: info.ExistingResponseID,
				Reason:             reason,
			})
			continue
		}

		if skipExisting && info.ExistingResponseID != "" {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusSkipped,
				ExistingResponseID: info.ExistingResponseID,
				Reason:             "existing-response",
			})
			continue
		}

		if dryRun {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusPlanned,
				ExistingResponseID: info.ExistingResponseID,
			})
			continue
		}

		created, err := client.CreateCustomerReviewResponse(ctx, target.ReviewID, target.Response)
		if err != nil {
			result.append(reviewBatchReviewResult{
				ReviewID:           target.ReviewID,
				Status:             reviewBatchStatusFailed,
				ExistingResponseID: info.ExistingResponseID,
				Error:              err.Error(),
			})
			continue
		}

		result.append(reviewBatchReviewResult{
			ReviewID:           target.ReviewID,
			Status:             reviewBatchStatusCreated,
			ResponseID:         created.Data.ID,
			ExistingResponseID: info.ExistingResponseID,
		})
	}

	return result, nil
}

func fetchReviewBatchReviewInfo(ctx context.Context, client *asc.Client, appID string, targets []reviewBatchTarget) (map[string]reviewBatchReviewInfo, error) {
	wanted := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		wanted[target.ReviewID] = struct{}{}
	}

	found := make(map[string]reviewBatchReviewInfo, len(targets))
	nextURL := ""
	for {
		var (
			response *asc.ReviewsResponse
			err      error
		)
		if nextURL != "" {
			response, err = client.GetReviews(ctx, appID, asc.WithNextURL(nextURL))
		} else {
			response, err = client.GetReviews(ctx, appID, asc.WithLimit(200), asc.WithReviewIncludeResponse())
		}
		if err != nil {
			return found, err
		}

		for _, review := range response.Data {
			if _, ok := wanted[review.ID]; !ok {
				continue
			}
			existingResponseID, _ := asc.CustomerReviewPublishedResponseID(review)
			found[review.ID] = reviewBatchReviewInfo{
				ReviewID:           review.ID,
				ExistingResponseID: existingResponseID,
			}
		}

		if len(found) == len(wanted) || strings.TrimSpace(response.Links.Next) == "" {
			break
		}
		nextURL = response.Links.Next
	}

	return found, nil
}

func shouldSkipForResponseState(responseState string, hasResponse bool) (bool, string) {
	switch responseState {
	case reviewResponseStateUnresponded:
		if hasResponse {
			return true, "response-state-mismatch"
		}
	case reviewResponseStateResponded:
		if !hasResponse {
			return true, "response-state-mismatch"
		}
	}
	return false, ""
}

func (r *reviewBatchResult) append(item reviewBatchReviewResult) {
	r.Results = append(r.Results, item)
	switch item.Status {
	case reviewBatchStatusCreated:
		r.Summary.Created++
	case reviewBatchStatusFailed:
		r.Summary.Failed++
	case reviewBatchStatusPlanned:
		r.Summary.Planned++
	case reviewBatchStatusSkipped:
		r.Summary.Skipped++
	}
}

func printReviewBatchResult(result reviewBatchResult, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			renderReviewBatchResult(result, false)
			return nil
		},
		func() error {
			renderReviewBatchResult(result, true)
			return nil
		},
	)
}

func renderReviewBatchResult(result reviewBatchResult, markdown bool) {
	headers := []string{"Review ID", "Status", "Response ID", "Existing Response ID", "Reason", "Error"}
	rows := make([][]string, 0, len(result.Results)+1)
	rows = append(rows, []string{
		"summary",
		fmt.Sprintf("created=%d skipped=%d failed=%d planned=%d", result.Summary.Created, result.Summary.Skipped, result.Summary.Failed, result.Summary.Planned),
		"",
		"",
		fmt.Sprintf("total=%d dryRun=%t", result.Summary.Total, result.DryRun),
		"",
	})
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.ReviewID,
			item.Status,
			item.ResponseID,
			item.ExistingResponseID,
			item.Reason,
			item.Error,
		})
	}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return
	}
	asc.RenderTable(headers, rows)
}
