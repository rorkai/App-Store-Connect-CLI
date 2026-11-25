package cmdtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAPIUsageErrorsRunBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing method and path",
			args:    []string{"api"},
			wantErr: "api: METHOD and PATH are required",
		},
		{
			// The root rewrites --data before METHOD to --body, so this is
			// --body twice with different values.
			name:    "alias before the positionals and canonical after",
			args:    []string{"api", "--data", `{"data":{"a":1}}`, "POST", "/v1/bundleIds", "--confirm", "--body", `{"data":{"b":2}}`},
			wantErr: "api: `--body` is given before and after the positional arguments with different values (`--data` is the same flag); pass one value",
		},
		{
			name:    "missing path",
			args:    []string{"api", "GET"},
			wantErr: "api: METHOD and PATH are required",
		},
		{
			name:    "too many positionals",
			args:    []string{"api", "GET", "/v1/apps", "extra"},
			wantErr: "api: unexpected argument \"extra\"",
		},
		{
			name:    "quoted method and path",
			args:    []string{"api", "GET /v1/ageRatingDeclarations"},
			wantErr: "api: METHOD and PATH must be separate arguments; pass e.g. `asc api GET /v1/apps`",
		},
		{
			name:    "fields flag points to query",
			args:    []string{"api", "GET", "/v1/apps", "--fields", "name"},
			wantErr: "api: --fields is not an `asc api` flag; pass sparse fields as a query parameter: --query \"fields[TYPE]=FIELD1,FIELD2\"",
		},
		{
			name:    "fields flag before method points to query",
			args:    []string{"api", "--fields=caller-fixture", "GET", "/v1/apps"},
			wantErr: "api: --fields is not an `asc api` flag; pass sparse fields as a query parameter: --query \"fields[TYPE]=FIELD1,FIELD2\"",
		},
		{
			// --json is --output json, so the body after it is a stray positional.
			name:    "json body after --json",
			args:    []string{"api", "post", "/v1/inAppPurchaseLocalizations", "--json", `{"data":{"type":"inAppPurchaseLocalizations"}}`, "--output", "json"},
			wantErr: "api: --json takes no value (it means --output json); pass the request body with --body '<json>' (or --body @file, or --body-file FILE)",
		},
		{
			name:    "json body as a positional",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", ` [{"type":"builds"}]`},
			wantErr: "api: the request body is not a positional argument; pass it with --body '<json>' (or --body @file, or --body-file FILE)",
		},
		{
			name:    "unsupported method",
			args:    []string{"api", "PUT", "/v1/apps"},
			wantErr: "api: METHOD must be one of GET, POST, PATCH, DELETE",
		},
		{
			// Without the index there is no version to complete the path with.
			name:    "path without a version under --allow-unknown-path",
			args:    []string{"api", "GET", "agreements", "--allow-unknown-path"},
			wantErr: `api: PATH must start with /v1/, /v2/, or /v3/, got "agreements"`,
		},
		{
			name:    "include flag",
			args:    []string{"api", "GET", "/v1/apps/123/appInfos", "--include", "appInfoLocalizations"},
			wantErr: "api: --include is not an `asc api` flag; pass it as a query parameter: --query include=appInfoLocalizations",
		},
		{
			name:    "include flag with equals",
			args:    []string{"api", "GET", "/v1/apps/123/appInfos", "--include=appInfoLocalizations"},
			wantErr: "api: --include is not an `asc api` flag; pass it as a query parameter: --query include=appInfoLocalizations",
		},
		{
			name:    "foreign host",
			args:    []string{"api", "GET", "https://example.com/v1/apps"},
			wantErr: "api: PATH must be an App Store Connect URL",
		},
		{
			name:    "encoded path separator in full URL",
			args:    []string{"api", "GET", "https://api.appstoreconnect.apple.com/v1/apps%2F123"},
			wantErr: "api: PATH contains an unsupported character sequence",
		},
		{
			name:    "encoded query delimiter in full URL path",
			args:    []string{"api", "GET", "https://api.appstoreconnect.apple.com/v1/apps/123%3Finclude=betaGroups"},
			wantErr: "api: PATH contains an unsupported character sequence",
		},
		{
			name:    "URL userinfo",
			args:    []string{"api", "GET", "https://operator@api.appstoreconnect.apple.com/v1/apps"},
			wantErr: "api: PATH must not include user information",
		},
		{
			name:    "URL fragment",
			args:    []string{"api", "GET", "https://api.appstoreconnect.apple.com/v1/apps#ignored"},
			wantErr: "api: PATH must not include a fragment",
		},
		{
			name:    "empty URL fragment delimiter",
			args:    []string{"api", "GET", "https://api.appstoreconnect.apple.com/v1/apps#"},
			wantErr: "api: PATH must not include a fragment",
		},
		{
			name:    "unknown path lists nearest operations",
			args:    []string{"api", "GET", "/v1/apps/123/buildz"},
			wantErr: "api: GET /v1/apps/123/buildz is not in the embedded schema index",
		},
		{
			name:    "unknown method for known path",
			args:    []string{"api", "POST", "/v1/apps/123/builds", "--confirm"},
			wantErr: "api: POST /v1/apps/123/builds is not in the embedded schema index",
		},
		{
			name:    "mutation without confirm",
			args:    []string{"api", "PATCH", "/v1/apps/123", "--body", `{"data":{}}`},
			wantErr: "--confirm is required for PATCH requests",
		},
		{
			name:    "paginate on mutation",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--paginate"},
			wantErr: "api: --paginate is only supported for GET requests",
		},
		{
			name:    "query on mutation",
			args:    []string{"api", "DELETE", "/v1/betaGroups/123", "--confirm", "--query", "limit=1"},
			wantErr: "api: --query is only supported for GET requests",
		},
		{
			name:    "undocumented inline mutation query",
			args:    []string{"api", "DELETE", "/v1/betaGroups/123?limit=1", "--confirm"},
			wantErr: `api: query parameter "limit" is not documented for DELETE /v1/betaGroups/{id}; it documents no query parameters`,
		},
		{
			name:    "undocumented GET query name",
			args:    []string{"api", "GET", "/v1/apps/123/builds", "--query", "sort=-uploadedDate"},
			wantErr: `api: query parameter "sort" is not documented for GET /v1/apps/{id}/builds (documented: fields[builds], limit)`,
		},
		{
			name:    "GET query name one edit from a documented one",
			args:    []string{"api", "GET", "/v1/betaTesters?filter[app]=123"},
			wantErr: `api: query parameter "filter[app]" is not documented for GET /v1/betaTesters; did you mean filter[apps]?`,
		},
		{
			name:    "shell-escaped brackets in a GET query name",
			args:    []string{"api", "GET", "/v1/apps/123/appStoreVersions", "--query", `filter\[appStoreState\]=READY_FOR_SALE`},
			wantErr: `api: query parameter "filter\\[appStoreState\\]" contains a backslash, which Apple receives literally; pass filter[appStoreState]`,
		},
		{
			name:    "percent-encoded brackets in a GET query name",
			args:    []string{"api", "GET", "/v1/apps/123/appStoreVersions", "--query", "filter%5BappStoreState%5D=READY_FOR_SALE"},
			wantErr: `api: query parameter "filter%5BappStoreState%5D" is percent-encoded, which --query sends literally; pass filter[appStoreState]`,
		},
		{
			name:    "undocumented GET include value",
			args:    []string{"api", "GET", "/v1/subscriptions/123/prices?include=pricePoint"},
			wantErr: `api: include value "pricePoint" is not documented for GET /v1/subscriptions/{id}/prices (documented: territory, subscriptionPricePoint)`,
		},
		{
			name:    "undocumented GET filter value in a list",
			args:    []string{"api", "GET", "/v1/apps/123/appStoreVersions", "--query", "filter[appVersionState]=READY_FOR_DISTRIBUTION,READY_FOR_SALE"},
			wantErr: `api: filter[appVersionState] value "READY_FOR_SALE" is not documented for GET /v1/apps/{id}/appStoreVersions (documented: `,
		},
		{
			name:    "attribute of another resource",
			args:    []string{"api", "PATCH", "/v1/appInfoLocalizations/AIL1", "--confirm", "--body", `{"data":{"type":"appInfoLocalizations","id":"AIL1","attributes":{"supportUrl":"https://example.com/support"}}}`},
			wantErr: `api: attribute "supportUrl" is not in AppInfoLocalizationUpdateRequest for PATCH /v1/appInfoLocalizations/{id} (attributes: name, privacyChoicesUrl, privacyPolicyText, privacyPolicyUrl, subtitle)`,
		},
		{
			name:    "attribute one edit from a documented one",
			args:    []string{"api", "PATCH", "/v1/appInfoLocalizations/AIL1", "--confirm", "--body", `{"data":{"type":"appInfoLocalizations","id":"AIL1","attributes":{"privacyPolicyURL":"https://example.com/privacy"}}}`},
			wantErr: `api: attribute "privacyPolicyURL" is not in AppInfoLocalizationUpdateRequest for PATCH /v1/appInfoLocalizations/{id}; did you mean privacyPolicyUrl?`,
		},
		{
			name:    "undocumented relationship",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `{"data":{"type":"betaGroups","attributes":{"name":"QA"},"relationships":{"apps":{"data":{"type":"apps","id":"123"}}}}}`},
			wantErr: `api: relationship "apps" is not in BetaGroupCreateRequest for POST /v1/betaGroups; did you mean app?`,
		},
		{
			name:    "body without the data member",
			args:    []string{"api", "PATCH", "/v1/appInfoLocalizations/AIL1", "--confirm", "--body", `{"attributes":{"name":"Demo"}}`},
			wantErr: `api: the request body for PATCH /v1/appInfoLocalizations/{id} must be a JSON:API document, {"data":{"type":...,"attributes":{...}}}; it has no top-level "data" member`,
		},
		{
			name:    "query on an unindexed mutation",
			args:    []string{"api", "DELETE", "/v1/notIndexed/1?limit=1", "--confirm", "--allow-unknown-path"},
			wantErr: "api: PATH query parameters must be documented for this operation",
		},
		{
			name:    "body on GET",
			args:    []string{"api", "GET", "/v1/apps", "--body", `{"data":{}}`},
			wantErr: "api: --body is only supported for POST, PATCH, and DELETE requests",
		},
		{
			name:    "body and body-file together",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `{}`, "--body-file", "payload.json"},
			wantErr: "api: --body and --body-file are mutually exclusive",
		},
		{
			name:    "explicit empty body",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", ""},
			wantErr: "api: --body must not be empty",
		},
		{
			name:    "whitespace body",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", " \t "},
			wantErr: "api: --body must not be empty",
		},
		{
			name:    "empty body and body-file remain mutually exclusive",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", "", "--body-file", "payload.json"},
			wantErr: "api: --body and --body-file are mutually exclusive",
		},
		{
			name:    "explicit empty body-file",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body-file", ""},
			wantErr: "api: --body-file must not be empty",
		},
		{
			name:    "malformed query pair",
			args:    []string{"api", "GET", "/v1/apps", "--query", "limit"},
			wantErr: "api: --query must be key=value",
		},
		{
			name:    "invalid inline body",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `{"data":`},
			wantErr: "api: --body: invalid JSON",
		},
		{
			name:    "inline body null is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `null`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "inline body array is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `[{"data":{}}]`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "inline body scalar is not an object",
			args:    []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", `"data"`},
			wantErr: "api: --body must be a JSON object",
		},
		{
			name:    "control character in path",
			args:    []string{"api", "GET", "/v1/apps/12\n3/builds"},
			wantErr: "api: PATH contains a control character",
		},
		{
			name:    "gzip sales report endpoint rejected",
			args:    []string{"api", "GET", "/v1/salesReports", "--query", "filter[vendorNumber]=123"},
			wantErr: "api: GET /v1/salesReports responds with application/a-gzip rather than a JSON envelope, which `asc api` cannot pass through; use `asc analytics sales` instead",
		},
		{
			name:    "gzip finance report endpoint rejected",
			args:    []string{"api", "GET", "/v1/financeReports", "--query", "filter[vendorNumber]=123"},
			wantErr: "api: GET /v1/financeReports responds with application/a-gzip rather than a JSON envelope, which `asc api` cannot pass through; use `asc finance reports` instead",
		},
		{
			name:    "csv offer code values endpoint rejected",
			args:    []string{"api", "GET", "/v1/subscriptionOfferCodeOneTimeUseCodes/CODE_ID/values"},
			wantErr: "api: GET /v1/subscriptionOfferCodeOneTimeUseCodes/{id}/values responds with text/csv rather than a JSON envelope, which `asc api` cannot pass through; use `asc subscriptions offers offer-codes values` instead",
		},
		{
			name:    "vendor json metrics endpoint rejected",
			args:    []string{"api", "GET", "/v1/apps/123/perfPowerMetrics"},
			wantErr: "api: GET /v1/apps/{id}/perfPowerMetrics responds with application/vnd.apple.xcode-metrics+json rather than a JSON envelope, which `asc api` cannot pass through; use `asc performance metrics list` instead",
		},
		{
			name:    "performance overview vendor endpoint rejected",
			args:    []string{"api", "GET", "/v1/apps/123/performanceOverviews"},
			wantErr: "api: GET /v1/apps/{id}/performanceOverviews responds with application/vnd.apple.xcode-overview+json rather than a JSON envelope, which `asc api` cannot pass through; use `asc performance overview` instead",
		},
		{
			name:    "non json endpoint rejected even with allow-unknown-path",
			args:    []string{"api", "GET", "/v1/diagnosticSignatures/SIG_ID/logs", "--allow-unknown-path"},
			wantErr: "api: GET /v1/diagnosticSignatures/{id}/logs responds with application/vnd.apple.diagnostic-logs+json rather than a JSON envelope, which `asc api` cannot pass through; use `asc performance diagnostics view` instead",
		},
		{
			name:    "table output rejected",
			args:    []string{"api", "GET", "/v1/apps", "--output", "table"},
			wantErr: "--output must be one of: json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before validation finished")
			}
		})
	}
}

func TestAPIUnknownPathAnswersWithWhatToSend(t *testing.T) {
	const allowFlag = "--allow-unknown-path"
	tests := []struct {
		name      string
		args      []string
		want      string
		wantAllow bool
	}{
		{
			name: "misspelled segment keeps the caller's ID",
			args: []string{"api", "GET", "/v1/apps/123/buildz"},
			want: "Did you mean GET /v1/apps/123/builds?",
		},
		{
			name: "other API version",
			args: []string{"api", "GET", "/v1/appAvailabilities/AV1/territoryAvailabilities"},
			want: "Did you mean GET /v2/appAvailabilities/AV1/territoryAvailabilities?",
		},
		{
			name: "expanded resource name",
			args: []string{"api", "GET", "/v1/appInformations/INFO1/appInfoLocalizations"},
			want: "Did you mean GET /v1/appInfos/INFO1/appInfoLocalizations?",
		},
		{
			name:      "indexed resource without a close operation",
			args:      []string{"api", "POST", "/v1/apps/123/requestTestNotification", "--confirm"},
			want:      "Nearest known operations: ",
			wantAllow: true,
		},
		{
			// A child is created at its own top-level path, with a
			// relationship to the parent, not under the parent's path.
			name:      "create under the parent's path",
			args:      []string{"api", "POST", "v1/inAppPurchases/IAP1/inAppPurchaseLocalizations", "--confirm", "--body", `{"data":{"type":"inAppPurchaseLocalizations"}}`},
			want:      `api: POST /v1/inAppPurchases/IAP1/inAppPurchaseLocalizations is not in the embedded schema index. To create one, use POST /v1/inAppPurchaseLocalizations with relationships.inAppPurchaseV2 = {"data":{"type":"inAppPurchases","id":"IAP1"}}. Other known operations: `,
			wantAllow: true,
		},
		{
			name:      "create under the parent's path in another API version",
			args:      []string{"api", "POST", "/v2/inAppPurchases/IAP1/inAppPurchaseLocalizations", "--confirm", "--body", `{"data":{"type":"inAppPurchaseLocalizations"}}`},
			want:      `To create one, use POST /v1/inAppPurchaseLocalizations with relationships.inAppPurchaseV2 = {"data":{"type":"inAppPurchases","id":"IAP1"}}.`,
			wantAllow: true,
		},
		{
			name:      "update under the parent's path",
			args:      []string{"api", "PATCH", "/v1/inAppPurchases/IAP1/inAppPurchaseLocalizations", "--confirm", "--body", `{"data":{"type":"inAppPurchaseLocalizations"}}`},
			want:      "To change one, use PATCH /v1/inAppPurchaseLocalizations/{id} with its ID from GET /v2/inAppPurchases/IAP1/inAppPurchaseLocalizations. Other known operations: ",
			wantAllow: true,
		},
		{
			name:      "delete under the parent's path",
			args:      []string{"api", "DELETE", "/v1/inAppPurchases/IAP1/inAppPurchaseLocalizations", "--confirm"},
			want:      "To delete one, use DELETE /v1/inAppPurchaseLocalizations/{id} with its ID from GET /v2/inAppPurchases/IAP1/inAppPurchaseLocalizations. Other known operations: ",
			wantAllow: true,
		},
		{
			// Apple removes a to-many child through the parent's relationship
			// path; deleting the child's own resource is a different request.
			name:      "remove under the parent's path",
			args:      []string{"api", "DELETE", "/v1/betaTesters/5/betaGroups", "--confirm"},
			want:      `api: DELETE /v1/betaTesters/5/betaGroups is not in the embedded schema index. To remove betaGroups from /v1/betaTesters/5, use DELETE /v1/betaTesters/5/relationships/betaGroups with the resources to remove in the body: {"data":[{"type":"TYPE","id":"ID"}]}. Other known operations: `,
			wantAllow: true,
		},
		{
			name:      "remove testers under an app's path",
			args:      []string{"api", "DELETE", "/v1/apps/123/betaTesters", "--confirm"},
			want:      `api: DELETE /v1/apps/123/betaTesters is not in the embedded schema index. To remove betaTesters from /v1/apps/123, use DELETE /v1/apps/123/relationships/betaTesters with the resources to remove in the body: {"data":[{"type":"TYPE","id":"ID"}]}. Other known operations: `,
			wantAllow: true,
		},
		{
			name:      "remove testers under a group's path",
			args:      []string{"api", "DELETE", "/v1/betaGroups/5/betaTesters", "--confirm"},
			want:      `api: DELETE /v1/betaGroups/5/betaTesters is not in the embedded schema index. To remove betaTesters from /v1/betaGroups/5, use DELETE /v1/betaGroups/5/relationships/betaTesters with the resources to remove in the body: {"data":[{"type":"TYPE","id":"ID"}]}. Other known operations: `,
			wantAllow: true,
		},
		{
			// A POST may add existing children or create a new one, so both
			// are named.
			name:      "add under the parent's path",
			args:      []string{"api", "POST", "/v1/betaGroups/5/betaTesters", "--confirm", "--body", `{"data":[]}`},
			want:      `api: POST /v1/betaGroups/5/betaTesters is not in the embedded schema index. To add existing betaTesters to /v1/betaGroups/5, use POST /v1/betaGroups/5/relationships/betaTesters with the resources to add in the body: {"data":[{"type":"TYPE","id":"ID"}]}. To create one, use POST /v1/betaTesters. Other known operations: `,
			wantAllow: true,
		},
		{
			// A PATCH may mean one child or the whole link, so both are named.
			name:      "update under a parent with a relationship operation",
			args:      []string{"api", "PATCH", "/v1/appScreenshotSets/5/appScreenshots", "--confirm", "--body", `{"data":[]}`},
			want:      "To change one, use PATCH /v1/appScreenshots/{id} with its ID from GET /v1/appScreenshotSets/5/appScreenshots. To change what /v1/appScreenshotSets/5 links to as appScreenshots, use PATCH /v1/appScreenshotSets/5/relationships/appScreenshots. Other known operations: ",
			wantAllow: true,
		},
		{
			// Apple documents this relationship DELETE as deleting the prices,
			// so the answer says nothing about what happens to them.
			name:      "remove prices under a subscription's path",
			args:      []string{"api", "DELETE", "/v1/subscriptions/5/prices", "--confirm"},
			want:      `api: DELETE /v1/subscriptions/5/prices is not in the embedded schema index. To remove prices from /v1/subscriptions/5, use DELETE /v1/subscriptions/5/relationships/prices with the resources to remove in the body: {"data":[{"type":"TYPE","id":"ID"}]}. Other known operations: `,
			wantAllow: true,
		},
		{
			name:      "update a to-one relationship under the parent's path",
			args:      []string{"api", "PATCH", "/v1/appStoreVersions/5/build", "--confirm", "--body", `{"data":{}}`},
			want:      "api: PATCH /v1/appStoreVersions/5/build is not in the embedded schema index. To change what /v1/appStoreVersions/5 links to as build, use PATCH /v1/appStoreVersions/5/relationships/build. Other known operations: ",
			wantAllow: true,
		},
		{
			// The index has no DELETE on a profile's certificates relationship.
			name:      "delete under a parent without a relationship operation",
			args:      []string{"api", "DELETE", "/v1/profiles/5/certificates", "--confirm"},
			want:      "api: DELETE /v1/profiles/5/certificates is not in the embedded schema index. To delete one, use DELETE /v1/certificates/{id} with its ID from GET /v1/profiles/5/certificates. Other known operations: ",
			wantAllow: true,
		},
		{
			name: "resource the index does not know",
			args: []string{"api", "GET", "/v1/agreements"},
			want: "No indexed operation shares its resource",
		},
		{
			name: "misspelled segment without a version",
			args: []string{"api", "GET", "apps/123/buildz"},
			want: "api: GET apps/123/buildz is not in the embedded schema index. Did you mean GET /v1/apps/123/builds?",
		},
		{
			name: "native command for a path without a version",
			args: []string{"api", "GET", "builds/B1/betaGroups"},
			want: "api: GET builds/B1/betaGroups is not in the embedded schema index. Use `asc builds groups list --build-id B1`.",
		},
		{
			// --allow-unknown-path cannot pick a version for the path.
			name:      "nearest operations without a version",
			args:      []string{"api", "DELETE", "apps/1", "--confirm"},
			want:      "Use `asc schema <query>` to inspect operations, or add the /v1/, /v2/, or /v3/ prefix to PATH and pass --allow-unknown-path to send the request anyway.",
			wantAllow: true,
		},
		{
			name: "resource the index does not know, without a version",
			args: []string{"api", "GET", "agreements"},
			want: "api: GET agreements is not in the embedded schema index. No indexed operation shares its resource",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := func() (*asc.Client, error) {
				return nil, errors.New("client factory must not run during validation")
			}
			stdout, stderr, code := runWithDeps(t, client, test.args...)
			assertUsageExitOutput(t, stdout, stderr, test.want)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			// The usage text after the first line lists every flag.
			message, _, _ := strings.Cut(stderr, "\n")
			if got := strings.Contains(message, allowFlag); got != test.wantAllow {
				t.Fatalf("error mentions %s = %v, want %v: %q", allowFlag, got, test.wantAllow, message)
			}
		})
	}
}

var apiNativeCommandAdvice = regexp.MustCompile("Use `(asc [^`]+)`\\.")

// TestAPIUnknownPathNamesTheNativeCommand sends each unindexed request that
// `asc api` answers with a native command and resolves that command, flags
// included, through the real root, so the advice cannot rot into an unknown
// command or flag.
func TestAPIUnknownPathNamesTheNativeCommand(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{path: "/v1/builds/B1/betaGroups", want: "asc builds groups list --build-id B1"},
		{path: "/v1/builds/B1/relationships/betaGroups", want: "asc builds groups list --build-id B1"},
		{path: "/v1/builds/B1/buildBundles", want: "asc builds info --build-id B1 --include buildBundles"},
		{path: "/v1/apps/123/appInfoLocalizations", want: "asc localizations list --app 123 --type app-info"},
		{path: "/v1/apps/123/appStoreLocalizations", want: "asc localizations list --app 123"},
		{path: "/v1/appStoreVersions/V1/localizations", want: "asc localizations list --version V1"},
		{path: "/v1/appStoreVersions/V1/appScreenshotSets", want: "asc screenshots list --version V1"},
		{path: "/v1/ageRatingDeclarations", want: "asc age-rating view --app APP_ID"},
		{path: "/v1/ageRatingDeclarations/D1", want: "asc age-rating view --app APP_ID"},
		{path: "/v1/apps/123/ageRatingDeclaration", want: "asc age-rating view --app 123"},
		{path: "/v1/subscriptionAppStoreReviewScreenshots", want: "asc subscriptions review app-store-screenshot view --subscription-id SUBSCRIPTION_ID"},
		{path: "/v1/reviewSubmissionItems", want: "asc review items list --submission SUBMISSION_ID"},
		{path: "/v1/reviewSubmissionItems/I1", want: "asc review items list --submission SUBMISSION_ID"},
		{path: "/v1/apps/123/prices", want: "asc pricing current --app 123"},
		{path: "/v1/apps/123/appPrices", want: "asc pricing current --app 123"},
		{path: "/v1/apps/123/availableTerritories", want: "asc pricing availability view --app 123"},
		{path: "/v1/apps/123/appDataUsages", want: "asc web privacy pull --app 123"},
		{path: "/v1/apps/123/appStoreVersions/V1/appStoreVersionLocalizations", want: "asc localizations list --version V1"},
		{path: "/v1/appStoreVersions/V1/appStoreVersionLocalizations/L1/appStoreVersionScreenshotSets", want: "asc screenshots list --version-localization L1"},
		{method: "POST", path: "/v1/betaGroups/G1/builds", want: "asc builds add-groups --build-id BUILD_ID --group G1"},
		{method: "POST", path: "/v1/builds/B1/betaGroups", want: "asc builds add-groups --build-id B1 --group GROUP_ID"},
	}
	root := RootCommand("1.2.3")
	for _, test := range tests {
		method := test.method
		if method == "" {
			method = "GET"
		}
		t.Run(method+" "+test.path, func(t *testing.T) {
			client := func() (*asc.Client, error) {
				return nil, errors.New("client factory must not run during validation")
			}
			args := []string{"api", method, test.path}
			if method != "GET" {
				args = append(args, "--confirm", "--body", `{"data":[{"type":"builds","id":"B1"}]}`)
			}
			_, stderr, code := runWithDeps(t, client, args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			match := apiNativeCommandAdvice.FindStringSubmatch(stderr)
			if match == nil || match[1] != test.want {
				t.Fatalf("stderr = %q, want advice `%s`", stderr, test.want)
			}
			if message, _, _ := strings.Cut(stderr, "\n"); strings.Contains(message, "--allow-unknown-path") {
				t.Fatalf("error = %q, want no --allow-unknown-path advice next to a native command", message)
			}

			words := strings.Fields(test.want)[1:]
			command := root
			for len(words) > 0 && !strings.HasPrefix(words[0], "-") {
				command = findSubcommand(command, words[0])
				if command == nil {
					t.Fatalf("`%s` names an unknown command at %q", test.want, words[0])
				}
				words = words[1:]
			}
			if len(command.Subcommands) > 0 {
				t.Fatalf("`%s` names a command group, not a leaf", test.want)
			}
			for _, word := range words {
				if name, ok := strings.CutPrefix(word, "--"); ok && command.FlagSet.Lookup(name) == nil {
					t.Fatalf("`%s` passes --%s, which the command does not define", test.want, name)
				}
			}
		})
	}
}

// TestAPIAcceptsCommonArgumentShapes sends the shapes callers use most often
// besides the canonical `asc api METHOD /vN/PATH --query|--body`: no
// method, no leading slash or no version, and the --data and --url-params
// spellings of --body and --query.
func TestAPIAcceptsCommonArgumentShapes(t *testing.T) {
	const body = `{"data":{"type":"betaGroups","id":"g1","attributes":{"name":"QA"}}}`
	tests := []struct {
		name    string
		args    []string
		matcher asctest.Matcher
		want    string
	}{
		{name: "path without method", args: []string{"api", "/v1/apps/123/appInfos?limit=10"}, matcher: asctest.GET("/v1/apps/123/appInfos", "limit=10"), want: "GET /v1/apps/123/appInfos?limit=10"},
		{name: "lone path without version or slash", args: []string{"api", "apps"}, matcher: asctest.GET("/v1/apps"), want: "GET /v1/apps"},
		{name: "path without leading slash", args: []string{"api", "GET", "v1/apps/123/appInfos"}, matcher: asctest.GET("/v1/apps/123/appInfos"), want: "GET /v1/apps/123/appInfos"},
		{name: "path without version", args: []string{"api", "GET", "apps/123/appInfos"}, matcher: asctest.GET("/v1/apps/123/appInfos"), want: "GET /v1/apps/123/appInfos"},
		{name: "path with slash but without version", args: []string{"api", "GET", "/apps/123"}, matcher: asctest.GET("/v1/apps/123"), want: "GET /v1/apps/123"},
		{name: "path only under v2", args: []string{"api", "GET", "appAvailabilities/123/territoryAvailabilities"}, matcher: asctest.GET("/v2/appAvailabilities/123/territoryAvailabilities"), want: "GET /v2/appAvailabilities/123/territoryAvailabilities"},
		{name: "url-params", args: []string{"api", "GET", "/v1/apps/123/appStoreVersions", "--url-params", "include=build"}, matcher: asctest.GET("/v1/apps/123/appStoreVersions", "include=build"), want: "GET /v1/apps/123/appStoreVersions?include=build"},
		{name: "url-params before and after the positionals", args: []string{"api", "--url-params", "limit=1", "GET", "/v1/apps", "--url-params", "fields[apps]=name"}, matcher: asctest.GET("/v1/apps", "fields[apps]=name", "limit=1"), want: "GET /v1/apps?fields%5Bapps%5D=name&limit=1"},
		{name: "query and url-params after the positionals", args: []string{"api", "GET", "/v1/apps", "--query", "limit=1", "--url-params", "fields[apps]=name"}, matcher: asctest.GET("/v1/apps", "fields[apps]=name", "limit=1"), want: "GET /v1/apps?fields%5Bapps%5D=name&limit=1"},
		{name: "query and url-params before the positionals", args: []string{"api", "--query", "limit=1", "--url-params", "fields[apps]=name", "GET", "/v1/apps"}, matcher: asctest.GET("/v1/apps", "fields[apps]=name", "limit=1"), want: "GET /v1/apps?fields%5Bapps%5D=name&limit=1"},
		{name: "data", args: []string{"api", "PATCH", "/v1/betaGroups/g1", "--confirm", "--data", body}, matcher: asctest.PATCH("/v1/betaGroups/g1").JSONBody(body), want: "PATCH /v1/betaGroups/g1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(test.matcher, asctest.JSON(http.StatusOK, `{"data":[]}`))

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)
			if code != rootcmd.ExitSuccess || stderr != "" || stdout != `{"data":[]}`+"\n" {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q, want success and Apple's envelope", code, stdout, stderr)
			}
			if got := reg.Requests(); len(got) != 1 || got[0].Method+" "+got[0].URL.RequestURI() != test.want {
				t.Fatalf("requests = %v, want %s", got, test.want)
			}
		})
	}
}

func TestAPIGetPrintsEnvelopeUnmodified(t *testing.T) {
	const body = `{"data":[{"type":"apps","id":"123","attributes":{"name":"Demo"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1"},"meta":{"paging":{"total":1,"limit":1}}}`
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "fields[apps]=name", "limit=1"), asctest.JSON(http.StatusOK, body))

	args := []string{"api", "get", "/v1/apps", "--query", "limit=1", "--query", "fields[apps]=name"}
	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != body+"\n" {
		t.Fatalf("stdout = %q, want unmodified envelope", stdout)
	}
	want := []string{"GET /v1/apps?fields%5Bapps%5D=name&limit=1"}
	var got []string
	for _, req := range reg.Requests() {
		got = append(got, req.Method+" "+req.URL.RequestURI())
		if auth := req.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want bearer token", auth)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}

// TestAPIGetAcceptsFullURLWithQueryAndPretty passes a links.next URL back:
// its `cursor` is undocumented in the snapshot but must reach Apple.
func TestAPIGetAcceptsFullURLWithQueryAndPretty(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "cursor=two", "limit=2", "sort=name"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	args := []string{"api", "--pretty", "GET", "https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=2", "--query", "sort=name"}
	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != "{\n  \"data\": []\n}\n" {
		t.Fatalf("stdout = %q, want pretty JSON", stdout)
	}
	if got := reg.Requests(); len(got) != 1 || got[0].Method+" "+got[0].URL.RequestURI() != "GET /v1/apps?cursor=two&limit=2&sort=name" {
		t.Fatalf("requests = %v, want merged query", got)
	}
}

func TestAPIPaginateMergesCollectionPages(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"1"}],"included":[{"type":"builds","id":"b1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?limit=1","next":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2,"limit":1}}}`))
	reg.Register(asctest.GET("/v1/apps", "cursor=two", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"2"}],"included":[{"type":"builds","id":"b1"},{"type":"builds","id":"b2"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps?cursor=two&limit=1"},"meta":{"paging":{"total":2,"limit":1}}}`))

	args := []string{"api", "GET", "/v1/apps", "--query", "limit=1", "--paginate"}
	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if got := reg.Requests(); len(got) != 2 {
		t.Fatalf("requests = %v, want two pages", got)
	}

	var merged struct {
		Data     []map[string]string `json:"data"`
		Included []map[string]string `json:"included"`
		Links    map[string]string   `json:"links"`
		Meta     json.RawMessage     `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &merged); err != nil {
		t.Fatalf("unmarshal merged output: %v\nstdout=%s", err, stdout)
	}
	if len(merged.Data) != 2 || merged.Data[0]["id"] != "1" || merged.Data[1]["id"] != "2" {
		t.Fatalf("data = %v, want both pages in order", merged.Data)
	}
	if len(merged.Included) != 2 || merged.Included[0]["id"] != "b1" || merged.Included[1]["id"] != "b2" {
		t.Fatalf("included = %v, want deduplicated union", merged.Included)
	}
	if _, ok := merged.Links["next"]; ok {
		t.Fatalf("links = %v, want next removed", merged.Links)
	}
	if merged.Links["self"] != "https://api.appstoreconnect.apple.com/v1/apps?limit=1" {
		t.Fatalf("links = %v, want first page self link", merged.Links)
	}
	if string(merged.Meta) != `{"paging":{"total":2,"limit":1}}` {
		t.Fatalf("meta = %s, want first page meta", merged.Meta)
	}
}

func TestAPIPaginateRejectsNonCollectionResponse(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/1"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"1"}}`))

	args := []string{"api", "GET", "/v1/apps/1", "--paginate"}
	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--paginate requires a collection response with a data array") {
		t.Fatalf("stderr = %q, want collection error", stderr)
	}
}

func TestAPIRejectsMalformedJSONResponse(t *testing.T) {
	// `json` is the command's only output format, so a successful response that
	// is not JSON must fail rather than hand machine consumers invalid bytes.
	for _, pretty := range []bool{false, true} {
		name := "plain"
		if pretty {
			name = "pretty"
		}
		t.Run(name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/apps"), asctest.JSON(http.StatusOK, `{"data":[`))

			args := []string{"api", "GET", "/v1/apps"}
			if pretty {
				args = append(args, "--pretty")
			}
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
			if code == rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want a failure", code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "response is not valid JSON") {
				t.Fatalf("stderr = %q, want an invalid JSON diagnostic", stderr)
			}
		})
	}
}

func TestAPIPostSendsBodyWithConfirm(t *testing.T) {
	const requestBody = `{"data":{"type":"betaGroups","attributes":{"name":"QA"},"relationships":{"app":{"data":{"type":"apps","id":"123"}}}}}`
	const responseBody = `{"data":{"type":"betaGroups","id":"g1"}}`
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/betaGroups").JSONBody(requestBody), asctest.JSON(http.StatusCreated, responseBody))

	args := []string{"api", "POST", "/v1/betaGroups", "--confirm", "--body", requestBody}
	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != responseBody+"\n" {
		t.Fatalf("stdout = %q, want response body", stdout)
	}
	sent := reg.Requests()[0]
	if gotBody := string(sent.Body); gotBody != requestBody {
		t.Fatalf("request body = %q, want %q", gotBody, requestBody)
	}
	if gotContentType := sent.Header.Get("Content-Type"); gotContentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", gotContentType)
	}
}

func TestAPIPostPreservesDocumentedMutationURLQuery(t *testing.T) {
	const path = "/v1/appAssetLibraryPlacementOrderingRequests"
	const requestBody = `{"data":{"type":"appAssetLibraryPlacementOrderingRequests","attributes":{"placementGroup":"IPHONE_65"},"relationships":{"orderedPlacements":{"data":[{"type":"appAssetLibraryPlacements","id":"placement-1"}]},"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":"locale-1"}}}}}`
	const responseBody = `{"data":{"type":"appAssetLibraryPlacementOrderingRequests","id":"request-1"},"included":[{"type":"appAssetLibraryPlacements","id":"placement-1"}]}`

	// --allow-unknown-path skips the index checks; it must not refuse a query
	// the operation documents.
	for _, extra := range [][]string{nil, {"--allow-unknown-path"}} {
		t.Run(strings.Join(append([]string{"flags"}, extra...), " "), func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.POST(path, "include=orderedPlacements").JSONBody(requestBody), asctest.JSON(http.StatusCreated, responseBody))

			args := append([]string{"api", "POST", path + "?include=orderedPlacements", "--confirm", "--body", requestBody}, extra...)
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
			if code != rootcmd.ExitSuccess || stdout != responseBody+"\n" || stderr != "" {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q, want success and Apple's envelope", code, stdout, stderr)
			}
			if data := reg.Requests()[0].Body; string(data) != requestBody {
				t.Errorf("request body = %q, want %q", data, requestBody)
			}
		})
	}
}

func TestAPIPatchReadsBodyFromFile(t *testing.T) {
	const requestBody = `{"data":{"type":"apps","id":"123","attributes":{"primaryLocale":"en-US"}}}`
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadPath, []byte(requestBody), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "body-file", args: []string{"api", "PATCH", "/v1/apps/123", "--confirm", "--body-file", payloadPath}},
		{name: "body at-file", args: []string{"api", "PATCH", "/v1/apps/123", "--confirm", "--body", "@" + payloadPath}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.PATCH("/v1/apps/123").JSONBody(requestBody), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123"}}`))

			_, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if gotBody := string(reg.Requests()[0].Body); gotBody != requestBody {
				t.Fatalf("request body = %q, want %q", gotBody, requestBody)
			}
		})
	}
}

func TestAPIDeleteEmptyResponsePrintsNothing(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.DELETE("/v1/betaGroups/g1"), asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "api", "DELETE", "/v1/betaGroups/g1", "--confirm")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want both empty", stdout, stderr)
	}
}

func TestAPIAllowUnknownPathSkipsSchemaCheck(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		query []string
	}{
		{name: "unindexed path", path: "/v1/notYetIndexed"},
		// Apple serves datingAndRelationships; the snapshot predates it.
		{name: "query value newer than the snapshot", path: "/v1/appInfos/INFO1/ageRatingDeclaration", query: []string{"fields[ageRatingDeclarations]=datingAndRelationships"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET(test.path, test.query...), asctest.JSON(http.StatusOK, `{"data":[]}`))

			args := []string{"api", "GET", test.path, "--allow-unknown-path"}
			for _, pair := range test.query {
				args = append(args, "--query", pair)
			}
			_, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitSuccess, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if got := reg.Requests(); len(got) != 1 || got[0].URL.Path != test.path {
				t.Fatalf("requests = %v, want one request to %s", got, test.path)
			}
		})
	}
}

// TestAPIAllowUnknownPathSendsBodyNewerThanSnapshot covers an attribute Apple
// serves that the snapshot predates: ageRatingDeclarations has
// datingAndRelationships live, so the body check must yield to the flag.
func TestAPIAllowUnknownPathSendsBodyNewerThanSnapshot(t *testing.T) {
	const requestBody = `{"data":{"type":"ageRatingDeclarations","id":"D1","attributes":{"datingAndRelationships":"NONE"}}}`
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.PATCH("/v1/ageRatingDeclarations/D1").JSONBody(requestBody), asctest.JSON(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"D1"}}`))

	args := []string{"api", "PATCH", "/v1/ageRatingDeclarations/D1", "--confirm", "--body", requestBody}
	_, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code != rootcmd.ExitUsage || !strings.Contains(stderr, `attribute "datingAndRelationships" is not in AgeRatingDeclarationUpdateRequest`) || !strings.Contains(stderr, "pass --allow-unknown-path to send it anyway") {
		t.Fatalf("exit = %d, stderr = %q, want a usage error that offers --allow-unknown-path", code, stderr)
	}

	_, stderr, code = runWithDeps(t, stubClient(t, reg), append(args, "--allow-unknown-path")...)
	if code != rootcmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q, want the body sent", code, stderr)
	}
	if got := reg.Requests(); len(got) != 1 {
		t.Fatalf("requests = %v, want one PATCH", got)
	}
}

func TestAPIRendersAppleErrorsWithStatusExitCode(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/999"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '999'"}]}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "api", "GET", "/v1/apps/999")
	if code != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitNotFound)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "There is no resource of type 'apps' with id '999'") {
		t.Fatalf("stderr = %q, want Apple error detail", stderr)
	}
}

func TestAPIFullURLStaysOnItsHost(t *testing.T) {
	reg := asctest.NewRegistry(t)

	args := []string{"api", "get", "https://api.enterprise.developer.apple.com/v1/certificates"}
	_, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
	if code == rootcmd.ExitSuccess {
		t.Fatal("api get succeeded with a URL for the other API")
	}
	if !strings.Contains(stderr, "rejected request URL") {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := reg.Requests(); len(got) != 0 {
		t.Fatalf("requests = %v, want none", got)
	}
}
