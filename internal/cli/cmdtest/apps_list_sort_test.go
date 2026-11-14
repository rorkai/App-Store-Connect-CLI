package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestAppsListSortsBySKU covers the sku and -sku members of the documented
// GET /v1/apps sort enum reaching the API unchanged.
func TestAppsListSortsBySKU(t *testing.T) {
	setupAuth(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "list ascending", args: []string{"apps", "list", "--sort", "sku", "--output", "json"}, want: "sku"},
		{name: "list descending", args: []string{"apps", "list", "--sort", "-sku", "--output", "json"}, want: "-sku"},
		{name: "group ascending", args: []string{"apps", "--sort", "sku", "--output", "json"}, want: "sku"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps", "sort="+test.want), asctest.JSON(http.StatusOK, `{"data":[]}`))

			_, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if calls := len(reg.Requests()); calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}

// TestAppsListRejectsUnsupportedSort keeps the allow-list aligned with the
// documented sort enum for GET /v1/apps.
func TestAppsListRejectsUnsupportedSort(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"apps", "list", "--sort", "createdDate"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected an error for an unsupported --sort value")
	}
	want := "--sort must be one of: name, -name, bundleId, -bundleId, sku, -sku"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want it to contain %q", err, want)
	}
}
