//go:build darwin

package signing

import (
	"context"
	"strings"
	"testing"
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
