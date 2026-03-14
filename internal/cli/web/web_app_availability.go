package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return client.GetAppAvailability(ctx, appID)
	}
	createWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppAvailabilityCreateAttributes) (*webcore.AppAvailability, error) {
		return client.CreateAppAvailability(ctx, attrs)
	}
)

type webAppAvailabilityCreateResult struct {
	AppID                     string   `json:"appId"`
	AvailabilityID            string   `json:"availabilityId"`
	AvailableInNewTerritories bool     `json:"availableInNewTerritories"`
	AvailableTerritories      []string `json:"availableTerritories"`
}

// WebAppsAvailabilityCommand returns the web app availability command group.
func WebAppsAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc web apps availability <subcommand> [flags]",
		ShortHelp:  "[experimental] Bootstrap app availability via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create the initial app availability record through Apple's internal web API.
This path is intended only for apps that do not yet have an availability record.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsAvailabilityCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsAvailabilityCreateCommand bootstraps missing app availability via the internal web API.
func WebAppsAvailabilityCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps availability create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Initial available territory IDs (comma-separated, e.g., USA,GBR)")
	var availableInNewTerritories shared.OptionalBool
	fs.Var(&availableInNewTerritories, "available-in-new-territories", "Set availability for new territories: true or false")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps availability create --app APP_ID --territory \"USA,GBR\" [flags]",
		ShortHelp:  "[experimental] Create initial app availability via web API.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create the initial app availability record for an app that does not yet have one.
The territories passed with --territory become initially available.

Examples:
  asc web apps availability create --app "123456789" --territory "USA" --available-in-new-territories false
  asc web apps availability create --app "123456789" --territory "USA,GBR" --available-in-new-territories true

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web apps availability create does not accept positional arguments")
			}

			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if strings.TrimSpace(*territory) == "" {
				return shared.UsageError("--territory is required")
			}
			if !availableInNewTerritories.IsSet() {
				return shared.UsageError("--available-in-new-territories is required (true or false)")
			}

			territoryIDs := shared.SplitCSVUpper(*territory)
			if len(territoryIDs) == 0 {
				return shared.UsageError("--territory must include at least one value")
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newWebClientFn(session)

			var existing *webcore.AppAvailability
			err = withWebSpinner("Checking app availability", func() error {
				var err error
				existing, err = getWebAppAvailabilityFn(requestCtx, client, resolvedAppID)
				if err != nil && webcore.IsNotFound(err) {
					return nil
				}
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps availability create")
			}
			if existing != nil {
				return fmt.Errorf("web apps availability create failed: app availability already exists for app %q", resolvedAppID)
			}

			createAttrs := webcore.AppAvailabilityCreateAttributes{
				AppID:                     resolvedAppID,
				AvailableInNewTerritories: availableInNewTerritories.Value(),
				AvailableTerritories:      territoryIDs,
			}

			var created *webcore.AppAvailability
			err = withWebSpinner("Creating app availability via Apple web API", func() error {
				var err error
				created, err = createWebAppAvailabilityFn(requestCtx, client, createAttrs)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps availability create")
			}
			if created == nil || strings.TrimSpace(created.ID) == "" {
				return fmt.Errorf("web apps availability create failed: app availability ID missing from response")
			}

			result := &webAppAvailabilityCreateResult{
				AppID:                     resolvedAppID,
				AvailabilityID:            strings.TrimSpace(created.ID),
				AvailableInNewTerritories: created.AvailableInNewTerritories,
				AvailableTerritories:      created.AvailableTerritories,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebAppAvailabilityCreateTable(result) },
				func() error { return renderWebAppAvailabilityCreateMarkdown(result) },
			)
		},
	}
}

func renderWebAppAvailabilityCreateTable(result *webAppAvailabilityCreateResult) error {
	asc.RenderTable(
		[]string{"App ID", "Availability ID", "Available In New Territories", "Available Territories"},
		[][]string{{
			result.AppID,
			result.AvailabilityID,
			fmt.Sprintf("%t", result.AvailableInNewTerritories),
			strings.Join(result.AvailableTerritories, ","),
		}},
	)
	return nil
}

func renderWebAppAvailabilityCreateMarkdown(result *webAppAvailabilityCreateResult) error {
	asc.RenderMarkdown(
		[]string{"App ID", "Availability ID", "Available In New Territories", "Available Territories"},
		[][]string{{
			result.AppID,
			result.AvailabilityID,
			fmt.Sprintf("%t", result.AvailableInNewTerritories),
			strings.Join(result.AvailableTerritories, ","),
		}},
	)
	return nil
}
