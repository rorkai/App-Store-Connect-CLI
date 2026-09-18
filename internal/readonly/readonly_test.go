package readonly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func resetFlag(t *testing.T) {
	t.Helper()
	SetFlagEnabled(false)
	t.Cleanup(func() { SetFlagEnabled(false) })
}

func TestEnabledFromEnvironment(t *testing.T) {
	resetFlag(t)
	tests := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"off", false},
		{"no", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"y", true},
		{"on", true},
		{" 1 ", true},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%q", test.value), func(t *testing.T) {
			t.Setenv(EnvVar, test.value)
			if got := Enabled(); got != test.want {
				t.Fatalf("Enabled() with %s=%q = %v, want %v", EnvVar, test.value, got, test.want)
			}
		})
	}
}

func TestFlagEnablesAndCannotDisableEnvironment(t *testing.T) {
	resetFlag(t)
	t.Setenv(EnvVar, "")
	if Enabled() {
		t.Fatal("Enabled() = true before any source is set")
	}

	SetFlagEnabled(true)
	if !Enabled() {
		t.Fatal("Enabled() = false after SetFlagEnabled(true)")
	}
	if got := Source(); got != "--read-only" {
		t.Fatalf("Source() = %q, want --read-only", got)
	}

	SetFlagEnabled(false)
	t.Setenv(EnvVar, "1")
	if !Enabled() {
		t.Fatal("Enabled() = false with env set and flag cleared; the flag must not disable the environment")
	}
	if got := Source(); got != EnvVar {
		t.Fatalf("Source() = %q, want %s", got, EnvVar)
	}
}

func TestCheckRefusesOnlyMutatingMethods(t *testing.T) {
	resetFlag(t)
	t.Setenv(EnvVar, "1")
	ctx := context.Background()

	for _, method := range []string{"", http.MethodGet, http.MethodHead, http.MethodOptions, "get"} {
		if err := Check(ctx, method, "/v1/apps"); err != nil {
			t.Fatalf("Check(%q) error = %v, want nil", method, err)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, "post"} {
		err := Check(ctx, method, "/v1/apps/123")
		if err == nil {
			t.Fatalf("Check(%q) error = nil, want refusal", method)
		}
		if !errors.Is(err, ErrRefused) {
			t.Fatalf("Check(%q) error = %v, want errors.Is ErrRefused", method, err)
		}
		var refused *RefusedError
		if !errors.As(err, &refused) {
			t.Fatalf("Check(%q) error type = %T, want *RefusedError", method, err)
		}
		if refused.Method != http.MethodPost && method == "post" {
			t.Fatalf("RefusedError.Method = %q, want upper-cased POST", refused.Method)
		}
	}

	err := Check(ctx, http.MethodPatch, "/v1/appStoreVersions/123")
	want := "ASC_READ_ONLY is set; refusing PATCH /v1/appStoreVersions/123"
	if err == nil || err.Error() != want {
		t.Fatalf("Check() error = %v, want %q", err, want)
	}

	wrapped := fmt.Errorf("failed to update version: %w", err)
	if !errors.Is(wrapped, ErrRefused) {
		t.Fatalf("wrapped refusal is not ErrRefused: %v", wrapped)
	}
}

func TestCheckPassesWhenDisabledOrReadIntent(t *testing.T) {
	resetFlag(t)
	t.Setenv(EnvVar, "")
	if err := Check(context.Background(), http.MethodDelete, "/v1/apps/1"); err != nil {
		t.Fatalf("Check() with mode disabled error = %v, want nil", err)
	}

	t.Setenv(EnvVar, "1")
	ctx := WithReadIntent(context.Background())
	if !HasReadIntent(ctx) {
		t.Fatal("HasReadIntent() = false after WithReadIntent")
	}
	if HasReadIntent(context.Background()) {
		t.Fatal("HasReadIntent() = true on a bare context")
	}
	if err := Check(ctx, http.MethodPost, "/v5/campaigns/find"); err != nil {
		t.Fatalf("Check() with read intent error = %v, want nil", err)
	}
	if err := Check(nil, http.MethodPost, "/v5/campaigns"); err == nil { //nolint:staticcheck // nil ctx must still refuse
		t.Fatal("Check() with nil ctx and mutating method error = nil, want refusal")
	}
}

func TestTargetStripsQueryAndKeepsHost(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"/v1/builds/abc", "/v1/builds/abc"},
		{"/v1/builds?limit=1", "/v1/builds"},
		{"https://appstoreconnect.apple.com/iris/v1/apps/1?include=x", "https://appstoreconnect.apple.com/iris/v1/apps/1"},
		{"https://upload.example.com/part?signature=secret", "https://upload.example.com/part"},
	}
	for _, test := range tests {
		if got := Target(test.raw); got != test.want {
			t.Fatalf("Target(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}
