package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
)

type keychainRunnerCall struct {
	args  []string
	stdin string
}

func TestSigningKeychainDeleteRefusesLoginBeforeSecurity(t *testing.T) {
	called := false
	setKeychainRunner(t, func(context.Context, []byte, ...string) ([]byte, []byte, error) {
		called = true
		return nil, nil, nil
	})
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

func TestSigningKeychainDeleteRefusesLinksToProtectedKeychains(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keychains := filepath.Join(home, "Library", "Keychains")
	if err := os.MkdirAll(keychains, 0o700); err != nil {
		t.Fatal(err)
	}
	login := filepath.Join(keychains, "login.keychain-db")
	if err := os.WriteFile(login, []byte("keychain"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	symlink := filepath.Join(work, "release.keychain-db")
	if err := os.Symlink(login, symlink); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(work, "ci.keychain-db")
	if err := os.Link(login, hardlink); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{symlink, hardlink} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var calls []keychainRunnerCall
			setKeychainRunner(t, recordingKeychainRunner(&calls, nil))
			cmd := SigningKeychainDeleteCommand()
			if err := cmd.Parse([]string{"--keychain", path, "--confirm"}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("error = %v", err)
			}
			if len(calls) != 0 {
				t.Fatalf("security ran for a protected keychain: %#v", calls)
			}
		})
	}
}

func TestSigningKeychainDeleteRefusesDefaultKeychain(t *testing.T) {
	keychain := writeTestKeychainFile(t)
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
		if args[0] == "default-keychain" {
			return []byte("    \"" + keychain + "\"\n"), nil, nil
		}
		return nil, nil, nil
	}))
	cmd := SigningKeychainDeleteCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "default keychain") {
		t.Fatalf("error = %v", err)
	}
	assertNoKeychainCall(t, calls, "delete-keychain")
}

func TestSigningKeychainDeleteRefusesWhenDefaultKeychainIsUnknown(t *testing.T) {
	keychain := writeTestKeychainFile(t)
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
		if args[0] == "default-keychain" {
			return nil, []byte("boom"), errors.New("exit status 1")
		}
		return nil, nil, nil
	}))
	cmd := SigningKeychainDeleteCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "default keychain") {
		t.Fatalf("error = %v", err)
	}
	assertNoKeychainCall(t, calls, "delete-keychain")
}

func TestSigningKeychainDeleteResolvesDedicatedKeychain(t *testing.T) {
	keychain := writeTestKeychainFile(t)
	link := filepath.Join(t.TempDir(), "link.keychain-db")
	if err := os.Symlink(keychain, link); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(keychain)
	if err != nil {
		t.Fatal(err)
	}
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
		if args[0] == "default-keychain" {
			return []byte("    \"/Users/example/Library/Keychains/login.keychain-db\"\n"), nil, nil
		}
		return nil, nil, nil
	}))
	cmd := SigningKeychainDeleteCommand()
	if err := cmd.Parse([]string{"--keychain", link, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, _ := captureOutput(t, func() {
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	last := calls[len(calls)-1]
	if strings.Join(last.args, " ") != "delete-keychain "+physical {
		t.Fatalf("calls = %#v", calls)
	}
	if !strings.Contains(stdout, `"keychainPath":"`+physical+`"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestSigningKeychainRequiresExistingKeychainBeforeSecurity(t *testing.T) {
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, nil))
	cmd := SigningKeychainLockCommand()
	if err := cmd.Parse([]string{"--keychain", filepath.Join(t.TempDir(), "missing.keychain-db")}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("error = %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("security ran for a missing keychain: %#v", calls)
	}
}

func TestSigningKeychainUnlockKeepsPasswordOffArgvAndIsRepeatable(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "s3cret\n")
	keychain := writeTestKeychainFile(t)
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, nil))
	for range 2 {
		cmd := SigningKeychainUnlockCommand()
		if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile}); err != nil {
			t.Fatal(err)
		}
		captureOutput(t, func() {
			if err := cmd.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %#v", calls)
	}
	for _, call := range calls {
		if call.args[0] != "unlock-keychain" || len(call.args) != 2 || call.stdin != "s3cret\n" {
			t.Fatalf("call = %#v", call)
		}
		if strings.Contains(strings.Join(call.args, " "), "s3cret") {
			t.Fatalf("password leaked into argv: %#v", call.args)
		}
	}
}

func TestSigningKeychainPasswordCommandsStopAfterFailedUnlock(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "s3cret\n")
	keychain := writeTestKeychainFile(t)
	for _, test := range []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "unlock", command: SigningKeychainUnlockCommand, args: []string{"--timeout", "600"}},
		{name: "set-timeout", command: SigningKeychainSetTimeoutCommand, args: []string{"--timeout", "600"}},
		{name: "set-partition-list", command: SigningKeychainSetPartitionListCommand},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls []keychainRunnerCall
			setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
				if args[0] == "unlock-keychain" {
					return nil, []byte("password to unlock " + keychain + ": security: SecKeychainUnlock " + keychain + ": The user name or passphrase you entered is not correct. s3cret\n"), errors.New("exit status 51")
				}
				return nil, nil, nil
			}))
			cmd := test.command()
			if err := cmd.Parse(append([]string{"--keychain", keychain, "--keychain-password-file", passwordFile}, test.args...)); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "passphrase you entered is not correct") {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "s3cret") || strings.Contains(err.Error(), "password to unlock") {
				t.Fatalf("error leaked the password or prompt: %v", err)
			}
			if len(calls) != 1 || calls[0].args[0] != "unlock-keychain" {
				t.Fatalf("security kept running after a failed unlock: %#v", calls)
			}
		})
	}
}

func TestSigningKeychainSetTimeoutPreservesLockOnSleep(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "s3cret\n")
	keychain := writeTestKeychainFile(t)
	physical, _ := filepath.EvalSymlinks(keychain)
	for _, test := range []struct {
		name string
		info string
		args []string
		want string
	}{
		{name: "timeout keeps lock-on-sleep", info: "lock-on-sleep no-timeout", args: []string{"--timeout", "600"}, want: "set-keychain-settings -l -u -t 600 " + physical},
		{name: "no-timeout keeps lock-on-sleep", info: "lock-on-sleep timeout=300s", args: []string{"--no-timeout"}, want: "set-keychain-settings -l " + physical},
		{name: "timeout without lock-on-sleep", info: "no-timeout", args: []string{"--timeout", "30"}, want: "set-keychain-settings -u -t 30 " + physical},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls []keychainRunnerCall
			setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
				if args[0] == "show-keychain-info" {
					return nil, []byte("Keychain \"" + physical + "\" " + test.info + "\n"), nil
				}
				return nil, nil, nil
			}))
			cmd := SigningKeychainSetTimeoutCommand()
			if err := cmd.Parse(append([]string{"--keychain", keychain, "--keychain-password-file", passwordFile, "--output", "json"}, test.args...)); err != nil {
				t.Fatal(err)
			}
			captureOutput(t, func() {
				if err := cmd.Run(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
			if len(calls) != 3 || calls[0].args[0] != "unlock-keychain" || calls[1].args[0] != "show-keychain-info" {
				t.Fatalf("calls = %#v", calls)
			}
			if got := strings.Join(calls[2].args, " "); got != test.want {
				t.Fatalf("settings = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSigningKeychainSetPartitionListSendsPasswordOnStdin(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "pa\"ss\\word\n")
	keychain := writeTestKeychainFile(t)
	physical, _ := filepath.EvalSymlinks(keychain)
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, nil))
	cmd := SigningKeychainSetPartitionListCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	captureOutput(t, func() {
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if len(calls) != 2 {
		t.Fatalf("calls = %#v", calls)
	}
	want := "set-key-partition-list -S apple-tool:,apple:,codesign: -s -t private " + physical
	if got := strings.Join(calls[1].args, " "); got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
	for _, call := range calls {
		if call.stdin != "pa\"ss\\word\n" || strings.Contains(strings.Join(call.args, " "), "ss\\word") {
			t.Fatalf("call = %#v", call)
		}
	}
}

func TestSigningKeychainSetPartitionListUnlocksBeforeIdentityLookup(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "s3cret\n")
	keychain := writeTestKeychainFile(t)
	certificate := newTestCodeSigningCertificate(t, "Release")
	sha := sha256Hex(certificate.Raw)
	var order []string
	setKeychainRunner(t, func(_ context.Context, _ []byte, args ...string) ([]byte, []byte, error) {
		order = append(order, args[0])
		switch args[0] {
		case "find-identity":
			return []byte("  1) " + sha1Hex(certificate.Raw) + " \"Release\"\n     1 identities found\n"), nil, nil
		case "find-certificate":
			return pemCertificate(certificate), nil, nil
		}
		return nil, nil, nil
	})
	cmd := SigningKeychainSetPartitionListCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile, "--identity-sha256", strings.ToUpper(sha), "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	captureOutput(t, func() {
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Join(order, ",") != "unlock-keychain,find-identity,find-certificate,set-key-partition-list" {
		t.Fatalf("order = %#v", order)
	}
}

func TestSigningKeychainSetPartitionListRequiresIdentityNotJustCertificate(t *testing.T) {
	passwordFile := writeTestPasswordFile(t, "s3cret\n")
	keychain := writeTestKeychainFile(t)
	certificate := newTestCodeSigningCertificate(t, "Intermediate")
	var calls []keychainRunnerCall
	setKeychainRunner(t, recordingKeychainRunner(&calls, func(args []string) ([]byte, []byte, error) {
		switch args[0] {
		case "find-identity":
			return []byte("     0 identities found\n"), nil, nil
		case "find-certificate":
			return pemCertificate(certificate), nil, nil
		}
		return nil, nil, nil
	}))
	cmd := SigningKeychainSetPartitionListCommand()
	if err := cmd.Parse([]string{"--keychain", keychain, "--keychain-password-file", passwordFile, "--identity-sha256", sha256Hex(certificate.Raw)}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("error = %v", err)
	}
	assertNoKeychainCall(t, calls, "set-key-partition-list")
}

func TestSigningKeychainListReportsLockStateWithoutPrompting(t *testing.T) {
	identity := newTestCodeSigningCertificate(t, "Apple Distribution: Example (ABCDE12345)")
	caCertificate := newTestCodeSigningCertificate(t, "Example CA")
	dump := append(pemCertificate(caCertificate), pemCertificate(identity)...)
	setKeychainRunner(t, func(_ context.Context, _ []byte, args ...string) ([]byte, []byte, error) {
		switch args[0] {
		case "list-keychains":
			return []byte("    \"/tmp/release.keychain-db\"\n    \"/tmp/missing.keychain-db\"\n"), nil, nil
		case "find-identity":
			return []byte("Policy: Code Signing\n  Matching identities\n  1) " + strings.ToUpper(sha1Hex(identity.Raw)) + " \"Apple Distribution: Example (ABCDE12345)\" (CSSMERR_TP_NOT_TRUSTED)\n     1 identities found\n"), nil, nil
		case "find-certificate":
			return dump, nil, nil
		default:
			t.Fatalf("list ran a command that can prompt or mutate: %#v", args)
			return nil, nil, nil
		}
	})
	previous := keychainLockState
	keychainLockState = func(path string) (bool, bool, error) {
		if path == "/tmp/missing.keychain-db" {
			return false, false, nil
		}
		return true, true, nil
	}
	t.Cleanup(func() { keychainLockState = previous })

	cmd := SigningKeychainListCommand()
	if err := cmd.Parse([]string{"--output", "json"}); err != nil {
		t.Fatal(err)
	}
	stdout, _ := captureOutput(t, func() {
		if err := cmd.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	var result struct {
		Keychains []struct {
			Path         string `json:"path"`
			Exists       bool   `json:"exists"`
			InSearchList bool   `json:"inSearchList"`
			Locked       bool   `json:"locked"`
			Identities   []struct {
				SHA256     string `json:"sha256"`
				SHA1       string `json:"sha1"`
				CommonName string `json:"commonName"`
				ExpiresAt  string `json:"expiresAt"`
			} `json:"identities"`
		} `json:"keychains"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	if len(result.Keychains) != 2 {
		t.Fatalf("result = %+v", result)
	}
	release := result.Keychains[0]
	if !release.Exists || !release.Locked || !release.InSearchList || len(release.Identities) != 1 {
		t.Fatalf("release = %+v", release)
	}
	if release.Identities[0].SHA256 != sha256Hex(identity.Raw) || release.Identities[0].SHA1 != sha1Hex(identity.Raw) || release.Identities[0].CommonName != "Apple Distribution: Example (ABCDE12345)" || release.Identities[0].ExpiresAt == "" {
		t.Fatalf("identity = %+v", release.Identities[0])
	}
	missing := result.Keychains[1]
	if missing.Exists || missing.Locked || len(missing.Identities) != 0 {
		t.Fatalf("missing = %+v", missing)
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

func setKeychainRunner(t *testing.T, runner func(context.Context, []byte, ...string) ([]byte, []byte, error)) {
	t.Helper()
	previousGOOS := keychainHostGOOS
	previousRunner := runKeychainSecurity
	keychainHostGOOS = "darwin"
	runKeychainSecurity = runner
	t.Cleanup(func() {
		keychainHostGOOS = previousGOOS
		runKeychainSecurity = previousRunner
	})
}

func recordingKeychainRunner(calls *[]keychainRunnerCall, respond func([]string) ([]byte, []byte, error)) func(context.Context, []byte, ...string) ([]byte, []byte, error) {
	return func(_ context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
		*calls = append(*calls, keychainRunnerCall{args: append([]string(nil), args...), stdin: string(stdin)})
		if respond != nil {
			return respond(args)
		}
		return nil, nil, nil
	}
}

func assertNoKeychainCall(t *testing.T, calls []keychainRunnerCall, command string) {
	t.Helper()
	for _, call := range calls {
		if call.args[0] == command {
			t.Fatalf("%s ran: %#v", command, calls)
		}
	}
}

func writeTestKeychainFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "release.keychain-db")
	if err := os.WriteFile(path, []byte("keychain"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestPasswordFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestCodeSigningCertificate(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func pemCertificate(certificate *x509.Certificate) []byte {
	return []byte("SHA-256 hash: " + strings.ToUpper(sha256Hex(certificate.Raw)) + "\nSHA-1 hash: " + strings.ToUpper(sha1Hex(certificate.Raw)) + "\n" +
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sha1Hex(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}
