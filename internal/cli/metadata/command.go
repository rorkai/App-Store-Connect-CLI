package metadata

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MetadataCommand returns the metadata command group.
func MetadataCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metadata",
		ShortUsage: "asc metadata <subcommand> [flags]",
		ShortHelp:  "Manage app metadata with deterministic workflows and keyword tooling.",
		LongHelp: `Manage app metadata with deterministic workflows and keyword tooling.

Phase 1 scope:
  - app-info localizations: name, subtitle, privacyPolicyUrl, privacyChoicesUrl, privacyPolicyText
  - version localizations: description, keywords, marketingUrl, promotionalText, supportUrl, whatsNew

Keyword workflow:
  - ` + "`asc metadata keywords ...`" + ` manages the canonical version-localization ` + "`keywords`" + ` field
  - ` + "`asc apps search-keywords list`" + ` reads app-level ` + "`searchKeywords`" + `
  - ` + "`asc apps search-keywords set`" + ` updates the supported version-localization ` + "`keywords`" + ` attribute
  - version-localization linkage operations remain under ` + "`asc localizations search-keywords ...`" + `

Opt-in scopes (add to --include for pull, plan, and apply):
` + metadataSettingsHelp("  ", true) + `

Not yet included in this group:
  - review information, age ratings, screenshots

Note: copyright is managed via "asc versions create --copyright" or "asc versions update --copyright".

Examples:
  asc metadata init --dir "./metadata" --version "1.2.3" --locale "en-US"
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
  asc metadata plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata approve --review-dir ".asc/metadata/review" --all
  asc metadata apply --app "APP_ID" --version "1.2.3" --dir "./metadata" --review-dir ".asc/metadata/review" --confirm
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MetadataInitCommand(),
			MetadataPullCommand(),
			MetadataPlanCommand(),
			MetadataApproveCommand(),
			MetadataApplyCommand(),
			MetadataStatusCommand(),
			MetadataKeywordsCommand(),
			MetadataPushCommand(),
			MetadataValidateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
