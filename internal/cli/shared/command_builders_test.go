package shared

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
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
