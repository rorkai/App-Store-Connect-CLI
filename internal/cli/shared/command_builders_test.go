package shared

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type testPaginatedResponse struct {
	Data  []map[string]string `json:"data"`
	Links asc.Links           `json:"links"`
}

func (r *testPaginatedResponse) GetLinks() *asc.Links {
	return &r.Links
}

func (r *testPaginatedResponse) GetData() any {
	return r.Data
}

func TestBuildIDGetCommand_MissingIDReturnsUsageError(t *testing.T) {
	cmd := BuildIDGetCommand(IDGetCommandConfig{
		FlagSetName: "test-id-get",
		Name:        "get",
		ShortUsage:  "test get",
		ShortHelp:   "test",
		ErrorPrefix: "test get",
		Fetch:       func(context.Context, *asc.Client, string) (any, error) { return nil, nil },
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --id is required") {
		t.Fatalf("expected missing id usage error, got %q", stderr)
	}
}

func TestBuildPaginatedListCommand_MissingParentIDReturnsUsageError(t *testing.T) {
	cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: "test-list",
		Name:        "list",
		ShortUsage:  "test list",
		ShortHelp:   "test",
		ParentFlag:  "app-id",
		ErrorPrefix: "test list",
		FetchPage: func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error) {
			return &testPaginatedResponse{}, nil
		},
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --app-id is required") {
		t.Fatalf("expected missing app-id usage error, got %q", stderr)
	}
}

func TestBuildPaginatedListCommand_InvalidLimitReturnsUsageError(t *testing.T) {
	cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: "test-list",
		Name:        "list",
		ShortUsage:  "test list",
		ShortHelp:   "test",
		ParentFlag:  "app-id",
		ErrorPrefix: "test list",
		LimitMax:    200,
		FetchPage: func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error) {
			return &testPaginatedResponse{}, nil
		},
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	if err := cmd.FlagSet.Parse([]string{"--app-id", "app-1", "--limit", "201"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: test list: --limit must be between 1 and 200") {
		t.Fatalf("expected limit usage error, got %q", stderr)
	}
}

func TestBuildPaginatedListCommand_InvalidNextURLReturnsUsageError(t *testing.T) {
	cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: "test-list",
		Name:        "list",
		ShortUsage:  "test list",
		ShortHelp:   "test",
		ParentFlag:  "app-id",
		ErrorPrefix: "test list",
		FetchPage: func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error) {
			return &testPaginatedResponse{}, nil
		},
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	if err := cmd.FlagSet.Parse([]string{"--next", "http://api.appstoreconnect.apple.com/v1/test?cursor=AQ"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: test list: --next must be an App Store Connect URL") {
		t.Fatalf("expected next URL usage error, got %q", stderr)
	}
}

func TestBuildPricePointEqualizationsCommand_UsesSharedHelpTemplate(t *testing.T) {
	cmd := BuildPricePointEqualizationsCommand(PricePointEqualizationsCommandConfig{
		FlagSetName: "pricing price-points equalizations",
		Name:        "equalizations",
		ShortUsage:  "asc pricing price-points equalizations --price-point PRICE_POINT_ID",
		BaseExample: `asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		Subject:     "a price point",
		ParentFlag:  "price-point",
		ParentUsage: "App price point ID",
		ErrorPrefix: "pricing price-points equalizations",
		FetchPage: func(context.Context, *asc.Client, string, int, string, []string, asc.IncludedFields) (asc.PaginatedResponse, error) {
			return &testPaginatedResponse{}, nil
		},
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	usage := cmd.UsageFunc(cmd)
	for _, want := range []string{
		"asc pricing price-points equalizations --price-point PRICE_POINT_ID [--limit N] [--next URL] [--paginate]",
		`asc pricing price-points equalizations --price-point "PRICE_POINT_ID" --limit 175`,
		`asc pricing price-points equalizations --price-point "PRICE_POINT_ID" --paginate`,
		`asc pricing price-points equalizations --next "NEXT_URL"`,
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("expected usage to contain %q, got %q", want, usage)
		}
	}
}

func TestBuildConfirmDeleteCommand_MissingConfirmReturnsUsageError(t *testing.T) {
	cmd := BuildConfirmDeleteCommand(ConfirmDeleteCommandConfig{
		FlagSetName: "test-delete",
		Name:        "delete",
		ShortUsage:  "test delete",
		ShortHelp:   "test",
		ErrorPrefix: "test delete",
		Delete:      func(context.Context, *asc.Client, string) error { return nil },
		Result:      func(string) any { return map[string]string{"status": "ok"} },
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	if err := cmd.FlagSet.Parse([]string{"--id", "abc"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected missing confirm usage error, got %q", stderr)
	}
}

func TestBuildPaginatedListCommand_PaginateUsesRequestedLimitForFirstPage(t *testing.T) {
	resetPrivateKeyTemp(t)

	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	var (
		calls       int
		firstLimit  int
		firstParent string
	)

	cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
		FlagSetName: "test-list",
		Name:        "list",
		ShortUsage:  "test list",
		ShortHelp:   "test",
		ParentFlag:  "app-id",
		ErrorPrefix: "test list",
		LimitMax:    200,
		FetchPage: func(ctx context.Context, _ *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			calls++
			if calls > 1 {
				t.Fatalf("unexpected extra FetchPage call %d", calls)
				return nil, nil
			}
			firstParent = parentID
			firstLimit = limit
			if next != "" {
				t.Fatalf("expected empty next URL on first page, got %q", next)
			}
			return &testPaginatedResponse{
				Data: []map[string]string{{"id": "item-1"}},
			}, nil
		},
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	})

	if err := cmd.FlagSet.Parse([]string{"--app-id", "app-1", "--paginate", "--limit", "77", "--output", "json"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	if firstParent != "app-1" {
		t.Fatalf("expected parent ID app-1, got %q", firstParent)
	}
	if firstLimit != 77 {
		t.Fatalf("expected first paginated request to use limit 77, got %d", firstLimit)
	}
	if calls != 1 {
		t.Fatalf("expected 1 paginated call, got %d", calls)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"item-1"`) {
		t.Fatalf("expected JSON output to contain item-1, got %q", stdout)
	}
}

const requiredIDStderr = "Error: --id is required\n"

type builderTestCall struct {
	parent string
	limit  int
	next   string
}

func newBuilderTestDeps(stdout, stderr *bytes.Buffer) *Deps {
	return &Deps{
		Client: func() (*asc.Client, error) { return nil, nil },
		IO:     &IO{Out: stdout, ErrOut: stderr},
		Timeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithCancel(ctx)
		},
	}
}

func TestBuildPaginatedListCommandOptions(t *testing.T) {
	t.Parallel()

	const (
		nextURL       = "https://api.appstoreconnect.apple.com/v1/items?cursor=AQ"
		secondPageURL = "https://api.appstoreconnect.apple.com/v1/items?cursor=BB"
	)
	tests := []struct {
		name            string
		topLevel        bool
		nextBeforeLimit bool
		args            []string
		failCall        int
		wantErr         string
		wantStderr      string
		wantCalls       []builderTestCall
		wantStdout      string
	}{
		{
			name:       "missing parent reports the required diagnostic before flag validation",
			args:       []string{"--filter", "bad"},
			wantErr:    "--id",
			wantStderr: requiredIDStderr,
		},
		{
			name:    "flag validation runs after the parent check",
			args:    []string{"--id", "parent-1", "--filter", "bad"},
			wantErr: "bad filter",
		},
		{
			name:    "next conflicts",
			args:    []string{"--next", nextURL, "--filter", "value"},
			wantErr: "items list: --next cannot be combined with --filter",
		},
		{
			name:      "single page error uses the fetch prefix",
			args:      []string{"--id", "parent-1", "--limit", "50"},
			failCall:  1,
			wantErr:   "items list: failed to fetch: boom",
			wantCalls: []builderTestCall{{parent: "parent-1", limit: 50}},
		},
		{
			name:       "paginate requests full pages despite --limit",
			args:       []string{"--id", "parent-1", "--paginate", "--limit", "50"},
			wantCalls:  []builderTestCall{{parent: "parent-1", limit: 200}, {parent: "parent-1", next: secondPageURL}},
			wantStdout: `{"data":[{"id":"item-1"},{"id":"item-2"}],"links":{}}` + "\n",
		},
		{
			name:      "paginate wraps the first page error with the fetch prefix",
			args:      []string{"--id", "parent-1", "--paginate"},
			failCall:  1,
			wantErr:   "items list: failed to fetch: boom",
			wantCalls: []builderTestCall{{parent: "parent-1", limit: 200}},
		},
		{
			name:      "paginate wraps a later page error with the error prefix",
			args:      []string{"--id", "parent-1", "--paginate"},
			failCall:  2,
			wantErr:   "items list: page 2: boom",
			wantCalls: []builderTestCall{{parent: "parent-1", limit: 200}, {parent: "parent-1", next: secondPageURL}},
		},
		{
			name:    "limit is checked before next",
			args:    []string{"--next", "http://example.test/items", "--limit", "500"},
			wantErr: "items list: --limit must be between 1 and 200",
		},
		{
			name:            "next and its conflicts are checked before limit on request",
			nextBeforeLimit: true,
			args:            []string{"--next", nextURL, "--filter", "value", "--limit", "500"},
			wantErr:         "items list: --next cannot be combined with --filter",
		},
		{
			name:            "limit is still checked after next on request",
			nextBeforeLimit: true,
			args:            []string{"--id", "parent-1", "--limit", "500"},
			wantErr:         "items list: --limit must be between 1 and 200",
		},
		{
			name:       "top-level collection has no parent flag",
			topLevel:   true,
			wantCalls:  []builderTestCall{{}},
			wantStdout: `{"data":[{"id":"item-1"}],"links":{"next":"` + secondPageURL + `"}}` + "\n",
			wantStderr: "Warning: showing 1 results; more pages exist (use --paginate or --next where supported)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			var calls []builderTestCall
			cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
				FlagSetName:          "list",
				Name:                 "list",
				ShortUsage:           "asc items list [flags]",
				ParentFlag:           "id",
				TopLevel:             tt.topLevel,
				NextBeforeLimit:      tt.nextBeforeLimit,
				RequiredDiagnostic:   true,
				PaginateIgnoresLimit: true,
				NoSpinner:            true,
				NextConflicts:        []string{"id", "filter"},
				Flags: func(fs *flag.FlagSet) func() error {
					filter := fs.String("filter", "", "Filter")
					return func() error {
						if *filter == "bad" {
							return errors.New("bad filter")
						}
						return nil
					}
				},
				ErrorPrefix:      "items list",
				FetchErrorPrefix: "items list: failed to fetch",
				Deps:             newBuilderTestDeps(&stdout, &stderr),
				FetchPage: func(_ context.Context, _ *asc.Client, parent string, limit int, next string) (asc.PaginatedResponse, error) {
					calls = append(calls, builderTestCall{parent: parent, limit: limit, next: next})
					if len(calls) == tt.failCall {
						return nil, errors.New("boom")
					}
					if len(calls) == 1 {
						return &testPaginatedResponse{
							Data:  []map[string]string{{"id": "item-1"}},
							Links: asc.Links{Next: secondPageURL},
						}, nil
					}
					return &testPaginatedResponse{Data: []map[string]string{{"id": "item-2"}}}, nil
				},
			})
			if tt.topLevel && cmd.FlagSet.Lookup("id") != nil {
				t.Fatal("top-level list registered a parent flag")
			}
			if err := cmd.FlagSet.Parse(append(tt.args, "--output", "json")); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Fatalf("Exec() error = %v, want %q", err, tt.wantErr)
			}
			if tt.wantStderr == requiredIDStderr {
				diagnostic, ok := DiagnosticFromError(err)
				if !ok || diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != "--id" {
					t.Fatalf("diagnostic = %+v, %v; want required --id", diagnostic, ok)
				}
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Fatalf("FetchPage calls = %+v, want %+v", calls, tt.wantCalls)
			}
		})
	}
}

func TestBuildIDGetCommandOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		fetchErr   error
		wantErr    string
		wantStderr string
		wantStdout string
	}{
		{
			name:       "missing id reports the required diagnostic before flag validation",
			args:       []string{"--fields", "bad"},
			wantErr:    "--id",
			wantStderr: requiredIDStderr,
		},
		{
			name:    "flag validation runs after the id check",
			args:    []string{"--id", "item-1", "--fields", "bad"},
			wantErr: "bad fields",
		},
		{
			name:     "fetch error uses the fetch prefix",
			args:     []string{"--id", "item-1"},
			fetchErr: errors.New("boom"),
			wantErr:  "items view: failed to fetch: boom",
		},
		{
			name:       "result prints to the deps stdout",
			args:       []string{"--id", "item-1", "--fields", "name"},
			wantStdout: `{"fields":"name","id":"item-1"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			var fields *string
			cmd := BuildIDGetCommand(IDGetCommandConfig{
				FlagSetName:        "view",
				Name:               "view",
				ShortUsage:         "asc items view --id ITEM_ID",
				RequiredDiagnostic: true,
				Flags: func(fs *flag.FlagSet) func() error {
					fields = fs.String("fields", "", "Fields")
					return func() error {
						if *fields == "bad" {
							return errors.New("bad fields")
						}
						return nil
					}
				},
				ErrorPrefix:      "items view",
				FetchErrorPrefix: "items view: failed to fetch",
				Deps:             newBuilderTestDeps(&stdout, &stderr),
				Fetch: func(_ context.Context, _ *asc.Client, id string) (any, error) {
					if tt.fetchErr != nil {
						return nil, tt.fetchErr
					}
					return map[string]string{"id": id, "fields": *fields}, nil
				},
			})
			if err := cmd.FlagSet.Parse(append(tt.args, "--output", "json")); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Fatalf("Exec() error = %v, want %q", err, tt.wantErr)
			}
			if tt.wantStderr == requiredIDStderr {
				diagnostic, ok := DiagnosticFromError(err)
				if !ok || diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != "--id" {
					t.Fatalf("diagnostic = %+v, %v; want required --id", diagnostic, ok)
				}
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
		})
	}
}
