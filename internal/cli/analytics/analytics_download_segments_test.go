package analytics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func TestDownloadAnalyticsInstanceWithoutNoReplaceRename(t *testing.T) {
	previous := renameNoReplaceInRoot
	t.Cleanup(func() { renameNoReplaceInRoot = previous })
	renameNoReplaceInRoot = func(*os.Root, string, string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	segment := compareGzipFixture(t, "Date\tCount\n2026-11-23\t12\n")
	client := newCompareHTTPClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/analyticsReportInstances/instance-1/segments":
			fmt.Fprint(w, `{"data":[{"type":"analyticsReportSegments","id":"seg-1","attributes":{"url":"https://mzstatic.com/one.gz"}}],"links":{}}`)
		case "/one.gz":
			_, _ = w.Write(segment)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			http.NotFound(w, req)
		}
	})
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return client, nil }
	d.IO = &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	parent := t.TempDir()
	destination := filepath.Join(parent, "report")

	_, err := downloadAnalyticsInstance(context.Background(), d, "11111111-2222-3333-4444-555555555555", "instance-1", destination, false)

	want := fmt.Sprintf("publish output directory %q: this filesystem has no atomic no-replace rename, which --all-segments needs; nothing was saved", destination)
	if err == nil || err.Error() != want || !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("error = %v, want %q wrapping the unsupported rename", err, want)
	}
	hinted, ok := errors.AsType[*shared.HintedError](err)
	if wantHint := "Choose an --output-dir on a local filesystem, or download one segment at a time with --segment-id SEGMENT_ID."; !ok || hinted.Hint != wantHint {
		t.Fatalf("hint = %+v, want %q", hinted, wantHint)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("output parent holds %v (%v), want neither the destination nor the staging directory", entries, err)
	}
}
