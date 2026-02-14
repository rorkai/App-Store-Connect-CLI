package shared

import (
	"context"
	"flag"
	"fmt"
	"os"
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
}

// NewCategoriesSetCommand builds a categories set command with shared behavior.
func NewCategoriesSetCommand(config CategoriesSetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", os.Getenv("ASC_APP_ID"), "App ID (required)")
	var appInfoID *string
	if config.IncludeAppInfo {
		appInfoID = fs.String("app-info", "", "App Info ID (optional override)")
	}
	primary := fs.String("primary", "", "Primary category ID (required)")
	secondary := fs.String("secondary", "", "Secondary category ID (optional)")
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appIDValue := strings.TrimSpace(*appID)
			primaryValue := strings.TrimSpace(*primary)
			secondaryValue := strings.TrimSpace(*secondary)

			appInfoIDValue := ""
			if appInfoID != nil {
				appInfoIDValue = strings.TrimSpace(*appInfoID)
			}

			if appIDValue == "" {
				return fmt.Errorf("%s: --app is required", config.ErrorPrefix)
			}
			if primaryValue == "" {
				return fmt.Errorf("%s: --primary is required", config.ErrorPrefix)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resolvedAppInfoID, err := ResolveAppInfoID(requestCtx, client, appIDValue, appInfoIDValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			resp, err := client.UpdateAppInfoCategories(requestCtx, resolvedAppInfoID, primaryValue, secondaryValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
