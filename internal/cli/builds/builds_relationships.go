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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type relationshipKind int

const (
	relationshipSingle relationshipKind = iota
	relationshipList
)

// buildIDNotFoundHint tells an operator which ID --build-id expects when
// App Store Connect does not know the build the flag named.
const buildIDNotFoundHint = `--build-id expects a build ID (list them with: asc builds list --app "APP_ID")`

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

// BuildsRelationshipsCommand returns the builds links command group.
func BuildsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc builds links <subcommand> [flags]",
		ShortHelp:  "View build relationship linkages.",
		LongHelp: `View build relationship linkages.

Examples:
  asc builds links view --build-id "BUILD_ID" --type "app"
  asc builds links view --app "123456789" --latest --type "app"
  asc builds links view --app "123456789" --latest --type "betaBuildLocalizations" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsRelationshipsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsRelationshipsGetCommand returns the builds links view subcommand.
func BuildsRelationshipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("links view", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(fs, buildSelectorFlagOptions{})
	relType := fs.String("type", "", shared.RelationshipTypeFlagUsage(buildRelationshipList()))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds links view (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "View relationship linkages for a build.",
		LongHelp: `View relationship linkages for a build.

Examples:
  asc builds links view --build-id "BUILD_ID" --type "app"
  asc builds links view --app "123456789" --latest --type "app"
  asc builds links view --app "123456789" --latest --type "individualTesters" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("builds links view: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("builds links view: %v", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				return shared.MissingRelationshipTypeUsageError(buildRelationshipList())
			}

			kind, ok := buildRelationshipKinds[relationshipType]
			if !ok {
				shared.PrintInvalidRelationshipTypeError(relationshipType, buildRelationshipList())
				return flag.ErrHelp
			}

			nextValue := strings.TrimSpace(*next)
			if nextValue == "" {
				if err := selectors.validate(); err != nil {
					return err
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds links view: %w", err)
			}

			if kind == relationshipSingle && (nextValue != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds links view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID := ""
			if nextValue == "" {
				buildID, err = selectors.resolveBuildID(requestCtx, client)
				if err != nil {
					return fmt.Errorf("builds links view: %w", err)
				}
			}

			// buildID stays empty when a next-page URL addressed the
			// request, so a 404 then belongs to that URL rather than to a
			// build ID. The --build-id hint applies only when that flag,
			// rather than an --app selector, named the build.
			parent := shared.RelationshipParent{
				ResourceType: "builds",
				Label:        "build",
				ID:           buildID,
			}
			if strings.TrimSpace(selectors.value(selectors.buildID)) != "" {
				parent.Hint = buildIDNotFoundHint
			}
			// Every page after the first is addressed by the previous
			// response's next URL, so a 404 there belongs to that URL.
			pageParent := parent
			pageParent.ID = ""

			switch kind {
			case relationshipSingle:
				resp, err := getBuildRelationship(requestCtx, client, relationshipType, buildID)
				if err != nil {
					return fmt.Errorf("builds links view: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			case relationshipList:
				opts := []asc.LinkagesOption{
					asc.WithLinkagesLimit(*limit),
					asc.WithLinkagesNextURL(*next),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithLinkagesLimit(200))
					resp, err := shared.PaginateWithSpinner(
						requestCtx,
						func(ctx context.Context) (asc.PaginatedResponse, error) {
							page, err := getBuildRelationshipList(ctx, client, relationshipType, buildID, paginateOpts...)
							if err != nil {
								return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, parent)
							}
							return page, nil
						},
						func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
							page, err := getBuildRelationshipList(ctx, client, relationshipType, buildID, asc.WithLinkagesNextURL(nextURL))
							if err != nil {
								return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, pageParent)
							}
							return page, nil
						},
					)
					if err != nil {
						return fmt.Errorf("builds links view: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := getBuildRelationshipList(requestCtx, client, relationshipType, buildID, opts...)
				if err != nil {
					return fmt.Errorf("builds links view: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("builds links view: unsupported relationship type %q", relationshipType)
			}
		},
	}
}

func getBuildRelationship(ctx context.Context, client *asc.Client, relationshipType, buildID string) (any, error) {
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
