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
	var endpoints []Endpoint
	if err := json.Unmarshal(schemaIndexData, &endpoints); err != nil {
		return nil, fmt.Errorf("schema index: %w", err)
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
	q := strings.ToLower(query)
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

Query by path substring, dot-notation, or method+path. Returns endpoint
details including parameters, request attributes and relationships, and response schema
names as machine-readable JSON.

This lets agents self-serve API field names, parameter types, and allowed
values without pre-stuffed documentation.

Examples:
  asc schema apps                           # All endpoints matching "apps"
  asc schema "GET /v1/apps"                 # Exact method+path match
  asc schema apps.list                      # Dot-notation query
  asc schema --method POST apps             # Only POST endpoints for apps
  asc schema --list                         # List all 1200+ endpoints
  asc schema --list --method DELETE          # List all DELETE endpoints
  asc schema "builds" --pretty              # Pretty-print results`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			endpoints, err := loadIndex()
			if err != nil {
				return err
			}

			methodFilter, err := normalizeMethodFilter(*method)
			if err != nil {
				return err
			}

			if *listAll {
				return listEndpoints(endpoints, methodFilter, *pretty)
			}

			if len(args) == 0 {
				return shared.UsageError("query argument is required (or use --list)")
			}

			query := strings.Join(args, " ")
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

	var results []summary
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
	var results []Endpoint
	for _, e := range endpoints {
		if methodFilter != "" && e.Method != methodFilter {
			continue
		}
		if matchEndpoint(e, query) {
			results = append(results, e)
		}
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "No endpoints matching %q\n", query)
		return nil
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
