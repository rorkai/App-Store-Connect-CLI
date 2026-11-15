package xcodecloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runXcodeCloudPaginatedList(
	ctx context.Context,
	d *shared.Deps,
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

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(d, ctx, 0)
	defer cancel()

	if paginate {
		resp, err := d.PaginateWithSpinner(
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
		return d.PrintOutput(resp, output, pretty)
	}

	resp, err := fetchPage(requestCtx, client, limit, nextURL)
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	return d.PrintOutput(resp, output, pretty)
}

// requireXcodeCloudParent reports a missing parent flag unless --next is
// given. parentHint, when non-empty, is appended to the error so callers learn
// how to discover the parent ID.
func requireXcodeCloudParent(d *shared.Deps, parentID, parentFlag, parentHint, next string) error {
	if strings.TrimSpace(parentID) != "" || strings.TrimSpace(next) != "" {
		return nil
	}
	if hint := strings.TrimSpace(parentHint); hint != "" {
		fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required. %s\n", parentFlag, hint)
	} else {
		fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required\n", parentFlag)
	}
	return shared.MissingRequiredUsageError("--" + parentFlag)
}

// runXcodeCloudPaginatedParentList lists a paginated child collection scoped to
// a parent resource; see requireXcodeCloudParent for parentHint.
func runXcodeCloudPaginatedParentList(
	ctx context.Context,
	d *shared.Deps,
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
	if err := requireXcodeCloudParent(d, parentID, parentFlag, parentHint, next); err != nil {
		return err
	}
	resolvedParentID := strings.TrimSpace(parentID)

	return runXcodeCloudPaginatedList(
		ctx,
		d,
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
