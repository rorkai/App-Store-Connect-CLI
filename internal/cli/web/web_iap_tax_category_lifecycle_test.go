package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// These tests exercise the command against a real HTTP server through the
// same web client used in production. The transport only rewrites Apple's
// host to the local server, leaving paths, query strings, methods, and bodies
// observable by the fake provider.
func TestWebIAPTaxCategorySetLifecycleUsesOneWriteAndReadback(t *testing.T) {
	tests := []struct {
		name                   string
		initialConfigured      bool
		initialConditions      []string
		args                   []string
		wantMethod             string
		wantChanged            bool
		wantConditions         []string
		wantPostRead           bool
		wantCategory           string
		wantWriteCount         int
		wantConditionInPayload []string
	}{
		{
			name:                   "inherited create",
			args:                   []string{"--iap", "iap-1", "--category", "C003", "--condition", "C003-Q01", "--confirm", "--output", "json"},
			wantMethod:             http.MethodPost,
			wantChanged:            true,
			wantConditions:         []string{"C003-Q01"},
			wantPostRead:           true,
			wantCategory:           "C003",
			wantWriteCount:         1,
			wantConditionInPayload: []string{"C003-Q01"},
		},
		{
			name:              "configured identical no-op",
			initialConfigured: true,
			initialConditions: []string{"C003-Q01"},
			args:              []string{"--iap", "iap-1", "--category", "C003", "--condition", "C003-Q01", "--confirm", "--output", "json"},
			wantChanged:       false,
			wantConditions:    []string{"C003-Q01"},
			wantCategory:      "C003",
			wantWriteCount:    0,
		},
		{
			name:                   "changed conditions clear explicitly",
			initialConfigured:      true,
			initialConditions:      []string{"C003-Q01"},
			args:                   []string{"--iap", "iap-1", "--category", "C003", "--confirm", "--output", "json"},
			wantMethod:             http.MethodPatch,
			wantChanged:            true,
			wantConditions:         []string{},
			wantPostRead:           true,
			wantCategory:           "C003",
			wantWriteCount:         1,
			wantConditionInPayload: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := &iapTaxCategoryCLIHTTPFixture{
				configured:   tt.initialConfigured,
				categoryID:   "C003",
				conditionIDs: append([]string(nil), tt.initialConditions...),
				recordID:     "tax-1",
			}
			setupIAPTaxCategoryCLIHTTP(t, fixture)

			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebIAPTaxCategorySetCommand(d)
			if err := command.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse command: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("execute command: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}

			var receipt struct {
				IAPID        string   `json:"iapId"`
				CategoryID   string   `json:"categoryId"`
				ConditionIDs []string `json:"conditionIds"`
				Changed      bool     `json:"changed"`
				Verified     bool     `json:"verified"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("decode receipt %q: %v", stdout, err)
			}
			if receipt.IAPID != "iap-1" || receipt.CategoryID != tt.wantCategory || receipt.Changed != tt.wantChanged || !receipt.Verified {
				t.Fatalf("unexpected receipt: %+v", receipt)
			}
			if !sameWebTaxConditionSet(receipt.ConditionIDs, tt.wantConditions) {
				t.Fatalf("receipt conditions = %v, want %v", receipt.ConditionIDs, tt.wantConditions)
			}
			if fixture.writeCount != tt.wantWriteCount {
				t.Fatalf("write count = %d, want %d; requests = %v", fixture.writeCount, tt.wantWriteCount, fixture.requests)
			}
			if tt.wantMethod != "" {
				if fixture.writeMethods[0] != tt.wantMethod {
					t.Fatalf("write method = %q, want %q", fixture.writeMethods[0], tt.wantMethod)
				}
				if !sameWebTaxConditionSet(fixture.writeConditions[0], tt.wantConditionInPayload) {
					t.Fatalf("write conditions = %v, want %v", fixture.writeConditions[0], tt.wantConditionInPayload)
				}
			}
			if fixture.postReadCount != boolToInt(tt.wantPostRead) {
				t.Fatalf("post-read count = %d, want %t", fixture.postReadCount, tt.wantPostRead)
			}
			if fixture.configured != true || fixture.categoryID != tt.wantCategory || !sameWebTaxConditionSet(fixture.conditionIDs, tt.wantConditions) {
				t.Fatalf("fixture final state configured=%t category=%q conditions=%v", fixture.configured, fixture.categoryID, fixture.conditionIDs)
			}
		})
	}
}

func TestWebIAPTaxCategoryResetLifecycleDeletesConfiguredRecordAndVerifiesNull(t *testing.T) {
	fixture := &iapTaxCategoryCLIHTTPFixture{
		configured:   true,
		categoryID:   "C003",
		conditionIDs: []string{"C003-Q01"},
		recordID:     "tax-1",
	}
	setupIAPTaxCategoryCLIHTTP(t, fixture)

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebIAPTaxCategoryResetCommand(d)
	if err := command.FlagSet.Parse([]string{"--iap", "iap-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("execute command: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var receipt struct {
		IAPID    string `json:"iapId"`
		Changed  bool   `json:"changed"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.IAPID != "iap-1" || !receipt.Changed || !receipt.Verified {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if fixture.writeCount != 1 || fixture.writeMethods[0] != http.MethodDelete {
		t.Fatalf("writes = %v, want one DELETE", fixture.writeMethods)
	}
	if fixture.postReadCount != 1 {
		t.Fatalf("post-read count = %d, want 1", fixture.postReadCount)
	}
	if fixture.configured {
		t.Fatal("expected reset to leave an explicit null relationship")
	}
}

func TestWebIAPTaxCategoryViewRendersInheritedStateWithoutInferringCategory(t *testing.T) {
	fixture := &iapTaxCategoryCLIHTTPFixture{
		categoryID: "C003",
		recordID:   "tax-1",
	}
	setupIAPTaxCategoryCLIHTTP(t, fixture)

	for _, output := range []string{"json", "table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebIAPTaxCategoryViewCommand(d)
			if err := command.FlagSet.Parse([]string{"--iap", "iap-1", "--output", output}); err != nil {
				t.Fatalf("parse command: %v", err)
			}
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("execute command: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}

			if output == "json" {
				var envelope struct {
					Data struct {
						Relationships map[string]struct {
							Data json.RawMessage `json:"data"`
						} `json:"relationships"`
					} `json:"data"`
					Meta map[string]any `json:"meta"`
				}
				if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
					t.Fatalf("decode raw envelope %q: %v", stdout, err)
				}
				linkage := envelope.Data.Relationships["inAppPurchaseTaxCategoryInfo"].Data
				if string(linkage) != "null" {
					t.Fatalf("tax-category linkage = %s, want explicit null", linkage)
				}
				if envelope.Meta["fixture"] != true {
					t.Fatalf("raw envelope meta = %#v, want preserved fixture marker", envelope.Meta)
				}
				return
			}
			if !strings.Contains(stdout, iapTaxCategoryInheritedLabel) {
				t.Fatalf("%s output = %q, want inherited label", output, stdout)
			}
			if strings.Contains(stdout, "C003") {
				t.Fatalf("%s output = %q, inferred category ID", output, stdout)
			}
		})
	}
}

func TestWebIAPTaxCategorySetRejectsInvalidCatalogSelectionBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		productType   string
		conditionArgs []string
		wantError     string
	}{
		{name: "wrong product type", productType: "APPLICATION", wantError: "ADDON tax catalog"},
		{name: "incompatible condition", productType: "ADDON", conditionArgs: []string{"--condition", "C003-Q99"}, wantError: "not compatible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &iapTaxCategoryCLIHTTPFixture{
				categoryProductType: tc.productType,
				categoryID:          "C003",
				recordID:            "tax-1",
			}
			setupIAPTaxCategoryCLIHTTP(t, fixture)

			args := []string{"--iap", "iap-1", "--category", "C003"}
			args = append(args, tc.conditionArgs...)
			args = append(args, "--confirm", "--output", "json")
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebIAPTaxCategorySetCommand(d)
			if err := command.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse command: %v", err)
			}
			var commandErr error
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				commandErr = command.Exec(context.Background(), nil)
			})
			if commandErr == nil {
				t.Fatalf("error = nil, want %q", tc.wantError)
			}
			if !strings.Contains(stderr, tc.wantError) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.wantError)
			}
			if fixture.writeCount != 0 {
				t.Fatalf("write count = %d, want 0; methods = %v", fixture.writeCount, fixture.writeMethods)
			}
		})
	}
}

func TestWebIAPTaxCategorySetDoesNotRetryMismatchedPostRead(t *testing.T) {
	fixture := &iapTaxCategoryCLIHTTPFixture{
		categoryID:          "C003",
		recordID:            "tax-1",
		mismatchPostRead:    true,
		categoryProductType: "ADDON",
	}
	setupIAPTaxCategoryCLIHTTP(t, fixture)

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebIAPTaxCategorySetCommand(d)
	if err := command.FlagSet.Parse([]string{"--iap", "iap-1", "--category", "C003", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse command: %v", err)
	}
	var commandErr error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		commandErr = command.Exec(context.Background(), nil)
	})
	if commandErr == nil || !strings.Contains(commandErr.Error(), "do not retry automatically") {
		t.Fatalf("error = %v, want an explicit non-retry verification error", commandErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for direct command execution", stderr)
	}
	if fixture.writeCount != 1 || fixture.writeMethods[0] != http.MethodPost {
		t.Fatalf("writes = %v, want one POST", fixture.writeMethods)
	}
	if fixture.postReadCount != 1 {
		t.Fatalf("post-read count = %d, want 1", fixture.postReadCount)
	}
}

type iapTaxCategoryCLIHTTPFixture struct {
	configured          bool
	categoryID          string
	categoryProductType string
	conditionIDs        []string
	recordID            string
	mismatchPostRead    bool
	postReadCount       int
	writeCount          int
	writeMethods        []string
	writeConditions     [][]string
	requests            []string
}

func setupIAPTaxCategoryCLIHTTP(t *testing.T, fixture *iapTaxCategoryCLIHTTPFixture) {
	t.Helper()
	if fixture.categoryID == "" {
		fixture.categoryID = "C003"
	}
	if fixture.categoryProductType == "" {
		fixture.categoryProductType = "ADDON"
	}
	if fixture.recordID == "" {
		fixture.recordID = "tax-1"
	}

	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		requestURL := *req.URL
		requestURL.Scheme = target.Scheme
		requestURL.Host = target.Host
		clone.URL = &requestURL
		return server.Client().Transport.RoundTrip(clone)
	})

	originalResolveSession := resolveSessionFn
	originalNewWebClient := newWebClientFn
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}
	newWebClientFn = webcore.NewClient
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewWebClient
	})
}

func (f *iapTaxCategoryCLIHTTPFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	switch r.URL.Path {
	case "/iris/v1/taxCategories":
		if r.Method != http.MethodGet {
			f.writeFixtureError(w, http.StatusMethodNotAllowed, "catalog method")
			return
		}
		if got := r.URL.Query().Get("filter[productType]"); got != "ADDON" {
			f.writeFixtureError(w, http.StatusBadRequest, "catalog product type")
			return
		}
		f.writeCatalog(w)
	case "/iris/v2/inAppPurchases/iap-1":
		if r.Method != http.MethodGet {
			f.writeFixtureError(w, http.StatusMethodNotAllowed, "discovery method")
			return
		}
		if f.writeCount > 0 {
			// Every post-write verification starts with a fresh discovery read.
			f.postReadCount++
		}
		f.writeDiscovery(w)
	case "/iris/v1/inAppPurchaseTaxCategoryInfos", "/iris/v1/inAppPurchaseTaxCategoryInfos/tax-1":
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path != "/iris/v1/inAppPurchaseTaxCategoryInfos/"+f.recordID {
				f.writeFixtureError(w, http.StatusNotFound, "record id path")
				return
			}
			f.writeRecord(w)
		case http.MethodPost, http.MethodPatch:
			if r.Method == http.MethodPost && r.URL.Path != "/iris/v1/inAppPurchaseTaxCategoryInfos" {
				f.writeFixtureError(w, http.StatusNotFound, "create path")
				return
			}
			if r.Method == http.MethodPatch && r.URL.Path != "/iris/v1/inAppPurchaseTaxCategoryInfos/"+f.recordID {
				f.writeFixtureError(w, http.StatusNotFound, "update path")
				return
			}
			if err := f.applyWrite(r); err != nil {
				f.writeFixtureError(w, http.StatusBadRequest, err.Error())
				return
			}
			f.writeCount++
			f.writeMethods = append(f.writeMethods, r.Method)
			status := http.StatusOK
			if r.Method == http.MethodPost {
				status = http.StatusCreated
			}
			writeJSON(w, status, fmt.Sprintf(`{"data":{"type":"inAppPurchaseTaxCategoryInfos","id":%q}}`, f.recordID))
		case http.MethodDelete:
			f.configured = false
			f.categoryID = ""
			f.conditionIDs = nil
			f.writeCount++
			f.writeMethods = append(f.writeMethods, r.Method)
			w.WriteHeader(http.StatusNoContent)
		default:
			f.writeFixtureError(w, http.StatusMethodNotAllowed, "record method")
		}
	default:
		f.writeFixtureError(w, http.StatusNotFound, "unexpected path")
	}
}

func (f *iapTaxCategoryCLIHTTPFixture) writeCatalog(w http.ResponseWriter) {
	conditionID := "C003-Q01"
	if f.categoryID != "C003" {
		conditionID = f.categoryID + "-Q01"
	}
	writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"taxCategories","id":%q,"attributes":{"name":"Digital Goods","productType":%q,"subcategoryRequired":false},"relationships":{"conditions":{"data":[{"type":"taxConditions","id":%q}]}}}],"included":[{"type":"taxConditions","id":%q,"attributes":{"name":"Digital goods condition"}}],"meta":{"fixture":true}}`, f.categoryID, f.categoryProductType, conditionID, conditionID))
}

func (f *iapTaxCategoryCLIHTTPFixture) writeDiscovery(w http.ResponseWriter) {
	linkage := "null"
	if f.configured {
		linkage = fmt.Sprintf(`{"type":"inAppPurchaseTaxCategoryInfos","id":%q}`, f.recordID)
	}
	writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"inAppPurchases","id":"iap-1","relationships":{"inAppPurchaseTaxCategoryInfo":{"data":%s}}},"meta":{"fixture":true}}`, linkage))
}

func (f *iapTaxCategoryCLIHTTPFixture) writeRecord(w http.ResponseWriter) {
	categoryID := f.categoryID
	if f.mismatchPostRead && f.writeCount > 0 {
		categoryID = "C010"
	}
	conditionData := make([]string, 0, len(f.conditionIDs))
	for _, conditionID := range f.conditionIDs {
		conditionData = append(conditionData, fmt.Sprintf(`{"type":"taxConditions","id":%q}`, conditionID))
	}
	writeJSON(w, http.StatusOK, []byte(fmt.Sprintf(`{"data":{"type":"inAppPurchaseTaxCategoryInfos","id":%q,"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}},"category":{"data":{"type":"taxCategories","id":%q}},"enabledConditions":{"data":[%s]}}},"included":[{"type":"taxCategories","id":%q,"attributes":{"name":"Digital Goods"}}],"meta":{"fixture":true}}`, f.recordID, categoryID, strings.Join(conditionData, ","), categoryID)))
}

func (f *iapTaxCategoryCLIHTTPFixture) applyWrite(r *http.Request) error {
	var envelope struct {
		Data struct {
			ID            string `json:"id"`
			Type          string `json:"type"`
			Relationships struct {
				Category struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"category"`
				EnabledConditions struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"enabledConditions"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode write: %w", err)
	}
	if envelope.Data.Type != "inAppPurchaseTaxCategoryInfos" || envelope.Data.Relationships.Category.Data.ID == "" {
		return fmt.Errorf("invalid write payload: %+v", envelope.Data)
	}
	if r.Method == http.MethodPost && envelope.Data.ID != "" {
		return fmt.Errorf("create unexpectedly included id %q", envelope.Data.ID)
	}
	if r.Method == http.MethodPatch && envelope.Data.ID != f.recordID {
		return fmt.Errorf("update id = %q, want %q", envelope.Data.ID, f.recordID)
	}
	conditions := make([]string, 0, len(envelope.Data.Relationships.EnabledConditions.Data))
	for _, condition := range envelope.Data.Relationships.EnabledConditions.Data {
		conditions = append(conditions, condition.ID)
	}
	f.writeConditions = append(f.writeConditions, conditions)
	f.categoryID = envelope.Data.Relationships.Category.Data.ID
	f.conditionIDs = conditions
	f.configured = true
	return nil
}

func (f *iapTaxCategoryCLIHTTPFixture) writeFixtureError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, fmt.Sprintf(`{"errors":[{"detail":%q}]}`, detail))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	switch value := body.(type) {
	case string:
		_, _ = io.WriteString(w, value)
	case []byte:
		_, _ = w.Write(value)
	default:
		_ = json.NewEncoder(w).Encode(value)
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
