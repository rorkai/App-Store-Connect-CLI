package shared

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// CategoriesSetCommandConfig configures the categories set command.
type CategoriesSetCommandConfig struct {
	FlagSetName    string
	ShortUsage     string
	ShortHelp      string
	LongHelp       string
	ErrorPrefix    string
	IncludeAppInfo bool
	// Deps supplies the streams, the request timeout and the output. It
	// defaults to DefaultDeps(). The client is not taken from it: the command
	// resolves credentials without the auth spinner GetASCClient shows.
	Deps *Deps
}

// NewCategoriesSetCommand builds a categories set command with shared behavior.
func NewCategoriesSetCommand(config CategoriesSetCommandConfig) *ffcli.Command {
	d := builderDeps(config.Deps)
	fs := d.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App ID (required)")
	var appInfoID *string
	if config.IncludeAppInfo {
		appInfoID = BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID (optional override)")
	}
	primary := BindResourceIDFlag(fs, "primary", "appCategories", "Primary category ID (required)")
	secondary := BindResourceIDFlag(fs, "secondary", "appCategories", "Secondary category ID (optional)")
	primarySubOne := BindResourceIDFlag(fs, "primary-subcategory-one", "appCategories", "Primary subcategory one (e.g. GAMES_ACTION)")
	primarySubTwo := BindResourceIDFlag(fs, "primary-subcategory-two", "appCategories", "Primary subcategory two (e.g. GAMES_SIMULATION)")
	secondarySubOne := BindResourceIDFlag(fs, "secondary-subcategory-one", "appCategories", "Secondary subcategory one")
	secondarySubTwo := BindResourceIDFlag(fs, "secondary-subcategory-two", "appCategories", "Secondary subcategory two")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appIDValue, err := d.AppIDFlagValue(ResolveAppID(*appID))
			if err != nil {
				return err
			}
			primaryValue := strings.TrimSpace(*primary)
			secondaryValue := strings.TrimSpace(*secondary)
			primarySubOneValue := strings.TrimSpace(*primarySubOne)
			primarySubTwoValue := strings.TrimSpace(*primarySubTwo)
			secondarySubOneValue := strings.TrimSpace(*secondarySubOne)
			secondarySubTwoValue := strings.TrimSpace(*secondarySubTwo)

			appInfoIDValue := ""
			if appInfoID != nil {
				appInfoIDValue = strings.TrimSpace(*appInfoID)
			}

			if appIDValue == "" {
				return d.UsageErrorf("%s: --app is required (or set ASC_APP_ID)", config.ErrorPrefix)
			}
			if primaryValue == "" {
				return d.UsageErrorf("%s: --primary is required", config.ErrorPrefix)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedAppInfoID, err := d.ResolveAppInfoID(requestCtx, client, appIDValue, appInfoIDValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			resp, err := client.UpdateAppInfoCategories(requestCtx, resolvedAppInfoID, primaryValue, secondaryValue, primarySubOneValue, primarySubTwoValue, secondarySubOneValue, secondarySubTwoValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
