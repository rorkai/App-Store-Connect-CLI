package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// betaTesterIDNotFoundHint tells an operator which ID --tester-id expects
// when App Store Connect does not know the beta tester the flag named.
const betaTesterIDNotFoundHint = `--tester-id expects a beta tester ID (list them with: asc testflight testers list --app "APP_ID")`

var betaTesterRelationshipKinds = map[string]relationshipKind{
	"apps":       relationshipList,
	"betaGroups": relationshipList,
	"builds":     relationshipList,
}

// newBetaTestersRelationshipsCommand returns the beta-testers relationships command group built on d.
func newBetaTestersRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc testflight beta-testers relationships <subcommand> [flags]",
		ShortHelp:  "View beta tester relationship linkages.",
		LongHelp: `View beta tester relationship linkages.

Examples:
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "apps"
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "betaGroups" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaTestersRelationshipsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaTestersRelationshipsGetCommand returns the beta-testers relationships get subcommand built on d.
func newBetaTestersRelationshipsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("relationships view", flag.ExitOnError)

	testerID := shared.BindResourceIDFlag(fs, "tester-id", "betaTesters", "Beta tester ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID (alias of --tester-id)")
	relType := fs.String("type", "", shared.RelationshipTypeFlagUsage(relationshipTypeList(betaTesterRelationshipKinds)))
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-testers relationships view --tester-id \"TESTER_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "View beta tester relationship linkages.",
		LongHelp: `View beta tester relationship linkages.

Examples:
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "apps"
  asc testflight beta-testers relationships view --tester-id "TESTER_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-testers relationships view", 200); err != nil {
				return err
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				return d.MissingRelationshipTypeUsageError(relationshipTypeList(betaTesterRelationshipKinds))
			}

			kind, ok := betaTesterRelationshipKinds[relationshipType]
			if !ok {
				d.PrintInvalidRelationshipTypeError(relationshipType, relationshipTypeList(betaTesterRelationshipKinds))
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--type")
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-testers relationships view: --tester-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}

			nextValue := strings.TrimSpace(*next)
			if testerValue == "" && nextValue == "" {
				return d.RequiredFlag("--tester-id")
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-testers relationships view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			// A next-page URL replaces the tester path in the request, so a
			// 404 belongs to that URL rather than to --tester-id.
			parent := shared.RelationshipParent{
				ResourceType: "betaTesters",
				Label:        "beta tester",
				ID:           testerValue,
				Hint:         betaTesterIDNotFoundHint,
			}
			if nextValue != "" {
				parent.ID = ""
			}
			// Every page after the first is addressed by the previous
			// response's next URL, so a 404 there belongs to that URL.
			pageParent := parent
			pageParent.ID = ""

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						page, err := getBetaTesterRelationshipList(ctx, client, relationshipType, testerValue, paginateOpts...)
						if err != nil {
							return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, parent)
						}
						return page, nil
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						page, err := getBetaTesterRelationshipList(ctx, client, relationshipType, testerValue, asc.NextURL[asc.LinkagesOption](nextURL))
						if err != nil {
							return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, pageParent)
						}
						return page, nil
					},
				)
				if err != nil {
					return fmt.Errorf("testflight beta-testers relationships view: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := getBetaTesterRelationshipList(requestCtx, client, relationshipType, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers relationships view: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
			}
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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
