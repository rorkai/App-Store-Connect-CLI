package install

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const defaultSkillsPackage = "rorkai/app-store-connect-cli-skills"

var (
	lookupNpx      = exec.LookPath
	runCommand     = defaultRunCommand
	errNpxNotFound = errors.New("npx not found")
)

// InstallSkillsCommand returns the top-level `install-skills` command.
func InstallSkillsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("install-skills", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "install-skills",
		ShortUsage: "asc install-skills",
		ShortHelp:  "Install the asc skill pack globally for App Store Connect workflows.",
		LongHelp: `Install the asc skill pack globally for App Store Connect workflows.

Examples:
  asc install-skills`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := installSkills(ctx); err != nil {
				return fmt.Errorf("install skills: %w", err)
			}
			return nil
		},
	}
}

func installSkills(ctx context.Context) error {
	ctx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	path, err := lookupNpx("npx")
	if err != nil {
		return fmt.Errorf("%w; install Node.js to continue", errNpxNotFound)
	}

	// `npx add-skill` is deprecated upstream; use the new subcommand style.
	return runCommand(ctx, path, "--yes", "skills", "add", defaultSkillsPackage, "--global", "--agent", "codex", "--yes")
}

func defaultRunCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
