package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsTaxCategorySetRequiresConfirmBeforeResolvingSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run before --confirm validation")
		return nil, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--category", "cat-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %v (stderr=%q)", err, stderr)
	}
}

func TestNormalizeWebTaxConditionIDsRejectsExplicitBlank(t *testing.T) {
	if _, err := normalizeWebTaxConditionIDs([]string{"cond-1", " "}); err == nil || !strings.Contains(err.Error(), "--condition must not be empty") {
		t.Fatalf("expected explicit blank condition error, got %v", err)
	}
}

func TestWebAppsTaxCategoryListPreservesRawJSONEnvelope(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalNewClient := newWebClientFn
	originalList := listWebAppTaxCategoriesFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewClient
		listWebAppTaxCategoriesFn = originalList
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return webcore.TaxCategoryCatalog{
			Categories: []webcore.TaxCategory{{ID: "cat-1", Name: "App Store Software"}},
			Conditions: []webcore.TaxCondition{{ID: "condition-1", Name: "Digital goods"}},
			Raw: json.RawMessage(`{
				"data":[{"type":"taxCategories","id":"cat-1","attributes":{"name":"App Store Software"}}],
				"included":[{"type":"taxConditions","id":"condition-1","attributes":{"name":"Digital goods"}}],
				"links":{"self":"/taxCategories","next":null},
				"meta":{"paging":{"total":1}},
				"unknownTopLevel":{"preserve":"this"}
			}`),
		}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategoryListCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	for _, member := range []string{"data", "included", "links", "meta", "unknownTopLevel"} {
		if _, ok := envelope[member]; !ok {
			t.Fatalf("JSON output omitted Apple envelope member %q: %s", member, stdout)
		}
	}
	if got := string(envelope["unknownTopLevel"]); got != `{"preserve":"this"}` {
		t.Fatalf("unknown top-level member = %s, want preserved object", got)
	}
	if _, flattened := envelope["categories"]; flattened {
		t.Fatalf("JSON output flattened Apple's envelope into categories: %s", stdout)
	}
}

func TestWebAppsTaxCategoryCatalogRendersInheritedConditions(t *testing.T) {
	catalog := webcore.TaxCategoryCatalog{
		Categories: []webcore.TaxCategory{
			{
				ID:            "cat-parent",
				Name:          "Books",
				Conditions:    []webcore.TaxCategoryReference{{ID: "cond-parent"}, {ID: "cond-shared"}},
				Subcategories: []webcore.TaxCategoryReference{{ID: "cat-child"}},
			},
			{
				ID:         "cat-child",
				Name:       "Books: Fiction",
				Conditions: []webcore.TaxCategoryReference{{ID: "cond-child"}, {ID: "cond-shared"}},
			},
		},
	}

	for _, output := range []string{"table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			var renderErr error
			d, depsStdout, depsStderr := newWebTestDeps()
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				renderErr = printWebAppTaxCategoryCatalog(d, catalog, output, false)
			})
			if renderErr != nil {
				t.Fatalf("render error: %v", renderErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, "cond-child,cond-shared,cond-parent") {
				t.Fatalf("expected direct and inherited conditions in %s output, got %q", output, stdout)
			}
			if strings.Count(stdout, "cond-child,cond-shared,cond-parent") != 1 {
				t.Fatalf("expected shared condition to render once for the child in %s output, got %q", output, stdout)
			}
		})
	}
}

func TestWebAppsTaxCategorySetRejectsCategoryRequiringSubcategory(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalNewClient := newWebClientFn
	originalList := listWebAppTaxCategoriesFn
	originalGet := getWebAppTaxCategoryFn
	originalSave := saveWebAppTaxCategoryFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewClient
		listWebAppTaxCategoriesFn = originalList
		getWebAppTaxCategoryFn = originalGet
		saveWebAppTaxCategoryFn = originalSave
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return webcore.TaxCategoryCatalog{
			Categories: []webcore.TaxCategory{
				{
					ID:                  "cat-root",
					Name:                "Books",
					SubcategoryRequired: true,
					Subcategories:       []webcore.TaxCategoryReference{{ID: "cat-child"}},
				},
				{ID: "cat-child", Name: "Books: Fiction"},
			},
		}, nil
	}
	getWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string) (*webcore.AppTaxCategory, error) {
		return &webcore.AppTaxCategory{AppID: "app-1", Configured: false}, nil
	}
	saveCalled := false
	saveWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string, string, []string, bool) error {
		saveCalled = true
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--category", "cat-root",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "requires a subcategory") || saveCalled {
		t.Fatalf("expected root category rejection before save, got err=%v saveCalled=%v stderr=%q", err, saveCalled, stderr)
	}
}

func TestWebAppsTaxCategorySetAcceptsParentConditionsForSubcategory(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalNewClient := newWebClientFn
	originalList := listWebAppTaxCategoriesFn
	originalGet := getWebAppTaxCategoryFn
	originalSave := saveWebAppTaxCategoryFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewClient
		listWebAppTaxCategoriesFn = originalList
		getWebAppTaxCategoryFn = originalGet
		saveWebAppTaxCategoryFn = originalSave
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return webcore.TaxCategoryCatalog{
			Categories: []webcore.TaxCategory{
				{
					ID:                  "cat-root",
					Name:                "Books",
					SubcategoryRequired: true,
					Conditions:          []webcore.TaxCategoryReference{{ID: "cond-1"}},
					Subcategories:       []webcore.TaxCategoryReference{{ID: "cat-child"}},
				},
				{ID: "cat-child", Name: "Books: Fiction"},
			},
		}, nil
	}
	readCount := 0
	getWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string) (*webcore.AppTaxCategory, error) {
		readCount++
		if readCount == 1 {
			return &webcore.AppTaxCategory{AppID: "app-1", Configured: false}, nil
		}
		return &webcore.AppTaxCategory{
			AppID:               "app-1",
			Configured:          true,
			CategoryID:          "cat-child",
			EnabledConditionIDs: []string{"cond-1"},
		}, nil
	}
	saveCalls := 0
	saveWebAppTaxCategoryFn = func(_ context.Context, _ *webcore.Client, appID, categoryID string, conditionIDs []string, configured bool) error {
		saveCalls++
		if appID != "app-1" || categoryID != "cat-child" || strings.Join(conditionIDs, ",") != "cond-1" || configured {
			t.Fatalf("unexpected save request: app=%q category=%q conditions=%v configured=%v", appID, categoryID, conditionIDs, configured)
		}
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--category", "cat-child",
		"--condition", "cond-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", saveCalls)
	}
	var receipt struct {
		CategoryID   string   `json:"categoryId"`
		ConditionIDs []string `json:"conditionIds"`
		Verified     bool     `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.CategoryID != "cat-child" || strings.Join(receipt.ConditionIDs, ",") != "cond-1" || !receipt.Verified {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
}

func TestWebAppsTaxCategorySetRejectsInvalidOutputBeforeSessionOrSave(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalSave := saveWebAppTaxCategoryFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		saveWebAppTaxCategoryFn = originalSave
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run for invalid output flags")
		return nil, "", nil
	}
	saveCalled := false
	saveWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string, string, []string, bool) error {
		saveCalled = true
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--category", "cat-1",
		"--confirm",
		"--output", "table",
		"--pretty",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") || saveCalled {
		t.Fatalf("expected invalid output rejection before session/save, got err=%v saveCalled=%v stderr=%q", err, saveCalled, stderr)
	}
}

func TestWebAppsTaxCategoryListRejectsInvalidOutputBeforeSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run for invalid output flags")
		return nil, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategoryListCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--output", "table", "--pretty"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("expected invalid output rejection before session, got err=%v stderr=%q", err, stderr)
	}
}

func TestWebAppsTaxCategoryViewRejectsInvalidOutputBeforeSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run for invalid output flags")
		return nil, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategoryViewCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "table", "--pretty"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("expected invalid output rejection before session, got err=%v stderr=%q", err, stderr)
	}
}

func TestWebAppsTaxCategorySetValidatesConditionCompatibility(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalNewClient := newWebClientFn
	originalList := listWebAppTaxCategoriesFn
	originalGet := getWebAppTaxCategoryFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewClient
		listWebAppTaxCategoriesFn = originalList
		getWebAppTaxCategoryFn = originalGet
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return webcore.TaxCategoryCatalog{
			Categories: []webcore.TaxCategory{{ID: "cat-1", Conditions: []webcore.TaxCategoryReference{{ID: "cond-1"}}}},
		}, nil
	}
	getWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string) (*webcore.AppTaxCategory, error) {
		return &webcore.AppTaxCategory{AppID: "app-1", Configured: false}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--category", "cat-1",
		"--condition", "cond-unknown",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "not compatible") {
		t.Fatalf("expected compatibility error, got %v (stderr=%q)", err, stderr)
	}
}

func TestWebAppsTaxCategorySetWritesAndVerifiesCompleteDesiredSet(t *testing.T) {
	originalResolveSession := resolveSessionFn
	originalNewClient := newWebClientFn
	originalList := listWebAppTaxCategoriesFn
	originalGet := getWebAppTaxCategoryFn
	originalSave := saveWebAppTaxCategoryFn
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		newWebClientFn = originalNewClient
		listWebAppTaxCategoriesFn = originalList
		getWebAppTaxCategoryFn = originalGet
		saveWebAppTaxCategoryFn = originalSave
	})
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
		return webcore.TaxCategoryCatalog{
			Categories: []webcore.TaxCategory{{ID: "cat-1", Name: "App Store Software", Conditions: []webcore.TaxCategoryReference{{ID: "cond-1"}, {ID: "cond-2"}}}},
		}, nil
	}
	readCount := 0
	getWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string) (*webcore.AppTaxCategory, error) {
		readCount++
		if readCount == 1 {
			return &webcore.AppTaxCategory{AppID: "app-1", Configured: false}, nil
		}
		return &webcore.AppTaxCategory{AppID: "app-1", Configured: true, CategoryID: "cat-1", EnabledConditionIDs: []string{"cond-1", "cond-2"}}, nil
	}
	var savedAppID, savedCategoryID string
	var savedConditions []string
	var savedConfigured bool
	saveWebAppTaxCategoryFn = func(_ context.Context, _ *webcore.Client, appID, categoryID string, conditionIDs []string, configured bool) error {
		savedAppID = appID
		savedCategoryID = categoryID
		savedConditions = append([]string{}, conditionIDs...)
		savedConfigured = configured
		return nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebAppsTaxCategorySetCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--category", "cat-1",
		"--condition", "cond-1",
		"--condition", "cond-2",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if savedAppID != "app-1" || savedCategoryID != "cat-1" || savedConfigured || strings.Join(savedConditions, ",") != "cond-1,cond-2" {
		t.Fatalf("unexpected save request: app=%q category=%q conditions=%v configured=%v", savedAppID, savedCategoryID, savedConditions, savedConfigured)
	}
	var receipt struct {
		AppID        string   `json:"appId"`
		CategoryID   string   `json:"categoryId"`
		ConditionIDs []string `json:"conditionIds"`
		Changed      bool     `json:"changed"`
		Verified     bool     `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.AppID != "app-1" || receipt.CategoryID != "cat-1" || strings.Join(receipt.ConditionIDs, ",") != "cond-1,cond-2" || !receipt.Changed || !receipt.Verified {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
}

func TestWebAppsTaxCategorySetReportsVerifiedStateWhenReceiptOutputFails(t *testing.T) {
	tests := []struct {
		name         string
		initial      *webcore.AppTaxCategory
		verified     *webcore.AppTaxCategory
		wantSave     int
		wantWriteMsg string
		wantNoWrite  bool
	}{
		{
			name:         "changed",
			initial:      &webcore.AppTaxCategory{AppID: "app-1", Configured: false},
			verified:     &webcore.AppTaxCategory{AppID: "app-1", Configured: true, CategoryID: "cat-1", EnabledConditionIDs: []string{"cond-1"}},
			wantSave:     1,
			wantWriteMsg: "was written and verified",
		},
		{
			name:         "already matched",
			initial:      &webcore.AppTaxCategory{AppID: "app-1", Configured: true, CategoryID: "cat-1", EnabledConditionIDs: []string{"cond-1"}},
			verified:     &webcore.AppTaxCategory{AppID: "app-1", Configured: true, CategoryID: "cat-1", EnabledConditionIDs: []string{"cond-1"}},
			wantSave:     0,
			wantWriteMsg: "no write occurred",
			wantNoWrite:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalResolveSession := resolveSessionFn
			originalNewClient := newWebClientFn
			originalList := listWebAppTaxCategoriesFn
			originalGet := getWebAppTaxCategoryFn
			originalSave := saveWebAppTaxCategoryFn
			t.Cleanup(func() {
				resolveSessionFn = originalResolveSession
				newWebClientFn = originalNewClient
				listWebAppTaxCategoriesFn = originalList
				getWebAppTaxCategoryFn = originalGet
				saveWebAppTaxCategoryFn = originalSave
			})

			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{}, "cache", nil
			}
			newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
			listWebAppTaxCategoriesFn = func(context.Context, *webcore.Client) (webcore.TaxCategoryCatalog, error) {
				return webcore.TaxCategoryCatalog{
					Categories: []webcore.TaxCategory{{ID: "cat-1", Name: "App Store Software", Conditions: []webcore.TaxCategoryReference{{ID: "cond-1"}}}},
				}, nil
			}
			readCount := 0
			getWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string) (*webcore.AppTaxCategory, error) {
				readCount++
				if readCount == 1 {
					return tt.initial, nil
				}
				return tt.verified, nil
			}
			saveCalls := 0
			saveWebAppTaxCategoryFn = func(context.Context, *webcore.Client, string, string, []string, bool) error {
				saveCalls++
				return nil
			}

			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebAppsTaxCategorySetCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--app", "app-1",
				"--category", "cat-1",
				"--condition", "cond-1",
				"--confirm",
				"--output", "json",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			var commandErr error
			stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
				closedReader, closedStdout, err := os.Pipe()
				if err != nil {
					t.Fatalf("closed stdout pipe: %v", err)
				}
				if err := closedReader.Close(); err != nil {
					t.Fatalf("close stdout reader: %v", err)
				}
				if err := closedStdout.Close(); err != nil {
					t.Fatalf("close stdout pipe: %v", err)
				}
				d.IO.Out = closedStdout
				commandErr = cmd.Exec(context.Background(), nil)
			})

			if commandErr == nil {
				t.Fatalf("expected receipt output failure, stdout=%q", stdout)
			}
			if !strings.Contains(commandErr.Error(), `app "app-1"`) || !strings.Contains(commandErr.Error(), `category "cat-1"`) || !strings.Contains(commandErr.Error(), "cond-1") {
				t.Fatalf("output failure omitted confirmed state: %v", commandErr)
			}
			if !strings.Contains(commandErr.Error(), tt.wantWriteMsg) || !strings.Contains(commandErr.Error(), "do not retry automatically") {
				t.Fatalf("output failure missing retry-safe receipt context: %v", commandErr)
			}
			if tt.wantNoWrite && strings.Contains(commandErr.Error(), "was written") {
				t.Fatalf("no-op output failure incorrectly claimed a write: %v", commandErr)
			}
			if saveCalls != tt.wantSave {
				t.Fatalf("save calls = %d, want %d", saveCalls, tt.wantSave)
			}
		})
	}
}
