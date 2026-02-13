package screenshots

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestAXeProvider_MissingBinary(t *testing.T) {
	t.Setenv("PATH", "")

	provider := &AXeProvider{}
	_, err := provider.Capture(context.Background(), CaptureRequest{
		UDID:      "booted",
		Name:      "home",
		OutputDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error when axe binary is missing")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected exec.ErrNotFound, got %v", err)
	}
}
