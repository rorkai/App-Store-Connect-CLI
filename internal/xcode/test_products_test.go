package xcode

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestBuildForTestingDiscoversExplicitProducts(t *testing.T) {
	t.Cleanup(overrideBuildProcess(t, "success"))
	previousRun := runXcodeTestCommand
	t.Cleanup(func() { runXcodeTestCommand = previousRun })
	for _, mode := range []string{"new", "new in existing directory", "replaced", "unchanged", "renamed", "rewritten in place", "ambiguous", "symlink", "directory symlink", "root replaced", "parent replaced"} {
		t.Run(mode, func(t *testing.T) {
			if strings.Contains(mode, "symlink") && runtime.GOOS == "windows" {
				t.Skip("Windows symlink creation requires elevated privileges")
			}
			parent := filepath.Join(t.TempDir(), "anchor")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			products := filepath.Join(parent, "Products")
			manifest := filepath.Join(products, "Demo.xctestrun")
			write := func(path, data string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "new in existing directory", "renamed":
				write(filepath.Join(products, "Old.xctestrun"), "old")
			case "replaced", "unchanged", "rewritten in place", "root replaced", "parent replaced":
				write(manifest, "old")
			}
			runXcodeTestCommand = func(_ context.Context, _ string, args []string, _ io.Writer) error {
				if !slices.Contains(args, "SYMROOT="+products) {
					t.Fatalf("custom products setting lost: %v", args)
				}
				switch mode {
				case "unchanged":
					return nil
				case "renamed":
					return os.Rename(filepath.Join(products, "Old.xctestrun"), manifest)
				case "replaced":
					replacement := filepath.Join(products, "replacement")
					write(replacement, "new")
					return os.Rename(replacement, manifest)
				case "symlink":
					target := filepath.Join(parent, "outside.xctestrun")
					write(target, "outside")
					if err := os.Mkdir(products, 0o755); err != nil {
						return err
					}
					return os.Symlink(target, manifest)
				case "directory symlink":
					target := filepath.Join(parent, "outside")
					write(filepath.Join(target, "Demo.xctestrun"), "outside")
					return os.Symlink(target, products)
				case "root replaced":
					if err := os.Rename(products, products+"-old"); err != nil {
						return err
					}
				case "parent replaced":
					if err := os.Rename(parent, parent+"-old"); err != nil {
						return err
					}
				case "ambiguous":
					write(filepath.Join(products, "Other.xctestrun"), "other")
				}
				write(manifest, "new")
				return nil
			}
			derivedData := filepath.Join(t.TempDir(), "DerivedData")
			write(filepath.Join(derivedData, "Build", "Products", "Stale.xctestrun"), "stale")
			var log bytes.Buffer
			result, err := Test(context.Background(), TestOptions{
				ProjectPath: createBuildTestContainer(t), Scheme: "Demo",
				Action: string(TestActionBuildForTesting), Destinations: []string{"platform=macOS"},
				DerivedDataPath: derivedData,
				XcodebuildArgs:  []string{"SYMROOT=" + products}, LogWriter: &log,
			})
			if err != nil || !result.Success || result.ExitStatus == nil || *result.ExitStatus != 0 {
				t.Fatalf("build result = %+v, error = %v", result, err)
			}
			want := ""
			if mode == "new" || mode == "new in existing directory" || mode == "replaced" {
				want = manifest
			}
			if result.XctestrunPath != want {
				t.Errorf("xctestrun path = %q, want %q", result.XctestrunPath, want)
			}
			if want != "" && log.Len() != 0 {
				t.Errorf("unexpected diagnostic: %s", log.String())
			}
			if want == "" && !strings.Contains(log.String(), "--xctestrun") {
				t.Errorf("missing artifact guidance: %s", log.String())
			}
		})
	}
}
