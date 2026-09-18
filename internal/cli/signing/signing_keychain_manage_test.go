package signing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigningKeychainDeleteRefusesLoginBeforeSecurity(t *testing.T) {
	called := false
	restore := setKeychainRunner(t, func(context.Context, []byte, ...string) ([]byte, []byte, error) {
		called = true
		return nil, nil, nil
	})
	defer restore()
	cmd := SigningKeychainDeleteCommand()
	if err := cmd.Parse([]string{"--keychain", "login.keychain-db", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "login or System") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("security ran for a refused keychain")
	}
}

func TestSigningKeychainDeleteRequiresConfirm(t *testing.T) {
	cmd := SigningKeychainDeleteCommand()
	if err := cmd.Parse([]string{"--keychain", filepath.Join(t.TempDir(), "app.keychain-db")}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--confirm is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestSigningKeychainUnlockKeepsPasswordOffArgvAndIsRepeatable(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	var stdin []byte
	restore := setKeychainRunner(t, func(_ context.Context, input []byte, args ...string) ([]byte, []byte, error) {
		calls = append(calls, args)
		stdin = append([]byte(nil), input...)
		return nil, nil, nil
	})
	defer restore()
	keychain := filepath.Join(t.TempDir(), "release.keychain-db")
	for range 2 {
		cmd := SigningKeychainUnlockCommand()
		if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile}); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 2 || strings.Join(calls[0], " ") != "-i" {
		t.Fatalf("argv = %#v", calls)
	}
	script := string(stdin)
	if strings.Contains(strings.Join(calls[0], " "), "s3cret") || !strings.Contains(script, "unlock-keychain -p \"s3cret\"") {
		t.Fatalf("stdin=%q argv=%#v", script, calls[0])
	}
}

func TestSigningKeychainListParsesIdentities(t *testing.T) {
	restore := setKeychainRunner(t, func(_ context.Context, _ []byte, args ...string) ([]byte, []byte, error) {
		switch args[0] {
		case "list-keychains":
			return []byte("\"/tmp/release.keychain-db\"\n"), nil, nil
		case "show-keychain-info":
			return []byte("Keychain \"/tmp/release.keychain-db\" no-timeout\n"), nil, nil
		case "find-certificate":
			return []byte("SHA-256 hash: " + strings.Repeat("ab", 32) + "\n-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"), nil, nil
		default:
			t.Fatalf("unexpected args %#v", args)
			return nil, nil, nil
		}
	})
	defer restore()
	cmd := SigningKeychainListCommand()
	if err := cmd.Parse([]string{"--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSigningKeychainSetPartitionListUnlocksBeforeIdentityLookup(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keychain := filepath.Join(t.TempDir(), "release.keychain-db")
	sha := strings.Repeat("ab", 32)
	var order []string
	unlocked := false
	restore := setKeychainRunner(t, func(_ context.Context, input []byte, args ...string) ([]byte, []byte, error) {
		order = append(order, args[0])
		if args[0] == "-i" && strings.Contains(string(input), "unlock-keychain") && !strings.Contains(string(input), "set-key-partition-list") {
			unlocked = true
			return nil, nil, nil
		}
		if args[0] == "find-certificate" {
			if !unlocked {
				t.Fatal("identity lookup ran before unlock")
			}
			return []byte("SHA-256 hash: " + sha + "\n"), nil, nil
		}
		return nil, nil, nil
	})
	defer restore()
	cmd := SigningKeychainSetPartitionListCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile, "--identity-sha256", sha, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(order) < 2 || order[0] != "-i" || order[1] != "find-certificate" {
		t.Fatalf("order = %#v", order)
	}
}

func TestSigningKeychainRefusesNonDarwin(t *testing.T) {
	previous := keychainHostGOOS
	keychainHostGOOS = "linux"
	t.Cleanup(func() { keychainHostGOOS = previous })
	cmd := SigningKeychainLockCommand()
	if err := cmd.Parse([]string{"--keychain", filepath.Join(t.TempDir(), "app.keychain-db")}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "supported only on macOS") {
		t.Fatalf("error = %v", err)
	}
}

func setKeychainRunner(t *testing.T, runner func(context.Context, []byte, ...string) ([]byte, []byte, error)) func() {
	t.Helper()
	previousGOOS := keychainHostGOOS
	previousRunner := runKeychainSecurity
	keychainHostGOOS = "darwin"
	runKeychainSecurity = runner
	return func() {
		keychainHostGOOS = previousGOOS
		runKeychainSecurity = previousRunner
	}
}
