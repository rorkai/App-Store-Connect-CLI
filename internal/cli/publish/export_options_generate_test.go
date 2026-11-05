package publish

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestPublishLocalBuildExportOptionsPrecedence(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)

	defaultPath := filepath.Join(workDir, defaultPublishExportOptionsPath)
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o755); err != nil {
		t.Fatalf("create default export-options directory: %v", err)
	}
	if err := os.WriteFile(defaultPath, []byte("default plist"), 0o600); err != nil {
		t.Fatalf("write default export options: %v", err)
	}

	for _, tc := range []struct {
		name      string
		explicit  string
		defaultOK bool
		wantPath  string
	}{
		{
			name:      "testflight explicit path wins over existing default",
			explicit:  "ExplicitExportOptions.plist",
			defaultOK: true,
			wantPath:  "ExplicitExportOptions.plist",
		},
		{
			name:      "appstore existing default is reused",
			defaultOK: true,
			wantPath:  defaultPublishExportOptionsPath,
		},
		{
			name:      "appstore missing options are deferred for generation after archive",
			defaultOK: false,
			wantPath:  "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.defaultOK {
				if err := os.WriteFile(defaultPath, []byte("default plist"), 0o600); err != nil {
					t.Fatalf("restore default export options: %v", err)
				}
			} else if err := os.Remove(defaultPath); err != nil {
				t.Fatalf("remove default export options: %v", err)
			}

			config, err := resolveLocalBuildConfig(
				publishExportOptionsTestFlagValues(t, tc.explicit),
				"IOS",
				"1.2.3",
				"42",
			)
			if err != nil {
				t.Fatalf("resolveLocalBuildConfig() error: %v", err)
			}
			if config.ExportOptionsPath != tc.wantPath {
				t.Fatalf("expected export options path %q, got %q", tc.wantPath, config.ExportOptionsPath)
			}
		})
	}
}

func TestPublishLocalBuildGenerationFlagsBypassConventionalOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(defaultPublishExportOptionsPath), 0o755); err != nil {
		t.Fatalf("create default export-options directory: %v", err)
	}
	if err := os.WriteFile(defaultPublishExportOptionsPath, []byte("checked-in plist"), 0o600); err != nil {
		t.Fatalf("write default export options: %v", err)
	}

	fs := flag.NewFlagSet("publish signing precedence", flag.ContinueOnError)
	values := bindPublishLocalBuildFlags(fs)
	if err := fs.Parse([]string{
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--signing-style", "manual",
		"--team-id", "TEAM123456",
	}); err != nil {
		t.Fatalf("parse local-build flags: %v", err)
	}
	setFlags := collectSetFlags(fs)
	if err := validatePublishExportOptionsFlags(values, setFlags); err != nil {
		t.Fatalf("validatePublishExportOptionsFlags() error: %v", err)
	}
	config, err := resolveLocalBuildConfig(values, "IOS", "1.2.3", "42")
	if err != nil {
		t.Fatalf("resolveLocalBuildConfig() error: %v", err)
	}
	if config.ExportOptionsPath != "" {
		t.Fatalf("generation flags must bypass conventional options, got %q", config.ExportOptionsPath)
	}
	if config.SigningStyle != "manual" || config.TeamID != "TEAM123456" {
		t.Fatalf("unexpected generated signing config: style=%q team=%q", config.SigningStyle, config.TeamID)
	}
}

func TestPublishLocalBuildGeneratesExportOptionsAfterArchive(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	originalGenerate := generatePublishExportOptionsFn
	t.Cleanup(func() {
		generatePublishExportOptionsFn = originalGenerate
	})

	artifactsDir := t.TempDir()
	archivePath := filepath.Join(artifactsDir, "Demo-IOS-1.2.3-42.xcarchive")
	ipaPath := filepath.Join(artifactsDir, "Demo-IOS-1.2.3-42.ipa")
	deterministicPath := localxcode.DefaultExportOptionsPathForArchive(archivePath)
	deterministicContents := []byte("pre-existing archive-adjacent options\n")
	if err := os.WriteFile(deterministicPath, deterministicContents, 0o600); err != nil {
		t.Fatalf("write deterministic export options: %v", err)
	}
	var generatedPath string
	callOrder := make([]string, 0, 4)

	runPublishArchiveFn = func(_ context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		callOrder = append(callOrder, "archive")
		if opts.ArchivePath != archivePath {
			t.Fatalf("expected archive path %q, got %q", archivePath, opts.ArchivePath)
		}
		return &localxcode.ArchiveResult{
			ArchivePath:   archivePath,
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	generatePublishExportOptionsFn = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		callOrder = append(callOrder, "generate")
		if opts.ArchivePath != archivePath {
			t.Fatalf("expected generator archive path %q, got %q", archivePath, opts.ArchivePath)
		}
		if opts.OutputPath == deterministicPath {
			t.Fatalf("implicit generation must use a unique path, got deterministic path %q", opts.OutputPath)
		}
		if !strings.HasPrefix(opts.OutputPath, strings.TrimSuffix(deterministicPath, ".plist")+"-") || !strings.HasSuffix(opts.OutputPath, ".plist") {
			t.Fatalf("expected unique archive-adjacent output path, got %q", opts.OutputPath)
		}
		if opts.Destination != "export" {
			t.Fatalf("expected generator destination export, got %q", opts.Destination)
		}
		if opts.SigningStyle != "automatic" {
			t.Fatalf("expected automatic signing, got %q", opts.SigningStyle)
		}
		if opts.Overwrite {
			t.Fatal("implicit generation must not enable overwrite")
		}
		generatedPath = opts.OutputPath
		return &localxcode.ExportOptionsGenerateResult{Path: generatedPath}, nil
	}
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		callOrder = append(callOrder, "export")
		if opts.ExportOptions != generatedPath {
			t.Fatalf("expected generated export options path %q, got %q", generatedPath, opts.ExportOptions)
		}
		return &localxcode.ExportResult{
			ArchivePath: archivePath,
			IPAPath:     ipaPath,
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		callOrder = append(callOrder, "upload")
		return &publishUploadResult{Version: version, BuildNumber: buildNumber}, nil
	}

	result, err := runPublishLocalBuild(
		context.Background(),
		nil,
		"app-123",
		"IOS",
		"1.2.3",
		"42",
		5*time.Second,
		time.Minute,
		false,
		publishLocalBuildConfig{
			WorkspacePath:     "Demo.xcworkspace",
			Scheme:            "Demo",
			Configuration:     "Release",
			ArchivePath:       archivePath,
			IPAPath:           ipaPath,
			ExportOptionsPath: "",
		},
	)
	if err != nil {
		t.Fatalf("runPublishLocalBuild() error: %v", err)
	}
	if !reflect.DeepEqual(callOrder, []string{"archive", "generate", "export", "upload"}) {
		t.Fatalf("expected archive, generation, export, upload order; got %v", callOrder)
	}
	if result.Export == nil || result.Export.ExportOptionsPath != generatedPath {
		t.Fatalf("expected generated export options in result, got %#v", result.Export)
	}
	preserved, err := os.ReadFile(deterministicPath)
	if err != nil {
		t.Fatalf("read deterministic export options: %v", err)
	}
	if string(preserved) != string(deterministicContents) {
		t.Fatalf("implicit generation overwrote deterministic export options: %q", preserved)
	}
}

func TestPublishLocalBuildThreadsManualSigningOptionsAfterArchive(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	wantErr := errors.New("stop after generation")
	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	var generatedOptions localxcode.ExportOptionsGenerateOptions
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		return &localxcode.ArchiveResult{ArchivePath: archivePath}, nil
	}
	generatePublishExportOptionsFn = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		generatedOptions = opts
		return nil, wantErr
	}
	runPublishExportFn = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		t.Fatal("export must not run after the generation sentinel")
		return nil, nil
	}

	_, err := runPublishLocalBuild(
		context.Background(), nil, "app-123", "IOS", "1.2.3", "42",
		5*time.Second, time.Minute, false,
		publishLocalBuildConfig{
			WorkspacePath: "Demo.xcworkspace",
			Scheme:        "Demo",
			ArchivePath:   archivePath,
			IPAPath:       filepath.Join(t.TempDir(), "Demo.ipa"),
			SigningStyle:  "manual",
			TeamID:        "TEAM123456",
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected generation sentinel, got %v", err)
	}
	if generatedOptions.SigningStyle != "manual" {
		t.Fatalf("expected manual signing style, got %q", generatedOptions.SigningStyle)
	}
	if generatedOptions.TeamID != "TEAM123456" {
		t.Fatalf("expected team ID passthrough, got %q", generatedOptions.TeamID)
	}
}

func TestPublishRejectsExplicitOptionsWithGenerationFlagsBeforeSideEffects(t *testing.T) {
	for _, command := range []struct {
		name     string
		cmd      func() *ffcli.Command
		baseArgs []string
	}{
		{
			name: "testflight",
			cmd:  PublishTestFlightCommand,
			baseArgs: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--group", "group-1", "--export-options", "ExportOptions.plist",
			},
		},
		{
			name: "appstore",
			cmd:  PublishAppStoreCommand,
			baseArgs: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--export-options", "ExportOptions.plist",
			},
		},
	} {
		for _, generationFlag := range [][]string{
			{"--signing-style", "automatic"},
			{"--team-id", "TEAM123456"},
		} {
			t.Run(command.name+" "+generationFlag[0], func(t *testing.T) {
				restore := overridePublishCommandTestHooks(t)
				defer restore()

				getPublishASCClientFn = func(time.Duration) (*asc.Client, error) {
					t.Fatal("ASC client must not be created for conflicting export-options flags")
					return nil, nil
				}
				cmd := command.cmd()
				cmd.FlagSet.SetOutput(io.Discard)
				if err := cmd.FlagSet.Parse(append(append([]string(nil), command.baseArgs...), generationFlag...)); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				runErr := cmd.Exec(context.Background(), nil)
				if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--export-options cannot be combined with --signing-style or --team-id") {
					t.Fatalf("expected explicit-options conflict usage error, got %v", runErr)
				}
			})
		}
	}
}

func TestPublishRejectsInvalidSigningStyleBeforeSideEffects(t *testing.T) {
	for _, command := range []struct {
		name string
		cmd  func() *ffcli.Command
		args []string
	}{
		{
			name: "testflight",
			cmd:  PublishTestFlightCommand,
			args: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--group", "group-1", "--signing-style", "heuristic",
			},
		},
		{
			name: "appstore",
			cmd:  PublishAppStoreCommand,
			args: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--signing-style", "heuristic",
			},
		},
	} {
		for _, signingStyle := range []string{"heuristic", ""} {
			t.Run(command.name+fmt.Sprintf(" value=%q", signingStyle), func(t *testing.T) {
				restore := overridePublishCommandTestHooks(t)
				defer restore()

				getPublishASCClientFn = func(time.Duration) (*asc.Client, error) {
					t.Fatal("ASC client must not be created for an invalid signing style")
					return nil, nil
				}
				cmd := command.cmd()
				cmd.FlagSet.SetOutput(io.Discard)
				args := append([]string(nil), command.args...)
				args[len(args)-1] = signingStyle
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				runErr := cmd.Exec(context.Background(), nil)
				if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--signing-style must be one of: automatic, manual") {
					t.Fatalf("expected invalid signing-style usage error, got %v", runErr)
				}
			})
		}
	}
}

func TestPublishRejectsExplicitlyEmptyTeamIDBeforeSideEffects(t *testing.T) {
	for _, command := range []struct {
		name string
		cmd  func() *ffcli.Command
		args []string
	}{
		{
			name: "testflight",
			cmd:  PublishTestFlightCommand,
			args: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--group", "group-1", "--team-id", "",
			},
		},
		{
			name: "appstore",
			cmd:  PublishAppStoreCommand,
			args: []string{
				"--app", "app-123", "--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--version", "1.2.3", "--team-id", "",
			},
		},
	} {
		t.Run(command.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()
			workDir := t.TempDir()
			t.Chdir(workDir)
			if err := os.MkdirAll(filepath.Dir(defaultPublishExportOptionsPath), 0o755); err != nil {
				t.Fatalf("create conventional export-options directory: %v", err)
			}
			if err := os.WriteFile(defaultPublishExportOptionsPath, []byte("conventional"), 0o600); err != nil {
				t.Fatalf("write conventional export-options plist: %v", err)
			}

			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) {
				t.Fatal("ASC client must not be created for an empty team ID")
				return nil, nil
			}
			cmd := command.cmd()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(command.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			runErr := cmd.Exec(context.Background(), nil)
			if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--team-id must not be empty") {
				t.Fatalf("expected empty team-id usage error, got %v", runErr)
			}
		})
	}
}

func TestPublishLocalBuildPreflightsIPADestinationBeforeSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ipaPath   func(*testing.T) string
		wantUsage bool
		errorHint string
	}{
		{
			name:      "invalid extension",
			ipaPath:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "Demo.zip") },
			wantUsage: true,
			errorHint: "must end with .ipa",
		},
		{
			name: "existing directory",
			ipaPath: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "Demo.ipa")
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantUsage: true,
			errorHint: "must not be a directory",
		},
		{
			name: "filesystem error",
			ipaPath: func(t *testing.T) string {
				parent := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(parent, "Demo.ipa")
			},
			errorHint: "lstat ipa path",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()

			runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				t.Fatal("archive ran before IPA destination preflight")
				return nil, nil
			}
			generatePublishExportOptionsFn = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				t.Fatal("export-options generation ran before IPA destination preflight")
				return nil, nil
			}
			runPublishExportFn = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				t.Fatal("export ran before IPA destination preflight")
				return nil, nil
			}

			_, err := runPublishLocalBuild(
				context.Background(), nil, "app-123", "IOS", "1.2.3", "42",
				5*time.Second, time.Minute, false,
				publishLocalBuildConfig{IPAPath: tc.ipaPath(t)},
			)
			if err == nil || !strings.Contains(err.Error(), tc.errorHint) {
				t.Fatalf("expected %q error, got %v", tc.errorHint, err)
			}
			if got := errors.Is(err, flag.ErrHelp); got != tc.wantUsage {
				t.Fatalf("usage classification = %v, want %v: %v", got, tc.wantUsage, err)
			}
		})
	}
}

func TestPublishLocalBuildChecksXcodeBeforeCreatingIPAParent(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	wantErr := errors.New("Xcode unavailable")
	outputParent := filepath.Join(t.TempDir(), "nested", "output")
	preflightPublishXcodeFn = func(context.Context) error { return wantErr }
	runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		t.Fatal("archive ran after failed Xcode preflight")
		return nil, nil
	}
	generatePublishExportOptionsFn = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		t.Fatal("export-options generation ran after failed Xcode preflight")
		return nil, nil
	}

	_, err := runPublishLocalBuild(
		context.Background(), nil, "app-123", "IOS", "1.2.3", "42",
		5*time.Second, time.Minute, false,
		publishLocalBuildConfig{IPAPath: filepath.Join(outputParent, "Demo.ipa")},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected Xcode preflight error, got %v", err)
	}
	if _, err := os.Stat(outputParent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Xcode preflight failure created IPA output parent: %v", err)
	}
}

func publishExportOptionsTestFlagValues(t *testing.T, explicit string) *publishLocalBuildFlagValues {
	t.Helper()

	fs := flag.NewFlagSet("publish export-options test", flag.ContinueOnError)
	values := bindPublishLocalBuildFlags(fs)
	if err := fs.Parse([]string{
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--export-options", explicit,
	}); err != nil {
		t.Fatalf("parse local-build flags: %v", err)
	}
	return values
}
