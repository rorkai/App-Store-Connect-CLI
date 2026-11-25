package cmdtest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAnalyticsDownloadAllSegments(t *testing.T) {
	setupAnalyticsResourceIDAuth(t)
	const segmentListPath = "/v1/analyticsReportInstances/instance-1/segments"
	const report = "Date\tCount\n2026-11-23\t12\n"
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(report)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	checksum, err := asc.ComputeChecksumFromReader(bytes.NewReader(compressed.Bytes()), asc.ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatal(err)
	}
	var badChecksum atomic.Bool
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		switch req.URL.Path {
		case segmentListPath:
			w.Header().Set("Content-Type", "application/json")
			switch req.URL.RawQuery {
			case "limit=200":
				assertAnalyticsResourceIDRequest(t, req, segmentListPath, "limit=200", true)
				fmt.Fprintf(w, `{"data":[{"type":"analyticsReportSegments","id":"seg-1","attributes":{"url":"https://mzstatic.com/one.gz"}}],"links":{"next":%q}}`, "https://api.appstoreconnect.apple.com"+segmentListPath+"?cursor=next")
			case "cursor=next":
				assertAnalyticsResourceIDRequest(t, req, segmentListPath, "cursor=next", true)
				expectedChecksum := checksum.Hash
				if badChecksum.Load() {
					expectedChecksum = "00000000000000000000000000000000"
				}
				fmt.Fprintf(w, `{"data":[{"type":"analyticsReportSegments","id":"seg-2","attributes":{"url":"https://mzstatic.com/two.gz","sizeInBytes":%d,"checksum":%q}}],"links":{}}`, compressed.Len(), expectedChecksum)
			default:
				t.Errorf("unexpected segment query: %q", req.URL.RawQuery)
				http.NotFound(w, req)
			}
		case "/one.gz", "/two.gz":
			assertAnalyticsResourceIDRequest(t, req, req.URL.Path, "", false)
			_, _ = w.Write(compressed.Bytes())
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	parent := t.TempDir()
	destination := filepath.Join(parent, "analytics")
	base := []string{"analytics", "download", "--request-id", analyticsViewRequestID, "--instance-id", "instance-1", "--all-segments"}
	args := append(slices.Clone(base), "--output-dir", destination, "--decompress")
	client := stubServerClient(t, server)

	badChecksum.Store(true)
	stdout, stderr, code := runWithDeps(t, client, args...)
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, "seg-2") || !strings.Contains(stderr, "checksum") {
		t.Fatalf("partial download: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("failed download left output: %v, %v", entries, err)
	}

	badChecksum.Store(false)
	stdout, stderr, code = runWithDeps(t, client, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("retry: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var receipt struct {
		RequestID  string `json:"requestId"`
		InstanceID string `json:"instanceId"`
		OutputDir  string `json:"outputDir"`
		Segments   []struct {
			ID               string `json:"segmentId"`
			FilePath         string `json:"filePath"`
			FileSize         int64  `json:"fileSize"`
			DecompressedPath string `json:"decompressedPath"`
			DecompressedSize int64  `json:"decompressedSize"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.RequestID != analyticsViewRequestID || receipt.InstanceID != "instance-1" || receipt.OutputDir != destination || len(receipt.Segments) != 2 {
		t.Fatalf("unexpected receipt: %s", stdout)
	}
	for i, segment := range receipt.Segments {
		if segment.ID != fmt.Sprintf("seg-%d", i+1) || segment.FileSize != int64(compressed.Len()) || segment.DecompressedSize != int64(len(report)) ||
			!strings.HasSuffix(segment.FilePath, ".txt.gz") || segment.DecompressedPath != strings.TrimSuffix(segment.FilePath, ".gz") {
			t.Fatalf("unexpected segment receipt: %+v", segment)
		}
		for _, artifact := range []struct {
			path string
			data string
		}{
			{segment.FilePath, compressed.String()},
			{segment.DecompressedPath, report},
		} {
			if filepath.Dir(artifact.path) != destination {
				t.Fatalf("artifact escaped destination: %s", artifact.path)
			}
			if got, err := os.ReadFile(artifact.path); err != nil || string(got) != artifact.data {
				t.Fatalf("artifact %s: expected %d bytes, got %d (%v)", artifact.path, len(artifact.data), len(got), err)
			}
		}
	}
	if count := requests.Load(); count != 8 {
		t.Fatalf("requests during failed attempt and successful retry = %d, want 8", count)
	}

	stdout, stderr, code = runWithDeps(t, client, args...)
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, "output directory already exists") {
		t.Fatalf("existing destination: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if count := requests.Load(); count != 8 {
		t.Fatalf("conflict made API requests: got %d requests, want 8", count)
	}

	missingParent := filepath.Join(parent, "missing")
	stdout, stderr, code = runWithDeps(t, client, append(slices.Clone(base), "--output-dir", filepath.Join(missingParent, "report"))...)
	wantMissing := fmt.Sprintf("Error: analytics download: parent directory of --output-dir does not exist: %q\n", missingParent)
	if code != rootcmd.ExitError || stdout != "" || stderr != wantMissing {
		t.Fatalf("missing parent: code=%d stdout=%q stderr=%q, want %q", code, stdout, stderr, wantMissing)
	}
	if count := requests.Load(); count != 8 {
		t.Fatalf("missing parent made API requests: got %d requests, want 8", count)
	}

	stdout, stderr, code = runWithDeps(t, client, append(base, "--output-dir", filepath.Join(parent, "table"), "--output-format", "table")...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Segment ID") || !strings.Contains(stdout, "seg-1") || !strings.Contains(stdout, "seg-2") {
		t.Fatalf("table receipt: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAnalyticsDownloadAllSegmentsFlags(t *testing.T) {
	setupAnalyticsResourceIDAuth(t)
	base := []string{"analytics", "download", "--request-id", analyticsViewRequestID, "--instance-id", "instance-1"}
	noClient := func() (*asc.Client, error) {
		t.Fatal("invalid flags requested an authenticated client")
		return nil, nil
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--all-segments"}, "--output-dir is required"},
		{[]string{"--all-segments", "--segment-id", "seg-1"}, "--all-segments cannot be combined with --segment-id"},
		{[]string{"--all-segments", "--output", "report.csv.gz"}, "--all-segments cannot be combined with --output"},
		{[]string{"--output-dir", "report"}, "--output-dir requires --all-segments"},
	} {
		assertUsageExitWithDeps(t, noClient, append(slices.Clone(base), test.args...), test.want)
	}
}
