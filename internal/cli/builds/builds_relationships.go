package builds

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

var buildRelationshipKinds = map[string]relationshipKind{
	"app":                    relationshipSingle,
	"appStoreVersion":        relationshipSingle,
	"buildBetaDetail":        relationshipSingle,
	"preReleaseVersion":      relationshipSingle,
	"betaBuildLocalizations": relationshipList,
	"diagnosticSignatures":   relationshipList,
	"individualTesters":      relationshipList,
	"icons":                  relationshipList,
}

// BuildsRelationshipsCommand returns the builds relationships command group.
func BuildsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc builds relationships <subcommand> [flags]",
		ShortHelp:  "View build relationship linkages.",
		LongHelp: `View build relationship linkages.

Examples:
  asc builds relationships get --build "BUILD_ID" --type "app"
  asc builds relationships get --build "BUILD_ID" --type "betaBuildLocalizations" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsRelationshipsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsRelationshipsGetCommand returns the builds relationships get subcommand.
func BuildsRelationshipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	relType := fs.String("type", "", "Relationship type: "+strings.Join(buildRelationshipList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds relationships get --build \"BUILD_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "Get relationship linkages for a build.",
		LongHelp: `Get relationship linkages for a build.

Examples:
  asc builds relationships get --build "BUILD_ID" --type "app"
  asc builds relationships get --build "BUILD_ID" --type "individualTesters" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds relationships get: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds relationships get: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}

			kind, ok := buildRelationshipKinds[relationshipType]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: --type must be one of: %s\n", strings.Join(buildRelationshipList(), ", "))
				return flag.ErrHelp
			}

			buildValue := strings.TrimSpace(*buildID)
			nextValue := strings.TrimSpace(*next)
			if buildValue == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds relationships get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			switch kind {
			case relationshipSingle:
				resp, err := getBuildRelationship(requestCtx, client, relationshipType, buildValue)
				if err != nil {
					return fmt.Errorf("builds relationships get: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			case relationshipList:
				opts := []asc.LinkagesOption{
					asc.WithLinkagesLimit(*limit),
					asc.WithLinkagesNextURL(*next),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithLinkagesLimit(200))
					firstPage, err := getBuildRelationshipList(requestCtx, client, relationshipType, buildValue, paginateOpts...)
					if err != nil {
						return fmt.Errorf("builds relationships get: failed to fetch: %w", err)
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return getBuildRelationshipList(ctx, client, relationshipType, buildValue, asc.WithLinkagesNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("builds relationships get: %w", err)
					}
					return printOutput(resp, *output, *pretty)
				}

				resp, err := getBuildRelationshipList(requestCtx, client, relationshipType, buildValue, opts...)
				if err != nil {
					return fmt.Errorf("builds relationships get: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			default:
				return fmt.Errorf("builds relationships get: unsupported relationship type %q", relationshipType)
			}
		},
	}
}

func getBuildRelationship(ctx context.Context, client *asc.Client, relationshipType, buildID string) (interface{}, error) {
	switch relationshipType {
	case "app":
		return client.GetBuildAppRelationship(ctx, buildID)
	case "appStoreVersion":
		return client.GetBuildAppStoreVersionRelationship(ctx, buildID)
	case "buildBetaDetail":
		return client.GetBuildBuildBetaDetailRelationship(ctx, buildID)
	case "preReleaseVersion":
		return client.GetBuildPreReleaseVersionRelationship(ctx, buildID)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func getBuildRelationshipList(ctx context.Context, client *asc.Client, relationshipType, buildID string, opts ...asc.LinkagesOption) (asc.PaginatedResponse, error) {
	switch relationshipType {
	case "betaBuildLocalizations":
		return client.GetBuildBetaBuildLocalizationsRelationships(ctx, buildID, opts...)
	case "diagnosticSignatures":
		return client.GetBuildDiagnosticSignaturesRelationships(ctx, buildID, opts...)
	case "individualTesters":
		return client.GetBuildIndividualTestersRelationships(ctx, buildID, opts...)
	case "icons":
		return client.GetBuildIconsRelationships(ctx, buildID, opts...)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func buildRelationshipList() []string {
	relationships := make([]string, 0, len(buildRelationshipKinds))
	for key := range buildRelationshipKinds {
		relationships = append(relationships, key)
	}
	sort.Strings(relationships)
	return relationships
}
