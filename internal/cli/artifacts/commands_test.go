package artifacts

import (
	"context"
	"strings"
	"testing"
)

func TestIPAInfoRequiresPath(t *testing.T) {
	cmd := IPAInfoCommand()
	if err := cmd.Parse([]string{"--output", "json"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestPKGInfoRequiresPath(t *testing.T) {
	cmd := PKGInfoCommand()
	if err := cmd.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("error = %v", err)
	}
}
