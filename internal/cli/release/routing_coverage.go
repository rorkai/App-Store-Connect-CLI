package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	routingcoveragecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/routingcoverage"
)

const routingCoveragePollInterval = 2 * time.Second

type routingCoverageStepDetails struct {
	Action        string `json:"action"`
	CoverageID    string `json:"coverageId,omitempty"`
	FileName      string `json:"fileName"`
	Checksum      string `json:"checksum"`
	DeliveryState string `json:"deliveryState,omitempty"`
}

func applyPreparedRoutingCoverageStep(ctx context.Context, client *asc.Client, versionID string, prepared routingcoveragecli.PreparedRoutingCoverageFile, dryRun bool) (stepOutcome, error) {
	if strings.TrimSpace(versionID) == "" {
		if dryRun {
			return stepOutcome{
				Status:  "dry-run",
				Message: "routing coverage plan deferred until version exists",
				Details: routingCoverageStepDetails{
					Action:   "deferred",
					FileName: prepared.FileName,
					Checksum: prepared.Checksum,
				},
			}, nil
		}
		return stepOutcome{}, fmt.Errorf("resolved version ID is empty")
	}

	existing, err := client.GetRoutingAppCoverageForVersion(ctx, versionID)
	if err != nil && !asc.IsNotFound(err) {
		return stepOutcome{}, fmt.Errorf("fetch current routing coverage: %w", err)
	}
	if asc.IsNotFound(err) {
		existing = nil
	}
	if existing != nil && strings.TrimSpace(existing.Data.ID) == "" {
		existing = nil
	}

	details := routingCoverageStepDetails{
		Action:   "create",
		FileName: prepared.FileName,
		Checksum: prepared.Checksum,
	}
	if existing != nil {
		details.CoverageID = strings.TrimSpace(existing.Data.ID)
		details.DeliveryState = routingCoverageDeliveryState(existing)
		sameChecksum := strings.EqualFold(
			strings.TrimSpace(existing.Data.Attributes.SourceFileChecksum),
			strings.TrimSpace(prepared.Checksum),
		)
		if sameChecksum && details.DeliveryState == "" {
			refreshed, refreshErr := client.GetRoutingAppCoverage(ctx, existing.Data.ID)
			if refreshErr != nil {
				return stepOutcome{Details: details}, fmt.Errorf("fetch routing coverage %s delivery state: %w", existing.Data.ID, refreshErr)
			}
			details.DeliveryState = routingCoverageDeliveryState(refreshed)
			if details.DeliveryState == "" {
				return stepOutcome{Details: details}, fmt.Errorf("routing coverage %s response is missing a delivery state", existing.Data.ID)
			}
		}
		if sameChecksum && details.DeliveryState == "COMPLETE" {
			if err := routingcoveragecli.RevalidatePreparedRoutingCoverageFile(prepared); err != nil {
				return stepOutcome{Details: details}, err
			}
			details.Action = "reuse"
			status := "skipped"
			message := "routing coverage already in sync"
			if dryRun {
				status = "dry-run"
				message += " (no action needed)"
			}
			return stepOutcome{Status: status, Message: message, Details: details, Persist: !dryRun}, nil
		}
		if sameChecksum && details.DeliveryState != "AWAITING_UPLOAD" && details.DeliveryState != "FAILED" {
			if err := routingcoveragecli.RevalidatePreparedRoutingCoverageFile(prepared); err != nil {
				return stepOutcome{Details: details}, err
			}
			details.Action = "wait"
			if dryRun {
				return stepOutcome{
					Status:  "dry-run",
					Message: "would wait for matching routing coverage to finish processing",
					Details: details,
				}, nil
			}
			state, waitErr := waitForRoutingCoverageDelivery(ctx, client, existing.Data.ID)
			details.DeliveryState = state
			if waitErr != nil {
				return stepOutcome{Details: details}, waitErr
			}
			if err := routingcoveragecli.RevalidatePreparedRoutingCoverageFile(prepared); err != nil {
				return stepOutcome{Details: details}, err
			}
			details.Action = "reuse"
			return stepOutcome{
				Status:  "skipped",
				Message: "routing coverage finished processing",
				Details: details,
				Persist: true,
			}, nil
		}
		details.Action = "replace"
	}

	if dryRun {
		message := "would upload routing coverage"
		if details.Action == "replace" {
			message = "would replace routing coverage"
		}
		return stepOutcome{Status: "dry-run", Message: message, Details: details}, nil
	}

	var committed *asc.RoutingAppCoverageResponse
	if existing != nil {
		committed, err = routingcoveragecli.ReplaceRoutingCoverageWithPreparedFile(ctx, client, versionID, existing.Data.ID, prepared)
	} else {
		committed, err = routingcoveragecli.UploadPreparedRoutingCoverageFile(ctx, client, versionID, prepared)
	}
	if err != nil {
		if committed != nil && strings.TrimSpace(committed.Data.ID) != "" {
			details.CoverageID = strings.TrimSpace(committed.Data.ID)
			details.DeliveryState = routingCoverageDeliveryState(committed)
		}
		return stepOutcome{Details: details}, err
	}
	details.CoverageID = committed.Data.ID
	details.DeliveryState = routingCoverageDeliveryState(committed)
	state, err := waitForRoutingCoverageDelivery(ctx, client, committed.Data.ID)
	details.DeliveryState = state
	if err != nil {
		return stepOutcome{Details: details}, err
	}

	message := "uploaded routing coverage"
	if details.Action == "replace" {
		message = "replaced routing coverage"
	}
	return stepOutcome{
		Status:  "ok",
		Message: message,
		Details: details,
		Persist: true,
	}, nil
}

func waitForRoutingCoverageDelivery(ctx context.Context, client *asc.Client, coverageID string) (string, error) {
	lastState := ""
	_, err := asc.PollUntil(ctx, routingCoveragePollInterval, func(ctx context.Context) (struct{}, bool, error) {
		response, err := client.GetRoutingAppCoverage(ctx, coverageID)
		if err != nil {
			return struct{}{}, false, err
		}
		lastState = routingCoverageDeliveryState(response)
		switch lastState {
		case "COMPLETE":
			return struct{}{}, true, nil
		case "FAILED":
			return struct{}{}, false, fmt.Errorf("routing coverage %s delivery failed%s", coverageID, routingCoverageDeliveryErrors(response))
		default:
			return struct{}{}, false, nil
		}
	})
	if err != nil {
		return lastState, err
	}
	return lastState, nil
}

func routingCoverageDeliveryState(response *asc.RoutingAppCoverageResponse) string {
	if response == nil || response.Data.Attributes.AssetDeliveryState == nil || response.Data.Attributes.AssetDeliveryState.State == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*response.Data.Attributes.AssetDeliveryState.State))
}

func routingCoverageDeliveryErrors(response *asc.RoutingAppCoverageResponse) string {
	if response == nil || response.Data.Attributes.AssetDeliveryState == nil {
		return ""
	}
	parts := make([]string, 0, len(response.Data.Attributes.AssetDeliveryState.Errors))
	for _, item := range response.Data.Attributes.AssetDeliveryState.Errors {
		description := strings.TrimSpace(item.Description)
		if description == "" {
			description = strings.TrimSpace(item.Message)
		}
		code := strings.TrimSpace(item.Code)
		switch {
		case code != "" && description != "":
			parts = append(parts, code+": "+description)
		case description != "":
			parts = append(parts, description)
		case code != "":
			parts = append(parts, code)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ": " + strings.Join(parts, "; ")
}
