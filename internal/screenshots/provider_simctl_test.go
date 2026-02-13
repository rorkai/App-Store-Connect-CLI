package screenshots

import (
	"path/filepath"
	"testing"
)

func TestSimctlProvider_OutputPath(t *testing.T) {
	req := CaptureRequest{
		Name:      "home",
		OutputDir: "/tmp/screenshots",
	}
	expected := filepath.Join(req.OutputDir, req.Name+".png")
	if filepath.Base(expected) != "home.png" {
		t.Fatalf("expected base home.png, got %s", filepath.Base(expected))
	}
	if filepath.Dir(expected) != req.OutputDir {
		t.Fatalf("expected dir %q, got %q", req.OutputDir, filepath.Dir(expected))
	}
}
