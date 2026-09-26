package artifacts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
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

func TestArtifactReceiptsDeclareUnverifiedSignatures(t *testing.T) {
	receipts := []any{
		ipaReceipt("Demo.ipa", artifacts.IPAManifest{Status: "readable"}),
		pkgReceipt("Demo.pkg", artifacts.PKGManifest{Status: "readable"}),
		unreadableReceipt("ipa-info", "missing.ipa"),
		unreadableReceipt("pkg-info", "missing.pkg"),
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
