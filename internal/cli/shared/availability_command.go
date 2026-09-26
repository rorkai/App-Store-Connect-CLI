package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// AvailabilityRemoveFromSaleCommandConfig configures the remove-from-sale command.
type AvailabilityRemoveFromSaleCommandConfig struct {
	ClientFactory func() (*asc.Client, error)
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

			var expectedAvailableInNewTerritories *bool
			if config.IncludeAvailableInNewTerritories && availableInNewTerritories.IsSet() {
				value := availableInNewTerritories.Value()
				expectedAvailableInNewTerritories = &value
			}
			resp, _, err := executeTerritoryAvailabilityUpdate(requestCtx, client, availabilityUpdateRequest{
				AppID:                             resolvedAppID,
				Territories:                       territories,
				AllTerritories:                    *allTerritories,
				Available:                         availableValue,
				ExpectedAvailableInNewTerritories: expectedAvailableInNewTerritories,
				ErrorPrefix:                       config.ErrorPrefix,
			})
			if err != nil {
				return err
			}
			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// NewAvailabilityRemoveFromSaleCommand builds a command that makes every
// existing territory unavailable while preserving the app's new-territory policy.
func NewAvailabilityRemoveFromSaleCommand(config AvailabilityRemoveFromSaleCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet("pricing availability remove-from-sale", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	confirm := fs.Bool("confirm", false, "Confirm removal from sale in all current territories")
	allPlatforms := fs.Bool("all-platforms", false, "Acknowledge removal of every live platform listing (required when more than one platform is live)")
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-from-sale",
		ShortUsage: "asc pricing availability remove-from-sale --app \"APP_ID\" --confirm",
		ShortHelp:  "Remove an app from sale in all current territories.",
		LongHelp: `Remove an app from sale in all current territories.

This command uses the public App Store Connect API. It makes every existing
territory unavailable and verifies the final state. It does not delete the app
record, and it preserves the existing availableInNewTerritories policy because
Apple does not expose an update operation for that setting.

Removal is always app-wide: every platform listing (iOS, macOS, tvOS,
visionOS) shares one availability record, and Apple does not support removing
a single platform from sale through the public API, the internal web API, or
the App Store Connect UI. When more than one platform has a live listing this
command lists them and requires --all-platforms as an extra acknowledgement.
If platform listings cannot be verified, the command fails closed unless
--all-platforms explicitly acknowledges the unknown app-wide blast radius.
To remove only one platform's listing, file an App Store Connect support
request instead. Preview the blast radius first with
"asc pricing availability platforms".

Examples:
  asc pricing availability remove-from-sale --app "123456789" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return UsageError("pricing availability remove-from-sale does not accept positional arguments")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return MissingRequiredUsageError("--confirm")
			}
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return MissingRequiredUsageError("--app")
			}
			if config.ClientFactory == nil {
				return fmt.Errorf("pricing availability remove-from-sale: client factory is not configured")
			}
			client, err := config.ClientFactory()
			if err != nil {
				return fmt.Errorf("pricing availability remove-from-sale: %w", err)
			}

			requestCtx, cancel := contextWithAvailabilityTimeout(ctx, true)
			defer cancel()

			liveListings, listingsErr := fetchLiveAvailabilityPlatformListings(requestCtx, client, resolvedAppID)
			platformListingsVerified := listingsErr == nil
			platformListingsVerificationError := ""
			if listingsErr != nil {
				platformListingsVerificationError = strings.TrimSpace(listingsErr.Error())
				if !*allPlatforms {
					return fmt.Errorf("pricing availability remove-from-sale: could not verify live platform listings; retry or pass --all-platforms to acknowledge the unknown app-wide blast radius: %w", listingsErr)
				}
				fmt.Fprintf(os.Stderr, "Warning: could not verify live platform listings; proceeding because --all-platforms acknowledges the unknown app-wide blast radius: %v\n", listingsErr)
			} else if len(liveListings) > 0 {
				fmt.Fprintf(os.Stderr, "Removal is app-wide; %d live platform listing(s) leave sale together:\n", len(liveListings))
				for _, listing := range liveListings {
					fmt.Fprintf(os.Stderr, "  %s %s (%s)\n", listing.Platform, listing.VersionString, listing.State)
				}
				if len(liveListings) > 1 && !*allPlatforms {
					fmt.Fprintln(os.Stderr, "Error: --all-platforms is required because more than one platform is live. Apple does not support removing a single platform from sale; to remove one platform listing, file an App Store Connect support request.")
					return MissingRequiredUsageError("--all-platforms")
				}
			}

			_, summary, updateErr := executeTerritoryAvailabilityUpdate(requestCtx, client, availabilityUpdateRequest{
				AppID:          resolvedAppID,
				AllTerritories: true,
				Available:      false,
				ErrorPrefix:    "pricing availability remove-from-sale",
			})
			if updateErr != nil && len(summary.FailedTerritories) == 0 {
				return updateErr
			}

			status := "removedFromSale"
			if updateErr != nil {
				status = "partialFailure"
			}
			removedPlatformListings := liveListings
			if updateErr != nil {
				removedPlatformListings = nil
			}
			result := &asc.AvailabilityRemoveFromSaleResult{
				AppID:                             resolvedAppID,
				AvailabilityID:                    summary.AvailabilityID,
				Status:                            status,
				AvailableInNewTerritories:         summary.AvailableInNewTerritories,
				TotalTerritories:                  summary.TotalTerritories,
				UpdatedTerritories:                summary.UpdatedTerritories,
				AlreadyUnavailableTerritories:     summary.SkippedTerritories,
				VerifiedUnavailableTerritories:    summary.VerifiedTerritories,
				FailedTerritories:                 append([]string{}, summary.FailedTerritories...),
				PlatformListingsVerified:          platformListingsVerified,
				PlatformListingsVerificationError: platformListingsVerificationError,
				RemovedPlatformListings:           removedPlatformListings,
			}
			if updateErr != nil {
				fmt.Fprintf(
					os.Stderr,
					"App %s removal is incomplete: %d of %d current territories are verified unavailable; preserved availableInNewTerritories=%t.\n",
					resolvedAppID,
					result.VerifiedUnavailableTerritories,
					result.TotalTerritories,
					result.AvailableInNewTerritories,
				)
			} else {
				fmt.Fprintf(
					os.Stderr,
					"App %s is unavailable in all %d current territories; preserved availableInNewTerritories=%t.\n",
					resolvedAppID,
					result.VerifiedUnavailableTerritories,
					result.AvailableInNewTerritories,
				)
			}
			if result.AvailableInNewTerritories {
				fmt.Fprintln(os.Stderr, "Warning: Apple may automatically enable future App Store territories under the preserved policy.")
			}
			renderErr := PrintOutput(result, *output.Output, *output.Pretty)
			return errors.Join(updateErr, renderErr)
		},
	}
}

type availabilityUpdateRequest struct {
	AppID                             string
	Territories                       []string
	AllTerritories                    bool
	Available                         bool
	ExpectedAvailableInNewTerritories *bool
	ErrorPrefix                       string
}

type availabilityUpdateSummary struct {
	AvailabilityID            string
	AvailableInNewTerritories bool
	TotalTerritories          int
	UpdatedTerritories        int
	SkippedTerritories        int
	VerifiedTerritories       int
	FailedTerritories         []string
}

func executeTerritoryAvailabilityUpdate(ctx context.Context, client *asc.Client, request availabilityUpdateRequest) (*asc.AppAvailabilityV2Response, availabilityUpdateSummary, error) {
	summary := availabilityUpdateSummary{}
	resp, err := client.GetAppAvailabilityV2(ctx, request.AppID)
	if err != nil {
		if isAppAvailabilityMissing(err) {
			return nil, summary, NewErrorWithCause(
				fmt.Errorf(
					"%s: app availability not found for app %q; this command only updates existing app availability, so use \"asc pricing availability create\" first. If Apple rejects public-API bootstrap, authenticate with \"asc web auth login --apple-id EMAIL\" and use \"asc web apps availability create\", or configure Pricing and Availability in App Store Connect: %w",
					request.ErrorPrefix,
					request.AppID,
					asc.ErrNotFound,
				),
				err,
			)
		}
		return nil, summary, fmt.Errorf("%s: %w", request.ErrorPrefix, err)
	}
	availabilityID := strings.TrimSpace(resp.Data.ID)
	if availabilityID == "" {
		return nil, summary, fmt.Errorf("%s: app availability ID missing from response", request.ErrorPrefix)
	}
	summary.AvailabilityID = availabilityID
	summary.AvailableInNewTerritories = resp.Data.Attributes.AvailableInNewTerritories

	if request.ExpectedAvailableInNewTerritories != nil && resp.Data.Attributes.AvailableInNewTerritories != *request.ExpectedAvailableInNewTerritories {
		return nil, summary, fmt.Errorf(
			"%s: --available-in-new-territories does not match the existing policy (current value: %t); the public API cannot change this setting",
			request.ErrorPrefix,
			resp.Data.Attributes.AvailableInNewTerritories,
		)
	}

	territoryResp, err := getAllTerritoryAvailabilities(ctx, client, availabilityID)
	if err != nil {
		return nil, summary, fmt.Errorf("%s: %w", request.ErrorPrefix, err)
	}
	territoryMap, err := mapTerritoryAvailabilities(territoryResp)
	if err != nil {
		return nil, summary, fmt.Errorf("%s: %w", request.ErrorPrefix, err)
	}

	var targets []availabilityEditTarget
	if request.AllTerritories {
		territoryIDs := make([]string, 0, len(territoryMap))
		for territoryID := range territoryMap {
			territoryIDs = append(territoryIDs, territoryID)
		}
		sort.Strings(territoryIDs)
		targets = make([]availabilityEditTarget, 0, len(territoryIDs))
		for _, territoryID := range territoryIDs {
			availability := territoryMap[territoryID]
			targets = append(targets, availabilityEditTarget{TerritoryID: territoryID, AvailabilityID: availability.ID, Available: availability.Attributes.Available})
		}
	} else {
		missingTerritories := make([]string, 0)
		targets = make([]availabilityEditTarget, 0, len(request.Territories))
		for _, territoryID := range request.Territories {
			availability, ok := territoryMap[territoryID]
			if !ok {
				missingTerritories = append(missingTerritories, territoryID)
				continue
			}
			targets = append(targets, availabilityEditTarget{TerritoryID: territoryID, AvailabilityID: availability.ID, Available: availability.Attributes.Available})
		}
		if len(missingTerritories) > 0 {
			return nil, summary, fmt.Errorf("%s: territory availability not found for territories: %s", request.ErrorPrefix, strings.Join(missingTerritories, ", "))
		}
	}

	summary.TotalTerritories = len(targets)
	pending := make([]availabilityEditTarget, 0, len(targets))
	for _, target := range targets {
		if target.Available == request.Available {
			summary.SkippedTerritories++
			continue
		}
		pending = append(pending, target)
	}
	if len(pending) == 0 {
		summary.VerifiedTerritories = summary.SkippedTerritories
		fmt.Fprintf(os.Stderr, "Updated 0 territories; %d already matched.\n", summary.SkippedTerritories)
		return resp, summary, nil
	}

	fmt.Fprintf(os.Stderr, "Updating availability for %d territories (%d already matched)...\n", len(pending), summary.SkippedTerritories)
	patchErrors := updateTerritoryAvailabilityTargets(ctx, client, pending, request.Available)
	verifiedResp, err := getAllTerritoryAvailabilities(ctx, client, availabilityID)
	if err != nil {
		return nil, summary, fmt.Errorf(
			"%s: attempted %d territory updates (%d request failures, %d skipped); final verification failed: %w",
			request.ErrorPrefix,
			len(pending),
			len(patchErrors),
			summary.SkippedTerritories,
			err,
		)
	}
	verifiedMap, err := mapTerritoryAvailabilities(verifiedResp)
	if err != nil {
		return nil, summary, fmt.Errorf("%s: verify territory availabilities: %w", request.ErrorPrefix, err)
	}

	failureDetails := make([]string, 0)
	for _, target := range targets {
		verified, ok := verifiedMap[target.TerritoryID]
		if ok && verified.Attributes.Available == request.Available {
			if target.Available != request.Available {
				summary.UpdatedTerritories++
			}
			continue
		}
		summary.FailedTerritories = append(summary.FailedTerritories, target.TerritoryID)
		if patchErr := patchErrors[target.TerritoryID]; patchErr != nil {
			failureDetails = append(failureDetails, fmt.Sprintf("%s: %v", target.TerritoryID, patchErr))
		} else if !ok {
			failureDetails = append(failureDetails, fmt.Sprintf("%s: missing from verification response", target.TerritoryID))
		} else if target.Available == request.Available {
			failureDetails = append(failureDetails, fmt.Sprintf("%s: state changed during verification", target.TerritoryID))
		} else {
			failureDetails = append(failureDetails, fmt.Sprintf("%s: requested state was not observed", target.TerritoryID))
		}
	}

	summary.VerifiedTerritories = len(targets) - len(summary.FailedTerritories)
	if len(summary.FailedTerritories) > 0 {
		sort.Strings(summary.FailedTerritories)
		sort.Strings(failureDetails)
		return nil, summary, fmt.Errorf(
			"%s: updated %d, skipped %d, failed %d (%s): %s",
			request.ErrorPrefix,
			summary.UpdatedTerritories,
			summary.SkippedTerritories,
			len(summary.FailedTerritories),
			strings.Join(summary.FailedTerritories, ", "),
			strings.Join(failureDetails, "; "),
		)
	}

	fmt.Fprintf(os.Stderr, "Updated %d territories; %d already matched; verified %d updated territories.\n", summary.UpdatedTerritories, summary.SkippedTerritories, len(pending))
	return resp, summary, nil
}

// FetchAvailabilityPlatformListings returns one listing per platform: the
// newest live version when one exists, otherwise the newest version overall.
func FetchAvailabilityPlatformListings(ctx context.Context, client *asc.Client, appID string) ([]asc.AvailabilityPlatformListing, error) {
	newestLive := map[string]asc.AvailabilityPlatformListing{}
	newestAny := map[string]asc.AvailabilityPlatformListing{}
	platformStateKnown := map[string]bool{}
	order := []string{}
	next := ""
	for {
		opts := []asc.AppStoreVersionsOption{asc.WithAppStoreVersionsLimit(200)}
		if next != "" {
			opts = append(opts, asc.WithAppStoreVersionsNextURL(next))
		}
		resp, err := client.GetAppStoreVersions(ctx, appID, opts...)
		if err != nil {
			return nil, err
		}
		for _, version := range resp.Data {
			platform := string(version.Attributes.Platform)
			if platform == "" {
				continue
			}
			state, live, stateKnown := availabilityListingStatus(version.Attributes)
			listing := asc.AvailabilityPlatformListing{
				Platform:      platform,
				VersionString: version.Attributes.VersionString,
				State:         state,
				Live:          live,
				StateKnown:    stateKnown,
				CreatedDate:   version.Attributes.CreatedDate,
			}
			if _, seen := newestAny[platform]; !seen {
				order = append(order, platform)
				platformStateKnown[platform] = true
			}
			if !listing.StateKnown {
				platformStateKnown[platform] = false
			}
			if current, seen := newestAny[platform]; !seen || availabilityListingIsNewer(listing, current) {
				newestAny[platform] = listing
			}
			if listing.Live {
				if current, seen := newestLive[platform]; !seen || availabilityListingIsNewer(listing, current) {
					newestLive[platform] = listing
				}
			}
		}
		next = strings.TrimSpace(resp.Links.Next)
		if next == "" {
			break
		}
	}
	listings := make([]asc.AvailabilityPlatformListing, 0, len(order))
	for _, platform := range order {
		if live, ok := newestLive[platform]; ok {
			live.StateKnown = live.StateKnown && platformStateKnown[platform]
			listings = append(listings, live)
			continue
		}
		listing := newestAny[platform]
		listing.StateKnown = listing.StateKnown && platformStateKnown[platform]
		listings = append(listings, listing)
	}
	return listings, nil
}

func availabilityListingIsNewer(candidate, current asc.AvailabilityPlatformListing) bool {
	candidateTime, candidateValid := parseAvailabilityCreatedDate(candidate.CreatedDate)
	currentTime, currentValid := parseAvailabilityCreatedDate(current.CreatedDate)
	switch {
	case candidateValid && !currentValid:
		return true
	case !candidateValid:
		return false
	default:
		return candidateTime.After(currentTime)
	}
}

func parseAvailabilityCreatedDate(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	return parsed, err == nil
}

// fetchLiveAvailabilityPlatformListings returns only platforms with a live listing.
func fetchLiveAvailabilityPlatformListings(ctx context.Context, client *asc.Client, appID string) ([]asc.AvailabilityPlatformListing, error) {
	listings, err := FetchAvailabilityPlatformListings(ctx, client, appID)
	if err != nil {
		return nil, err
	}
	live := make([]asc.AvailabilityPlatformListing, 0, len(listings))
	unverifiable := make([]string, 0)
	for _, listing := range listings {
		if !listing.StateKnown {
			state := strings.TrimSpace(listing.State)
			if state == "" {
				state = "<missing>"
			}
			unverifiable = append(unverifiable, fmt.Sprintf("%s %s (%s)", listing.Platform, listing.VersionString, state))
			continue
		}
		if listing.Live {
			live = append(live, listing)
		}
	}
	if len(unverifiable) > 0 {
		sort.Strings(unverifiable)
		return nil, fmt.Errorf("platform listing states are unverifiable: %s", strings.Join(unverifiable, "; "))
	}
	return live, nil
}

func availabilityListingState(attributes asc.AppStoreVersionAttributes) string {
	if attributes.AppVersionState != "" {
		return strings.TrimSpace(attributes.AppVersionState)
	}
	return strings.TrimSpace(attributes.AppStoreState)
}

func availabilityListingStatus(attributes asc.AppStoreVersionAttributes) (string, bool, bool) {
	appStoreState := strings.TrimSpace(attributes.AppStoreState)
	appVersionState := strings.TrimSpace(attributes.AppVersionState)
	state := availabilityListingState(attributes)
	if appStoreState == "" && appVersionState == "" {
		return state, false, false
	}

	stateKnown := true
	for _, candidate := range []string{appStoreState, appVersionState} {
		if candidate == "" {
			continue
		}
		if _, ok := appStoreVersionStates[strings.ToUpper(candidate)]; !ok {
			stateKnown = false
		}
	}

	appStoreStateUpper := strings.ToUpper(appStoreState)
	appVersionStateUpper := strings.ToUpper(appVersionState)
	live := appStoreStateUpper == "READY_FOR_SALE" ||
		appStoreStateUpper == "PREORDER_READY_FOR_SALE" ||
		appVersionStateUpper == "READY_FOR_DISTRIBUTION" ||
		appVersionStateUpper == "PREORDER_READY_FOR_SALE"
	return state, live, stateKnown
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

// TerritoryAvailabilityUpdateRequest describes a territory availability change
// applied to an app's existing availability record.
type TerritoryAvailabilityUpdateRequest struct {
	// AppID is the App Store Connect app whose availability is updated.
	AppID string
	// Territories lists the territories to change. Ignored when AllTerritories
	// is set.
	Territories []string
	// AllTerritories applies the change to every territory in the record.
	AllTerritories bool
	// Available is the availability value applied to the selected territories.
	Available bool
	// ExpectedAvailableInNewTerritories, when set, verifies the record's
	// existing new-territory policy. Apple exposes no update operation for it.
	ExpectedAvailableInNewTerritories *bool
	// ErrorPrefix prefixes returned errors, for example "pricing availability create".
	ErrorPrefix string
}

// ApplyTerritoryAvailabilityUpdate applies request to an app's existing
// availability record and returns Apple's response together with the number of
// territories whose availability actually changed (0 when every requested
// territory already matched). It is the same code path
// "asc pricing availability edit" runs, exported so a create-style command can
// route --if-exists update to it instead of duplicating the update logic.
func ApplyTerritoryAvailabilityUpdate(ctx context.Context, client *asc.Client, request TerritoryAvailabilityUpdateRequest) (*asc.AppAvailabilityV2Response, int, error) {
	resp, summary, err := executeTerritoryAvailabilityUpdate(ctx, client, availabilityUpdateRequest(request))
	if err != nil {
		return nil, 0, err
	}
	return resp, summary.UpdatedTerritories, nil
}
