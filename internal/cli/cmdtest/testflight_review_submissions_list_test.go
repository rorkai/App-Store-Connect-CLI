package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightReviewSubmissionsListMissingBuild(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "review", "submissions", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --build-id is required") {
		t.Fatalf("expected error %q, got %q", "Error: --build-id is required", stderr)
	}
}

func TestTestFlightReviewSubmissionsListOutputTable(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-1", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2024-01-01T00:00:00Z"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "review", "submissions", "list", "--build-id", "build-1", "--limit", "2", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "submission-1") {
		t.Fatalf("expected submission in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "WAITING_FOR_REVIEW") {
		t.Fatalf("expected beta review state in output, got %q", stdout)
	}
}
