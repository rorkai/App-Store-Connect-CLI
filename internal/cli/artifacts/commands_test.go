package artifacts

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
}

func TestIPAInfoRequiresPath(t *testing.T) {
	cmd := NewIPAInfoCommand(newTestDeps())
	if err := cmd.Parse([]string{"--output", "json"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestPKGInfoRequiresPath(t *testing.T) {
	cmd := NewPKGInfoCommand(newTestDeps())
	if err := cmd.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestArtifactReceiptsDeclareUnverifiedSignatures(t *testing.T) {
	receipts := []any{
		ipaReceipt("Demo.ipa", artifacts.IPAManifest{Status: "readable"}),
		pkgReceipt("Demo.pkg", artifacts.PKGManifest{Status: "readable"}),
		unreadableReceipt("ipa-info", "missing.ipa", false),
		unreadableReceipt("pkg-info", "missing.pkg", false),
	}
	for _, receipt := range receipts {
		encoded, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		if value["signatureVerification"] != "not-verified" {
			t.Fatalf("receipt=%s", encoded)
		}
	}
}
