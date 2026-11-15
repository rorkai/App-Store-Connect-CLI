package shared

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestResolveTiersWithFetcherWarnsWhenCacheSaveFails(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	tiers, resolveErr := resolveTiersWithFetcher(
		os.Stderr,
		true,
		func() ([]TierEntry, error) { return nil, nil },
		func([]TierEntry) error { return errors.New("disk full") },
		func(nextURL string) (tierPage, error) {
			return tierPage{entries: []pricePointEntry{{id: "pp1", customerPrice: 0.99, rawPrice: "0.99", proceeds: "0.69"}}}, nil
		},
	)

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	os.Stderr = origStderr
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stderr: %v", err)
	}

	if resolveErr != nil {
		t.Fatalf("resolveTiersWithFetcher returned error: %v", resolveErr)
	}
	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(tiers))
	}
	stderrText := string(captured)
	if !strings.Contains(stderrText, "Warning: failed to cache price tiers: disk full") {
		t.Fatalf("expected cache-save warning on stderr, got %q", stderrText)
	}
}
