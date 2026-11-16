package shared

import (
	"context"
	"net/url"
)

// APIOperation is what `asc api` parsed from its arguments, kept for the
// invocation's telemetry event: the method, the request path and the names of
// the query parameters, never their values. The telemetry event reduces the
// path to a schema template and the names to schema names before anything
// leaves the process.
type APIOperation struct {
	Method string
	Path   string
	// QueryNames is nil until the query parameters are known.
	QueryNames []string
}

type apiOperationKey struct{}

// WithAPIOperationRecord returns a context in which RecordAPIOperation and
// RecordAPIQueryNames fill the returned APIOperation.
func WithAPIOperationRecord(ctx context.Context) (context.Context, *APIOperation) {
	operation := &APIOperation{}
	return context.WithValue(ctx, apiOperationKey{}, operation), operation
}

// RecordAPIOperation records the method and request path of an `asc api`
// invocation in ctx's record, if it has one.
func RecordAPIOperation(ctx context.Context, method, path string) {
	if operation, ok := ctx.Value(apiOperationKey{}).(*APIOperation); ok {
		operation.Method = method
		operation.Path = path
	}
}

// RecordAPIQueryNames records the names of the query parameters an
// `asc api` request sends in ctx's record, if it has one.
func RecordAPIQueryNames(ctx context.Context, query url.Values) {
	operation, ok := ctx.Value(apiOperationKey{}).(*APIOperation)
	if !ok {
		return
	}
	names := make([]string, 0, len(query))
	for name := range query {
		names = append(names, name)
	}
	operation.QueryNames = names
}
