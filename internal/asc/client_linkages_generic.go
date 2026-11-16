package asc

import (
	"context"
	"fmt"
	"strings"
)

func getTypedResourceLinkages[T any](
	c *Client,
	ctx context.Context,
	resourceID string,
	relationship string,
	resourceIDName string,
	pathFmt string,
	nextURLErrorContext string,
	opts ...LinkagesOption,
) (*T, error) {
	query := applyOptions(opts)

	resourceID = strings.TrimSpace(resourceID)
	if query.nextURL == "" && resourceID == "" {
		return nil, fmt.Errorf("%s is required", resourceIDName)
	}

	return getJSON[T](ctx, c, readRequest{
		Path:        fmt.Sprintf(pathFmt, resourceID, relationship),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: nextURLErrorContext,
		ParseErr:    fmt.Sprintf("failed to parse %s relationship response", relationship),
	})
}
