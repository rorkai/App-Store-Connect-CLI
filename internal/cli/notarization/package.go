package notarization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func packageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("notarization package", flag.ExitOnError)
	file := bindSingleStringFlag(fs, "file", "Path to a signed macOS .app directory (required)")
	destination := bindSingleStringFlag(fs, "output-path", "New .zip file outside the app bundle (required; never overwritten)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "package",
		ShortUsage: "asc notarization package --file <app> --output-path <zip> [flags]",
		ShortHelp:  "Package a signed macOS app in a new ZIP archive locally.",
		LongHelp: `Create a ZIP containing one signed .app using Apple's ditto on macOS.
Preserve signatures, relative internal symlinks, permissions and resource
metadata. The app must be self-contained; absolute links and links outside
the bundle are rejected. The source is unchanged and an existing output
is never replaced.

Signature verification checks integrity only, not notarization eligibility.
This command does not sign, submit, staple or authenticate. Distribution
still requires the appropriate Developer ID signing and notarization.

Examples:
  asc notarization package --file ./MyApp.app --output-path ./MyApp.zip
  asc notarization package --file ./MyApp.app --output-path ./MyApp.zip --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("notarization package does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			if strings.TrimSpace(file.String()) == "" || !strings.EqualFold(filepath.Ext(filepath.Clean(file.String())), ".app") {
				return d.UsageError("notarization package: --file must specify a signed .app directory")
			}
			if strings.TrimSpace(destination.String()) == "" || !strings.EqualFold(filepath.Ext(destination.String()), ".zip") {
				return d.UsageError("notarization package: --output-path must specify a new .zip file")
			}
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("notarization package requires macOS and Apple's codesign and ditto tools")
			}
			target, err := validateStaplerTargetDetails(staplerTargetPath(d, file.String()))
			if err != nil {
				if isStaplerTargetUsageError(err) {
					return d.UsageErrorf("notarization package: %v", err)
				}
				return fmt.Errorf("notarization package: %w", err)
			}
			defer target.close()
			if !target.directory {
				return d.UsageError("notarization package: --file must be an .app directory")
			}
			archivePath, err := filepath.Abs(staplerTargetPath(d, destination.String()))
			if err != nil {
				return fmt.Errorf("notarization package: resolve output: %w", err)
			}
			appRoot, err := rootfs.New(target.path)
			if err != nil {
				return fmt.Errorf("notarization package: anchor app: %w", err)
			}
			defer appRoot.Close()
			inside, err := appRoot.ContainsPath(archivePath)
			if err != nil {
				return fmt.Errorf("notarization package: inspect output location: %w", err)
			}
			if inside {
				return d.UsageError("notarization package: --output-path must be outside the app bundle")
			}
			result, err := packageApp(ctx, target, archivePath, d.IO.ErrOut)
			if err != nil {
				return fmt.Errorf("notarization package: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func packageApp(ctx context.Context, target *validatedStaplerTarget, archivePath string, diagnostics io.Writer) (*asc.NotarizationPackageResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	destination, err := rootfs.New(filepath.Dir(archivePath))
	if err != nil {
		return nil, err
	}
	defer destination.Close()
	if err := destination.CheckCreateNewFile(filepath.Base(archivePath)); err != nil {
		return nil, fmt.Errorf("create output: %w", err)
	}
	if err := target.verifyIdentity("before packaging"); err != nil {
		return nil, err
	}
	inventory, err := target.captureDirectoryInventoryAtStage(ctx, "before packaging")
	if err != nil {
		return nil, err
	}
	if err := runPackageTool(ctx, diagnostics, "/usr/bin/codesign", "--verify", "--deep", "--strict", target.path); err != nil {
		return nil, fmt.Errorf("verify app signature: %w", err)
	}
	temporary, err := os.MkdirTemp("", ".asc-notarization-package-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temporary)
	archive := filepath.Join(temporary, "app.zip")
	if err := runPackageTool(ctx, diagnostics, "/usr/bin/ditto", "-c", "-k", "--keepParent", "--sequesterRsrc", "--rsrc", "--extattr", "--acl", target.path, archive); err != nil {
		return nil, fmt.Errorf("create ZIP: %w", err)
	}
	if err := target.verifyIdentity("after packaging"); err != nil {
		return nil, err
	}
	if err := target.verifyDirectoryInventory(ctx, inventory, "after packaging"); err != nil {
		return nil, err
	}
	staging, err := rootfs.New(temporary)
	if err != nil {
		return nil, err
	}
	defer staging.Close()
	file, err := staging.OpenFile("app.zip")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := destination.CreateNewFrom(filepath.Base(archivePath), io.TeeReader(packageReader{ctx: ctx, Reader: file}, hash), 0o600)
	if err != nil {
		return nil, fmt.Errorf("publish ZIP: %w", err)
	}
	return &asc.NotarizationPackageResult{AppPath: target.path, ArchivePath: archivePath, SizeBytes: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

var runPackageTool = func(ctx context.Context, diagnostics io.Writer, tool string, args ...string) error {
	command := exec.CommandContext(ctx, tool, args...)
	command.Stdout, command.Stderr = diagnostics, diagnostics
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "DITTO") {
			command.Env = append(command.Env, entry)
		}
	}
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

type packageReader struct {
	ctx context.Context
	io.Reader
}

func (reader packageReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.Reader.Read(buffer)
}
