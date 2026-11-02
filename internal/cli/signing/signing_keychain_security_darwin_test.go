//go:build darwin

package signing

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestRunKeychainUtilityFailsInsteadOfTruncatingOutput(t *testing.T) {
	stdout, _, err := runKeychainUtility(context.Background(), "/bin/sh", nil, 1024, "-c", "head -c 4096 /dev/zero")
	if err == nil || !strings.Contains(err.Error(), "exceeded the 1024-byte limit") {
		t.Fatalf("err = %v", err)
	}
	if len(stdout) != 1024 {
		t.Fatalf("stdout length = %d", len(stdout))
	}
	stdout, _, err = runKeychainUtility(context.Background(), "/bin/sh", nil, 1024, "-c", "printf keychain")
	if err != nil || string(stdout) != "keychain" {
		t.Fatalf("stdout = %q err = %v", stdout, err)
	}
}

func TestSecurityUtilitiesReadStdinUnderControllingTerminal(t *testing.T) {
	const keychainEnv = "ASC_SIGNING_TERMINAL_TEST_KEYCHAIN"
	if keychain := os.Getenv(keychainEnv); keychain != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for name, run := range map[string]persistentSigningUtilityRunner{
			"runSigningUtility":          runSigningUtility,
			"defaultRunKeychainSecurity": defaultRunKeychainSecurity,
		} {
			_, stderr, _ := run(ctx, []byte("unused\n"), "unlock-keychain", keychain)
			if !strings.Contains(string(stderr), "could not be found") {
				t.Errorf("%s did not read stdin: stderr = %q, context = %v", name, stderr, ctx.Err())
			}
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestSecurityUtilitiesReadStdinUnderControllingTerminal$")
	cmd.Env = append(os.Environ(), keychainEnv+"="+filepath.Join(t.TempDir(), "missing.keychain-db"))
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ptmx.Close() }()
	output, _ := io.ReadAll(ptmx)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}
