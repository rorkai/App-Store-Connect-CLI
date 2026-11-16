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

var setWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool) (*webcore.MedicalDeviceDeclarationResult, error) {
	return client.SetMedicalDeviceDeclaration(ctx, accountID, appID, declared)
}

var setWebMedicalDeviceDeclarationWithOptionsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool, options webcore.MedicalDeviceDeclarationOptions) (*webcore.MedicalDeviceDeclarationResult, error) {
	return client.SetMedicalDeviceDeclarationWithOptions(ctx, accountID, appID, declared, options)
}

// newWebAppsMedicalDeviceCommand returns the regulated medical device command group.
func newWebAppsMedicalDeviceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps medical-device", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "medical-device",
		ShortUsage: "asc web apps medical-device <subcommand> [flags]",
		ShortHelp:  "Manage the regulated medical device declaration via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage the regulated medical device declaration exposed in App Store Connect
under App Information -> App Store Regulations & Permits.

Use ` + "`view`" + ` to read the stored declaration and ` + "`set`" + ` to answer it.
Use ` + "`region set`" + ` to update one detailed regional answer after the
app-level declaration is already "yes".

Writing supports the app-level "Yes" or "No" answer and the captured region
selection for the medical-device form. The regional command accepts only the
captured registration and localized support fields; contact information is
read from the existing form, preserved, and never supplied by the CLI.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAppsMedicalDeviceViewCommand(d),
			newWebAppsMedicalDeviceSetCommand(d),
			newWebAppsMedicalDeviceRegionCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAppsMedicalDeviceSetCommand sets the regulated medical device declaration.
func newWebAppsMedicalDeviceSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps medical-device set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	var declared shared.OptionalBool
	fs.Var(&declared, "declared", "Set regulated medical device declaration: true or false")
	countriesOrRegions := fs.String("countries-or-regions", "", "Comma-separated medical-device regions: EEA,GBR,USA (default: all)")
	confirm := fs.Bool("confirm", false, "Confirm saving an affirmative regulated medical-device declaration")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps medical-device set --app APP_ID --declared true|false [--confirm] [flags]",
		ShortHelp:  "Set regulated medical device declaration via web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Set the regulated medical device declaration through Apple web-session compliance-form web endpoint used by App Store Connect.

The app-level "Yes" path uses Apple's captured web form contract and accepts
the EEA, GBR, and USA region selection. Region-specific registration,
support-information, and contact-information fields are preserved when
present, but are not entered by this command.

The stored declaration is read first; when it already matches, no write is sent
and the receipt reports ` + "`changed: false`" + `.

Examples:
  asc web apps medical-device set --app "6748252780" --declared false
  asc web apps medical-device set --app "6748252780" --declared true --countries-or-regions "EEA,GBR" --confirm

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if !declared.IsSet() {
				return d.UsageError("--declared is required (supported values: true, false)")
			}

			regionsProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "countries-or-regions" {
					regionsProvided = true
				}
			})
			if regionsProvided && strings.TrimSpace(*countriesOrRegions) == "" {
				return d.UsageError("--countries-or-regions must not be empty")
			}

			var selectedRegions []string
			if regionsProvided {
				if !declared.Value() {
					return d.UsageError("--countries-or-regions requires --declared true")
				}
				values := strings.Split(*countriesOrRegions, ",")
				var err error
				selectedRegions, err = webcore.NormalizeMedicalDeviceDeclarationRegions(values)
				if err != nil {
					return d.UsageError(err.Error())
				}
			}
			if declared.Value() {
				if err := d.RequireConfirm(*confirm); err != nil {
					return err
				}
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, d, authFlags, "web apps medical-device set")
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.MedicalDeviceDeclarationResult
			err = withWebSpinnerWith(d, "Saving regulated medical device declaration", func() error {
				var err error
				if declared.Value() || len(selectedRegions) > 0 {
					result, err = setWebMedicalDeviceDeclarationWithOptionsFn(requestCtx, client, accountID, resolvedAppID, declared.Value(), webcore.MedicalDeviceDeclarationOptions{CountriesOrRegions: selectedRegions})
				} else {
					result, err = setWebMedicalDeviceDeclarationFn(requestCtx, client, accountID, resolvedAppID, false)
				}
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps medical-device set")
			}
			if result == nil {
				return fmt.Errorf("web apps medical-device set failed: missing declaration result")
			}

			return d.PrintOutput(webMedicalDeviceDeclarationResultOutput(result), *output.Output, *output.Pretty)
		},
	}
}

func webMedicalDeviceDeclarationResultOutput(result *webcore.MedicalDeviceDeclarationResult) *asc.WebMedicalDeviceDeclarationResult {
	if result == nil {
		return nil
	}
	return &asc.WebMedicalDeviceDeclarationResult{
		AppID:              result.AppID,
		RequirementID:      result.RequirementID,
		RequirementName:    result.RequirementName,
		Status:             result.Status,
		FormID:             result.FormID,
		Declared:           result.Declared,
		Changed:            result.Changed,
		CountriesOrRegions: result.CountriesOrRegions,
	}
}
