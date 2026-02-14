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

// VersionsRelationshipsCommand returns the relationships subcommand.
func VersionsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions relationships", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID")
	relType := fs.String("type", "", "Relationship type: "+strings.Join(appStoreVersionRelationshipList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc versions relationships --version-id \"VERSION_ID\" --type \"RELATIONSHIP\" [flags]",
		ShortHelp:  "List relationship linkages for an app store version.",
		LongHelp: `List relationship linkages for an app store version.

Examples:
  asc versions relationships --version-id "VERSION_ID" --type "appStoreReviewDetail"
  asc versions relationships --version-id "VERSION_ID" --type "appStoreVersionExperiments" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("versions relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("versions relationships: %w", err)
			}

			relationshipType := strings.TrimSpace(*relType)
			if relationshipType == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}

			kind, ok := appStoreVersionRelationshipKinds[relationshipType]
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: --type must be one of: %s\n", strings.Join(appStoreVersionRelationshipList(), ", "))
				return flag.ErrHelp
			}

			trimmedID := strings.TrimSpace(*versionID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			if kind == relationshipSingle && (trimmedNext != "" || *paginate || *limit != 0) {
				fmt.Fprintln(os.Stderr, "Error: --limit, --next, and --paginate are only valid for to-many relationships")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			switch kind {
			case relationshipSingle:
				resp, err := getAppStoreVersionRelationship(requestCtx, client, relationshipType, trimmedID)
				if err != nil {
					return fmt.Errorf("versions relationships: %w", err)
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
						return fmt.Errorf("versions relationships: failed to fetch: %w", err)
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return getAppStoreVersionRelationshipList(ctx, client, relationshipType, trimmedID, asc.WithLinkagesNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("versions relationships: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := getAppStoreVersionRelationshipList(requestCtx, client, relationshipType, trimmedID, opts...)
				if err != nil {
					return fmt.Errorf("versions relationships: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("versions relationships: unsupported relationship type %q", relationshipType)
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
