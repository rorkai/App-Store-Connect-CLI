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

const webReviewBodyFileRefusal = "--body-file must be a regular file inside the working directory"

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
// directory and returns it with a secret file that sits outside it.
func chdirWebReviewBodyFileProject(t *testing.T) (project string, secret string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	secret = filepath.Join(root, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	return project, secret
}

func TestWebReviewBodyFileRefusesPathsOutsideWorkingDirectory(t *testing.T) {
	tests := []struct {
		name string
		// path builds the refused --body-file argument inside a fresh project.
		path func(t *testing.T, project, secret string) string
	}{
		{name: "absolute path outside the working directory", path: func(_ *testing.T, _, secret string) string { return secret }},
		{name: "parent traversal", path: func(*testing.T, string, string) string { return "../secret.txt" }},
		{name: "symlink to a file outside", path: func(t *testing.T, project, secret string) string {
			if err := os.Symlink(secret, filepath.Join(project, "link.txt")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return "link.txt"
		}},
		{name: "directory", path: func(t *testing.T, project, _ string) string {
			if err := os.Mkdir(filepath.Join(project, "dir"), 0o700); err != nil {
				t.Fatal(err)
			}
			return "dir"
		}},
		{name: "FIFO", path: func(t *testing.T, project, _ string) string {
			makeWebReviewBodyFIFO(t, filepath.Join(project, "pipe"))
			return "pipe"
		}},
	}
	for _, test := range tests {
		for _, command := range webReviewBodyFileCommands {
			t.Run(test.name+" with "+command.name, func(t *testing.T) {
				project, secret := chdirWebReviewBodyFileProject(t)
				path := test.path(t, project, secret)
				requested := stubWebReviewDraftSequence(t, nil)
				d, _, _ := newWebTestDeps()
				cmd := command.command(d)
				args := append(append([]string{}, command.args...), "--body-file", path, "--confirm", "--output", "json")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := cmd.Exec(context.Background(), nil)
				if err == nil || !strings.Contains(err.Error(), webReviewBodyFileRefusal) {
					t.Fatalf("error = %v, want %q", err, webReviewBodyFileRefusal)
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want a usage-class error (exit 2)", err)
				}
				if len(*requested) != 0 {
					t.Fatalf("requests = %#v, want none after a refusal", *requested)
				}
			})
		}
	}
}

func TestWebReviewBodyFileAcceptsRegularFileInsideWorkingDirectory(t *testing.T) {
	project, _ := chdirWebReviewBodyFileProject(t)
	if err := os.WriteFile(filepath.Join(project, "reply.txt"), []byte("Reply body."), 0o600); err != nil {
		t.Fatal(err)
	}
	errSessionReached := errors.New("session reached")
	tests := []struct {
		name string
		path string
	}{
		{name: "relative path", path: "reply.txt"},
		{name: "absolute path inside the working directory", path: filepath.Join(project, "reply.txt")},
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
				d, _, _ := newWebTestDeps()
				cmd := command.command(d)
				args := append(append([]string{}, command.args...), "--body-file", test.path, "--confirm", "--output", "json")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, errSessionReached) || calls != 1 {
					t.Fatalf("error = %v, session calls = %d; want the body read and the session reached", err, calls)
				}
			})
		}
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
