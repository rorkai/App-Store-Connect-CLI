package doctor

import (
	"os"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestResolveProfile(t *testing.T) {
	tests := []struct {
		name        string
		flagProfile string
		cfgDefault  string
		want        string
	}{
		{"flag wins", " personal ", "default", "personal"},
		{"cfg default", "", " default ", "default"},
		{"both empty", " ", "\t", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveProfile(tt.flagProfile, tt.cfgDefault); got != tt.want {
				t.Fatalf("ResolveProfile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectCredential(t *testing.T) {
	cfg := &config.Config{
		KeyID:          "topKey",
		IssuerID:       "topIssuer",
		PrivateKeyPath: "/tmp/top.p8",
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{Name: "personal", KeyID: "k1", IssuerID: "i1", PrivateKeyPath: "/tmp/p1.p8"},
			{Name: "work", KeyID: "k2", IssuerID: "i2", PrivateKeyPath: "/tmp/p2.p8"},
		},
	}

	{
		k, i, p, ok := SelectCredential(nil, "personal")
		if ok {
			t.Fatalf("expected not found, got %v %v %v", k, i, p)
		}
	}

	{
		k, i, p, ok := SelectCredential(cfg, " personal ")
		if !ok || k != "k1" || i != "i1" || p != "/tmp/p1.p8" {
			t.Fatalf("profile key match unexpected: ok=%v k=%q i=%q p=%q", ok, k, i, p)
		}
	}

	{
		k, i, p, ok := SelectCredential(cfg, "missing")
		if !ok || k != "topKey" || i != "topIssuer" || p != "/tmp/top.p8" {
			t.Fatalf("fallback top-level unexpected: ok=%v k=%q i=%q p=%q", ok, k, i, p)
		}
	}

	{
		empty := &config.Config{}
		_, _, _, ok := SelectCredential(empty, "")
		if ok {
			t.Fatalf("expected not found")
		}
	}
}

func TestBuildReport_ReadablePrivateKey(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "authkey-*.p8")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cfg := &config.Config{KeyID: "k", IssuerID: "i", PrivateKeyPath: tmp.Name()}
	report := BuildReport(tmp.Name(), cfg, nil, "")
	if !report.OK {
		for _, c := range report.Checks {
			t.Logf("%s ok=%v msg=%q", c.Name, c.OK, c.Message)
		}
		t.Fatalf("expected report.OK=true")
	}

	// Ensure readability check is present and OK.
	found := false
	for _, c := range report.Checks {
		if c.Name == "private_key_path.readable" {
			found = true
			if !c.OK {
				t.Fatalf("expected readable check OK, got msg=%q", c.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected private_key_path.readable check")
	}
}
