package versions

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newVersionsPromotionsCommand returns the promotions command group.
func newVersionsPromotionsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "promotions",
		ShortUsage: "asc versions promotions <subcommand> [flags]",
		ShortHelp:  "Manage App Store version promotions.",
		LongHelp: `Manage App Store version promotions.

Note: The App Store Connect API spec currently lists only create support for
app store version promotions, so this CLI exposes create only.

Examples:
  asc versions promotions create --version-id "VERSION_ID" --treatment-id "TREATMENT_ID"`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newVersionsPromotionsCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newVersionsPromotionsCreateCommand returns the create subcommand.
func newVersionsPromotionsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions promotions create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	treatmentID := shared.BindResourceIDFlag(fs, "treatment-id", "appStoreVersionExperimentTreatments", "App Store version experiment treatment ID (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions promotions create [flags]",
		ShortHelp:  "Create an app store version promotion.",
		LongHelp: `Create an app store version promotion.

Examples:
  asc versions promotions create --version-id "VERSION_ID" --treatment-id "TREATMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return d.RequiredFlag("--version-id")
			}
			treatment := strings.TrimSpace(*treatmentID)
			if treatment == "" {
				return d.RequiredFlag("--treatment-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("versions promotions create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionPromotion(requestCtx, version, treatment)
			if err != nil {
				return fmt.Errorf("versions promotions create: %w", err)
			}

			result := &asc.AppStoreVersionPromotionCreateResult{
				PromotionID: resp.Data.ID,
				VersionID:   version,
				TreatmentID: treatment,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
