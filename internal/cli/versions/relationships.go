package versions

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

var appStoreVersionRelationshipKinds = map[string]relationshipKind{
	"ageRatingDeclaration":           relationshipSingle,
	"appStoreReviewDetail":           relationshipSingle,
	"appClipDefaultExperience":       relationshipSingle,
	"appStoreVersionExperiments":     relationshipList,
	"appStoreVersionExperimentsV2":   relationshipList,
	"appStoreVersionSubmission":      relationshipSingle,
	"customerReviews":                relationshipList,
	"routingAppCoverage":             relationshipSingle,
	"alternativeDistributionPackage": relationshipSingle,
	"gameCenterAppVersion":           relationshipSingle,
}

const appStoreVersionIDNotFoundHint = `--version-id expects an App Store version ID, not an app ID (list them with: asc versions list --app "APP_ID")`

func paginationConflictParameter(limit int, next string, paginate bool) string {
	parameters := make([]string, 0, 3)
	if limit != 0 {
		parameters = append(parameters, "--limit")
	}
	if strings.TrimSpace(next) != "" {
		parameters = append(parameters, "--next")
	}
	if paginate {
		parameters = append(parameters, "--paginate")
	}
	if len(parameters) == 1 {
		return parameters[0]
	}
	return ""
}

// VersionsRelationshipsCommand returns the links subcommand.
func VersionsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions links", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (not an app ID; list IDs with \"asc versions list --app APP_ID\")")
	relType := fs.String("type", "", shared.RelationshipTypeFlagUsage(appStoreVersionRelationshipList()))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc versions links --version-id \"VERSION_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "List relationship linkages for an app store version.",
		LongHelp: `List relationship linkages for an app store version.

Examples:
  asc versions links --version-id "VERSION_ID" --type "appStoreReviewDetail"
  asc versions links --version-id "VERSION_ID" --type "appStoreVersionExperiments" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.UsageError("versions links: --limit must be between 1 and 200"),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.WithDiagnostic(
					shared.UsageErrorf("versions links: %v", err),
					shared.DiagnosticInvalidInput,
					"--next",
				)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				return shared.MissingRelationshipTypeUsageError(appStoreVersionRelationshipList())
			}

			kind, ok := appStoreVersionRelationshipKinds[relationshipType]
			if !ok {
				shared.PrintInvalidRelationshipTypeError(relationshipType, appStoreVersionRelationshipList())
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--type")
			}

			trimmedID := strings.TrimSpace(*versionID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}

			if kind == relationshipSingle && (trimmedNext != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return shared.WithDiagnostic(
					flag.ErrHelp,
					shared.DiagnosticConflictingInput,
					paginationConflictParameter(*limit, trimmedNext, *paginate),
				)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions links: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// A next-page URL replaces the version path in the request, so a
			// 404 belongs to that URL rather than to --version-id.
			parent := shared.RelationshipParent{
				ResourceType: "appStoreVersions",
				Label:        "app store version",
				ID:           trimmedID,
				Hint:         appStoreVersionIDNotFoundHint,
			}
			if trimmedNext != "" {
				parent.ID = ""
			}
			// Every page after the first is addressed by the previous
			// response's next URL, so a 404 there belongs to that URL.
			pageParent := parent
			pageParent.ID = ""

			switch kind {
			case relationshipSingle:
				resp, err := getAppStoreVersionRelationship(requestCtx, client, relationshipType, trimmedID)
				if err != nil {
					return fmt.Errorf("versions links: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			case relationshipList:
				opts := []asc.LinkagesOption{
					asc.WithLinkagesLimit(*limit),
					asc.WithLinkagesNextURL(*next),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithLinkagesLimit(200))
					firstPage, err := getAppStoreVersionRelationshipList(requestCtx, client, relationshipType, trimmedID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("versions links: failed to fetch: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						page, err := getAppStoreVersionRelationshipList(ctx, client, relationshipType, trimmedID, asc.WithLinkagesNextURL(nextURL))
						if err != nil {
							return nil, shared.DescribeRelationshipLookupFailure(err, relationshipType, pageParent)
						}
						return page, nil
					})
					if err != nil {
						return fmt.Errorf("versions links: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := getAppStoreVersionRelationshipList(requestCtx, client, relationshipType, trimmedID, opts...)
				if err != nil {
					return fmt.Errorf("versions links: %w", shared.DescribeRelationshipLookupFailure(err, relationshipType, parent))
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("versions links: unsupported relationship type %q", relationshipType)
			}
		},
	}
}

func getAppStoreVersionRelationship(ctx context.Context, client *asc.Client, relationshipType, versionID string) (any, error) {
	switch relationshipType {
	case "ageRatingDeclaration":
		return client.GetAppStoreVersionAgeRatingDeclarationRelationship(ctx, versionID)
	case "appStoreReviewDetail":
		return client.GetAppStoreVersionReviewDetailRelationship(ctx, versionID)
	case "appClipDefaultExperience":
		return client.GetAppStoreVersionAppClipDefaultExperienceRelationship(ctx, versionID)
	case "appStoreVersionSubmission":
		return client.GetAppStoreVersionSubmissionRelationship(ctx, versionID)
	case "routingAppCoverage":
		return client.GetAppStoreVersionRoutingAppCoverageRelationship(ctx, versionID)
	case "alternativeDistributionPackage":
		return client.GetAppStoreVersionAlternativeDistributionPackageRelationship(ctx, versionID)
	case "gameCenterAppVersion":
		return client.GetAppStoreVersionGameCenterAppVersionRelationship(ctx, versionID)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func getAppStoreVersionRelationshipList(ctx context.Context, client *asc.Client, relationshipType, versionID string, opts ...asc.LinkagesOption) (asc.PaginatedResponse, error) {
	switch relationshipType {
	case "appStoreVersionExperiments":
		return client.GetAppStoreVersionExperimentsRelationships(ctx, versionID, opts...)
	case "appStoreVersionExperimentsV2":
		return client.GetAppStoreVersionExperimentsV2Relationships(ctx, versionID, opts...)
	case "customerReviews":
		return client.GetAppStoreVersionCustomerReviewsRelationships(ctx, versionID, opts...)
	default:
		return nil, fmt.Errorf("unsupported relationship type %q", relationshipType)
	}
}

func appStoreVersionRelationshipList() []string {
	relationships := make([]string, 0, len(appStoreVersionRelationshipKinds))
	for key := range appStoreVersionRelationshipKinds {
		relationships = append(relationships, key)
	}
	sort.Strings(relationships)
	return relationships
}
