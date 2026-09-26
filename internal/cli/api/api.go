// Package api implements `asc api`, an authenticated raw request passthrough
// for the App Store Connect API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	apiHost             = "api.appstoreconnect.apple.com"
	nearestOperationMax = 5
)

var supportedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}

// Command returns the `asc api` command.
func Command() *ffcli.Command {
	fs := flag.NewFlagSet("api", flag.ExitOnError)

	var query shared.MultiStringFlag
	fs.Var(&query, "query", "Query parameter as key=value (repeatable; GET only)")
	body := fs.String("body", "", "Inline JSON request body, or @path to read a file (@- for stdin); POST, PATCH, and DELETE only")
	bodyFile := fs.String("body-file", "", "Path to a JSON request body file (- for stdin); mutually exclusive with --body")
	paginate := fs.Bool("paginate", false, "Follow links.next and print one merged collection envelope (GET only)")
	confirm := fs.Bool("confirm", false, "Required for POST, PATCH, and DELETE requests")
	allowUnknownPath := fs.Bool("allow-unknown-path", false, "Send the request even when METHOD and PATH are not in the embedded schema index")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "json", "Output format: json", "json")

	return &ffcli.Command{
		Name:       "api",
		ShortUsage: "asc api <METHOD> <PATH> [flags]",
		ShortHelp:  "Send an authenticated raw request to the App Store Connect API.",
		LongHelp: `Send an authenticated raw request to the App Store Connect API.

METHOD is GET, POST, PATCH, or DELETE. PATH is a versioned API path such as
/v1/apps/{id}/builds or a full https://api.appstoreconnect.apple.com URL.
Responses print Apple's JSON envelope unmodified; an empty response body
prints nothing.

METHOD and PATH are checked against the embedded schema index (see asc schema).
An unknown operation is a usage error that lists the nearest known operations
unless --allow-unknown-path is set. Every non-GET request requires --confirm.

Examples:
  asc api GET /v1/apps --query limit=5 --query "fields[apps]=name,bundleId"
  asc api GET /v1/apps/APP_ID/relationships/builds --paginate
  asc api GET /v1/builds --query "filter[app]=APP_ID" --pretty
  asc api POST /v1/betaGroups --confirm --body '{"data":{"type":"betaGroups","attributes":{"name":"QA"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}'
  asc api PATCH /v1/apps/APP_ID --confirm --body-file update.json
  asc api DELETE /v1/betaGroups/GROUP_ID --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			positional, err := shared.ParseInterspersedFlags(fs, args)
			if err != nil {
				return shared.UsageErrorf("api: %v", err)
			}
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return shared.UsageErrorf("api: %v", err)
			}

			bodyProvided, bodyFileProvided := false, false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "body":
					bodyProvided = true
				case "body-file":
					bodyFileProvided = true
				}
			})

			request, err := parseRequest(positional, query, *body, *bodyFile, bodyProvided, bodyFileProvided, *paginate, *confirm, *allowUnknownPath)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("api: %w", err)
			}

			var response []byte
			if request.paginate {
				response, err = client.RawPaginatedGET(ctx, request.target, shared.ContextWithTimeout)
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()
				response, err = client.RawRequest(requestCtx, request.method, request.target, request.body)
			}
			if err != nil {
				return fmt.Errorf("api: %w", err)
			}

			return printResponse(response, *output.Pretty)
		},
	}
}

type rawRequest struct {
	method   string
	target   string
	body     []byte
	paginate bool
}

// parseRequest validates the invocation before any credential or network
// access. Every failure is a usage error with exit code 2.
func parseRequest(positional []string, query []string, body, bodyFile string, bodyProvided, bodyFileProvided, paginate, confirm, allowUnknownPath bool) (rawRequest, error) {
	if len(positional) < 2 {
		return rawRequest{}, shared.UsageError("api: METHOD and PATH are required")
	}
	if len(positional) > 2 {
		return rawRequest{}, shared.UsageErrorf("api: unexpected argument %q", positional[2])
	}

	method, err := normalizeMethod(positional[0])
	if err != nil {
		return rawRequest{}, err
	}
	path, values, err := normalizePath(positional[1])
	if err != nil {
		return rawRequest{}, err
	}

	if bodyProvided && bodyFileProvided {
		return rawRequest{}, shared.UsageError("api: --body and --body-file are mutually exclusive")
	}
	if bodyProvided && strings.TrimSpace(body) == "" {
		return rawRequest{}, shared.UsageError("api: --body must not be empty")
	}
	if bodyFileProvided && strings.TrimSpace(bodyFile) == "" {
		return rawRequest{}, shared.UsageError("api: --body-file must not be empty")
	}
	hasBody := bodyProvided || bodyFileProvided
	if method == http.MethodGet {
		if hasBody {
			return rawRequest{}, shared.UsageError("api: --body is only supported for POST, PATCH, and DELETE requests")
		}
	} else {
		if paginate {
			return rawRequest{}, shared.UsageError("api: --paginate is only supported for GET requests")
		}
		if len(query) > 0 || len(values) > 0 {
			return rawRequest{}, shared.UsageError("api: --query is only supported for GET requests")
		}
		if !confirm {
			return rawRequest{}, shared.WithDiagnostic(
				shared.UsageErrorf("api: --confirm is required for %s requests", method),
				shared.DiagnosticRequiredInputMissing,
				"--confirm",
			)
		}
	}

	for _, pair := range query {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return rawRequest{}, shared.UsageErrorf("api: --query must be key=value, got %q", pair)
		}
		values.Add(key, value)
	}

	endpoint, indexed, err := schema.MatchOperation(method, path)
	if err != nil {
		return rawRequest{}, fmt.Errorf("api: %w", err)
	}
	if !indexed && !allowUnknownPath {
		return rawRequest{}, unknownOperationError(method, path)
	}
	// The media-type check uses the matched template, so it applies under
	// --allow-unknown-path too: the limitation is the response representation,
	// not a gap in the index.
	if indexed {
		if err := rejectNonJSONOperation(method, endpoint.Path); err != nil {
			return rawRequest{}, err
		}
	}

	payload, err := readBody(body, bodyFile)
	if err != nil {
		return rawRequest{}, err
	}

	target := path
	if len(values) > 0 {
		target += "?" + values.Encode()
	}
	return rawRequest{method: method, target: target, body: payload, paginate: paginate}, nil
}

func normalizeMethod(raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	for _, supported := range supportedMethods {
		if method == supported {
			return method, nil
		}
	}
	return "", shared.UsageErrorf("api: METHOD must be one of %s, got %q", strings.Join(supportedMethods, ", "), raw)
}

// normalizePath accepts a versioned path or a full App Store Connect URL and
// returns the path plus any query parameters carried in the argument.
func normalizePath(raw string) (string, url.Values, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil, shared.UsageError("api: METHOD and PATH are required")
	}

	pathPart := trimmed
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", nil, shared.UsageErrorf("api: PATH is not a valid URL: %v", err)
		}
		if parsed.User != nil {
			return "", nil, shared.UsageError("api: PATH must not include user information")
		}
		// url.Parse accepts an empty fragment delimiter with an empty Fragment
		// field, so inspect the raw URL to reject both `#fragment` and trailing `#`.
		if strings.Contains(trimmed, "#") {
			return "", nil, shared.UsageError("api: PATH must not include a fragment")
		}
		if parsed.Scheme != "https" || parsed.Host != apiHost {
			return "", nil, shared.UsageErrorf("api: PATH must be an App Store Connect URL (https://%s/...)", apiHost)
		}
		// Validate the escaped form. Parsed.Path decodes %2F and %3F before the
		// checks below, allowing encoded data to become path or query delimiters.
		pathPart = parsed.EscapedPath()
		if parsed.RawQuery != "" {
			pathPart += "?" + parsed.RawQuery
		}
	}

	path, rawQuery, _ := strings.Cut(pathPart, "?")
	values := url.Values{}
	if rawQuery != "" {
		parsedValues, err := url.ParseQuery(rawQuery)
		if err != nil {
			return "", nil, shared.UsageErrorf("api: PATH query string is invalid: %v", err)
		}
		values = parsedValues
	}

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if !strings.HasPrefix(path, "/v1/") && !strings.HasPrefix(path, "/v2/") && !strings.HasPrefix(path, "/v3/") {
		return "", nil, shared.UsageErrorf("api: PATH must start with /v1/, /v2/, or /v3/, got %q", raw)
	}
	if strings.Contains(path, "//") || strings.ContainsAny(path, "#%\\") {
		return "", nil, shared.UsageErrorf("api: PATH contains an unsupported character sequence: %q", raw)
	}
	// The client rejects control characters too, but only once credentials have
	// been loaded. Rejecting them here keeps every path failure a usage error
	// that runs before any credential or network access.
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return "", nil, shared.UsageErrorf("api: PATH contains a control character: %q", raw)
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return "", nil, shared.UsageErrorf("api: PATH must not contain relative segments: %q", raw)
		}
	}
	return path, values, nil
}

// nonJSONOperation describes an indexed operation that `asc api` cannot serve.
// The shared client negotiates `Accept: application/json` and this command
// prints a JSON envelope, so an operation whose success response declares no
// `application/json` representation, or only a vendor JSON media type that
// must be negotiated explicitly, belongs to its dedicated command.
type nonJSONOperation struct {
	mediaType string
	command   string
}

// nonJSONOperations is keyed by templated path and covers every operation in
// docs/openapi/latest.json whose 2xx responses lack a plain `application/json`
// representation. TestNonJSONOperationsMatchOpenAPISnapshot derives the same
// set from the snapshot, so this table cannot drift when the snapshot updates.
var nonJSONOperations = map[string]nonJSONOperation{
	"/v1/salesReports": {
		mediaType: "application/a-gzip",
		command:   "asc analytics sales",
	},
	"/v1/financeReports": {
		mediaType: "application/a-gzip",
		command:   "asc finance reports",
	},
	"/v1/subscriptionOfferCodeOneTimeUseCodes/{id}/values": {
		mediaType: "text/csv",
		command:   "asc subscriptions offers offer-codes values",
	},
	"/v1/inAppPurchaseOfferCodeOneTimeUseCodes/{id}/values": {
		mediaType: "text/csv",
		command:   "asc iap offer-codes one-time-codes values",
	},
	"/v1/apps/{id}/perfPowerMetrics": {
		mediaType: "application/vnd.apple.xcode-metrics+json",
		command:   "asc performance metrics list",
	},
	"/v1/apps/{id}/performanceOverviews": {
		mediaType: "application/vnd.apple.xcode-overview+json",
		command:   "asc performance overview",
	},
	"/v1/builds/{id}/perfPowerMetrics": {
		mediaType: "application/vnd.apple.xcode-metrics+json",
		command:   "asc performance metrics view",
	},
	"/v1/diagnosticSignatures/{id}/logs": {
		mediaType: "application/vnd.apple.diagnostic-logs+json",
		command:   "asc performance diagnostics view",
	},
}

// rejectNonJSONOperation keeps the passthrough's JSON contract honest.
// templatePath is the matched schema template, so concrete IDs resolve to the
// same entry as the snapshot.
func rejectNonJSONOperation(method, templatePath string) error {
	operation, ok := nonJSONOperations[templatePath]
	if !ok || method != http.MethodGet {
		return nil
	}
	return shared.WithDiagnostic(
		shared.UsageErrorf(
			"api: %s %s responds with %s rather than a JSON envelope, which `asc api` cannot pass through; use `%s` instead",
			method, templatePath, operation.mediaType, operation.command,
		),
		shared.DiagnosticInvalidInput,
		"",
	)
}

func unknownOperationError(method, path string) error {
	nearest, err := schema.NearestOperations(method, path, nearestOperationMax)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}

	var message strings.Builder
	fmt.Fprintf(&message, "api: %s %s is not in the embedded schema index", method, path)
	if len(nearest) > 0 {
		names := make([]string, 0, len(nearest))
		for _, endpoint := range nearest {
			names = append(names, endpoint.Method+" "+endpoint.Path)
		}
		fmt.Fprintf(&message, " (nearest known operations: %s)", strings.Join(names, ", "))
	}
	message.WriteString(". Use `asc schema <query>` to inspect operations, or pass --allow-unknown-path to send the request anyway.")
	// The diagnostic keeps telemetry's failure parameter empty: the hint
	// mentions --allow-unknown-path, but that flag did not cause the failure.
	return shared.WithDiagnostic(shared.UsageError(message.String()), shared.DiagnosticInvalidInput, "")
}

// readBody loads the request payload from --body (inline JSON or @path) or
// --body-file and validates that it is a JSON object.
func readBody(body, bodyFile string) ([]byte, error) {
	inline := strings.TrimSpace(body)
	file := strings.TrimSpace(bodyFile)
	switch {
	case inline == "" && file == "":
		return nil, nil
	case inline != "" && strings.HasPrefix(inline, "@"):
		payload, err := shared.ReadJSONFilePayloadKind(strings.TrimPrefix(inline, "@"), shared.JSONPayloadObject)
		if err != nil {
			return nil, shared.UsageErrorf("api: --body: %v", err)
		}
		return payload, nil
	case inline != "":
		if !json.Valid([]byte(inline)) {
			return nil, shared.UsageError("api: --body: invalid JSON")
		}
		var object map[string]json.RawMessage
		// A JSON null unmarshals into a nil map without error, so the nil
		// check is what rejects `--body null` as a non-object payload.
		if err := json.Unmarshal([]byte(inline), &object); err != nil || object == nil {
			return nil, shared.UsageError("api: --body must be a JSON object")
		}
		return []byte(inline), nil
	default:
		payload, err := shared.ReadJSONFilePayloadKind(file, shared.JSONPayloadObject)
		if err != nil {
			return nil, shared.UsageErrorf("api: --body-file: %v", err)
		}
		return payload, nil
	}
}

// printResponse writes Apple's response body to stdout unmodified, or
// re-indented when --pretty is set. Empty bodies print nothing. `json` is the
// only output format, so a non-empty body that is not JSON fails on both
// paths rather than handing machine consumers invalid bytes.
func printResponse(response []byte, pretty bool) error {
	if len(bytes.TrimSpace(response)) == 0 {
		return nil
	}
	if !json.Valid(response) {
		return errors.New("api: response is not valid JSON")
	}
	out := response
	if pretty {
		var indented bytes.Buffer
		if err := json.Indent(&indented, response, "", "  "); err != nil {
			return fmt.Errorf("api: response is not valid JSON: %w", err)
		}
		out = indented.Bytes()
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		if _, err := os.Stdout.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
