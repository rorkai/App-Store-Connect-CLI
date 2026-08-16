package testflight

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

var betaTesterRelationshipKinds = map[string]relationshipKind{
	"apps":       relationshipList,
	"betaGroups": relationshipList,
	"builds":     relationshipList,
}

// BetaTestersRelationshipsCommand returns the beta-testers relationships command group.
func BetaTestersRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc testflight beta-testers relationships <subcommand> [flags]",
		ShortHelp:  "View beta tester relationship linkages.",
		LongHelp: `View beta tester relationship linkages.

Examples:
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "apps"
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "betaGroups" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersRelationshipsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaTestersRelationshipsGetCommand returns the beta-testers relationships get subcommand.
func BetaTestersRelationshipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships view", flag.ExitOnError)

	testerID := fs.String("tester-id", "", "Beta tester ID")
	aliasID := fs.String("id", "", "Beta tester ID (alias of --tester-id)")
	relType := fs.String("type", "", "Relationship type: "+strings.Join(relationshipTypeList(betaTesterRelationshipKinds), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-testers relationships view --tester-id \"TESTER_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "View beta tester relationship linkages.",
		LongHelp: `View beta tester relationship linkages.

Examples:
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "apps"
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.NewValidationError(fmt.Errorf("testflight beta-testers relationships view: --limit must be between 1 and 200")),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-testers relationships view: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return shared.MissingRequiredUsageError("--type")
			}

			kind, ok := betaTesterRelationshipKinds[relationshipType]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: --type must be one of: %s\n", strings.Join(relationshipTypeList(betaTesterRelationshipKinds), ", "))
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--type")
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					shared.NewValidationError(fmt.Errorf("testflight beta-testers relationships view: --tester-id and --id must match")),
					shared.DiagnosticConflictingInput,
					"",
				)
			}

			nextValue := strings.TrimSpace(*next)
			if testerValue == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return shared.MissingRequiredUsageError("--tester-id")
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers relationships view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return getBetaTesterRelationshipList(ctx, client, relationshipType, testerValue, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return getBetaTesterRelationshipList(ctx, client, relationshipType, testerValue, asc.WithLinkagesNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("testflight beta-testers relationships view: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := getBetaTesterRelationshipList(requestCtx, client, relationshipType, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers relationships view: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func getBetaTesterRelationshipList(ctx context.Context, client *asc.Client, relationshipType, testerID string, opts ...asc.LinkagesOption) (asc.PaginatedResponse, error) {
	switch relationshipType {
	case "apps":
		return client.GetBetaTesterAppsRelationships(ctx, testerID, opts...)
	case "betaGroups":
		return client.GetBetaTesterBetaGroupsRelationships(ctx, testerID, opts...)
	case "builds":
		return client.GetBetaTesterBuildsRelationships(ctx, testerID, opts...)
	default:
		return nil, shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("unsupported relationship type %q", relationshipType)),
			shared.DiagnosticInvalidInput,
			"--type",
		)
	}
}
