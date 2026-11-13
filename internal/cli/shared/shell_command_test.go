package shared

import (
	"runtime"
	"testing"
)

func TestShellCommandArgsSkipLoginShell(t *testing.T) {
	args := shellCommandArgs("printf '123456\\n'")

	if runtime.GOOS == "windows" {
		want := []string{"/d", "/s", "/c", "printf '123456\\n'"}
		if len(args) != len(want) {
			t.Fatalf("expected %d args, got %d (%v)", len(want), len(args), args)
		}
		for i, part := range want {
			if args[i] != part {
				t.Fatalf("expected arg %d to be %q, got %q", i, part, args[i])
			}
		}
		return
	}

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d (%v)", len(args), args)
	}
	if args[0] != "-c" {
		t.Fatalf("expected non-login shell flag %q, got %q", "-c", args[0])
	}
}
