package distribute

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// NewDistributeCommand returns the local distribution command group, built
// on d.
func NewDistributeCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("distribute", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "distribute",
		ShortUsage: "asc distribute <subcommand> [flags]",
		ShortHelp:  "Plan, execute, inspect, and publish iOS distribution artifacts.",
		LongHelp: `Plan, execute, inspect, and publish provider-neutral iOS release-testing artifacts.

Inspect and prepare perform local, deterministic work without App Store Connect,
a keychain, a storage provider, or a network. Publish is an explicit networked
step to a caller-owned S3-compatible endpoint. Plan, apply, resume, status, and
verify compose those primitives into an agent-safe private distribution run.

Examples:
  asc distribute inspect --ipa "./App.ipa" --output json
  asc distribute prepare --ipa "./App.ipa" --channel "pull-request-42"
  asc distribute plan --archive-path "./App.xcarchive" --config ".asc/distribution.json" --plan ".asc/distribution/plan.json"
  asc distribute publish --help`,
		FlagSet: fs,
		Subcommands: []*ffcli.Command{
			inspectCommand(d),
			prepareCommand(d),
			PublishCommand(),
			distributionPlanCommand(),
			distributionApplyCommand(),
			distributionResumeCommand(),
			distributionStatusCommand(),
			distributionVerifyCommand(),
		},
		UsageFunc: d.DefaultUsageFunc,
		Exec:      func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func inspectCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("inspect", flag.ExitOnError)
	ipaPath := fs.String("ipa", "", "Path to the IPA to inspect")
	includeDevices := fs.Bool("include-devices", false, "Include raw registered-device UDIDs in inspection output")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "inspect",
		ShortUsage: "asc distribute inspect --ipa PATH [--include-devices] [flags]",
		ShortHelp:  "Inspect an IPA for release-testing distribution readiness.",
		LongHelp: `Inspect an IPA without persistent extraction or access to Apple services.

Verification uses bounded private temporary materialization and removes it when
the command exits.

Raw device UDIDs are omitted by default. Use --include-devices only when the
consumer explicitly needs the registered-device list.

Examples:
  asc distribute inspect --ipa "./App.ipa"
  asc distribute inspect --ipa "./App.ipa" --include-devices --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if strings.TrimSpace(*ipaPath) == "" {
				return d.UsageError("--ipa is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			file, size, err := openIPA(d, *ipaPath)
			if err != nil {
				return fmt.Errorf("distribute inspect: %w", err)
			}
			defer file.Close()
			result, err := distribution.InspectIPAContext(ctx, file, size, distribution.InspectOptions{IncludeDevices: *includeDevices})
			if err != nil {
				return fmt.Errorf("distribute inspect: %w", err)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return printInspection(d, result, *output.Output, *output.Pretty)
		},
	}
}

func prepareCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("prepare", flag.ExitOnError)
	ipaPath := fs.String("ipa", "", "Path to the IPA to prepare")
	outputDir := fs.String("output-dir", "", "Exact output bundle directory (default: deterministic path under .asc/distribution)")
	title := fs.String("title", "", "Override the app title in bundle metadata")
	channel := fs.String("channel", "", "Logical build channel recorded as provenance")
	sourceRevision := fs.String("source-revision", "", "Source revision recorded as provenance")
	sourceURL := fs.String("source-url", "", "Absolute HTTPS source URL without credentials")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "prepare",
		ShortUsage: "asc distribute prepare --ipa PATH [--output-dir DIR] [flags]",
		ShortHelp:  "Prepare an immutable provider-neutral distribution bundle.",
		LongHelp: `Prepare a metadata-valid ad hoc IPA as bundle.json plus payload/app.ipa.

The command records separate bounded checks for provisioning-profile CMS
integrity, Apple profile trust, and the main executable's signature/profile
certificate binding. On macOS it verifies the complete main app, nested code,
resource seal, signed entitlements, and signer/profile certificate binding.
Preparation fails closed before writing unless every required check is verified.

The command never overwrites a bundle. An exactly equivalent destination is
reused; any incomplete or different destination is a conflict. No install URL,
web page, object-store credential, or raw device UDID is written.

Examples:
  asc distribute prepare --ipa "./App.ipa"
  asc distribute prepare --ipa "./App.ipa" --channel "pull-request-42" --source-revision "abc123"
  asc distribute prepare --ipa "./App.ipa" --output-dir "./artifacts/install-bundle"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if strings.TrimSpace(*ipaPath) == "" {
				return d.UsageError("--ipa is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			prepareOptions := distribution.PrepareOptions{
				Title: *title, Channel: *channel, SourceRevision: *sourceRevision, SourceURL: *sourceURL,
			}
			if err := distribution.ValidatePrepareOptions(prepareOptions); err != nil {
				return d.UsageError(err.Error())
			}
			root, relativeOutput, err := resolveOutput(d, *outputDir)
			if err != nil {
				return d.UsageError(err.Error())
			}
			file, size, err := openIPA(d, *ipaPath)
			if err != nil {
				return fmt.Errorf("distribute prepare: %w", err)
			}
			defer file.Close()
			prepareOptions.Root = root
			prepareOptions.OutputDir = relativeOutput
			result, err := distribution.PrepareIPAContext(ctx, file, size, prepareOptions)
			if err != nil {
				return fmt.Errorf("distribute prepare: %w", err)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return printPrepareResult(d, result, *output.Output, *output.Pretty)
		},
	}
}

func openIPA(d *shared.Deps, pathValue string) (*os.File, int64, error) {
	absolute, err := d.Abs(strings.TrimSpace(pathValue))
	if err != nil {
		return nil, 0, fmt.Errorf("resolve IPA path: %w", err)
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, 0, fmt.Errorf("open IPA root: %w", err)
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.Base(absolute))
	if err != nil {
		return nil, 0, fmt.Errorf("open IPA: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("inspect IPA: %w", err)
	}
	return file, info.Size(), nil
}

func resolveOutput(d *shared.Deps, requested string) (string, string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		cwd, err := d.Getwd()
		return cwd, "", err
	}
	if !filepath.IsAbs(requested) {
		if err := rootfs.ValidateRelative(requested); err != nil {
			return "", "", fmt.Errorf("invalid --output-dir: %w", err)
		}
		cwd, err := d.Getwd()
		return cwd, filepath.Clean(requested), err
	}
	absolute := filepath.Clean(requested)
	ancestor := filepath.Dir(absolute)
	for {
		info, err := os.Lstat(ancestor)
		if err == nil {
			if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				return "", "", fmt.Errorf("invalid --output-dir: existing parent %q is not a directory", ancestor)
			}
			physical, err := filepath.EvalSymlinks(ancestor)
			if err != nil {
				return "", "", fmt.Errorf("resolve --output-dir parent: %w", err)
			}
			relative, err := filepath.Rel(ancestor, absolute)
			if err != nil {
				return "", "", fmt.Errorf("resolve --output-dir: %w", err)
			}
			return physical, relative, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("inspect --output-dir parent: %w", err)
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", "", fmt.Errorf("invalid --output-dir: no existing parent")
		}
		ancestor = next
	}
}

func printInspection(d *shared.Deps, result distribution.Inspection, format string, pretty bool) error {
	return d.PrintOutputWithRenderers(
		result, format, pretty,
		func() error { return renderInspection(d.IO.Out, result, false) },
		func() error { return renderInspection(d.IO.Out, result, true) },
	)
}

func renderInspection(w io.Writer, result distribution.Inspection, markdown bool) error {
	render := asc.RenderTableTo
	if markdown {
		render = asc.RenderMarkdownTo
	}
	rows := [][]string{
		{"Metadata Eligible", fmt.Sprintf("%t", result.Preparation.MetadataEligible)},
		{"Code Signature", string(result.Signing.CodeSignatureVerification.Status)},
		{"Profile Integrity", string(result.Signing.ProfileIntegrityVerification.Status)},
		{"Profile Trust", string(result.Signing.ProfileTrustVerification.Status)},
		{"Bundle ID", result.App.BundleID},
		{"Title", result.App.Title},
		{"Version", result.App.Version},
		{"Build", result.App.BuildNumber},
		{"Profile Class", string(result.Signing.ProfileClass)},
		{"Profile UUID", result.Signing.ProfileUUID},
		{"Team ID", result.Signing.TeamID},
		{"Devices", fmt.Sprintf("%d", result.Signing.DeviceCount)},
	}
	if len(result.Signing.Devices) > 0 {
		rows = append(rows, []string{"Device UDIDs", strings.Join(result.Signing.Devices, ", ")})
	}
	rows = append(
		rows,
		[]string{"IPA SHA-256", result.Artifact.SHA256},
		[]string{"Issues", strings.Join(result.Preparation.Issues, "; ")},
	)
	render(w, []string{"Field", "Value"}, rows)
	return nil
}

func printPrepareResult(d *shared.Deps, result distribution.PrepareResult, format string, pretty bool) error {
	return d.PrintOutputWithRenderers(
		result, format, pretty,
		func() error { return renderPrepareResult(d.IO.Out, result, false) },
		func() error { return renderPrepareResult(d.IO.Out, result, true) },
	)
}

func renderPrepareResult(w io.Writer, result distribution.PrepareResult, markdown bool) error {
	render := asc.RenderTableTo
	if markdown {
		render = asc.RenderMarkdownTo
	}
	render(w, []string{"Field", "Value"}, [][]string{
		{"Bundle Path", result.BundlePath},
		{"Reused", fmt.Sprintf("%t", result.Reused)},
		{"Bundle ID", result.Descriptor.App.BundleID},
		{"Version", result.Descriptor.App.Version},
		{"Build", result.Descriptor.App.BuildNumber},
		{"IPA SHA-256", result.Descriptor.Artifact.SHA256},
	})
	return nil
}
