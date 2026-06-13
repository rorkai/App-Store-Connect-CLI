package users

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestWarnDeprecatedUserRolesEmitsWarning(t *testing.T) {
	stderr := captureStderr(t, func() {
		warnDeprecatedUserRoles([]string{"DEVELOPER", "access_to_reports"})
	})
	if stderr != deprecatedAccessToReportsWarning+"\n" {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
}

func TestWarnDeprecatedUserRolesNoWarningForOtherRoles(t *testing.T) {
	stderr := captureStderr(t, func() {
		warnDeprecatedUserRoles([]string{"ADMIN", "DEVELOPER"})
	})
	if stderr != "" {
		t.Fatalf("expected no warning, got %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stderr = old
	return <-done
}
