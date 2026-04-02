package reviews

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReviewCommand returns the review parent command.
func ReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc review <subcommand> [flags]",
		ShortHelp:  "Manage App Store review details, attachments, and submissions.",
		LongHelp: `Manage App Store review details, attachments, submissions, and items.

Examples:
  asc review status --app "123456789"
  asc review doctor --app "123456789"
  asc review details-get --id "DETAIL_ID"
  asc review details-for-version --version-id "VERSION_ID"
  asc review details-create --version-id "VERSION_ID" --contact-email "dev@example.com"
  asc review details-update --id "DETAIL_ID" --notes "Updated review notes"
  asc review attachments-list --review-detail "DETAIL_ID"
  asc review submissions-list --app "123456789"
  asc review submissions-create --app "123456789" --platform IOS
  asc review submissions-submit --id "SUBMISSION_ID" --confirm
  asc review submissions-update --id "SUBMISSION_ID" --canceled=true
  asc review submissions-items-ids --id "SUBMISSION_ID"
  asc review items-get --id "ITEM_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items-update --id "ITEM_ID" --state READY_FOR_REVIEW
  asc review history --app "123456789"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ReviewStatusCommand(),
			ReviewDoctorCommand(),
			ReviewDetailsGetCommand(),
			ReviewDetailsForVersionCommand(),
			ReviewDetailsCreateCommand(),
			ReviewDetailsUpdateCommand(),
			ReviewDetailsAttachmentsListCommand(),
			ReviewDetailsAttachmentsGetCommand(),
			ReviewDetailsAttachmentsUploadCommand(),
			ReviewDetailsAttachmentsDeleteCommand(),
			ReviewHistoryCommand(),
			ReviewSubmissionsListCommand(),
			ReviewSubmissionsGetCommand(),
			ReviewSubmissionsCreateCommand(),
			ReviewSubmissionsSubmitCommand(),
			ReviewSubmissionsCancelCommand(),
			ReviewSubmissionsUpdateCommand(),
			ReviewSubmissionsItemsIDsCommand(),
			ReviewItemsGetCommand(),
			ReviewItemsListCommand(),
			ReviewItemsAddCommand(),
			ReviewItemsUpdateCommand(),
			ReviewItemsRemoveCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
