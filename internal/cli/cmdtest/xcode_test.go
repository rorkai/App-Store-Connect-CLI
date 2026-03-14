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
