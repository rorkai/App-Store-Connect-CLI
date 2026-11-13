package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeBenchmarkXCConfigChain(b *testing.B, dir string, count int) string {
	b.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	for index := range count {
		contents := "CODE_SIGN_STYLE = Manual\n"
		if index+1 < count {
			contents = fmt.Sprintf("#include \"Source-%04d.xcconfig\"\n", index+1)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("Source-%04d.xcconfig", index)), []byte(contents), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	return filepath.Join(dir, "Source-0000.xcconfig")
}

func BenchmarkXCConfigIdentityCollector(b *testing.B) {
	for _, count := range []int{128, 512} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			root := writeBenchmarkXCConfigChain(b, b.TempDir(), count)
			b.ReportAllocs()
			for b.Loop() {
				files, err := collectXCConfigFiles(root, xcconfigCollectOptions{
					identify: os.Stat,
					maxFiles: signingPlanMaxFiles,
					budget:   &xcconfigSourceBudget{},
				})
				if err != nil || len(files) != count {
					b.Fatalf("collector files=%d error=%v", len(files), err)
				}
			}
		})
	}
}

func BenchmarkSigningXCConfigConsumers(b *testing.B) {
	for _, count := range []int{128, 512} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			projectPath := writeStructuredVersionProject(b, false)
			configDir := filepath.Join(filepath.Dir(projectPath), "Configs")
			writeBenchmarkXCConfigChain(b, configDir, count)
			attachSigningWidgetXCConfig(b, projectPath, "#include \"Source-0000.xcconfig\"\n")
			project, err := openSigningStructuredVersionProject(projectPath)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				_, _, _, _, protected, blocked, _, _, _, err := project.signingXCConfigConsumersWithOptionalMissing(nil, false)
				if err != nil || len(protected) != count+1 || len(blocked) != 0 {
					b.Fatalf("consumer protected=%d blocked=%d error=%v", len(protected), len(blocked), err)
				}
			}
		})
	}
}

// A bucket key must preserve EqualFold equivalence beyond ASCII lowercase,
// while the filesystem check must still retain distinct hard-linked spellings.
func TestXCConfigCollectorUnicodeBucketsPreserveCaseAndIdentityChecks(t *testing.T) {
	for _, insensitive := range []bool{true, false} {
		t.Run(fmt.Sprint(insensitive), func(t *testing.T) {
			previous := signingCaseInsensitiveVolumeFn
			signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return insensitive, true }
			t.Cleanup(func() { signingCaseInsensitiveVolumeFn = previous })
			dir := t.TempDir()
			identityPath := filepath.Join(dir, "identity")
			if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
				t.Fatal(err)
			}
			identity, err := os.Stat(identityPath)
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(dir, "Σ.xcconfig")
			alias := filepath.Join(dir, "ς.xcconfig")
			reads := 0
			files, err := collectXCConfigFiles(root, xcconfigCollectOptions{
				read: func(path string) ([]byte, error) {
					reads++
					if path == root {
						return []byte("#include \"ς.xcconfig\"\n"), nil
					}
					if path != alias {
						t.Fatalf("unexpected path %q", path)
					}
					return []byte("CODE_SIGN_STYLE = Manual\n"), nil
				},
				identify: func(string) (os.FileInfo, error) { return identity, nil },
			})
			want := 2
			if insensitive {
				want = 1
			}
			if err != nil || len(files) != want || reads != want {
				t.Fatalf("files=%v reads=%d error=%v want %d", files, reads, err, want)
			}
		})
	}
}
