package versions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VersionsPromotionsCommand returns the promotions command group.
func VersionsPromotionsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "promotions",
		ShortUsage: "asc versions promotions <subcommand> [flags]",
		ShortHelp:  "Manage App Store version promotions.",
		LongHelp: `Manage App Store version promotions.

Note: The App Store Connect API spec currently lists only create support for
app store version promotions, so this CLI exposes create only.

Examples:
  asc versions promotions create --version-id "VERSION_ID" --treatment-id "TREATMENT_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsPromotionsCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionsPromotionsCreateCommand returns the create subcommand.
func VersionsPromotionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions promotions create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	treatmentID := fs.String("treatment-id", "", "App Store version experiment treatment ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions promotions create [flags]",
		ShortHelp:  "Create an app store version promotion.",
		LongHelp: `Create an app store version promotion.

Examples:
  asc versions promotions create --version-id "VERSION_ID" --treatment-id "TREATMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			treatment := strings.TrimSpace(*treatmentID)
			if treatment == "" {
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions promotions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
