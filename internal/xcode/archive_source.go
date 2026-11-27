package xcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func readArchiveProjectCheckout(ctx context.Context, project string) (*asc.XcodeProjectCheckout, error) {
	project, err := filepath.EvalSymlinks(project)
	if err != nil {
		return nil, fmt.Errorf("resolve --project for --source-provenance: %w", err)
	}
	project, err = filepath.Abs(project)
	if err != nil {
		return nil, err
	}
	var environment []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "GIT_") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	git := func(args ...string) (string, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}, args...)...)
		command.Dir = filepath.Dir(project)
		command.Env = environment
		output := newBoundedToolOutput(4096)
		command.Stdout = output
		if err := command.Run(); err != nil {
			return "", fmt.Errorf("--source-provenance could not read the project checkout; use --project inside a Git checkout with a committed HEAD: %w", err)
		}
		if output.exceeded && args[0] != "status" {
			return "", fmt.Errorf("--source-provenance Git output exceeded its size limit")
		}
		return strings.TrimSpace(output.String()), nil
	}
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, project)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("--source-provenance Git checkout does not contain --project")
	}
	revision, err := git("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, err
	}
	status, err := git("status", "--porcelain=v1", "-z", "--untracked-files=normal", "--ignore-submodules=none")
	if err != nil {
		return nil, err
	}
	current, err := git("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, err
	}
	if current != revision {
		return nil, fmt.Errorf("--source-provenance project revision changed while observing it; retry with a stable checkout")
	}
	return &asc.XcodeProjectCheckout{Revision: revision, Dirty: status != "", ObservedAt: "buildStart"}, nil
}
