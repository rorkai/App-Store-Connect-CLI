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

// newWebAppsAvailabilityCommand returns the web app availability command group.
func newWebAppsAvailabilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc web apps availability <subcommand> [flags]",
		ShortHelp:  "Bootstrap app availability via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Create the initial app availability record through Apple's internal web API.
This path is intended only for apps that do not yet have an availability record.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAppsAvailabilityCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAppsAvailabilityCreateCommand bootstraps missing app availability via the internal web API.
func newWebAppsAvailabilityCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps availability create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Initial available territories (comma-separated; accepts alpha-2, alpha-3, or exact English country names)")
	var availableInNewTerritories shared.OptionalBool
	fs.Var(&availableInNewTerritories, "available-in-new-territories", "Set availability for new territories: true or false")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web apps availability create --app APP_ID --territory \"US,France\" [flags]",
		ShortHelp:  "Create initial app availability via web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Create the initial app availability record for an app that does not yet have one.
The territories passed with --territory become initially available. All other
current territories are initialized as unavailable.

Examples:
  asc web apps availability create --app "123456789" --territory "United States" --available-in-new-territories false
  asc web apps availability create --app "123456789" --territory "US,France" --available-in-new-territories true

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if strings.TrimSpace(*territory) == "" {
				return d.UsageError("--territory is required")
			}
			if !availableInNewTerritories.IsSet() {
				return d.UsageError("--available-in-new-territories is required (true or false)")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territory)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return d.UsageError("--territory must include at least one value")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			client := newWebClientFn(session)

			var existing *webcore.AppAvailability
			err = withWebSpinnerWith(d, "Checking app availability", func() error {
				var err error
				existing, err = getWebAppAvailabilityFn(requestCtx, client, resolvedAppID)
				if err != nil && webcore.IsAppAvailabilityNotFound(err) {
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
			err = withWebSpinnerWith(d, "Creating app availability via Apple web API", func() error {
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
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebAppAvailabilityCreateTable(d, result) },
				func() error { return renderWebAppAvailabilityCreateMarkdown(d, result) },
			)
		},
	}
}

func renderWebAppAvailabilityCreateTable(d *shared.Deps, result *webAppAvailabilityCreateResult) error {
	asc.RenderTableTo(d.IO.Out,
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

func renderWebAppAvailabilityCreateMarkdown(d *shared.Deps, result *webAppAvailabilityCreateResult) error {
	asc.RenderMarkdownTo(d.IO.Out,
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
