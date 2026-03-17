package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestXcodeCommandExists(t *testing.T) {
	root := RootCommand("1.2.3")

	xcodeCmd := findSubcommand(root, "xcode")
	if xcodeCmd == nil {
		t.Fatal("expected xcode command")
	}
	if strings.HasPrefix(xcodeCmd.ShortHelp, "[experimental]") {
		t.Fatalf("expected xcode command not to be experimental, got %q", xcodeCmd.ShortHelp)
	}
	if findSubcommand(root, "xcode", "archive") == nil {
		t.Fatal("expected xcode archive command")
	}
	if findSubcommand(root, "xcode", "export") == nil {
		t.Fatal("expected xcode export command")
	}
	if findSubcommand(root, "xcode", "validate") == nil {
		t.Fatal("expected xcode validate command")
	}
	if findSubcommand(root, "xcode", "version") == nil {
		t.Fatal("expected xcode version command")
	}
	if findSubcommand(root, "xcode", "version", "view") == nil {
		t.Fatal("expected xcode version view command")
	}
	viewCmd := findSubcommand(root, "xcode", "version", "view")
	if viewCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version view to expose --project")
	}
	editCmd := findSubcommand(root, "xcode", "version", "edit")
	if editCmd == nil {
		t.Fatal("expected xcode version edit command")
	}
	if editCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version edit to expose --project")
	}
	if editCmd.FlagSet.Lookup("target") != nil {
		t.Fatal("expected xcode version edit to omit --target")
	}
	bumpCmd := findSubcommand(root, "xcode", "version", "bump")
	if bumpCmd == nil {
		t.Fatal("expected xcode version bump command")
	}
	if bumpCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version bump to expose --project")
	}
	if bumpCmd.FlagSet.Lookup("target") == nil {
		t.Fatal("expected xcode version bump to expose --target")
	}
	if findSubcommand(root, "xcode", "version", "get") != nil {
		t.Fatal("expected xcode version get command to be absent")
	}
	if findSubcommand(root, "xcode", "version", "set") != nil {
		t.Fatal("expected xcode version set command to be absent")
	}
}

func TestXcodeVersionHelpShowsCanonicalSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "version"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"view", "edit", "bump", "asc xcode version view", "asc xcode version edit"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected help to contain %q, got %q", want, stderr)
		}
	}
	for _, hidden := range []string{"\n  get", "\n  set", "asc xcode version get", "asc xcode version set"} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected help to hide %q, got %q", hidden, stderr)
		}
	}
}

func TestXcodeExportHelpMentionsDirectUploadMode(t *testing.T) {
	root := RootCommand("1.2.3")

	exportCmd := findSubcommand(root, "xcode", "export")
	if exportCmd == nil {
		t.Fatal("expected xcode export command")
	}
	if !strings.Contains(exportCmd.ShortHelp, "direct upload") {
		t.Fatalf("expected short help to mention direct upload, got %q", exportCmd.ShortHelp)
	}
	if !strings.Contains(exportCmd.LongHelp, "destination=upload") {
		t.Fatalf("expected long help to mention destination=upload, got %q", exportCmd.LongHelp)
	}
	if !strings.Contains(exportCmd.LongHelp, "without writing a local") {
		t.Fatalf("expected long help to explain no local IPA is written, got %q", exportCmd.LongHelp)
	}
	if got := exportCmd.FlagSet.Lookup("ipa-path").Usage; !strings.Contains(got, "when one is produced") {
		t.Fatalf("expected ipa-path usage to mention produced IPA behavior, got %q", got)
	}
}

func TestXcodeValidateHelpMentionsAltool(t *testing.T) {
	root := RootCommand("1.2.3")

	validateCmd := findSubcommand(root, "xcode", "validate")
	if validateCmd == nil {
		t.Fatal("expected xcode validate command")
	}
	if !strings.Contains(validateCmd.LongHelp, "xcrun altool --validate-app") {
		t.Fatalf("expected long help to mention altool validation, got %q", validateCmd.LongHelp)
	}
	if validateCmd.FlagSet.Lookup("ipa") == nil {
		t.Fatal("expected xcode validate to expose --ipa")
	}
	if validateCmd.FlagSet.Lookup("api-key") == nil {
		t.Fatal("expected xcode validate to expose --api-key")
	}
	if validateCmd.FlagSet.Lookup("api-issuer") == nil {
		t.Fatal("expected xcode validate to expose --api-issuer")
	}
}

func TestXcodeArchiveRequiresWorkspaceOrProject(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: exactly one of --workspace or --project is required") {
		t.Fatalf("expected workspace/project error, got %q", stderr)
	}
}

func TestXcodeArchiveRejectsWorkspaceAndProjectTogether(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"xcode", "archive",
			"--workspace", "Demo.xcworkspace",
			"--project", "Demo.xcodeproj",
			"--scheme", "Demo",
			"--archive-path", "Demo.xcarchive",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: exactly one of --workspace or --project is required") {
		t.Fatalf("expected workspace/project error, got %q", stderr)
	}
}

func TestXcodeArchiveRequiresScheme(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--project", "Demo.xcodeproj", "--archive-path", "Demo.xcarchive"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --scheme is required") {
		t.Fatalf("expected scheme error, got %q", stderr)
	}
}

func TestXcodeArchiveRequiresArchivePath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--project", "Demo.xcodeproj", "--scheme", "Demo"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --archive-path is required") {
		t.Fatalf("expected archive-path error, got %q", stderr)
	}
}

func TestXcodeExportRequiresArchivePath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--export-options", "ExportOptions.plist", "--ipa-path", "Demo.ipa"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --archive-path is required") {
		t.Fatalf("expected archive-path error, got %q", stderr)
	}
}

func TestXcodeExportRequiresExportOptions(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--ipa-path", "Demo.ipa"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --export-options is required") {
		t.Fatalf("expected export-options error, got %q", stderr)
	}
}

func TestXcodeValidateRequiresIPA(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --ipa is required") {
		t.Fatalf("expected ipa error, got %q", stderr)
	}
}

func TestXcodeExportRequiresIPAPath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--export-options", "ExportOptions.plist"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --ipa-path is required") {
		t.Fatalf("expected ipa-path error, got %q", stderr)
	}
}
