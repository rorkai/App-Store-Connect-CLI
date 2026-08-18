package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	bulkAvailabilityTimeout = 5 * time.Minute
	bulkAvailabilityWorkers = 4
)

var availabilityClientFactory = getASCClient

// AvailabilitySetCommandConfig configures the availability set command.
type AvailabilitySetCommandConfig struct {
	FlagSetName                      string
	CommandName                      string
	ShortUsage                       string
	ShortHelp                        string
	LongHelp                         string
	ErrorPrefix                      string
	IncludeAvailableInNewTerritories bool
}

// NewAvailabilitySetCommand builds a shared availability set command.
func NewAvailabilitySetCommand(config AvailabilitySetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := BindOnceCSVFlag(fs, "territory", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names, e.g., US,USA,France)")
	allTerritories := fs.Bool("all-territories", false, "Apply to all territories (overrides --territory)")
	var available OptionalBool
	fs.Var(&available, "available", "Set availability: true or false")
	var availableInNewTerritories OptionalBool
	if config.IncludeAvailableInNewTerritories {
		fs.Var(&availableInNewTerritories, "available-in-new-territories", "Verify the existing new-territory policy (optional; this API cannot change it): true or false")
	}
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       config.CommandName,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return MissingRequiredUsageError("--app")
			}
			if !*allTerritories && strings.TrimSpace(territory.String()) == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory or --all-territories is required")
				return MissingRequiredUsageError("")
			}
			if !available.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available is required (true or false)")
				return MissingRequiredUsageError("--available")
			}
			var territories []string
			if !*allTerritories {
				normalizedTerritories, normalizeErr := normalizeASCTerritoryCSV(territory.String())
				if normalizeErr != nil {
					return UsageError(normalizeErr.Error())
				}
				territories = normalizedTerritories
				if len(territories) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --territory must include at least one value")
					return flag.ErrHelp
				}
			}

			availableValue := available.Value()

			client, err := availabilityClientFactory()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := contextWithAvailabilityTimeout(ctx, *allTerritories)
			defer cancel()

			resp, err := client.GetAppAvailabilityV2(requestCtx, resolvedAppID)
			if err != nil {
				if isAppAvailabilityMissing(err) {
					return NewErrorWithCause(
						fmt.Errorf(
							"%s: app availability not found for app %q; this command only updates existing app availability, so use \"asc pricing availability create\" first. If Apple rejects public-API bootstrap, authenticate with \"asc web auth login --apple-id EMAIL\" and use \"asc web apps availability create\", or configure Pricing and Availability in App Store Connect: %w",
							config.ErrorPrefix,
							resolvedAppID,
							asc.ErrNotFound,
						),
						err,
					)
				}
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			availabilityID := strings.TrimSpace(resp.Data.ID)
			if availabilityID == "" {
				return fmt.Errorf("%s: app availability ID missing from response", config.ErrorPrefix)
			}

			territoryResp, err := getAllTerritoryAvailabilities(requestCtx, client, availabilityID)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			if config.IncludeAvailableInNewTerritories && availableInNewTerritories.IsSet() {
				availableInNewTerritoriesValue := availableInNewTerritories.Value()
				if resp.Data.Attributes.AvailableInNewTerritories != availableInNewTerritoriesValue {
					return fmt.Errorf(
						"%s: --available-in-new-territories does not match the existing policy (current value: %t); the public API cannot change this setting",
						config.ErrorPrefix,
						resp.Data.Attributes.AvailableInNewTerritories,
					)
				}
			}

			territoryMap, err := mapTerritoryAvailabilities(territoryResp)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			var targets []availabilityEditTarget
			if *allTerritories {
				territoryIDs := make([]string, 0, len(territoryMap))
				for territoryID := range territoryMap {
					territoryIDs = append(territoryIDs, territoryID)
				}
				sort.Strings(territoryIDs)
				targets = make([]availabilityEditTarget, 0, len(territoryIDs))
				for _, territoryID := range territoryIDs {
					availability := territoryMap[territoryID]
					targets = append(targets, availabilityEditTarget{
						TerritoryID:    territoryID,
						AvailabilityID: availability.ID,
						Available:      availability.Attributes.Available,
					})
				}
			} else {
				missingTerritories := make([]string, 0)
				targets = make([]availabilityEditTarget, 0, len(territories))
				for _, territoryID := range territories {
					availability, ok := territoryMap[territoryID]
					if !ok {
						missingTerritories = append(missingTerritories, territoryID)
						continue
					}
					targets = append(targets, availabilityEditTarget{
						TerritoryID:    territoryID,
						AvailabilityID: availability.ID,
						Available:      availability.Attributes.Available,
					})
				}
				if len(missingTerritories) > 0 {
					return fmt.Errorf("%s: territory availability not found for territories: %s", config.ErrorPrefix, strings.Join(missingTerritories, ", "))
				}
			}

			pending := make([]availabilityEditTarget, 0, len(targets))
			skipped := 0
			for _, target := range targets {
				if target.Available == availableValue {
					skipped++
					continue
				}
				pending = append(pending, target)
			}

			if len(pending) == 0 {
				fmt.Fprintf(os.Stderr, "Updated 0 territories; %d already matched.\n", skipped)
				return printOutput(resp, *output.Output, *output.Pretty)
			}

			fmt.Fprintf(os.Stderr, "Updating availability for %d territories (%d already matched)...\n", len(pending), skipped)
			patchErrors := updateTerritoryAvailabilityTargets(requestCtx, client, pending, availableValue)

			verifiedResp, err := getAllTerritoryAvailabilities(requestCtx, client, availabilityID)
			if err != nil {
				return fmt.Errorf(
					"%s: attempted %d territory updates (%d request failures, %d skipped); final verification failed: %w",
					config.ErrorPrefix,
					len(pending),
					len(patchErrors),
					skipped,
					err,
				)
			}
			verifiedMap, err := mapTerritoryAvailabilities(verifiedResp)
			if err != nil {
				return fmt.Errorf("%s: verify territory availabilities: %w", config.ErrorPrefix, err)
			}

			failedTerritories := make([]string, 0)
			failureDetails := make([]string, 0)
			for _, target := range pending {
				verified, ok := verifiedMap[target.TerritoryID]
				if ok && verified.Attributes.Available == availableValue {
					continue
				}
				failedTerritories = append(failedTerritories, target.TerritoryID)
				if patchErr := patchErrors[target.TerritoryID]; patchErr != nil {
					failureDetails = append(failureDetails, fmt.Sprintf("%s: %v", target.TerritoryID, patchErr))
				} else if !ok {
					failureDetails = append(failureDetails, fmt.Sprintf("%s: missing from verification response", target.TerritoryID))
				} else {
					failureDetails = append(failureDetails, fmt.Sprintf("%s: requested state was not observed", target.TerritoryID))
				}
			}

			updated := len(pending) - len(failedTerritories)
			if len(failedTerritories) > 0 {
				sort.Strings(failedTerritories)
				sort.Strings(failureDetails)
				return fmt.Errorf(
					"%s: updated %d, skipped %d, failed %d (%s): %s",
					config.ErrorPrefix,
					updated,
					skipped,
					len(failedTerritories),
					strings.Join(failedTerritories, ", "),
					strings.Join(failureDetails, "; "),
				)
			}

			fmt.Fprintf(os.Stderr, "Updated %d territories; %d already matched; verified %d requested territories.\n", updated, skipped, len(targets))
			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func contextWithAvailabilityTimeout(ctx context.Context, allTerritories bool) (context.Context, context.CancelFunc) {
	if allTerritories {
		return ContextWithResolvedTimeout(ctx, bulkAvailabilityTimeout)
	}
	return contextWithTimeout(ctx)
}

type availabilityEditTarget struct {
	TerritoryID    string
	AvailabilityID string
	Available      bool
}

type territoryAvailabilityUpdateResult struct {
	TerritoryID string
	Err         error
}

func updateTerritoryAvailabilityTargets(ctx context.Context, client *asc.Client, targets []availabilityEditTarget, available bool) map[string]error {
	workerCount := min(bulkAvailabilityWorkers, len(targets))
	jobs := make(chan availabilityEditTarget)
	results := make(chan territoryAvailabilityUpdateResult, len(targets))

	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for target := range jobs {
				_, err := client.UpdateTerritoryAvailability(ctx, target.AvailabilityID, asc.TerritoryAvailabilityUpdateAttributes{
					Available: &available,
				})
				results <- territoryAvailabilityUpdateResult{TerritoryID: target.TerritoryID, Err: err}
			}
		}()
	}

	go func() {
		for _, target := range targets {
			jobs <- target
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	errs := make(map[string]error)
	for result := range results {
		if result.Err != nil {
			errs[result.TerritoryID] = result.Err
		}
	}
	return errs
}

func getAllTerritoryAvailabilities(ctx context.Context, client *asc.Client, availabilityID string) (*asc.TerritoryAvailabilitiesResponse, error) {
	firstPage, err := client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesLimit(200))
	if err != nil {
		return nil, err
	}
	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := paginated.(*asc.TerritoryAvailabilitiesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected territory availabilities response")
	}
	return resp, nil
}

type territoryAvailabilityIDPayload struct {
	Territory string `json:"t"`
}

// MapTerritoryAvailabilityIDs maps territory IDs to territory-availability IDs.
func MapTerritoryAvailabilityIDs(resp *asc.TerritoryAvailabilitiesResponse) (map[string]string, error) {
	availabilities, err := mapTerritoryAvailabilities(resp)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(availabilities))
	for territoryID, availability := range availabilities {
		ids[territoryID] = availability.ID
	}
	return ids, nil
}

func mapTerritoryAvailabilities(resp *asc.TerritoryAvailabilitiesResponse) (map[string]asc.Resource[asc.TerritoryAvailabilityAttributes], error) {
	if resp == nil {
		return nil, fmt.Errorf("territory availabilities response is nil")
	}
	availabilities := make(map[string]asc.Resource[asc.TerritoryAvailabilityAttributes], len(resp.Data))
	for _, item := range resp.Data {
		territoryID := ""
		if len(item.Relationships) > 0 {
			var relationships asc.TerritoryAvailabilityRelationships
			if err := json.Unmarshal(item.Relationships, &relationships); err != nil {
				return nil, fmt.Errorf("decode territory availability relationships for %q: %w", item.ID, err)
			}
			territoryID = strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID))
		}
		if territoryID == "" {
			var ok bool
			territoryID, ok = territoryIDFromAvailabilityID(item.ID)
			if !ok {
				return nil, fmt.Errorf("territory availability %q missing territory id", item.ID)
			}
		}
		availabilities[territoryID] = item
	}
	return availabilities, nil
}

func territoryIDFromAvailabilityID(availabilityID string) (string, bool) {
	trimmed := strings.TrimSpace(availabilityID)
	if trimmed == "" {
		return "", false
	}
	decoded, err := base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			decoded, err = base64.RawURLEncoding.DecodeString(trimmed)
			if err != nil {
				decoded, err = base64.URLEncoding.DecodeString(trimmed)
				if err != nil {
					return "", false
				}
			}
		}
	}
	var payload territoryAvailabilityIDPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return "", false
	}
	territoryID := strings.TrimSpace(payload.Territory)
	if territoryID == "" {
		return "", false
	}
	return strings.ToUpper(territoryID), true
}
