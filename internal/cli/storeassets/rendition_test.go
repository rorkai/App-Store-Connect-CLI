package storeassets

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDeliveredPreviewComparisonBoundsUnknownLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.mp4")
	if err := os.WriteFile(path, []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := &unboundedPreviewBody{}
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: previewRoundTripper(func(req *http.Request) (*http.Response, error) {
		if _, ok := req.Context().Deadline(); !ok {
			t.Error("comparison request has no deadline")
		}
		if req.Header.Get("Authorization") != "" {
			t.Error("media comparison forwarded authorization")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: -1, Header: http.Header{}}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = previous })
	match, err := matchesDeliveredMedia(context.Background(), path, "https://media.example/video", maxPreviewBytes)
	if err != nil || match {
		t.Fatalf("match=%v err=%v", match, err)
	}
	if body.read != len("local")+1 || !body.closed {
		t.Fatalf("unbounded body read=%d, closed=%v; want only staged size plus sentinel", body.read, body.closed)
	}
}

type previewRoundTripper func(*http.Request) (*http.Response, error)

func (fn previewRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

type unboundedPreviewBody struct {
	read   int
	closed bool
}

func (b *unboundedPreviewBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	b.read += len(p)
	return len(p), nil
}

func (b *unboundedPreviewBody) Close() error { b.closed = true; return nil }

var _ io.ReadCloser = (*unboundedPreviewBody)(nil)
