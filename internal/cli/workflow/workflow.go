package workflow

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	wf "github.com/rudrankriyam/App-Store-Connect-CLI/internal/workflow"
)

// WorkflowCommand returns the top-level workflow command group.
func WorkflowCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflow", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "workflow",
		ShortUsage: "asc workflow <subcommand> [flags]",
		ShortHelp:  "Run multi-step automation workflows.",
		LongHelp: `Define named, multi-step automation sequences in .asc/workflow.json.
Each workflow composes existing asc commands and shell commands.
The file supports JSONC comments ('//' and '/* */').
Hooks are supported at the definition level: before_all, after_all, and error.
stdout is JSON-only; step/hook command output streams to stderr.
Commands run via bash (with pipefail) when available, otherwise sh; at least one must be in PATH.
On failure, stdout remains JSON-only and includes a top-level error message plus hook results.

Security note:
  Workflows intentionally execute arbitrary shell commands.
  Only run workflow files you trust (especially when using --file).
  Treat .asc/workflow.json like code: review it before running.
  Be careful with --file: it can point to any path, not just .asc/workflow.json.
  Steps inherit your process environment; be careful with secrets.
  Declared step outputs are persisted in the run-state file; do not map secrets into outputs.
  In CI, avoid running workflows on untrusted PRs with secrets/tokens.
  asc workflow validate checks structure, not safety of commands.

Tips:
  Use asc workflow validate before running a new workflow file.
  Preview the plan with asc workflow run --dry-run <name>.
  Run-step outputs can be referenced later as ${steps.resolve_build.BUILD_ID}.
  For asc commands that declare outputs, usually pass --output json.
  A proven local Xcode -> TestFlight shape is: asc builds latest --next -> asc xcode archive -> asc xcode export -> asc publish testflight --group ... --wait.

Example workflow file (.asc/workflow.json):

{
  "env": {
    "APP_ID": "123456789",
    "VERSION": "1.0.0"
  },
  "before_all": "asc auth status",
  "after_all": "echo workflow_done",
  "error": "echo workflow_failed",
  "workflows": {
    "beta": {
      "description": "Distribute a build to a TestFlight group",
      "env": {
        "GROUP_ID": ""
      },
      "steps": [
        {
          "name": "resolve_build",
          "run": "asc builds latest --app $APP_ID --platform IOS --output json",
          "outputs": {
            "BUILD_ID": "$.id"
          }
        },
        {
          "name": "add_build_to_group",
          "run": "asc builds add-groups --build ${steps.resolve_build.BUILD_ID} --group $GROUP_ID"
        }
      ]
    },
    "release": {
      "description": "Run the full App Store release pipeline (validate, attach, submit)",
      "steps": [
        {
          "workflow": "preflight",
          "with": {
            "NOTE": "running private sub-workflow"
          }
        },
        {
          "name": "release",
          "run": "asc release run --app $APP_ID --version $VERSION --build $BUILD_ID --metadata-dir ./metadata/version/$VERSION --confirm"
        }
      ]
    },
    "preflight": {
      "private": true,
      "description": "Private helper workflow (callable only via workflow steps)",
      "steps": [
        {
          "name": "preflight",
          "run": "echo \"$NOTE\""
        }
      ]
    }
  }
}

After running the release workflow, monitor submission progress with:
  asc status --app "APP_ID"

Try it:
  asc workflow validate
  asc workflow list
  asc workflow run --dry-run beta
  asc workflow run beta BUILD_ID:123456789 GROUP_ID:abcdef
  asc workflow run release --resume beta-20260312T120000Z-deadbeef

More docs:
  https://github.com/rudrankriyam/App-Store-Connect-CLI/blob/main/docs/WORKFLOWS.md

Examples:
  asc workflow list
  asc workflow validate
  asc workflow run beta
  asc workflow run beta SUBMIT_BETA:true
  asc workflow run release VERSION:2.1.0
  asc workflow run --dry-run beta
  asc workflow run release --resume beta-20260312T120000Z-deadbeef`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			workflowRunCommand(),
			workflowValidateCommand(),
			workflowListCommand(),
		},
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}
}

func workflowRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflow run", flag.ExitOnError)
	filePath := fs.String("file", wf.DefaultPath, "Path to workflow.json")
	dryRun := fs.Bool("dry-run", false, "Preview steps without executing")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	resume := fs.String("resume", "", "Resume a prior workflow run by run ID")

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc workflow run [flags] <name> [KEY:VALUE ...]",
		ShortHelp:  "Run a named workflow.",
		LongHelp: `Run a named workflow from workflow.json.

Run state is persisted in a repo-local runs directory next to the workflow file.
Use --resume with the emitted run ID to continue a partially completed run without
rerunning already-persisted successful steps.
Resume automatically reuses the original workflow file, saved params, and persisted outputs.
Do not pass extra KEY:VALUE params with --resume.
If a step declares "outputs", the command must emit JSON on stdout; for asc commands,
usually pass --output json.
stdout stays machine-parseable JSON even on failure; step and hook output streams to stderr.

Security note:
  Workflows intentionally execute arbitrary shell commands.
  Only run workflow files you trust (especially when using --file).
  In CI, avoid running workflows on untrusted PRs with secrets/tokens.
  Declared step outputs are persisted in the run-state file; do not map secrets into outputs.

Tip: See "asc workflow --help" for a complete workflow.json example and file format tips.

Examples:
  asc workflow run beta
  asc workflow run beta BUILD_ID:123456789 GROUP_ID:abcdef
  asc workflow run --dry-run beta
  asc workflow run release --resume beta-20260312T120000Z-deadbeef`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return shared.UsageError("workflow name is required")
			}

			workflowName := args[0]
			paramArgs, err := parseRunTailArgs(args[1:], fs)
			if err != nil {
				return err
			}

			absPath, err := filepath.Abs(strings.TrimSpace(*filePath))
			if err != nil {
				return fmt.Errorf("workflow run: resolve path: %w", err)
			}

			def, err := wf.Load(absPath)
			if err != nil {
				return fmt.Errorf("workflow run: %w", err)
			}

			params, err := wf.ParseParams(paramArgs)
			if err != nil {
				return shared.UsageErrorf("%s", err)
			}
			if *dryRun && strings.TrimSpace(*resume) != "" {
				return shared.UsageError("--resume cannot be used with --dry-run")
			}
			if strings.TrimSpace(*resume) != "" && len(paramArgs) > 0 {
				return shared.UsageError("resume runs do not accept additional KEY:VALUE parameters")
			}

			stateDir := filepath.Join(filepath.Dir(absPath), "runs")

			result, err := wf.Run(ctx, def, wf.RunOptions{
				WorkflowName: workflowName,
				Params:       params,
				DryRun:       *dryRun,
				WorkflowFile: absPath,
				StateDir:     stateDir,
				ResumeRunID:  strings.TrimSpace(*resume),
				// Keep stdout machine-parseable JSON; stream step output to stderr.
				Stdout: os.Stderr,
				Stderr: os.Stderr,
			})
			if err != nil {
				if result != nil {
					_ = printJSON(os.Stdout, result, *pretty)
					return shared.NewReportedError(err)
				}
				return fmt.Errorf("workflow run: %w", err)
			}

			return printJSON(os.Stdout, result, *pretty)
		},
	}
}

func workflowValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflow validate", flag.ExitOnError)
	filePath := fs.String("file", wf.DefaultPath, "Path to workflow.json")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc workflow validate [flags]",
		ShortHelp:  "Validate workflow.json for errors and cycles.",
		LongHelp: `Validate workflow.json for structure, references, cycles, and output declarations.
This checks schema and wiring only; it does not assess shell-command safety.

Examples:
  asc workflow validate
  asc workflow validate --file ./.asc/workflow.json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			absPath, err := filepath.Abs(strings.TrimSpace(*filePath))
			if err != nil {
				return fmt.Errorf("workflow validate: resolve path: %w", err)
			}

			def, err := wf.LoadUnvalidated(absPath)
			if err != nil {
				return fmt.Errorf("workflow validate: %w", err)
			}

			errs := wf.Validate(def)

			type validationResult struct {
				Valid  bool                  `json:"valid"`
				Errors []*wf.ValidationError `json:"errors,omitempty"`
			}
			result := validationResult{
				Valid:  len(errs) == 0,
				Errors: errs,
			}

			if printErr := printJSON(os.Stdout, result, *pretty); printErr != nil {
				return printErr
			}

			if !result.Valid {
				return shared.NewReportedError(
					fmt.Errorf("workflow validate: found %d error(s)", len(errs)),
				)
			}
			return nil
		},
	}
}

func workflowListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflow list", flag.ExitOnError)
	filePath := fs.String("file", wf.DefaultPath, "Path to workflow.json")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	all := fs.Bool("all", false, "Include private workflows in listing")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc workflow list [flags]",
		ShortHelp:  "List available workflows.",
		LongHelp: `List public workflows from workflow.json.
Use --all to include private helper workflows.

Examples:
  asc workflow list
  asc workflow list --all`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			absPath, err := filepath.Abs(strings.TrimSpace(*filePath))
			if err != nil {
				return fmt.Errorf("workflow list: resolve path: %w", err)
			}

			def, err := wf.LoadUnvalidated(absPath)
			if err != nil {
				return fmt.Errorf("workflow list: %w", err)
			}

			type workflowInfo struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Private     bool   `json:"private,omitempty"`
				StepCount   int    `json:"step_count"`
			}

			workflows := make([]workflowInfo, 0, len(def.Workflows))
			for name, w := range def.Workflows {
				if w.Private && !*all {
					continue
				}
				workflows = append(workflows, workflowInfo{
					Name:        name,
					Description: w.Description,
					Private:     w.Private,
					StepCount:   len(w.Steps),
				})
			}

			sort.Slice(workflows, func(i, j int) bool {
				return workflows[i].Name < workflows[j].Name
			})

			return printJSON(os.Stdout, workflows, *pretty)
		},
	}
}

func parseRunTailArgs(args []string, fs *flag.FlagSet) ([]string, error) {
	params := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			params = append(params, args[i+1:]...)
			break
		}

		if strings.HasPrefix(token, "--") {
			nameValue := strings.TrimPrefix(token, "--")
			name, value, hasValue := strings.Cut(nameValue, "=")

			switch name {
			case "dry-run", "pretty":
				if !hasValue {
					value = "true"
				}
				if err := fs.Set(name, value); err != nil {
					return nil, shared.UsageErrorf("invalid value for --%s: %v", name, err)
				}
				continue
			case "file", "resume":
				if !hasValue {
					if i+1 >= len(args) {
						return nil, shared.UsageErrorf("--%s requires a value", name)
					}
					if isRunTailFlagToken(args[i+1]) || strings.HasPrefix(args[i+1], "--") {
						return nil, shared.UsageErrorf("--%s requires a value", name)
					}
					i++
					value = args[i]
				}
				if strings.TrimSpace(value) == "" {
					return nil, shared.UsageErrorf("--%s requires a value", name)
				}
				if err := fs.Set(name, value); err != nil {
					return nil, shared.UsageErrorf("invalid value for --%s: %v", name, err)
				}
				continue
			default:
				return nil, shared.UsageErrorf("unknown flag %q", token)
			}
		}

		if strings.HasPrefix(token, "-") {
			return nil, shared.UsageErrorf("unknown flag %q", token)
		}

		params = append(params, token)
	}
	return params, nil
}

func isRunTailFlagToken(token string) bool {
	if !strings.HasPrefix(token, "--") {
		return false
	}
	nameValue := strings.TrimPrefix(token, "--")
	name, _, _ := strings.Cut(nameValue, "=")
	switch name {
	case "dry-run", "pretty", "file", "resume":
		return true
	default:
		return false
	}
}

// printJSON encodes data as JSON to the writer.
func printJSON(w io.Writer, data any, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}
