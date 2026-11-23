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
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

const (
	apiHost             = "api.appstoreconnect.apple.com"
	nearestOperationMax = 5
)

var supportedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}

// NewCommand returns the `asc api` command built on d.
func NewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("api", flag.ExitOnError)

	var query shared.MultiStringFlag
	fs.Var(&query, "query", "Query parameter as key=value (repeatable; GET only); names and documented values are checked against the operation")
	body := fs.String("body", "", "Inline JSON request body, or @path to read a file (@- for stdin); POST, PATCH, and DELETE only")
	bodyFile := fs.String("body-file", "", "Path to a JSON request body file (- for stdin); mutually exclusive with --body")
	paginate := fs.Bool("paginate", false, "Follow links.next and print one merged collection envelope (GET only)")
	confirm := fs.Bool("confirm", false, "Required for POST, PATCH, and DELETE requests")
	allowUnknownPath := fs.Bool("allow-unknown-path", false, "Send the request even when METHOD and PATH, its query parameter names and values, or the body's attribute and relationship names, are not in the embedded schema index")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", "json", "Output format: json", "json")
	// curl's and gh's spellings, and a common guess for the query flag.
	shared.BindFlagAlias(fs, "data", "body")
	shared.BindFlagAlias(fs, "url-params", "query")

	return &ffcli.Command{
		Name:       "api",
		ShortUsage: "asc api [METHOD] <PATH> [flags]",
		ShortHelp:  "Send an authenticated raw request to the App Store Connect API.",
		LongHelp: `Send an authenticated raw request to the App Store Connect API.

METHOD is GET, POST, PATCH, or DELETE, and defaults to GET when only PATH is
given. PATH is a versioned API path such as /v1/apps/{id}/builds or a full
https://api.appstoreconnect.apple.com URL; v1/apps/{id}/builds works too, and
apps/{id}/builds when exactly one API version has that operation.
Responses print Apple's JSON envelope unmodified; an empty response body
prints nothing.

METHOD and PATH are checked against the embedded schema index (see asc schema).
An unknown operation is a usage error unless --allow-unknown-path is set. The
error names the asc command that serves the request, or the indexed path it
most likely means (another API version or a misspelled segment), or else the
nearest known operations. Every non-GET request requires --confirm.
For non-GET requests, pass query parameters documented for the operation in
PATH itself. Query parameter names, and the values of parameters that
document them (include, fields[...], sort, enum filters), must be documented
for the operation; anything else is a usage error before any request unless
--allow-unknown-path is set, for values Apple added after the snapshot.
A POST or PATCH body must be a JSON:API document whose attribute
and relationship names are in the operation's request schema; anything else
is a usage error before any request unless --allow-unknown-path is set, for
attributes Apple added after the snapshot.

Examples:
  asc api GET /v1/apps --query limit=5 --query "fields[apps]=name,bundleId"
  asc api GET /v1/apps/APP_ID/relationships/builds --paginate
  asc api GET /v1/builds --query "filter[app]=APP_ID" --pretty
  asc api POST /v1/betaGroups --confirm --body '{"data":{"type":"betaGroups","attributes":{"name":"QA"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}'
  asc api PATCH /v1/apps/APP_ID --confirm --body-file update.json
  asc api DELETE /v1/betaGroups/GROUP_ID --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			positional, err := shared.ParseInterspersedFlags(fs, args)
			if err != nil {
				if value, ok := includeFlagValue(args); ok && err.Error() == "flag provided but not defined: --include" {
					return d.UsageErrorf("api: --include is not an `asc api` flag; pass it as a query parameter: --query include=%s", value)
				}
				return d.UsageErrorf("api: %v", err)
			}
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return d.UsageErrorf("api: %v", err)
			}

			if err := positionalBodyError(d, args, positional); err != nil {
				return err
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

			request, err := parseRequest(ctx, d, positional, query, *body, *bodyFile, bodyProvided, bodyFileProvided, *paginate, *confirm, *allowUnknownPath)
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("api: %w", err)
			}

			ctx = asc.WithUnlistedOperations(ctx)
			var response []byte
			if request.paginate {
				response, err = client.RawPaginatedGET(ctx, request.target, d.Timeout)
			} else {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				response, err = client.RawRequestWithQuery(requestCtx, request.method, request.target, request.query, request.body)
			}
			if err != nil {
				return fmt.Errorf("api: %w", err)
			}

			return printResponse(d.IO.Out, response, *output.Pretty)
		},
	}
}

// positionalBodyError answers a JSON document passed as a positional
// argument. No METHOD or PATH starts with `{` or `[`, so such an argument is
// a request body. The usual cause is `--json BODY`: --json is the value-less
// spelling of --output json, so the body after it lands among the
// positionals.
func positionalBodyError(d *shared.Deps, args, positional []string) error {
	for _, argument := range positional {
		trimmed := strings.TrimSpace(argument)
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			continue
		}
		if index := slices.Index(args, argument); index > 0 && followsJSONOutput(args[:index]) {
			return d.UsageError("api: --json takes no value (it means --output json); pass the request body with --body '<json>' (or --body @file, or --body-file FILE)")
		}
		return d.UsageError("api: the request body is not a positional argument; pass it with --body '<json>' (or --body @file, or --body-file FILE)")
	}
	return nil
}

// followsJSONOutput reports whether before ends in --json, or in --output
// json, the spelling the root may rewrite --json to.
func followsJSONOutput(before []string) bool {
	switch before[len(before)-1] {
	case "--json", "-json", "--output=json", "-output=json":
		return true
	case "json":
		return len(before) > 1 && (before[len(before)-2] == "--output" || before[len(before)-2] == "-output")
	}
	return false
}

type rawRequest struct {
	method   string
	target   string
	query    url.Values
	body     []byte
	paginate bool
}

// parseRequest validates the invocation before any credential or network
// access. Every failure is a usage error with exit code 2.
func parseRequest(ctx context.Context, d *shared.Deps, positional []string, query []string, body, bodyFile string, bodyProvided, bodyFileProvided, paginate, confirm, allowUnknownPath bool) (rawRequest, error) {
	// A lone PATH is a GET: anything but a method name is a PATH, such as
	// /v1/apps, apps/123 or apps.
	if len(positional) == 1 && !isMethodName(positional[0]) {
		positional = []string{http.MethodGet, positional[0]}
	}
	if len(positional) < 2 {
		return rawRequest{}, d.UsageError("api: METHOD and PATH are required")
	}
	if len(positional) > 2 {
		return rawRequest{}, d.UsageErrorf("api: unexpected argument %q", positional[2])
	}

	method, err := normalizeMethod(d, positional[0])
	if err != nil {
		return rawRequest{}, err
	}
	path, values, unversioned, err := normalizePath(d, method, positional[1])
	if err != nil {
		return rawRequest{}, err
	}
	shared.RecordAPIOperation(ctx, method, path)

	if bodyProvided && bodyFileProvided {
		return rawRequest{}, d.UsageError("api: --body and --body-file are mutually exclusive")
	}
	if bodyProvided && strings.TrimSpace(body) == "" {
		return rawRequest{}, d.UsageError("api: --body must not be empty")
	}
	if bodyFileProvided && strings.TrimSpace(bodyFile) == "" {
		return rawRequest{}, d.UsageError("api: --body-file must not be empty")
	}
	hasBody := bodyProvided || bodyFileProvided
	if method == http.MethodGet {
		if hasBody {
			return rawRequest{}, d.UsageError("api: --body is only supported for POST, PATCH, and DELETE requests")
		}
	} else {
		if paginate {
			return rawRequest{}, d.UsageError("api: --paginate is only supported for GET requests")
		}
		if len(query) > 0 {
			return rawRequest{}, d.UsageError("api: --query is only supported for GET requests")
		}
		if !confirm {
			return rawRequest{}, shared.WithDiagnostic(
				d.UsageErrorf("api: --confirm is required for %s requests", method),
				shared.DiagnosticRequiredInputMissing,
				"--confirm",
			)
		}
	}

	for _, pair := range query {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return rawRequest{}, d.UsageErrorf("api: --query must be key=value, got %q", pair)
		}
		values.Add(key, value)
	}
	shared.RecordAPIQueryNames(ctx, values)

	endpoint, indexed, err := schema.MatchOperation(method, path)
	if err != nil {
		return rawRequest{}, fmt.Errorf("api: %w", err)
	}
	if unversioned {
		// No API version indexes the operation. Answer as for the /v1 path, but
		// name the path as given; --allow-unknown-path cannot pick a version.
		if allowUnknownPath {
			return rawRequest{}, d.UsageErrorf("api: PATH must start with /v1/, /v2/, or /v3/, got %q", positional[1])
		}
		shown, _, _ := strings.Cut(strings.TrimSpace(positional[1]), "?")
		return rawRequest{}, unknownOperationError(d, method, path, shown, true)
	}
	if !indexed && !allowUnknownPath {
		return rawRequest{}, unknownOperationError(d, method, path, path, false)
	}
	if indexed && !allowUnknownPath {
		if err := validateQuery(d, endpoint, values); err != nil {
			// The failure is the query, not the --allow-unknown-path the
			// message offers, so telemetry's failure parameter stays empty.
			return rawRequest{}, shared.WithDiagnostic(err, shared.DiagnosticInvalidInput, "")
		}
	} else if !indexed && method != http.MethodGet && len(values) > 0 {
		// An unindexed operation documents no query parameters to send in
		// PATH. An indexed one under --allow-unknown-path skips the check, as
		// a GET does.
		return rawRequest{}, d.UsageError("api: PATH query parameters must be documented for this operation")
	}
	// The media-type check uses the matched template, so it applies under
	// --allow-unknown-path too: the limitation is the response representation,
	// not a gap in the index.
	if indexed {
		if err := rejectNonJSONOperation(d, method, endpoint.Path); err != nil {
			return rawRequest{}, err
		}
	}

	payload, err := readBody(d, body, bodyFile)
	if err != nil {
		return rawRequest{}, err
	}
	if indexed && !allowUnknownPath && len(payload) > 0 {
		if err := validateBody(d, endpoint, payload); err != nil {
			// The failure is the body, not the --allow-unknown-path the
			// message offers, so telemetry's failure parameter stays empty.
			return rawRequest{}, shared.WithDiagnostic(err, shared.DiagnosticInvalidInput, "")
		}
	}

	target := path
	if method == http.MethodGet && len(values) > 0 {
		target += "?" + values.Encode()
		values = nil
	}
	// Keep a full URL's host so the client refuses one meant for the other API.
	if parsed, err := url.Parse(strings.TrimSpace(positional[1])); err == nil && parsed.Host != "" {
		target = "https://" + parsed.Host + target
	}
	return rawRequest{method: method, target: target, query: values, body: payload, paginate: paginate}, nil
}

// paginationCursor is the query parameter Apple adds to links.next. The
// OpenAPI snapshot documents no operation with it, but a caller following a
// next link passes it back.
const paginationCursor = "cursor"

// schemaLagHint ends a query or body error that the embedded schema decides: the
// snapshot can lag Apple, which adds attributes and states between releases.
const schemaLagHint = ". If Apple accepts it after this snapshot, pass --allow-unknown-path to send it anyway."

// validateQuery checks every query parameter name, and each comma-separated
// value of a parameter with documented values, against the matched
// operation, so a request Apple would answer with 400 PARAMETER_ERROR fails
// first as a usage error that lists what the operation accepts.
func validateQuery(d *shared.Deps, endpoint schema.Endpoint, values url.Values) error {
	operation := endpoint.Method + " " + endpoint.Path
	documented := map[string]schema.Parameter{}
	var names []string
	for _, param := range endpoint.Parameters {
		if param.In == "query" {
			documented[param.Name] = param
			names = append(names, param.Name)
		}
	}
	keys := slices.Sorted(maps.Keys(values))
	for _, key := range keys {
		param, ok := documented[key]
		if !ok && endpoint.Method == http.MethodGet && key == paginationCursor {
			continue
		}
		if !ok {
			return undocumentedQueryNameError(d, operation, key, names, documented)
		}
		if len(param.Enum) == 0 {
			continue
		}
		for _, value := range values[key] {
			for item := range strings.SplitSeq(value, ",") {
				item = strings.TrimSpace(item)
				if item != "" && !slices.Contains(param.Enum, item) {
					return d.UsageErrorf("api: %s value %q is not documented for %s (documented: %s)"+schemaLagHint, key, item, operation, strings.Join(param.Enum, ", "))
				}
			}
		}
	}
	return nil
}

func undocumentedQueryNameError(d *shared.Deps, operation, key string, names []string, documented map[string]schema.Parameter) error {
	// Shell escaping (filter\[app\]) and percent-encoding in --query
	// (filter%5Bapp%5D) both reach Apple literally.
	if strings.Contains(key, `\`) {
		unescaped := strings.ReplaceAll(key, `\`, "")
		if _, ok := documented[unescaped]; ok {
			return d.UsageErrorf("api: query parameter %q contains a backslash, which Apple receives literally; pass %s", key, unescaped)
		}
	}
	if decoded, err := url.QueryUnescape(key); err == nil && decoded != key {
		if _, ok := documented[decoded]; ok {
			return d.UsageErrorf("api: query parameter %q is percent-encoded, which --query sends literally; pass %s", key, decoded)
		}
	}
	if len(names) == 0 {
		return d.UsageErrorf("api: query parameter %q is not documented for %s; it documents no query parameters"+schemaLagHint, key, operation)
	}
	closest, distance := "", 3
	for _, name := range names {
		if candidate := suggest.Distance(key, name); candidate < distance {
			closest, distance = name, candidate
		}
	}
	if closest != "" {
		return d.UsageErrorf("api: query parameter %q is not documented for %s; did you mean %s? (documented: %s)"+schemaLagHint, key, operation, closest, strings.Join(names, ", "))
	}
	return d.UsageErrorf("api: query parameter %q is not documented for %s (documented: %s)"+schemaLagHint, key, operation, strings.Join(names, ", "))
}

// isMethodName reports whether raw names a method, supported or not, so a
// lone `asc api PUT` still asks for METHOD and PATH.
func isMethodName(raw string) bool {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

func normalizeMethod(d *shared.Deps, raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	for _, supported := range supportedMethods {
		if method == supported {
			return method, nil
		}
	}
	return "", d.UsageErrorf("api: METHOD must be one of %s, got %q", strings.Join(supportedMethods, ", "), raw)
}

// normalizePath accepts a versioned path or a full App Store Connect URL and
// returns the path plus any query parameters carried in the argument. A path
// may omit its leading slash (v1/apps), and may omit its version when exactly
// one API version indexes the operation (apps/{id}/appInfos is /v1). When no
// version indexes a path without one, it returns the /v1 path and
// unversioned, so the caller can answer it as an unknown operation.
func normalizePath(d *shared.Deps, method, raw string) (path string, values url.Values, unversioned bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil, false, d.UsageError("api: METHOD and PATH are required")
	}

	pathPart := trimmed
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", nil, false, d.UsageErrorf("api: PATH is not a valid URL: %v", err)
		}
		if parsed.User != nil {
			return "", nil, false, d.UsageError("api: PATH must not include user information")
		}
		// url.Parse accepts an empty fragment delimiter with an empty Fragment
		// field, so inspect the raw URL to reject both `#fragment` and trailing `#`.
		if strings.Contains(trimmed, "#") {
			return "", nil, false, d.UsageError("api: PATH must not include a fragment")
		}
		if parsed.Scheme != "https" || !asc.IsAPIHost(parsed.Host) {
			return "", nil, false, d.UsageErrorf("api: PATH must be an App Store Connect URL (https://%s/...)", apiHost)
		}
		// Validate the escaped form. Parsed.Path decodes %2F and %3F before the
		// checks below, allowing encoded data to become path or query delimiters.
		pathPart = parsed.EscapedPath()
		if parsed.RawQuery != "" {
			pathPart += "?" + parsed.RawQuery
		}
	}

	path, rawQuery, _ := strings.Cut(pathPart, "?")
	values = url.Values{}
	if rawQuery != "" {
		parsedValues, err := url.ParseQuery(rawQuery)
		if err != nil {
			return "", nil, false, d.UsageErrorf("api: PATH query string is invalid: %v", err)
		}
		values = parsedValues
	}

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if !hasVersionPrefix(path) {
		versioned, ambiguous, err := versionedPath(method, path)
		if err != nil {
			return "", nil, false, fmt.Errorf("api: %w", err)
		}
		switch {
		case versioned != "":
			path = versioned
		case ambiguous || strings.Trim(path, "/") == "":
			return "", nil, false, d.UsageErrorf("api: PATH must start with /v1/, /v2/, or /v3/, got %q", raw)
		default:
			path = "/v1/" + strings.TrimPrefix(path, "/")
			unversioned = true
		}
	}
	if strings.Contains(path, "//") || strings.ContainsAny(path, "#%\\") {
		return "", nil, false, d.UsageErrorf("api: PATH contains an unsupported character sequence: %q", raw)
	}
	// The client rejects control characters too, but only once credentials have
	// been loaded. Rejecting them here keeps every path failure a usage error
	// that runs before any credential or network access.
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return "", nil, false, d.UsageErrorf("api: PATH contains a control character: %q", raw)
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return "", nil, false, d.UsageErrorf("api: PATH must not contain relative segments: %q", raw)
		}
	}
	return path, values, unversioned, nil
}

var apiVersions = []string{"/v1", "/v2", "/v3"}

func hasVersionPrefix(path string) bool {
	for _, version := range apiVersions {
		if strings.HasPrefix(path, version+"/") {
			return true
		}
	}
	return false
}

// versionedPath completes a path missing its leading slash or its version:
// v1/apps becomes /v1/apps, and apps/{id} the one versioned path the index
// has for method. It returns "" when no single completion exists, and
// ambiguous when more than one API version indexes the operation.
func versionedPath(method, path string) (versioned string, ambiguous bool, err error) {
	trimmed := strings.TrimPrefix(path, "/")
	if hasVersionPrefix("/" + trimmed) {
		return "/" + trimmed, false, nil
	}
	if trimmed == "" {
		return "", false, nil
	}
	found := ""
	for _, version := range apiVersions {
		candidate := version + "/" + trimmed
		_, indexed, err := schema.MatchOperation(method, candidate)
		if err != nil {
			return "", false, err
		}
		if indexed {
			if found != "" {
				return "", true, nil
			}
			found = candidate
		}
	}
	return found, false, nil
}

// includeFlagValue returns the value of an --include flag in args. `asc api`
// has no such flag; other asc commands use it for JSON:API include.
func includeFlagValue(args []string) (string, bool) {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != "include" {
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", false
			}
			value = args[i+1]
		}
		return value, true
	}
	return "", false
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
func rejectNonJSONOperation(d *shared.Deps, method, templatePath string) error {
	operation, ok := nonJSONOperations[templatePath]
	if !ok || method != http.MethodGet {
		return nil
	}
	return shared.WithDiagnostic(
		d.UsageErrorf(
			"api: %s %s responds with %s rather than a JSON envelope, which `asc api` cannot pass through; use `%s` instead",
			method, templatePath, operation.mediaType, operation.command,
		),
		shared.DiagnosticInvalidInput,
		"",
	)
}

// nativeCommands maps an operation the embedded schema index does not have,
// keyed by method and path template, to the asc command that serves what a
// caller means by it. These are the unindexed requests callers send most:
// relationships Apple does not expose, collections Apple reads through
// another resource, nested paths Apple serves one level at a time, and data
// only asc's own commands reach. A `{name}` segment in a key matches any path
// segment, and `{name}` in a command is the first segment it matched, so
// `{id}` in a one-ID path is the request's ID. TestNativeCommandsAreUnindexed
// keeps every key out of the index, and
// TestAPIUnknownPathNamesTheNativeCommand resolves every command.
var nativeCommands = map[string]string{
	"GET /v1/builds/{id}/betaGroups":                  "asc builds groups list --build-id {id}",
	"GET /v1/builds/{id}/relationships/betaGroups":    "asc builds groups list --build-id {id}",
	"GET /v1/builds/{id}/buildBundles":                "asc builds info --build-id {id} --include buildBundles",
	"GET /v1/apps/{id}/appInfoLocalizations":          "asc localizations list --app {id} --type app-info",
	"GET /v1/apps/{id}/appStoreLocalizations":         "asc localizations list --app {id}",
	"GET /v1/appStoreVersions/{id}/localizations":     "asc localizations list --version {id}",
	"GET /v1/appStoreVersions/{id}/appScreenshotSets": "asc screenshots list --version {id}",
	"GET /v1/ageRatingDeclarations/{id}":              "asc age-rating view --app APP_ID",
	"GET /v1/apps/{id}/ageRatingDeclaration":          "asc age-rating view --app {id}",
	"GET /v1/subscriptionAppStoreReviewScreenshots":   "asc subscriptions review app-store-screenshot view --subscription-id SUBSCRIPTION_ID",
	"GET /v1/reviewSubmissionItems":                   "asc review items list --submission SUBMISSION_ID",
	"GET /v1/reviewSubmissionItems/{id}":              "asc review items list --submission SUBMISSION_ID",
	"GET /v1/apps/{id}/prices":                        "asc pricing current --app {id}",
	"GET /v1/apps/{id}/appPrices":                     "asc pricing current --app {id}",
	"GET /v1/apps/{id}/availableTerritories":          "asc pricing availability view --app {id}",
	"GET /v1/apps/{id}/appDataUsages":                 "asc web privacy pull --app {id}",
	"POST /v1/betaGroups/{id}/builds":                 "asc builds add-groups --build-id BUILD_ID --group {id}",
	"POST /v1/builds/{id}/betaGroups":                 "asc builds add-groups --build-id {id} --group GROUP_ID",

	"GET /v1/apps/{app}/appStoreVersions/{id}/appStoreVersionLocalizations":                              "asc localizations list --version {id}",
	"GET /v1/appStoreVersions/{version}/appStoreVersionLocalizations/{id}/appStoreVersionScreenshotSets": "asc screenshots list --version-localization {id}",
}

// nativeCommand returns the command nativeCommands names for an unindexed
// request, with the request's path IDs filled in.
func nativeCommand(method, path string) (string, bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for key, command := range nativeCommands {
		keyMethod, template, _ := strings.Cut(key, " ")
		if keyMethod != method {
			continue
		}
		parts := strings.Split(strings.Trim(template, "/"), "/")
		if len(parts) != len(segments) {
			continue
		}
		ids, matched := map[string]string{}, true
		for i, part := range parts {
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				if _, ok := ids[part]; !ok {
					ids[part] = segments[i]
				}
				continue
			}
			if part != segments[i] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		// One pass, so an ID that spells a placeholder stays as given.
		pairs := make([]string, 0, 2*len(ids))
		for placeholder, id := range ids {
			quoted, ok := shared.ShellQuote(id)
			if !ok {
				quoted = "ID"
			}
			pairs = append(pairs, placeholder, quoted)
		}
		return strings.NewReplacer(pairs...).Replace(command), true
	}
	return "", false
}

// unknownOperationError answers a request the index does not have with what
// to run instead: the native command, the indexed operation the path most
// likely misspells (another API version, a plural, a misspelled segment), or
// the nearest operations on the same resource. --allow-unknown-path is
// offered only for that last case, where the resource is indexed and Apple
// may have added the operation after the snapshot. A resource the index does
// not know under any spelling is not in Apple's public API. An unversioned
// path is shown as given and needs its version prefix with that flag.
func unknownOperationError(d *shared.Deps, method, path, shown string, unversioned bool) error {
	nearest, err := schema.NearestOperations(method, path, nearestOperationMax)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}

	var message strings.Builder
	fmt.Fprintf(&message, "api: %s %s is not in the embedded schema index.", method, shown)
	command, hasCommand := nativeCommand(method, path)
	if hasCommand {
		fmt.Fprintf(&message, " Use `%s`.", command)
	}
	corrected := len(nearest) > 0 && nearest[0].Correction
	if corrected {
		fmt.Fprintf(&message, " Did you mean %s %s?", nearest[0].Method, fillTemplate(nearest[0].Path, path))
		nearest = nearest[1:]
	}
	childAdvice := ""
	if !hasCommand && !corrected {
		childAdvice, err = childOperationAdvice(method, path)
		if err != nil {
			return fmt.Errorf("api: %w", err)
		}
		message.WriteString(childAdvice)
	}
	switch {
	case len(nearest) > 0:
		names := make([]string, 0, len(nearest))
		for _, endpoint := range nearest {
			names = append(names, endpoint.Method+" "+endpoint.Path)
		}
		label := "Nearest known operations"
		if corrected || hasCommand || childAdvice != "" {
			label = "Other known operations"
		}
		fmt.Fprintf(&message, " %s: %s.", label, strings.Join(names, ", "))
	case !corrected && !hasCommand && childAdvice == "":
		message.WriteString(" No indexed operation shares its resource, under any API version or a similar name: the App Store Connect API does not document it.")
	}
	if corrected || hasCommand || len(nearest) == 0 {
		message.WriteString(" Use `asc schema <query>` to inspect operations.")
	} else if unversioned {
		// --allow-unknown-path cannot pick a version for the path.
		message.WriteString(" Use `asc schema <query>` to inspect operations, or add the /v1/, /v2/, or /v3/ prefix to PATH and pass --allow-unknown-path to send the request anyway.")
	} else {
		message.WriteString(" Use `asc schema <query>` to inspect operations, or pass --allow-unknown-path to send the request anyway.")
	}
	// The diagnostic keeps telemetry's failure parameter empty: the hint
	// mentions --allow-unknown-path, but that flag did not cause the failure.
	return shared.WithDiagnostic(d.UsageError(message.String()), shared.DiagnosticInvalidInput, "")
}

// childOperationAdvice answers a write to a parent's child collection,
// /{version}/{parents}/{id}/{children}, which Apple does not serve: a child is
// created at its own collection with a relationship to the parent, and
// changed or deleted by its own ID. It names that operation, and for a create
// the relationship the body needs, from the index, or returns "" when the
// index has none.
func childOperationAdvice(method, path string) (string, error) {
	if method == http.MethodGet {
		return "", nil
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) != 4 {
		return "", nil
	}
	version, parents, parentID, children := segments[0], segments[1], segments[2], segments[3]
	versions := append([]string{version}, slices.DeleteFunc([]string{"v1", "v2", "v3"}, func(v string) bool { return v == version })...)
	// find returns the indexed path of method on /{version}/{suffix}, trying
	// the request's API version first, with suffix's own values filled in.
	find := func(method, suffix string) (string, bool, error) {
		for _, v := range versions {
			concrete := "/" + v + "/" + suffix
			endpoint, ok, err := schema.MatchOperation(method, concrete)
			if err != nil || ok {
				return fillTemplate(endpoint.Path, concrete), ok, err
			}
		}
		return "", false, nil
	}

	if method == http.MethodPost {
		return createAdvice(versions, parents, parentID, children)
	}

	item, ok, err := find(method, children+"/{id}")
	if err != nil || !ok {
		return "", err
	}
	verb := "change"
	if method == http.MethodDelete {
		verb = "delete"
	}
	advice := fmt.Sprintf(" To %s one, use %s %s", verb, method, item)
	list, ok, err := find(http.MethodGet, parents+"/"+parentID+"/"+children)
	if err != nil {
		return "", err
	}
	if ok {
		advice += " with its ID from GET " + list
	}
	return advice + ".", nil
}

// createAdvice names the indexed create for children, preferring one whose
// request takes a to-one relationship to the parent's type, and that
// relationship filled in with the parent's ID. Without one it names the
// create in the first API version that has it, with its required
// relationships.
func createAdvice(versions []string, parents, parentID, children string) (string, error) {
	var fallback schema.Endpoint
	for _, version := range versions {
		create, ok, err := schema.MatchOperation(http.MethodPost, "/"+version+"/"+children)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(create.RequestRelationships)) {
			relationship := create.RequestRelationships[name]
			if relationship.ResourceType != parents || relationship.Cardinality != "one" {
				continue
			}
			var linkage struct {
				Data struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			}
			linkage.Data.Type, linkage.Data.ID = parents, parentID
			encoded, err := json.Marshal(linkage)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf(" To create one, use POST %s with relationships.%s = %s.", create.Path, name, encoded), nil
		}
		if fallback.Path == "" {
			fallback = create
		}
	}
	if fallback.Path == "" {
		return "", nil
	}
	var required []string
	for _, name := range slices.Sorted(maps.Keys(fallback.RequestRelationships)) {
		if relationship := fallback.RequestRelationships[name]; relationship.Required {
			required = append(required, fmt.Sprintf("relationships.%s (%s)", name, relationship.ResourceType))
		}
	}
	if len(required) == 0 {
		return fmt.Sprintf(" To create one, use POST %s.", fallback.Path), nil
	}
	return fmt.Sprintf(" To create one, use POST %s with %s.", fallback.Path, strings.Join(required, ", ")), nil
}

// validateBody checks a request document against the matched operation's
// request schema: Apple answers a body without a top-level `data` member
// with 422, and an attribute or relationship the resource does not have
// with 409 ENTITY_ERROR.ATTRIBUTE.UNKNOWN. The embedded index carries each
// request schema's attribute and relationship names, so both fail first as a
// usage error naming what the request takes. Linkage requests, whose data is
// an array of resource identifiers, have no attributes to check.
func validateBody(d *shared.Deps, endpoint schema.Endpoint, payload []byte) error {
	if endpoint.RequestSchema == "" {
		return nil
	}
	operation := endpoint.Method + " " + endpoint.Path
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil
	}
	data, ok := document["data"]
	if !ok {
		return d.UsageErrorf(`api: the request body for %s must be a JSON:API document, {"data":{"type":...,"attributes":{...}}}; it has no top-level "data" member`, operation)
	}
	var resource struct {
		Attributes    map[string]json.RawMessage `json:"attributes"`
		Relationships map[string]json.RawMessage `json:"relationships"`
	}
	if json.Unmarshal(data, &resource) != nil {
		return nil
	}
	if err := checkMemberNames(d, "attribute", resource.Attributes, slices.Collect(maps.Keys(endpoint.RequestAttributes)), endpoint.RequestSchema, operation); err != nil {
		return err
	}
	return checkMemberNames(d, "relationship", resource.Relationships, slices.Collect(maps.Keys(endpoint.RequestRelationships)), endpoint.RequestSchema, operation)
}

func checkMemberNames(d *shared.Deps, kind string, members map[string]json.RawMessage, documented []string, requestSchema, operation string) error {
	slices.Sort(documented)
	for _, name := range slices.Sorted(maps.Keys(members)) {
		if slices.Contains(documented, name) {
			continue
		}
		if len(documented) == 0 {
			return d.UsageErrorf("api: %s %q is not in %s for %s, which takes no %ss"+schemaLagHint, kind, name, requestSchema, operation, kind)
		}
		closest, distance := "", 3
		for _, candidate := range documented {
			if edits := suggest.Distance(strings.ToLower(name), strings.ToLower(candidate)); edits < distance {
				closest, distance = candidate, edits
			}
		}
		if closest != "" {
			return d.UsageErrorf("api: %s %q is not in %s for %s; did you mean %s? (%ss: %s)"+schemaLagHint, kind, name, requestSchema, operation, closest, kind, strings.Join(documented, ", "))
		}
		return d.UsageErrorf("api: %s %q is not in %s for %s (%ss: %s)"+schemaLagHint, kind, name, requestSchema, operation, kind, strings.Join(documented, ", "))
	}
	return nil
}

// fillTemplate renders an indexed template with the request's own value in
// each parameter segment, so a corrected path keeps the caller's IDs.
func fillTemplate(template, path string) string {
	parts := strings.Split(strings.Trim(template, "/"), "/")
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != len(segments) {
		return template
	}
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = segments[i]
		}
	}
	return "/" + strings.Join(parts, "/")
}

// readBody loads the request payload from --body (inline JSON or @path) or
// --body-file and validates that it is a JSON object.
func readBody(d *shared.Deps, body, bodyFile string) ([]byte, error) {
	inline := strings.TrimSpace(body)
	file := strings.TrimSpace(bodyFile)
	switch {
	case inline == "" && file == "":
		return nil, nil
	case inline != "" && strings.HasPrefix(inline, "@"):
		payload, err := d.ReadJSONFilePayloadKind(strings.TrimPrefix(inline, "@"), shared.JSONPayloadObject)
		if err != nil {
			return nil, d.UsageErrorf("api: --body: %v", err)
		}
		return payload, nil
	case inline != "":
		if !json.Valid([]byte(inline)) {
			return nil, d.UsageError("api: --body: invalid JSON")
		}
		var object map[string]json.RawMessage
		// A JSON null unmarshals into a nil map without error, so the nil
		// check is what rejects `--body null` as a non-object payload.
		if err := json.Unmarshal([]byte(inline), &object); err != nil || object == nil {
			return nil, d.UsageError("api: --body must be a JSON object")
		}
		return []byte(inline), nil
	default:
		payload, err := d.ReadJSONFilePayloadKind(file, shared.JSONPayloadObject)
		if err != nil {
			return nil, d.UsageErrorf("api: --body-file: %v", err)
		}
		return payload, nil
	}
}

// printResponse writes Apple's response body to stdout unmodified, or
// re-indented when --pretty is set. Empty bodies print nothing. `json` is the
// only output format, so a non-empty body that is not JSON fails on both
// paths rather than handing machine consumers invalid bytes.
func printResponse(w io.Writer, response []byte, pretty bool) error {
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
	if _, err := w.Write(out); err != nil {
		return err
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
