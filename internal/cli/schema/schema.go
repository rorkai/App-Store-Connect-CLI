package schema

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

//go:embed schema_index.json
var schemaIndexData []byte

// Endpoint is a compact representation of an API endpoint.
type Endpoint struct {
	Method               string                         `json:"method"`
	Path                 string                         `json:"path"`
	Parameters           []Parameter                    `json:"parameters,omitempty"`
	RequestSchema        string                         `json:"requestSchema,omitempty"`
	RequestAttributes    map[string]any                 `json:"requestAttributes,omitempty"`
	RequestRelationships map[string]RequestRelationship `json:"requestRelationships,omitempty"`
	ResponseSchema       string                         `json:"responseSchema,omitempty"`
	getAction            string
}

type indexedEndpoint struct {
	Endpoint
	GetAction string `json:"getAction,omitempty"`
}

// RequestRelationship describes a JSON:API relationship accepted by a request.
type RequestRelationship struct {
	ResourceType string `json:"resourceType"`
	Cardinality  string `json:"cardinality"`
	Required     bool   `json:"required"`
}

// Parameter describes a query/path parameter.
type Parameter struct {
	Name     string   `json:"name"`
	In       string   `json:"in"`
	Enum     []string `json:"enum,omitempty"`
	Required bool     `json:"required,omitempty"`
}

func loadIndex() ([]Endpoint, error) {
	var indexed []indexedEndpoint
	if err := json.Unmarshal(schemaIndexData, &indexed); err != nil {
		return nil, fmt.Errorf("schema index: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(indexed))
	for _, item := range indexed {
		endpoint := item.Endpoint
		endpoint.getAction = item.GetAction
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

// EndpointCount returns the number of endpoints in the embedded schema index.
func EndpointCount() (int, error) {
	endpoints, err := loadIndex()
	if err != nil {
		return 0, err
	}
	return len(endpoints), nil
}

func matchEndpoint(e Endpoint, query string) bool {
	if method, path, exact := parseExactEndpointQuery(query); exact {
		return strings.EqualFold(e.Method, method) && e.Path == path
	}

	q := strings.ToLower(strings.TrimSpace(query))
	if isActionDotNotationQuery(q) {
		return strings.EqualFold(pathToActionDotNotation(e), q) ||
			strings.EqualFold(pathToVersionedActionDotNotation(e), q)
	}
	if strings.Contains(strings.ToLower(e.Path), q) {
		return true
	}
	combined := strings.ToLower(e.Method + " " + e.Path)
	if strings.Contains(combined, q) {
		return true
	}
	dotNotation := pathToDotNotation(e.Method, e.Path)
	return strings.Contains(strings.ToLower(dotNotation), q)
}

func isActionDotNotationQuery(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	lastDot := strings.LastIndex(normalized, ".")
	if lastDot <= 0 {
		return false
	}
	action := normalized[lastDot+1:]
	switch action {
	case "list", "get", "create", "update", "delete":
		return true
	default:
		return false
	}
}

func parseExactEndpointQuery(query string) (string, string, bool) {
	parts := strings.Fields(strings.TrimSpace(query))
	if len(parts) != 2 || !strings.HasPrefix(parts[1], "/") {
		return "", "", false
	}

	method := strings.ToUpper(parts[0])
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
		return method, parts[1], true
	default:
		return "", "", false
	}
}

func pathToDotNotation(method, path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.Split(trimmed, "/")

	var segments []string
	for _, p := range parts {
		if strings.HasPrefix(p, "v") && len(p) <= 3 {
			continue
		}
		if strings.HasPrefix(p, "{") {
			continue
		}
		segments = append(segments, p)
	}

	result := strings.Join(segments, ".")
	if method != "" && method != "GET" {
		result = strings.ToLower(method) + ":" + result
	}
	return result
}

func pathToActionDotNotation(endpoint Endpoint) string {
	dotPath := pathToDotNotation("", endpoint.Path)
	if dotPath == "" {
		return ""
	}

	action := strings.ToLower(endpoint.Method)
	switch endpoint.Method {
	case http.MethodGet:
		action = getEndpointAction(endpoint)
	case http.MethodPost:
		action = "create"
	case http.MethodPatch:
		action = "update"
	case http.MethodDelete:
		action = "delete"
	}

	if action == "" {
		return dotPath
	}
	return dotPath + "." + action
}

func pathToVersionedActionDotNotation(endpoint Endpoint) string {
	actionPath := pathToActionDotNotation(endpoint)
	trimmed := strings.TrimPrefix(endpoint.Path, "/")
	version, _, found := strings.Cut(trimmed, "/")
	if !found || !strings.HasPrefix(version, "v") || len(version) > 3 {
		return actionPath
	}
	return version + "." + actionPath
}

func getEndpointAction(endpoint Endpoint) string {
	switch endpoint.getAction {
	case "get", "list":
		return endpoint.getAction
	}

	trimmed := strings.TrimSuffix(endpoint.Path, "/")
	if lastSlash := strings.LastIndex(trimmed, "/"); lastSlash >= 0 {
		lastSegment := trimmed[lastSlash+1:]
		if strings.HasPrefix(lastSegment, "{") && strings.HasSuffix(lastSegment, "}") {
			return "get"
		}
	}
	return "list"
}

func normalizeMethodFilter(raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if method == "" {
		return "", nil
	}

	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
		return method, nil
	default:
		return "", shared.UsageErrorf("invalid --method %q (allowed: GET, POST, PATCH, DELETE)", method)
	}
}

// SchemaCommand returns the schema command.
func SchemaCommand() *ffcli.Command {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	listAll := fs.Bool("list", false, "List all endpoints (compact summary)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	method := fs.String("method", "", "Filter by HTTP method (GET, POST, PATCH, DELETE)")

	return &ffcli.Command{
		Name:       "schema",
		ShortUsage: "asc schema [flags] [query]",
		ShortHelp:  "Inspect App Store Connect API endpoint schemas at runtime.",
		LongHelp: `Inspect App Store Connect API endpoint schemas at runtime.

Query by fuzzy path substring, exact method+path, or exact action dot notation.
Dot notation uses .list, .get, .create, .update, or .delete actions. Prefix an
action with v1. or v2. to select an exact API version when both exist. Queries
with no matches return an empty JSON array. Results include parameters, request
attributes and relationships, and response schema names as machine-readable JSON.

This lets agents self-serve API field names, parameter types, and allowed
values without pre-stuffed documentation.

Examples:
  asc schema apps                           # All endpoints matching "apps"
  asc schema "GET /v1/apps"                 # Exact method+path match
  asc schema apps.list                      # Dot-notation query
  asc schema v2.gameCenterAchievements.get  # Version-qualified dot notation
  asc schema --method POST apps             # Only POST endpoints for apps
  asc schema --list                         # List all 1200+ endpoints
  asc schema --list --method DELETE          # List all DELETE endpoints
  asc schema "builds" --pretty              # Pretty-print results`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			queryArgs, err := shared.ParseInterspersedFlags(fs, args)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			endpoints, err := loadIndex()
			if err != nil {
				return err
			}

			methodFilter, err := normalizeMethodFilter(*method)
			if err != nil {
				return err
			}

			if *listAll {
				if len(queryArgs) > 0 {
					return shared.UsageError("--list does not accept a query")
				}
				return listEndpoints(endpoints, methodFilter, *pretty)
			}

			if len(queryArgs) == 0 || strings.TrimSpace(strings.Join(queryArgs, " ")) == "" {
				return shared.UsageError("query argument is required (or use --list)")
			}

			query := strings.Join(queryArgs, " ")
			return queryEndpoints(endpoints, query, methodFilter, *pretty)
		},
	}
}

func listEndpoints(endpoints []Endpoint, methodFilter string, pretty bool) error {
	type summary struct {
		Method         string `json:"method"`
		Path           string `json:"path"`
		ResponseSchema string `json:"responseSchema,omitempty"`
		ParamCount     int    `json:"paramCount"`
	}

	results := make([]summary, 0)
	for _, e := range endpoints {
		if methodFilter != "" && e.Method != methodFilter {
			continue
		}
		results = append(results, summary{
			Method:         e.Method,
			Path:           e.Path,
			ResponseSchema: e.ResponseSchema,
			ParamCount:     len(e.Parameters),
		})
	}

	return printJSON(results, pretty)
}

func queryEndpoints(endpoints []Endpoint, query, methodFilter string, pretty bool) error {
	results := make([]Endpoint, 0)
	for _, e := range endpoints {
		if methodFilter != "" && e.Method != methodFilter {
			continue
		}
		if matchEndpoint(e, query) {
			results = append(results, e)
		}
	}

	return printJSON(results, pretty)
}

func printJSON(data any, pretty bool) error {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}
