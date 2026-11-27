package web

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// webReviewBodyFileCommands are the three commands that read --body-file.
var webReviewBodyFileCommands = []struct {
	name    string
	command func(*shared.Deps) *ffcli.Command
	args    []string
}{
	{name: "reply", command: newWebReviewReplyCommand, args: []string{"--app", "app-1", "--thread-id", "thread-1"}},
	{name: "drafts create", command: newWebReviewDraftCreateCommand, args: []string{"--app", "app-1", "--thread-id", "thread-1"}},
	{name: "drafts update", command: newWebReviewDraftUpdateCommand, args: []string{"--app", "app-1", "--thread-id", "thread-1"}},
}

// chdirWebReviewBodyFileProject makes a project directory the process working
// directory and returns it with a reply template that sits outside it.
func chdirWebReviewBodyFileProject(t *testing.T) (project string, outside string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	outside = filepath.Join(root, "reply-template.txt")
	if err := os.WriteFile(outside, []byte("outside body"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	return project, outside
}

func TestWebReviewBodyFileRejectsNonRegularFilesAndSymlinks(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		// path builds the refused --body-file argument inside a fresh project.
		path func(t *testing.T, project, outside string) string
	}{
		{name: "symlink to a file outside", wantErr: "refusing to follow symlink", path: func(t *testing.T, project, outside string) string {
			if err := os.Symlink(outside, filepath.Join(project, "link.txt")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return "link.txt"
		}},
		{name: "directory", wantErr: "not a regular file", path: func(t *testing.T, project, _ string) string {
			if err := os.Mkdir(filepath.Join(project, "dir"), 0o700); err != nil {
				t.Fatal(err)
			}
			return "dir"
		}},
		{name: "FIFO", wantErr: "not a regular file", path: func(t *testing.T, project, _ string) string {
			makeWebReviewBodyFIFO(t, filepath.Join(project, "pipe"))
			return "pipe"
		}},
	}
	for _, test := range tests {
		for _, command := range webReviewBodyFileCommands {
			t.Run(test.name+" with "+command.name, func(t *testing.T) {
				project, outside := chdirWebReviewBodyFileProject(t)
				path := test.path(t, project, outside)
				requested := stubWebReviewDraftSequence(t, nil)
				d, _, _ := newWebTestDeps()
				cmd := command.command(d)
				args := append(append([]string{}, command.args...), "--body-file", path, "--confirm", "--output", "json")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want the existing file error classification", err)
				}
				if len(*requested) != 0 {
					t.Fatalf("requests = %#v, want none after a refusal", *requested)
				}
			})
		}
	}
}

func TestWebReviewBodyFileAcceptsRegularFilesWithDeprecation(t *testing.T) {
	project, outside := chdirWebReviewBodyFileProject(t)
	if err := os.WriteFile(filepath.Join(project, "reply.txt"), []byte("Reply body."), 0o600); err != nil {
		t.Fatal(err)
	}
	errSessionReached := errors.New("session reached")
	tests := []struct {
		name string
		path string
		warn bool
	}{
		{name: "relative path", path: "reply.txt"},
		{name: "absolute path inside the working directory", path: filepath.Join(project, "reply.txt")},
		{name: "absolute path outside the working directory", path: outside, warn: true},
		{name: "parent relative path", path: "../reply-template.txt", warn: true},
	}
	for _, test := range tests {
		for _, command := range webReviewBodyFileCommands {
			t.Run(test.name+" with "+command.name, func(t *testing.T) {
				original := resolveSessionFn
				t.Cleanup(func() { resolveSessionFn = original })
				calls := 0
				resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
					calls++
					return nil, "", errSessionReached
				}
				d, _, stderr := newWebTestDeps()
				cmd := command.command(d)
				args := append(append([]string{}, command.args...), "--body-file", test.path, "--confirm", "--output", "json")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, errSessionReached) || calls != 1 {
					t.Fatalf("error = %v, session calls = %d; want the body read and the session reached", err, calls)
				}
				if test.warn {
					if !strings.Contains(stderr.String(), "Warning: --body-file outside the working directory or through a symlink is deprecated") || !strings.Contains(stderr.String(), "use --message, or copy the file into the project directory") {
						t.Fatalf("expected deprecation guidance, got %q", stderr.String())
					}
				} else if stderr.Len() != 0 {
					t.Fatalf("unexpected warning for an in-directory file: %q", stderr.String())
				}
			})
		}
	}
}

func TestWebReviewBodyFilePreservesReadErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	d, _, stderr := newWebTestDeps()
	command := newWebReviewReplyCommand(d)
	if err := command.FlagSet.Parse([]string{"--thread-id", "thread-1", "--body-file", "missing.txt", "--confirm"}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if !errors.Is(err, os.ErrNotExist) || errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "read --body-file:") {
		t.Fatalf("error = %v, want wrapped file-not-found error", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("read error must be reported by the caller, got stderr %q", stderr.String())
	}
}

func TestWebReviewBodyFileRefusesOversizeBeforeSession(t *testing.T) {
	project, _ := chdirWebReviewBodyFileProject(t)
	oversize := strings.Repeat("a", int(webReviewDraftBodyMaxBytes)+1)
	if err := os.WriteFile(filepath.Join(project, "big.txt"), []byte(oversize), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range webReviewBodyFileCommands {
		t.Run(command.name, func(t *testing.T) {
			original := resolveSessionFn
			t.Cleanup(func() { resolveSessionFn = original })
			calls := 0
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				calls++
				return nil, "", errors.New("unexpected session resolution")
			}
			d, _, _ := newWebTestDeps()
			cmd := command.command(d)
			args := append(append([]string{}, command.args...), "--body-file", "big.txt", "--confirm", "--output", "json")
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatal(err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "--body-file exceeds the 1048576-byte size limit") {
				t.Fatalf("error = %v, want the oversize refusal", err)
			}
			if !errors.Is(err, flag.ErrHelp) || calls != 0 {
				t.Fatalf("error = %v, session calls = %d; want a usage error before any session", err, calls)
			}
		})
	}
}
