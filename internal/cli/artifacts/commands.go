package artifacts

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// IPAInfoCommand prints an offline IPA manifest.
func IPAInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ipa-info", flag.ExitOnError)
	path := fs.String("path", "", "Path to an .ipa")
	includeEntitlements := fs.Bool("include-entitlements", false, "Include the embedded profile entitlements map")
	includeProfile := fs.Bool("include-profile", false, "Include the embedded profile summary")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "ipa-info",
		ShortUsage: "asc ipa-info --path PATH [--include-entitlements] [--include-profile]",
		ShortHelp:  "Inspect a local IPA without contacting App Store Connect.",
		LongHelp: `Inspect a local IPA and print bundle identity, nested bundles, and optional profile fields.

The command does not upload the artifact or call Apple. An unreadable archive or an IPA without an embedded profile exits 1 after writing a receipt.

Examples:
  asc ipa-info --path ./App.ipa --output json
  asc ipa-info --path ./App.ipa --include-profile --include-entitlements --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runArtifactInfo(ctx, args, artifactInfoConfig{
				Kind:                "ipa-info",
				Path:                *path,
				IncludeEntitlements: *includeEntitlements,
				IncludeProfile:      *includeProfile,
				Output:              *output.Output,
				Pretty:              *output.Pretty,
			})
		},
	}
}

// PKGInfoCommand prints an offline flat package manifest.
func PKGInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pkg-info", flag.ExitOnError)
	path := fs.String("path", "", "Path to a flat component .pkg")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "pkg-info",
		ShortUsage: "asc pkg-info --path PATH",
		ShortHelp:  "Inspect a local flat component package without contacting Apple.",
		LongHelp: `Inspect a local flat xar .pkg and print the product identifier, version, install location, and component bundle identifiers.

The command does not expand the package onto disk or call Apple. An unreadable package exits 1 after writing a receipt.

Examples:
  asc pkg-info --path ./App.pkg --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runArtifactInfo(ctx, args, artifactInfoConfig{
				Kind:   "pkg-info",
				Path:   *path,
				Output: *output.Output,
				Pretty: *output.Pretty,
			})
		},
	}
}

type artifactInfoConfig struct {
	Kind                string
	Path                string
	IncludeEntitlements bool
	IncludeProfile      bool
	Output              string
	Pretty              bool
}

func runArtifactInfo(ctx context.Context, args []string, config artifactInfoConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) > 0 {
		return shared.UsageErrorf("%s does not accept positional arguments", config.Kind)
	}
	path := strings.TrimSpace(config.Path)
	if path == "" {
		return shared.UsageErrorf("%s: --path is required", config.Kind)
	}
	if _, err := shared.ValidateOutputFormat(config.Output, config.Pretty); err != nil {
		return shared.UsageError(err.Error())
	}
	file, err := rootfs.OpenFile(path)
	if err != nil {
		receipt := unreadableReceipt(config.Kind, path)
		_ = shared.PrintOutput(receipt, config.Output, config.Pretty)
		return fmt.Errorf("%s: %w", config.Kind, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("%s: %w", config.Kind, err)
	}
	if info.Size() < 0 || info.Size() > 512<<20 {
		return fmt.Errorf("%s: artifact exceeds the 512 MiB offline inspection limit", config.Kind)
	}
	data, err := io.ReadAll(io.LimitReader(file, info.Size()+1))
	if err != nil {
		receipt := unreadableReceipt(config.Kind, path)
		_ = shared.PrintOutput(receipt, config.Output, config.Pretty)
		return fmt.Errorf("%s: %w", config.Kind, err)
	}
	switch config.Kind {
	case "ipa-info":
		manifest, inspectErr := artifacts.InspectIPA(data, config.IncludeEntitlements, config.IncludeProfile)
		receipt := ipaReceipt(path, manifest)
		if printErr := shared.PrintOutput(receipt, config.Output, config.Pretty); printErr != nil {
			return printErr
		}
		if inspectErr != nil || manifest.Status != "readable" {
			if inspectErr == nil {
				inspectErr = fmt.Errorf("IPA is unsigned")
			}
			return fmt.Errorf("ipa-info: %w", inspectErr)
		}
		return nil
	case "pkg-info":
		manifest, inspectErr := artifacts.InspectPKG(data)
		receipt := pkgReceipt(path, manifest)
		if printErr := shared.PrintOutput(receipt, config.Output, config.Pretty); printErr != nil {
			return printErr
		}
		if inspectErr != nil || manifest.Status != "readable" {
			if inspectErr == nil {
				inspectErr = fmt.Errorf("package is unreadable")
			}
			return fmt.Errorf("pkg-info: %w", inspectErr)
		}
		return nil
	default:
		return fmt.Errorf("unknown artifact inspector %q", config.Kind)
	}
}

func ipaReceipt(path string, manifest artifacts.IPAManifest) *asc.ArtifactIPAInfo {
	info := &asc.ArtifactIPAInfo{
		Path:             path,
		BundleID:         manifest.BundleID,
		Name:             manifest.Name,
		Version:          manifest.Version,
		BuildNumber:      manifest.BuildNumber,
		MinimumOSVersion: manifest.MinimumOSVersion,
		Platforms:        manifest.Platforms,
		TeamID:           manifest.TeamID,
		SignerCommonName: manifest.SignerCommonName,
		Status:           manifest.Status,
		NestedBundles:    make([]asc.ArtifactNestedBundle, 0, len(manifest.NestedBundles)),
	}
	for _, nested := range manifest.NestedBundles {
		info.NestedBundles = append(info.NestedBundles, asc.ArtifactNestedBundle{
			BundleID: nested.BundleID,
			Name:     nested.Name,
			Path:     nested.Path,
		})
	}
	if manifest.Entitlements != nil {
		info.Entitlements = manifest.Entitlements
	}
	if manifest.Profile != nil {
		info.Profile = &asc.ArtifactProfileSummary{
			Name:           manifest.Profile.Name,
			UUID:           manifest.Profile.UUID,
			ExpirationDate: manifest.Profile.ExpirationDate,
			ProfileType:    manifest.Profile.ProfileType,
		}
	}
	return info
}

func pkgReceipt(path string, manifest artifacts.PKGManifest) *asc.ArtifactPKGInfo {
	return &asc.ArtifactPKGInfo{
		Path:             path,
		ProductID:        manifest.ProductID,
		Version:          manifest.Version,
		InstallLocation:  manifest.InstallLocation,
		BundleIDs:        manifest.BundleIDs,
		SignerCommonName: manifest.SignerCommonName,
		Status:           manifest.Status,
	}
}

func unreadableReceipt(kind, path string) any {
	if kind == "pkg-info" {
		return &asc.ArtifactPKGInfo{Path: path, Status: "unreadable"}
	}
	return &asc.ArtifactIPAInfo{Path: path, Status: "unreadable"}
}
