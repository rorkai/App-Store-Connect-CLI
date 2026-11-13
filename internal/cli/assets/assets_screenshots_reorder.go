package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AssetsScreenshotsReorderCommand returns the screenshots reorder subcommand.
func AssetsScreenshotsReorderCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reorder", flag.ExitOnError)

	setID := shared.BindResourceIDFlag(fs, "set-id", "appScreenshotSets", "App screenshot set ID")
	ids := fs.String("ids", "", "Every screenshot ID in the set, comma-separated, in the desired order")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "reorder",
		ShortUsage: "asc screenshots reorder --set-id \"SET_ID\" --ids \"ID1,ID2,ID3\"",
		ShortHelp:  "Reorder the screenshots in a screenshot set.",
		LongHelp: `Reorder the screenshots in a screenshot set.

--ids must list every screenshot currently in the set exactly once, in the
desired order. The command reads the set first and refuses to send the update
when an ID is missing from --ids or does not belong to the set. It does not
add, remove, or delete screenshots. If a screenshot is added to the set after
that read, App Store Connect rejects the update and the set keeps its order;
list the set again and re-run with its current IDs.

Examples:
  asc screenshots list --set-id "SET_ID"
  asc screenshots reorder --set-id "SET_ID" --ids "THIRD_ID,FIRST_ID,SECOND_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedSetID := strings.TrimSpace(*setID)
			if trimmedSetID == "" {
				return shared.UsageError("screenshots reorder: --set-id is required")
			}
			if strings.TrimSpace(*ids) == "" {
				return shared.UsageError("screenshots reorder: --ids is required")
			}
			orderedIDs, err := parseScreenshotReorderIDs(*ids)
			if err != nil {
				return shared.UsageErrorf("screenshots reorder: --ids %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots reorder: %w", err)
			}

			readCtx, cancelRead := shared.ContextWithTimeout(ctx)
			currentIDs, err := GetOrderedAppScreenshotIDs(readCtx, client, trimmedSetID)
			cancelRead()
			if err != nil {
				return fmt.Errorf("screenshots reorder: failed to read set order: %w", err)
			}
			if err := screenshotReorderMembershipError(trimmedSetID, currentIDs, orderedIDs); err != nil {
				return fmt.Errorf("screenshots reorder: %w", err)
			}
			writeCtx, cancelWrite := shared.ContextWithTimeout(ctx)
			defer cancelWrite()
			if err := SetOrderedAppScreenshots(writeCtx, client, trimmedSetID, orderedIDs); err != nil {
				if isScreenshotSetMembershipConflict(err) {
					return fmt.Errorf("screenshots reorder: screenshot set %s changed after it was read, so App Store Connect rejected the new order and left the set as it was; list it with: asc screenshots list --set-id %s, then re-run with its current screenshot IDs: %w", trimmedSetID, trimmedSetID, err)
				}
				return fmt.Errorf("screenshots reorder: %w", err)
			}

			return shared.PrintOutput(&asc.AppScreenshotReorderResult{
				SetID:         trimmedSetID,
				Reordered:     true,
				ScreenshotIDs: orderedIDs,
			}, *output.Output, *output.Pretty)
		},
	}
}

func parseScreenshotReorderIDs(raw string) ([]string, error) {
	values := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("contains an empty ID")
		}
		if _, ok := seen[value]; ok {
			return nil, fmt.Errorf("contains duplicate ID %q", value)
		}
		seen[value] = struct{}{}
		values[i] = value
	}
	return values, nil
}

func screenshotReorderMembershipError(setID string, currentIDs, orderedIDs []string) error {
	current := make(map[string]struct{}, len(currentIDs))
	for _, id := range currentIDs {
		current[id] = struct{}{}
	}
	requested := make(map[string]struct{}, len(orderedIDs))
	var unknown []string
	for _, id := range orderedIDs {
		requested[id] = struct{}{}
		if _, ok := current[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	var missing []string
	for _, id := range currentIDs {
		if _, ok := requested[id]; !ok {
			missing = append(missing, id)
		}
	}
	var problems []string
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("not in set %s: %s", setID, strings.Join(unknown, ",")))
	}
	if len(missing) > 0 {
		problems = append(problems, "missing from --ids: "+strings.Join(missing, ","))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("--ids must list every screenshot in the set exactly once; %s (inspect with: asc screenshots list --set-id %s)", strings.Join(problems, "; "), setID)
}

// isScreenshotSetMembershipConflict reports Apple's 409 for a full-replace
// relationship update whose IDs no longer match the set's screenshots.
func isScreenshotSetMembershipConflict(err error) bool {
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		return false
	}
	for _, detail := range apiErr.AllDetails {
		if strings.Contains(detail, "Add/Remove Relationship") {
			return true
		}
	}
	return false
}
