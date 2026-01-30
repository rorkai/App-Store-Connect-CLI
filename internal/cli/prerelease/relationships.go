package prerelease

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type relationshipKind int

const (
	relationshipSingle relationshipKind = iota
	relationshipList
)

var preReleaseRelationshipKinds = map[string]relationshipKind{
	"app":    relationshipSingle,
	"builds": relationshipList,
}

// PreReleaseVersionsRelationshipsCommand returns the relationships command group.
func PreReleaseVersionsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc pre-release-versions relationships <subcommand> [flags]",
		ShortHelp:  "View pre-release version relationship linkages.",
		LongHelp: `View pre-release version relationship linkages.

Examples:
  asc pre-release-versions relationships get --id "PR_ID" --type "app"
  asc pre-release-versions relationships get --id "PR_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PreReleaseVersionsRelationshipsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PreReleaseVersionsRelationshipsGetCommand returns the relationships get subcommand.
func PreReleaseVersionsRelationshipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships get", flag.ExitOnError)

	versionID := fs.String("id", "", "Pre-release version ID")
	relType := fs.String("type", "", "Relationship type: "+strings.Join(preReleaseRelationshipList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pre-release-versions relationships get --id \"PR_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "Get relationship linkages for a pre-release version.",
		LongHelp: `Get relationship linkages for a pre-release version.

Examples:
  asc pre-release-versions relationships get --id "PR_ID" --type "app"
  asc pre-release-versions relationships get --id "PR_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pre-release-versions relationships get: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pre-release-versions relationships get: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}

			kind, ok := preReleaseRelationshipKinds[relationshipType]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: --type must be one of: %s\n", strings.Join(preReleaseRelationshipList(), ", "))
				return flag.ErrHelp
			}

			versionValue := strings.TrimSpace(*versionID)
			nextValue := strings.TrimSpace(*next)
			if versionValue == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pre-release-versions relationships get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			switch kind {
			case relationshipSingle:
				resp, err := getPreReleaseRelationship(requestCtx, client, relationshipType, versionValue)
				if err != nil {
					return fmt.Errorf("pre-release-versions relationships get: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			case relationshipList:
				opts := []asc.LinkagesOption{
					asc.WithLinkagesLimit(*limit),
					asc.WithLinkagesNextURL(*next),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithLinkagesLimit(200))
					firstPage, err := getPreReleaseRelationshipList(requestCtx, client, relationshipType, versionValue, paginateOpts...)
					if err != nil {
						return fmt.Errorf("pre-release-versions relationships get: failed to fetch: %w", err)
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return getPreReleaseRelationshipList(ctx, client, relationshipType, versionValue, asc.WithLinkagesNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("pre-release-versions relationships get: %w", err)
					}
					return printOutput(resp, *output, *pretty)
				}

				resp, err := getPreReleaseRelationshipList(requestCtx, client, relationshipType, versionValue, opts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions relationships get: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			default:
				return fmt.Errorf("pre-release-versions relationships get: unsupported relationship type %q", relationshipType)
			}
		},
	}
}

func getPreReleaseRelationship(ctx context.Context, client *asc.Client, relationshipType, versionID string) (interface{}, error) {
	switch relationshipType {
	case "app":
		return client.GetPreReleaseVersionAppRelationship(ctx, versionID)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func getPreReleaseRelationshipList(ctx context.Context, client *asc.Client, relationshipType, versionID string, opts ...asc.LinkagesOption) (asc.PaginatedResponse, error) {
	switch relationshipType {
	case "builds":
		return client.GetPreReleaseVersionBuildsRelationships(ctx, versionID, opts...)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func preReleaseRelationshipList() []string {
	relationships := make([]string, 0, len(preReleaseRelationshipKinds))
	for key := range preReleaseRelationshipKinds {
		relationships = append(relationships, key)
	}
	sort.Strings(relationships)
	return relationships
}
