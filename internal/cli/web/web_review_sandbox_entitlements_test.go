package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type sandboxUsageRow struct {
	entitlement   string
	justification string
}

// sandboxEntitlementsFixture models one macOS version, its review detail, the
// entitlement catalog, and the App Sandbox Information rows App Store Connect
// stores for that review detail.
type sandboxEntitlementsFixture struct {
	asserter       *handlertest.Asserter
	platform       string
	reviewDetailID string
	catalog        []string
	usages         map[string]sandboxUsageRow
	nextID         string
	writes         []string
	// Faults Apple could return: another page, a paging total that disagrees
	// with the rows, rows without their entitlement linkage, and a write that
	// succeeds without changing the stored rows.
	extraPage           bool
	pagingTotalDelta    int
	omitEntitlementLink bool
	dropWrites          bool
}

func newSandboxEntitlementsFixture(t *testing.T) *sandboxEntitlementsFixture {
	t.Helper()
	fixture := &sandboxEntitlementsFixture{
		asserter:       handlertest.New(t),
		platform:       "MAC_OS",
		reviewDetailID: "detail-1",
		catalog:        []string{"com.apple.security.network.client", "com.apple.security.network.server"},
		usages:         map[string]sandboxUsageRow{},
		nextID:         "usage-new",
	}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse fixture URL: %v", err)
	}
	transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
	session := &webcore.AuthSession{Client: &http.Client{Transport: transport}}

	originalResolve := resolveSessionFn
	originalNewClient := newWebClientFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return session, "fixture", nil
	}
	newWebClientFn = webcore.NewClient
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	t.Cleanup(func() {
		resolveSessionFn = originalResolve
		newWebClientFn = originalNewClient
	})
	return fixture
}

func (f *sandboxEntitlementsFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/iris/v1")
	query := r.URL.Query()
	if r.Method != http.MethodGet {
		f.writes = append(f.writes, r.Method+" "+path)
		if r.Header.Get("X-CSRF-ITC") != "[asc-ui]" {
			f.asserter.Respond(w, "%s %s is missing the X-CSRF-ITC header", r.Method, path)
			return
		}
	}
	switch {
	case r.Method == http.MethodGet && path == "/appStoreVersions/version-1":
		if query.Get("include") != "appStoreReviewDetail" || query.Get("fields[appStoreVersions]") != "platform,appStoreReviewDetail" {
			f.asserter.Respond(w, "unexpected version query: %s", r.URL.RawQuery)
			return
		}
		detail := `null`
		if f.reviewDetailID != "" {
			detail = `{"type":"appStoreReviewDetails","id":"` + f.reviewDetailID + `"}`
		}
		_, _ = io.WriteString(w, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"`+f.platform+`"},"relationships":{"appStoreReviewDetail":{"data":`+detail+`}}}}`)
	case r.Method == http.MethodGet && path == "/appSandboxEntitlements":
		if query.Get("limit") != "200" {
			f.asserter.Respond(w, "catalog must request limit=200, got %q", r.URL.RawQuery)
			return
		}
		data := make([]map[string]any, 0, len(f.catalog))
		for _, key := range f.catalog {
			data = append(data, map[string]any{"type": "appSandboxEntitlements", "id": key, "attributes": map[string]any{"optional": true}})
		}
		f.writeCollection(w, data)
	case r.Method == http.MethodGet && path == "/appStoreReviewDetails/detail-1/appSandboxEntitlementUsages":
		if query.Get("include") != "appSandboxEntitlement" || query.Get("limit") != "200" {
			f.asserter.Respond(w, "usage list must request include=appSandboxEntitlement and limit=200, got %q", r.URL.RawQuery)
			return
		}
		ids := make([]string, 0, len(f.usages))
		for id := range f.usages {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		data := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			row := f.usages[id]
			resource := map[string]any{
				"type":       "appSandboxEntitlementUsages",
				"id":         id,
				"attributes": map[string]any{"justification": row.justification},
			}
			if !f.omitEntitlementLink {
				resource["relationships"] = map[string]any{"appSandboxEntitlement": map[string]any{"data": map[string]any{"type": "appSandboxEntitlements", "id": row.entitlement}}}
			}
			data = append(data, resource)
		}
		f.writeCollection(w, data)
	case r.Method == http.MethodPost && path == "/appSandboxEntitlementUsages":
		var payload struct {
			Data struct {
				Type       string `json:"type"`
				ID         string `json:"id"`
				Attributes struct {
					Justification string `json:"justification"`
				} `json:"attributes"`
				Relationships struct {
					AppStoreReviewDetail  struct{ Data resourceRefForTest } `json:"appStoreReviewDetail"`
					AppSandboxEntitlement struct{ Data resourceRefForTest } `json:"appSandboxEntitlement"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			f.asserter.Respond(w, "decode create body: %v", err)
			return
		}
		detail, entitlement := payload.Data.Relationships.AppStoreReviewDetail.Data, payload.Data.Relationships.AppSandboxEntitlement.Data
		if payload.Data.Type != "appSandboxEntitlementUsages" || payload.Data.ID != "" ||
			detail != (resourceRefForTest{Type: "appStoreReviewDetails", ID: f.reviewDetailID}) || entitlement.Type != "appSandboxEntitlements" {
			f.asserter.Respond(w, "unexpected create body: %+v", payload.Data)
			return
		}
		if !f.dropWrites {
			f.usages[f.nextID] = sandboxUsageRow{entitlement: entitlement.ID, justification: payload.Data.Attributes.Justification}
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"appSandboxEntitlementUsages","id":"`+f.nextID+`","attributes":{"justification":`+jsonStringForTest(payload.Data.Attributes.Justification)+`}}}`)
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/appSandboxEntitlementUsages/"):
		id := strings.TrimPrefix(path, "/appSandboxEntitlementUsages/")
		var payload struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			f.asserter.Respond(w, "decode update body: %v", err)
			return
		}
		var resource struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Justification string `json:"justification"`
			} `json:"attributes"`
		}
		encoded, _ := json.Marshal(payload.Data)
		_ = json.Unmarshal(encoded, &resource)
		row, ok := f.usages[id]
		if _, hasRelationships := payload.Data["relationships"]; !ok || hasRelationships || resource.Type != "appSandboxEntitlementUsages" || resource.ID != id {
			f.asserter.Respond(w, "unexpected update of %q: %s", id, encoded)
			return
		}
		if !f.dropWrites {
			row.justification = resource.Attributes.Justification
			f.usages[id] = row
		}
		_, _ = io.WriteString(w, `{"data":{"type":"appSandboxEntitlementUsages","id":"`+id+`","attributes":{"justification":`+jsonStringForTest(resource.Attributes.Justification)+`}}}`)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/appSandboxEntitlementUsages/"):
		id := strings.TrimPrefix(path, "/appSandboxEntitlementUsages/")
		if _, ok := f.usages[id]; !ok {
			f.asserter.Respond(w, "delete of unknown usage %q", id)
			return
		}
		if !f.dropWrites {
			delete(f.usages, id)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		f.asserter.Respond(w, "unexpected request: %s %s", r.Method, r.URL.RequestURI())
	}
}

func (f *sandboxEntitlementsFixture) writeCollection(w http.ResponseWriter, data []map[string]any) {
	links := map[string]any{"self": "https://appstoreconnect.apple.com/iris/v1/collection"}
	if f.extraPage {
		links["next"] = "https://appstoreconnect.apple.com/iris/v1/collection?cursor=Mg"
	}
	encoded, _ := json.Marshal(map[string]any{
		"data":            data,
		"links":           links,
		"meta":            map[string]any{"paging": map[string]any{"total": len(data) + f.pagingTotalDelta, "limit": 200}},
		"unknownTopLevel": map[string]any{"keep": true},
	})
	_, _ = w.Write(encoded)
}

type resourceRefForTest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func jsonStringForTest(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func runSandboxEntitlementsCommand(t *testing.T, newCommand func(*shared.Deps) *ffcli.Command, args ...string) (string, error) {
	t.Helper()
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newCommand(d)
	if err := command.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	var runErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = command.Exec(context.Background(), command.FlagSet.Args())
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	return stdout, runErr
}

func TestWebReviewSandboxEntitlementsSetWritesOnlyWhatChanged(t *testing.T) {
	for _, tc := range []struct {
		name       string
		existing   map[string]sandboxUsageRow
		wantWrites []string
		wantID     string
		wantStatus string
	}{
		{
			name:       "creates a missing row",
			wantWrites: []string{"POST /appSandboxEntitlementUsages"},
			wantID:     "usage-new",
			wantStatus: "created",
		},
		{
			name:       "updates a row with other usage information",
			existing:   map[string]sandboxUsageRow{"usage-1": {entitlement: "com.apple.security.network.server", justification: "Old text"}},
			wantWrites: []string{"PATCH /appSandboxEntitlementUsages/usage-1"},
			wantID:     "usage-1",
			wantStatus: "updated",
		},
		{
			name:       "leaves a matching row unchanged",
			existing:   map[string]sandboxUsageRow{"usage-1": {entitlement: "com.apple.security.network.server", justification: "Listens for paired devices."}},
			wantID:     "usage-1",
			wantStatus: "unchanged",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			for id, row := range tc.existing {
				fixture.usages[id] = row
			}
			stdout, err := runSandboxEntitlementsCommand(t, newWebReviewSandboxEntitlementsSetCommand,
				"--version-id", "version-1",
				"--entitlement", "com.apple.security.network.server",
				"--justification", "  Listens for paired devices.\n",
				"--confirm", "--output", "json")
			if err != nil {
				t.Fatalf("set: %v", err)
			}
			if strings.Join(fixture.writes, ",") != strings.Join(tc.wantWrites, ",") {
				t.Fatalf("writes = %v, want %v", fixture.writes, tc.wantWrites)
			}
			if row := fixture.usages[tc.wantID]; row != (sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "Listens for paired devices."}) {
				t.Fatalf("stored row = %+v, want the entitlement with the trimmed text", row)
			}
			assertSandboxReceiptJSON(t, stdout, map[string]any{
				"versionId":      "version-1",
				"reviewDetailId": "detail-1",
				"entitlement":    "com.apple.security.network.server",
				"usageId":        tc.wantID,
				"justification":  "Listens for paired devices.",
				"changed":        tc.wantStatus != "unchanged",
				"status":         tc.wantStatus,
			})
		})
	}
}

func TestWebReviewSandboxEntitlementsDeleteRemovesOnlyAnExistingRow(t *testing.T) {
	for _, tc := range []struct {
		name        string
		existing    map[string]sandboxUsageRow
		wantWrites  []string
		wantReceipt map[string]any
	}{
		{
			name:       "deletes the row of the entitlement",
			existing:   map[string]sandboxUsageRow{"usage-1": {entitlement: "com.apple.security.network.server", justification: "Text"}, "usage-2": {entitlement: "com.apple.security.network.client", justification: "Other"}},
			wantWrites: []string{"DELETE /appSandboxEntitlementUsages/usage-1"},
			wantReceipt: map[string]any{
				"versionId": "version-1", "reviewDetailId": "detail-1", "entitlement": "com.apple.security.network.server",
				"usageId": "usage-1", "changed": true, "status": "deleted",
			},
		},
		{
			name:     "reports an absent row as unchanged",
			existing: map[string]sandboxUsageRow{"usage-2": {entitlement: "com.apple.security.network.client", justification: "Other"}},
			wantReceipt: map[string]any{
				"versionId": "version-1", "reviewDetailId": "detail-1", "entitlement": "com.apple.security.network.server",
				"changed": false, "status": "unchanged",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			for id, row := range tc.existing {
				fixture.usages[id] = row
			}
			stdout, err := runSandboxEntitlementsCommand(t, newWebReviewSandboxEntitlementsDeleteCommand,
				"--version-id", "version-1", "--entitlement", "com.apple.security.network.server", "--confirm", "--output", "json")
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if strings.Join(fixture.writes, ",") != strings.Join(tc.wantWrites, ",") {
				t.Fatalf("writes = %v, want %v", fixture.writes, tc.wantWrites)
			}
			if _, kept := fixture.usages["usage-2"]; !kept {
				t.Fatal("the row of another entitlement was removed")
			}
			assertSandboxReceiptJSON(t, stdout, tc.wantReceipt)
		})
	}
}

func TestWebReviewSandboxEntitlementsRefusesBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prepare     func(*sandboxEntitlementsFixture)
		entitlement string
		wantErr     string
	}{
		{
			name:    "a version of another platform",
			prepare: func(f *sandboxEntitlementsFixture) { f.platform = "IOS" },
			wantErr: "applies only to MAC_OS versions",
		},
		{
			name:    "a version without App Review details",
			prepare: func(f *sandboxEntitlementsFixture) { f.reviewDetailID = "" },
			wantErr: "asc review details-create --version-id version-1 --contact-first-name NAME --contact-phone PHONE",
		},
		{
			name:        "an entitlement key outside the catalog",
			prepare:     func(*sandboxEntitlementsFixture) {},
			entitlement: "com.apple.security.network.typo",
			wantErr:     "not in the App Sandbox catalog",
		},
		{
			name:    "a collection Apple reports as partial",
			prepare: func(f *sandboxEntitlementsFixture) { f.extraPage = true },
			wantErr: "refusing to use a partial collection",
		},
		{
			name: "several rows for the same entitlement",
			prepare: func(f *sandboxEntitlementsFixture) {
				f.usages["usage-1"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "A"}
				f.usages["usage-2"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "B"}
			},
			wantErr: "several App Sandbox Information rows",
		},
		{
			name: "rows without their entitlement key",
			prepare: func(f *sandboxEntitlementsFixture) {
				f.usages["usage-1"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "A"}
				f.omitEntitlementLink = true
			},
			wantErr: "has no entitlement key",
		},
		{
			name:    "a paging total larger than the rows returned",
			prepare: func(f *sandboxEntitlementsFixture) { f.pagingTotalDelta = 1 },
			wantErr: "paging total",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			tc.prepare(fixture)
			key := tc.entitlement
			if key == "" {
				key = "com.apple.security.network.server"
			}
			stdout, err := runSandboxEntitlementsCommand(t, newWebReviewSandboxEntitlementsSetCommand,
				"--version-id", "version-1", "--entitlement", key, "--justification", "Text", "--confirm", "--output", "json")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
			}
			if stdout != "" || len(fixture.writes) != 0 {
				t.Fatalf("stdout = %q, writes = %v; want no receipt and no write", stdout, fixture.writes)
			}
		})
	}
}

func TestWebReviewSandboxEntitlementsReportsAWriteAppleDidNotKeep(t *testing.T) {
	for _, tc := range []struct {
		name      string
		existing  map[string]sandboxUsageRow
		command   func(*shared.Deps) *ffcli.Command
		args      []string
		wantWrite string
		wantErr   string
	}{
		{
			name:      "set creating a row",
			command:   newWebReviewSandboxEntitlementsSetCommand,
			args:      []string{"--justification", "Text"},
			wantWrite: "POST /appSandboxEntitlementUsages",
			wantErr:   "rerun the command to reconcile",
		},
		{
			name:      "set updating a row",
			existing:  map[string]sandboxUsageRow{"usage-1": {entitlement: "com.apple.security.network.server", justification: "Old text"}},
			command:   newWebReviewSandboxEntitlementsSetCommand,
			args:      []string{"--justification", "Text"},
			wantWrite: "PATCH /appSandboxEntitlementUsages/usage-1",
			wantErr:   "rerun the command to reconcile",
		},
		{
			name:      "delete",
			existing:  map[string]sandboxUsageRow{"usage-1": {entitlement: "com.apple.security.network.server", justification: "Text"}},
			command:   newWebReviewSandboxEntitlementsDeleteCommand,
			wantWrite: "DELETE /appSandboxEntitlementUsages/usage-1",
			wantErr:   "still present",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			fixture.dropWrites = true
			for id, row := range tc.existing {
				fixture.usages[id] = row
			}
			args := append([]string{"--version-id", "version-1", "--entitlement", "com.apple.security.network.server", "--confirm", "--output", "json"}, tc.args...)
			stdout, err := runSandboxEntitlementsCommand(t, tc.command, args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
			}
			if stdout != "" || strings.Join(fixture.writes, ",") != tc.wantWrite {
				t.Fatalf("stdout = %q, writes = %v; want no receipt and exactly %q", stdout, fixture.writes, tc.wantWrite)
			}
		})
	}
}

// assertSandboxReceiptJSON compares the receipt by its JSON keys, which are
// part of the command contract.
func assertSandboxReceiptJSON(t *testing.T, stdout string, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt = %v, want %v", got, want)
	}
}

func TestWebReviewSandboxEntitlementsReadsPreserveAppleEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		command   func(*shared.Deps) *ffcli.Command
		args      []string
		tableWant []string
	}{
		{
			name:      "list",
			command:   newWebReviewSandboxEntitlementsListCommand,
			args:      []string{"--version-id", "version-1"},
			tableWant: []string{"Entitlement", "Justification", "Usage ID", "com.apple.security.network.server", "Listens for paired devices.", "usage-1"},
		},
		{
			name:      "catalog",
			command:   newWebReviewSandboxEntitlementsCatalogCommand,
			tableWant: []string{"Entitlement", "Optional", "com.apple.security.network.client", "true"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			fixture.usages["usage-1"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "Listens for paired devices."}

			stdout, err := runSandboxEntitlementsCommand(t, tc.command, append(tc.args, "--output", "json")...)
			if err != nil {
				t.Fatalf("json read: %v", err)
			}
			var envelope struct {
				Data            []json.RawMessage `json:"data"`
				UnknownTopLevel map[string]bool   `json:"unknownTopLevel"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || len(envelope.Data) == 0 || !envelope.UnknownTopLevel["keep"] {
				t.Fatalf("json output does not preserve Apple's envelope: %q (%v)", stdout, err)
			}

			stdout, err = runSandboxEntitlementsCommand(t, tc.command, append(tc.args, "--output", "table")...)
			if err != nil {
				t.Fatalf("table read: %v", err)
			}
			for _, want := range tc.tableWant {
				if !strings.Contains(stdout, want) {
					t.Fatalf("table output missing %q: %q", want, stdout)
				}
			}
			if len(fixture.writes) != 0 {
				t.Fatalf("read commands wrote: %v", fixture.writes)
			}
		})
	}
}

func writeSandboxFileForTest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sandbox.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write sandbox file: %v", err)
	}
	return path
}

func TestWebReviewSandboxEntitlementsApplyWritesOnlyWhatChanged(t *testing.T) {
	fixture := newSandboxEntitlementsFixture(t)
	fixture.catalog = append(fixture.catalog, "com.apple.security.device.microphone")
	fixture.usages["usage-1"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "Old text"}
	fixture.usages["usage-2"] = sandboxUsageRow{entitlement: "com.apple.security.network.client", justification: "Downloads data."}
	path := writeSandboxFileForTest(t, `{
		"com.apple.security.network.client": "Downloads data.",
		"com.apple.security.network.server": " Listens for paired devices.\n",
		"com.apple.security.device.microphone": "Hears spoken moves."
	}`)

	stdout, err := runSandboxEntitlementsCommand(t, newWebReviewSandboxEntitlementsApplyCommand,
		"--version-id", "version-1", "--file", path, "--confirm", "--output", "json")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	wantWrites := []string{"POST /appSandboxEntitlementUsages", "PATCH /appSandboxEntitlementUsages/usage-1"}
	if !reflect.DeepEqual(fixture.writes, wantWrites) {
		t.Fatalf("writes = %v, want %v", fixture.writes, wantWrites)
	}
	wantRows := map[string]sandboxUsageRow{
		"usage-1":   {entitlement: "com.apple.security.network.server", justification: "Listens for paired devices."},
		"usage-2":   {entitlement: "com.apple.security.network.client", justification: "Downloads data."},
		"usage-new": {entitlement: "com.apple.security.device.microphone", justification: "Hears spoken moves."},
	}
	if !reflect.DeepEqual(fixture.usages, wantRows) {
		t.Fatalf("stored rows = %+v, want %+v", fixture.usages, wantRows)
	}
	assertSandboxReceiptJSON(t, stdout, map[string]any{
		"versionId":      "version-1",
		"reviewDetailId": "detail-1",
		"changed":        true,
		"rows": []any{
			map[string]any{"entitlement": "com.apple.security.device.microphone", "usageId": "usage-new", "justification": "Hears spoken moves.", "status": "created"},
			map[string]any{"entitlement": "com.apple.security.network.client", "usageId": "usage-2", "justification": "Downloads data.", "status": "unchanged"},
			map[string]any{"entitlement": "com.apple.security.network.server", "usageId": "usage-1", "justification": "Listens for paired devices.", "status": "updated"},
		},
	})
}

func TestWebReviewSandboxEntitlementsApplyRefusals(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prepare    func(*sandboxEntitlementsFixture)
		file       string
		noFile     bool
		wantErr    string
		wantWrites []string
	}{
		{name: "no file", noFile: true, wantErr: "--file is required"},
		{name: "an empty file", file: `{}`, wantErr: "at least one entitlement"},
		{name: "a value that is not text", file: `{"com.apple.security.network.server": true}`, wantErr: "invalid --file"},
		{name: "empty usage information", file: `{"com.apple.security.network.server": "  "}`, wantErr: "usage information for com.apple.security.network.server is empty"},
		{
			name: "a key outside the catalog beside a valid change",
			prepare: func(f *sandboxEntitlementsFixture) {
				f.usages["usage-1"] = sandboxUsageRow{entitlement: "com.apple.security.network.server", justification: "Old"}
			},
			file:    `{"com.apple.security.network.server": "New", "com.apple.security.network.typo": "Text"}`,
			wantErr: "not in the App Sandbox catalog",
		},
		{
			name:       "a write Apple did not keep",
			prepare:    func(f *sandboxEntitlementsFixture) { f.dropWrites = true },
			file:       `{"com.apple.security.network.server": "Text"}`,
			wantErr:    "rerun the command to reconcile",
			wantWrites: []string{"POST /appSandboxEntitlementUsages"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newSandboxEntitlementsFixture(t)
			if tc.prepare != nil {
				tc.prepare(fixture)
			}
			args := []string{"--version-id", "version-1", "--confirm", "--output", "json"}
			if !tc.noFile {
				args = append(args, "--file", writeSandboxFileForTest(t, tc.file))
			}
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebReviewSandboxEntitlementsApplyCommand(d)
			if err := command.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			var err error
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				err = command.Exec(context.Background(), nil)
			})
			// Usage errors are printed to stderr; other errors are returned.
			if err == nil || !strings.Contains(err.Error()+stderr, tc.wantErr) {
				t.Fatalf("error = %v, stderr = %q, want them to mention %q", err, stderr, tc.wantErr)
			}
			if stdout != "" || strings.Join(fixture.writes, ",") != strings.Join(tc.wantWrites, ",") {
				t.Fatalf("stdout = %q, writes = %v; want no receipt and writes %v", stdout, fixture.writes, tc.wantWrites)
			}
		})
	}
}
