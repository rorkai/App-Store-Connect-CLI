package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const bulkAvailabilityTimeout = 5 * time.Minute

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
	territory := fs.String("territory", "", "Territory IDs (comma-separated, e.g., USA,GBR)")
	allTerritories := fs.Bool("all-territories", false, "Apply to all territories (overrides --territory)")
	var available OptionalBool
	fs.Var(&available, "available", "Set availability: true or false")
	var availableInNewTerritories OptionalBool
	if config.IncludeAvailableInNewTerritories {
		fs.Var(&availableInNewTerritories, "available-in-new-territories", "Set availability for new territories: true or false")
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
				return flag.ErrHelp
			}
			if !*allTerritories && strings.TrimSpace(*territory) == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory or --all-territories is required")
				return flag.ErrHelp
			}
			if !available.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available is required (true or false)")
				return flag.ErrHelp
			}
			if config.IncludeAvailableInNewTerritories && !availableInNewTerritories.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available-in-new-territories is required (true or false)")
				return flag.ErrHelp
			}

			var territories []string
			if !*allTerritories {
				territories = splitCSVUpper(*territory)
				if len(territories) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --territory must include at least one value")
					return flag.ErrHelp
				}
			}

			availableValue := available.Value()

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := contextWithAvailabilityTimeout(ctx, *allTerritories)
			defer cancel()

			resp, err := client.GetAppAvailabilityV2(requestCtx, resolvedAppID)
			if err != nil {
				if isAppAvailabilityMissing(err) {
					return fmt.Errorf(
						"%s: app availability not found for app %q; this command only updates existing app availability, so initialize availability in App Store Connect first or use the experimental \"asc web apps availability create\" flow",
						config.ErrorPrefix,
						resolvedAppID,
					)
				}
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			availabilityID := strings.TrimSpace(resp.Data.ID)
			if availabilityID == "" {
				return fmt.Errorf("%s: app availability ID missing from response", config.ErrorPrefix)
			}

			firstPage, err := client.GetTerritoryAvailabilities(requestCtx, availabilityID, asc.WithTerritoryAvailabilitiesLimit(200))
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesNextURL(nextURL))
			})
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			territoryResp, ok := paginated.(*asc.TerritoryAvailabilitiesResponse)
			if !ok {
				return fmt.Errorf("%s: unexpected territory availabilities response", config.ErrorPrefix)
			}

			if config.IncludeAvailableInNewTerritories {
				availableInNewTerritoriesValue := availableInNewTerritories.Value()
				if resp.Data.Attributes.AvailableInNewTerritories != availableInNewTerritoriesValue {
					return fmt.Errorf(
						"%s: cannot change --available-in-new-territories for an existing app availability (current value: %t)",
						config.ErrorPrefix,
						resp.Data.Attributes.AvailableInNewTerritories,
					)
				}
			}

			territoryMap, err := MapTerritoryAvailabilityIDs(territoryResp)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			var territoryAvailabilityIDs []string
			if *allTerritories {
				territoryAvailabilityIDs = make([]string, 0, len(territoryMap))
				for _, availabilityID := range territoryMap {
					territoryAvailabilityIDs = append(territoryAvailabilityIDs, availabilityID)
				}
				fmt.Fprintf(os.Stderr, "Updating availability for %d territories...\n", len(territoryAvailabilityIDs))
			} else {
				missingTerritories := make([]string, 0)
				territoryAvailabilityIDs = make([]string, 0, len(territories))
				for _, territoryID := range territories {
					territoryAvailabilityID := territoryMap[territoryID]
					if territoryAvailabilityID == "" {
						missingTerritories = append(missingTerritories, territoryID)
						continue
					}
					territoryAvailabilityIDs = append(territoryAvailabilityIDs, territoryAvailabilityID)
				}
				if len(missingTerritories) > 0 {
					return fmt.Errorf("%s: territory availability not found for territories: %s", config.ErrorPrefix, strings.Join(missingTerritories, ", "))
				}
			}

			for _, territoryAvailabilityID := range territoryAvailabilityIDs {
				if _, err := client.UpdateTerritoryAvailability(requestCtx, territoryAvailabilityID, asc.TerritoryAvailabilityUpdateAttributes{
					Available: &availableValue,
				}); err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
			}

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

type territoryAvailabilityIDPayload struct {
	Territory string `json:"t"`
}

// MapTerritoryAvailabilityIDs maps territory IDs to territory-availability IDs.
func MapTerritoryAvailabilityIDs(resp *asc.TerritoryAvailabilitiesResponse) (map[string]string, error) {
	if resp == nil {
		return nil, fmt.Errorf("territory availabilities response is nil")
	}
	ids := make(map[string]string, len(resp.Data))
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
		ids[territoryID] = item.ID
	}
	return ids, nil
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
