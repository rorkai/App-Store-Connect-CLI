package xcodecloud

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runXcodeCloudPaginatedList(
	ctx context.Context,
	limit int,
	next string,
	paginate bool,
	output string,
	pretty bool,
	errorPrefix string,
	fetchPage func(context.Context, *asc.Client, int, string) (asc.PaginatedResponse, error),
	fetchNextPage func(context.Context, *asc.Client, string) (asc.PaginatedResponse, error),
) error {
	nextURL := strings.TrimSpace(next)

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	if paginate {
		resp, err := shared.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return fetchPage(ctx, client, 200, nextURL)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return fetchNextPage(ctx, client, nextURL)
			},
		)
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := fetchPage(requestCtx, client, limit, nextURL)
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	return shared.PrintOutput(resp, output, pretty)
}

// runXcodeCloudPaginatedParentList lists a paginated child collection scoped to
// a parent resource. parentHint, when non-empty, is appended to the missing-flag
// error so callers learn how to discover the parent ID.
func runXcodeCloudPaginatedParentList(
	ctx context.Context,
	parentID string,
	parentFlag string,
	parentHint string,
	limit int,
	next string,
	paginate bool,
	output string,
	pretty bool,
	errorPrefix string,
	fetchPage func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error),
	fetchNextPage func(context.Context, *asc.Client, string, string) (asc.PaginatedResponse, error),
) error {
	resolvedParentID := strings.TrimSpace(parentID)
	if resolvedParentID == "" && strings.TrimSpace(next) == "" {
		if hint := strings.TrimSpace(parentHint); hint != "" {
			fmt.Fprintf(os.Stderr, "Error: --%s is required. %s\n", parentFlag, hint)
		} else {
			fmt.Fprintf(os.Stderr, "Error: --%s is required\n", parentFlag)
		}
		return shared.MissingRequiredUsageError("--" + parentFlag)
	}

	return runXcodeCloudPaginatedList(
		ctx,
		limit,
		next,
		paginate,
		output,
		pretty,
		errorPrefix,
		func(ctx context.Context, client *asc.Client, limit int, next string) (asc.PaginatedResponse, error) {
			return fetchPage(ctx, client, resolvedParentID, limit, next)
		},
		func(ctx context.Context, client *asc.Client, next string) (asc.PaginatedResponse, error) {
			return fetchNextPage(ctx, client, resolvedParentID, next)
		},
	)
}
