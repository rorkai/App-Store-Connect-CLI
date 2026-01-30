package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var betaGroupRelationshipKinds = map[string]relationshipKind{
	"betaTesters": relationshipList,
	"builds":      relationshipList,
}

// BetaGroupsRelationshipsCommand returns the beta-groups relationships command group.
func BetaGroupsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc testflight beta-groups relationships <subcommand> [flags]",
		ShortHelp:  "View beta group relationship linkages.",
		LongHelp: `View beta group relationship linkages.

Examples:
  asc testflight beta-groups relationships get --group-id "GROUP_ID" --type "betaTesters"
  asc testflight beta-groups relationships get --group-id "GROUP_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsRelationshipsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsRelationshipsGetCommand returns the beta-groups relationships get subcommand.
func BetaGroupsRelationshipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships get", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	relType := fs.String("type", "", "Relationship type: "+strings.Join(relationshipTypeList(betaGroupRelationshipKinds), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-groups relationships get --group-id \"GROUP_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "Get beta group relationship linkages.",
		LongHelp: `Get beta group relationship linkages.

Examples:
  asc testflight beta-groups relationships get --group-id "GROUP_ID" --type "betaTesters"
  asc testflight beta-groups relationships get --group-id "GROUP_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-groups relationships get: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-groups relationships get: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}

			kind, ok := betaGroupRelationshipKinds[relationshipType]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: --type must be one of: %s\n", strings.Join(relationshipTypeList(betaGroupRelationshipKinds), ", "))
				return flag.ErrHelp
			}

			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups relationships get: --group-id and --id must match")
			}

			nextValue := strings.TrimSpace(*next)
			if groupValue == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups relationships get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := getBetaGroupRelationshipList(requestCtx, client, relationshipType, groupValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight beta-groups relationships get: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return getBetaGroupRelationshipList(ctx, client, relationshipType, groupValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight beta-groups relationships get: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := getBetaGroupRelationshipList(requestCtx, client, relationshipType, groupValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-groups relationships get: %w", err)
			}
			return printOutput(resp, *output, *pretty)
		},
	}
}

func getBetaGroupRelationshipList(ctx context.Context, client *asc.Client, relationshipType, groupID string, opts ...asc.LinkagesOption) (asc.PaginatedResponse, error) {
	switch relationshipType {
	case "betaTesters":
		return client.GetBetaGroupBetaTestersRelationships(ctx, groupID, opts...)
	case "builds":
		return client.GetBetaGroupBuildsRelationships(ctx, groupID, opts...)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}
