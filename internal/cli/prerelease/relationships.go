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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type relationshipKind int

const (
	relationshipSingle relationshipKind = iota
	relationshipList
)

// preReleaseVersionIDNotFoundHint tells an operator which ID --id expects
// when App Store Connect does not know the pre-release version it named.
const preReleaseVersionIDNotFoundHint = `--id expects a pre-release version ID (list them with: asc testflight pre-release list --app "APP_ID")`

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
  asc pre-release-versions relationships view --id "PR_ID" --type "app"
  asc pre-release-versions relationships view --id "PR_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
	fs := flag.NewFlagSet("relationships view", flag.ExitOnError)

	versionID := fs.String("id", "", "Pre-release version ID")
	relType := fs.String("type", "", shared.RelationshipTypeFlagUsage(preReleaseRelationshipList()))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pre-release-versions relationships view --id \"PR_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "View relationship linkages for a pre-release version.",
		LongHelp: `View relationship linkages for a pre-release version.

Examples:
  asc pre-release-versions relationships view --id "PR_ID" --type "app"
  asc pre-release-versions relationships view --id "PR_ID" --type "builds" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pre-release-versions relationships view: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pre-release-versions relationships view: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				return shared.MissingRelationshipTypeUsageError(preReleaseRelationshipList())
			}

			kind, ok := preReleaseRelationshipKinds[relationshipType]
			if !ok {
				shared.PrintInvalidRelationshipTypeError(relationshipType, preReleaseRelationshipList())
				return flag.ErrHelp
			}

			versionValue := strings.TrimSpace(*versionID)
			nextValue := strings.TrimSpace(*next)
			if versionValue == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-release-versions relationships view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// A next-page URL replaces the version path in the request, so a
			// 404 belongs to that URL rather than to --id.
			parent := shared.RelationshipParent{
				ResourceType: "preReleaseVersions",
				Label:        "pre-release version",
				ID:           versionValue,
				Hint:         preReleaseVersionIDNotFoundHint,
			}
			if nextValue != "" {
				parent.ID = ""
			}
			// Every page after the first is addressed by the previous
			// response's next URL, so a 404 there belongs to that URL.
			pageParent := parent
			pageParent.ID = ""

			switch kind {
			case relationshipSingle:
				resp, err := getPreReleaseRelationship(requestCtx, client, relationshipType, versionValue)
				if err != nil {
					return fmt.Errorf("pre-release-versions relationships view: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			case relationshipList:
				opts := []asc.LinkagesOption{
					asc.WithLinkagesLimit(*limit),
					asc.WithLinkagesNextURL(*next),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithLinkagesLimit(200))
					firstPage, err := getPreReleaseRelationshipList(requestCtx, client, relationshipType, versionValue, paginateOpts...)
					if err != nil {
						return fmt.Errorf("pre-release-versions relationships view: failed to fetch: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						page, err := getPreReleaseRelationshipList(ctx, client, relationshipType, versionValue, asc.WithLinkagesNextURL(nextURL))
						if err != nil {
							return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, pageParent)
						}
						return page, nil
					})
					if err != nil {
						return fmt.Errorf("pre-release-versions relationships view: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := getPreReleaseRelationshipList(requestCtx, client, relationshipType, versionValue, opts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions relationships view: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("pre-release-versions relationships view: unsupported relationship type %q", relationshipType)
			}
		},
	}
}

func getPreReleaseRelationship(ctx context.Context, client *asc.Client, relationshipType, versionID string) (any, error) {
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
