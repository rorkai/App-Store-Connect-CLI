package telemetry

import (
	"net/url"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
)

const (
	apiCommandFamily  = "api"
	apiOperationOther = "other"
	apiQueryCursor    = "cursor"
	maxAPIQueryParams = 64
)

// apiOperationFields reduces what `asc api` parsed to the three schema v6
// fields. Every other command, and an `asc api` invocation that failed before
// its METHOD and PATH were parsed, sends all three as null.
func apiOperationFields(family string, eventContext EventContext) (*string, *string, *[]string) {
	if family != apiCommandFamily {
		return nil, nil, nil
	}
	method, ok := classifyAPIMethod(eventContext.APIMethod)
	if !ok {
		return nil, nil, nil
	}
	template := classifyAPIPathTemplate(eventContext.APIPath)
	var query *[]string
	if eventContext.APIQueryNames != nil {
		if names, ok := classifyAPIQueryParams(eventContext.APIQueryNames); ok {
			query = &names
		}
	}
	return &method, &template, query
}

func classifyAPIMethod(raw string) (string, bool) {
	switch method := strings.ToUpper(strings.TrimSpace(raw)); method {
	case "GET", "POST", "PATCH", "DELETE":
		return method, true
	default:
		return "", false
	}
}

// classifyAPIPathTemplate returns the embedded schema's template for the
// request path, with every path parameter written {id}, or "other". It never
// returns any part of raw: a full URL is reduced to its path and the query
// string is ignored before matching.
func classifyAPIPathTemplate(raw string) string {
	path := strings.TrimSpace(raw)
	if strings.Contains(path, "://") {
		parsed, err := url.Parse(path)
		if err != nil || parsed.Host == "" {
			return apiOperationOther
		}
		path = parsed.EscapedPath()
	}
	path, _, _ = strings.Cut(path, "?")
	template, ok, err := schema.PathTemplate(path)
	if err != nil || !ok {
		return apiOperationOther
	}
	return template
}

// classifyAPIQueryParams maps each query parameter name to itself when the
// embedded schema defines it on some operation, to "cursor", or to "other",
// and returns them sorted, without duplicates, and at most maxAPIQueryParams
// long (the first ones after sorting). It never returns a name the schema does
// not define, and never a value. ok is false when the schema could not be read.
func classifyAPIQueryParams(names []string) ([]string, bool) {
	classified := map[string]struct{}{}
	for _, name := range names {
		switch known, err := schema.IsQueryParameterName(name); {
		case err != nil:
			return nil, false
		case known:
			classified[name] = struct{}{}
		case name == apiQueryCursor:
			classified[apiQueryCursor] = struct{}{}
		default:
			classified[apiOperationOther] = struct{}{}
		}
	}
	params := make([]string, 0, len(classified))
	for name := range classified {
		params = append(params, name)
	}
	sort.Strings(params)
	if len(params) > maxAPIQueryParams {
		params = params[:maxAPIQueryParams]
	}
	return params, true
}
