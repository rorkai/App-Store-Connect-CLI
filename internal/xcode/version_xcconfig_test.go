package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestXCConfigImplicitLookupShadowsConditionalDefault(t *testing.T) {
	for _, tc := range []struct {
		name, setting, contents, implicit, want string
	}{
		{"conditional", "PROJECT_DIR", "PROJECT_DIR ?= /fallback\n", "/project", "/project"},
		{"conditional selector", "PROJECT_DIR", "PROJECT_DIR[sdk=iphoneos*] ?= /fallback\n", "/project", "/project"},
		{"inherited", "PROJECT_DIR", "PROJECT_DIR = $(inherited)/Sub\n", "/project", "/project/Sub"},
		{"append", "PROJECT_NAME", "PROJECT_NAME += Suffix\n", "App", "App Suffix"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "App.xcconfig")
			if err := os.WriteFile(path, []byte(tc.contents), 0o644); err != nil {
				t.Fatal(err)
			}
			resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(path, tc.setting, xcconfigResolvedValue{}, os.ReadFile, os.Stat, nil, func(string) (string, bool) { return tc.implicit, true })
			if err != nil {
				t.Fatal(err)
			}
			if !resolved.found || !resolved.exact || resolved.value != tc.want {
				t.Fatalf("resolved = %#v, want an exact implicit value %q", resolved, tc.want)
			}
		})
	}
}

func TestXCConfigImplicitLookupPreservesExplicitConditional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App.xcconfig")
	contents := "PROJECT_DIR[sdk=iphoneos*] = /special\nPROJECT_DIR = $(inherited)/Sub\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path,
		"PROJECT_DIR",
		xcconfigResolvedValue{},
		os.ReadFile,
		os.Stat,
		nil,
		func(string) (string, bool) { return "/project", true },
	)
	if err == nil || !strings.Contains(err.Error(), "differing conditional") {
		t.Fatalf("resolve implicit value error = %v, want divergent explicit conditional error", err)
	}
}

func TestXCConfigConditionalAssignmentExpandsInheritedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App.xcconfig")
	contents := "OTHER[sdk=iphoneos*] = $(inherited)-child\nOTHER = base-child\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path, "OTHER", xcconfigResolvedValue{}, os.ReadFile, os.Stat, nil,
		func(string) (string, bool) { return "base", true },
	)
	if err != nil {
		t.Fatalf("resolve conditional inherited value: %v", err)
	}
	if len(resolved.conditionals) != 1 || resolved.conditionals[0].value != "base-child" {
		t.Fatalf("conditional state = %#v, want expanded base-child", resolved.conditionals)
	}
}

func TestXCConfigImplicitLookupComposesInheritedConditional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App.xcconfig")
	contents := "PROJECT_DIR[sdk=iphoneos*] = $(inherited)\nPROJECT_DIR = $(inherited)\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path,
		"PROJECT_DIR",
		xcconfigResolvedValue{},
		os.ReadFile,
		os.Stat,
		nil,
		func(string) (string, bool) { return "/project", true },
	)
	if err != nil {
		t.Fatalf("resolve implicit inherited value error = %v", err)
	}
	if !resolved.found || !resolved.exact || resolved.value != "/project" {
		t.Fatalf("resolved = %#v, want an exact implicit value %q", resolved, "/project")
	}
}

func TestXCConfigImplicitLookupExpandsConditionalReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App.xcconfig")
	contents := "PROJECT_DIR[sdk=iphoneos*] = $(SRCROOT)\nPROJECT_DIR = $(inherited)\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path,
		"PROJECT_DIR",
		xcconfigResolvedValue{},
		os.ReadFile,
		os.Stat,
		nil,
		func(name string) (string, bool) {
			if name == "SRCROOT" || name == "PROJECT_DIR" {
				return "/project", true
			}
			return "", false
		},
	)
	if err != nil {
		t.Fatalf("resolve implicit conditional reference error = %v", err)
	}
	if !resolved.found || !resolved.exact || resolved.value != "/project" {
		t.Fatalf("resolved = %#v, want an exact implicit value %q", resolved, "/project")
	}
}

func TestXCConfigImplicitLookupComposesRepeatedConditionalInheritedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App.xcconfig")
	contents := "PROJECT_DIR = /base\n" +
		"PROJECT_DIR[sdk=iphoneos*] = /special\n" +
		"PROJECT_DIR[sdk=iphoneos*] = $(inherited)/Suffix\n" +
		"PROJECT_DIR = /special/Suffix\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentityAndLookup(
		path,
		"PROJECT_DIR",
		xcconfigResolvedValue{},
		os.ReadFile,
		os.Stat,
		nil,
		func(string) (string, bool) { return "/implicit", true },
	)
	if err != nil {
		t.Fatalf("resolve repeated conditional inherited value error = %v", err)
	}
	if !resolved.found || !resolved.exact || resolved.value != "/special/Suffix" {
		t.Fatalf("resolved = %#v, want an exact value %q", resolved, "/special/Suffix")
	}
}

func TestXCConfigRecursiveIncludesHandleCyclesOptionalFilesAndOrder(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	sharedPath := filepath.Join(root, "Shared.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include? \"Missing\"\n#include \"Shared\"\nMARKETING_VERSION = 3.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedPath, []byte("#include \"Root.xcconfig\"\nMARKETING_VERSION = 2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectXCConfigFiles(rootPath)
	if err != nil {
		t.Fatalf("collectXCConfigFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v, want root and shared", files)
	}
	resolved, err := resolveXCConfigSetting(rootPath, marketingVersionSetting)
	if err != nil {
		t.Fatalf("resolveXCConfigSetting() error = %v", err)
	}
	if !resolved.found || resolved.value != "3.0.0" || resolved.path != rootPath {
		t.Fatalf("unexpected resolved value: %#v", resolved)
	}
}

func TestXCConfigCollectorBoundsSigningSourceGraph(t *testing.T) {
	paths := make([]string, signingPlanMaxFiles+1)
	for i := range paths {
		paths[i] = filepath.Join(t.TempDir(), "source.xcconfig")
	}
	contents := make(map[string][]byte, len(paths))
	for i, path := range paths {
		if i+1 < len(paths) {
			contents[path] = []byte(`#include "` + paths[i+1] + `"`)
		} else {
			contents[path] = []byte("CODE_SIGN_STYLE = Manual")
		}
	}
	_, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(paths[0], func(path string) ([]byte, error) {
		data, ok := contents[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}, func(string) error { return nil }, nil, nil, nil, nil, signingPlanMaxFiles, nil)
	if err == nil || !strings.Contains(err.Error(), "more than 4096 files") {
		t.Fatalf("collectXCConfigFilesWithReader() error = %v, want aggregate source limit", err)
	}
}

// TestXCConfigDeepIncludeTraversalAllocationsScaleLinearly pins the cost model
// of both include walkers. The ancestor stack is pushed and popped per level,
// so doubling the chain depth must roughly double the allocation count.
// Copying that stack at every level made it quadruple instead, which turned a
// signing plan over a deep include chain into minutes of work on a loaded
// host.
func TestXCConfigDeepIncludeTraversalAllocationsScaleLinearly(t *testing.T) {
	const (
		shallowDepth = 256
		deepDepth    = 2 * shallowDepth
		growthBudget = 2.5
	)
	// Allocation counts alone would miss a copied map, whose per-level cost is
	// one growing allocation. Measure the allocated bytes as well.
	measure := func(run func()) (allocations, bytes float64) {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		run()
		runtime.ReadMemStats(&after)
		return float64(after.Mallocs - before.Mallocs), float64(after.TotalAlloc - before.TotalAlloc)
	}
	walkers := []struct {
		name string
		walk func(depth int) func()
	}{
		{
			name: "collector",
			walk: func(depth int) func() {
				root, read, identify := deepXCConfigIncludeChain(depth)
				return func() {
					files, err := collectXCConfigFilesWithHooksAndIdentity(root, read, identify)
					if err != nil {
						t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
					}
					if len(files) != depth {
						t.Fatalf("files = %d, want %d", len(files), depth)
					}
				}
			},
		},
		{
			name: "resolver",
			walk: func(depth int) func() {
				root, read, stat := deepXCConfigIncludeChain(depth)
				return func() {
					resolved, _, err := resolveXCConfigSettingStateWithReaderAndIdentity(
						root, "CODE_SIGN_STYLE", xcconfigResolvedValue{}, read, stat, stat, nil, nil,
					)
					if err != nil {
						t.Fatalf("resolveXCConfigSettingStateWithReaderAndIdentity() error = %v", err)
					}
					if resolved.value != "Manual" {
						t.Fatalf("resolved value = %q, want Manual", resolved.value)
					}
				}
			},
		},
	}
	for _, walker := range walkers {
		t.Run(walker.name, func(t *testing.T) {
			shallowAllocations, shallowBytes := measure(walker.walk(shallowDepth))
			deepAllocations, deepBytes := measure(walker.walk(deepDepth))
			for _, metric := range []struct {
				name    string
				shallow float64
				deep    float64
			}{
				{name: "allocations", shallow: shallowAllocations, deep: deepAllocations},
				{name: "allocated bytes", shallow: shallowBytes, deep: deepBytes},
			} {
				if metric.shallow <= 0 {
					t.Fatalf("depth %d %s = %.0f, want a positive baseline", shallowDepth, metric.name, metric.shallow)
				}
				if growth := metric.deep / metric.shallow; growth > growthBudget {
					t.Fatalf("%s grew %.2fx from depth %d to depth %d (%.0f -> %.0f), want at most %.2fx: the include ancestor stack must be pushed and popped, not copied per level",
						metric.name, growth, shallowDepth, deepDepth, metric.shallow, metric.deep, growthBudget)
				}
			}
		})
	}
}

func TestXCConfigCollectorStopsWideGraphAtLimit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Root.xcconfig")
	left := filepath.Join(filepath.Dir(root), "Left.xcconfig")
	right := filepath.Join(filepath.Dir(root), "Right.xcconfig")
	contents := map[string][]byte{root: []byte(`#include "Left.xcconfig"
#include "Right.xcconfig"`), left: []byte("A = 1"), right: []byte("B = 2")}
	readCount := make(map[string]int)
	errorCount := 0
	_, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(root, func(path string) ([]byte, error) {
		readCount[path]++
		return contents[path], nil
	}, nil, nil, func(string, error) { errorCount++ }, nil, nil, 2, nil)
	if err == nil || errorCount != 1 || readCount[right] != 0 {
		t.Fatalf("wide graph err=%v errors=%d reads=%#v, want one overflow and no sibling read", err, errorCount, readCount)
	}
}

func TestXCConfigCollectorSharesUniqueBudgetAcrossRoots(t *testing.T) {
	rootDir := t.TempDir()
	firstRoot := filepath.Join(rootDir, "First.xcconfig")
	firstChild := filepath.Join(rootDir, "First-child.xcconfig")
	secondRoot := filepath.Join(rootDir, "Second.xcconfig")
	secondChild := filepath.Join(rootDir, "Second-child.xcconfig")
	contents := map[string][]byte{
		firstRoot:   []byte(`#include "First-child.xcconfig"`),
		firstChild:  []byte("A = 1"),
		secondRoot:  []byte(`#include "Second-child.xcconfig"`),
		secondChild: []byte("B = 2"),
	}
	readCount := make(map[string]int)
	read := func(path string) ([]byte, error) {
		readCount[path]++
		data, ok := contents[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}
	budget := &xcconfigSourceBudget{}
	collect := func(root string) ([]string, error) {
		return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
			root, read, nil, nil, nil, nil, nil, 3, nil, budget,
		)
	}
	if files, err := collect(firstRoot); err != nil || len(files) != 2 {
		t.Fatalf("first collection files=%#v error=%v, want two sources", files, err)
	}
	// Repeating an already observed root must not consume the plan-wide budget.
	if files, err := collect(firstRoot); err != nil || len(files) != 2 {
		t.Fatalf("repeated first collection files=%#v error=%v, want two sources", files, err)
	}
	_, err := collect(secondRoot)
	if err == nil || !isXCConfigSourceGraphLimitError(err) || !strings.Contains(err.Error(), "more than 3 files") {
		t.Fatalf("second collection error=%v, want shared unique-source limit", err)
	}
	if readCount[secondChild] != 0 {
		t.Fatalf("second child was read after shared budget exhaustion: %d reads", readCount[secondChild])
	}
}

func TestXCConfigCollectorReplaysCompletedRootWithoutRepeatedWork(t *testing.T) {
	const sourceCount = signingPlanMaxFiles
	rootDir := t.TempDir()
	paths := make([]string, sourceCount)
	for index := range paths {
		paths[index] = filepath.Join(rootDir, fmt.Sprintf("Source-%04d.xcconfig", index))
		contents := []byte("CODE_SIGN_STYLE = Manual")
		if index+1 < len(paths) {
			contents = []byte(fmt.Sprintf("#include \"Source-%04d.xcconfig\"", index+1))
		}
		if err := os.WriteFile(paths[index], contents, 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", paths[index], err)
		}
	}

	readCount := make(map[string]int)
	statCount := make(map[string]int)
	read := func(path string) ([]byte, error) {
		readCount[path]++
		return os.ReadFile(path)
	}
	identify := func(path string) (os.FileInfo, error) {
		statCount[path]++
		return os.Stat(path)
	}
	budget := &xcconfigSourceBudget{}
	collect := func() ([]string, error) {
		return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
			paths[0], read, nil, nil, nil, identify, nil, sourceCount, nil, budget,
		)
	}
	for attempt := 0; attempt < 2; attempt++ {
		files, err := collect()
		if err != nil || len(files) != sourceCount {
			t.Fatalf("collection %d files=%d error=%v, want %d sources", attempt+1, len(files), err, sourceCount)
		}
	}
	for _, path := range paths {
		if readCount[path] != 1 || statCount[path] != 1 {
			t.Fatalf("source %s read/stat counts = %d/%d, want 1/1", path, readCount[path], statCount[path])
		}
	}
}

func TestSigningXCConfigConsumersReuseRootIdentityWork(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, false)
	attachSigningAppXCConfig(t, projectPath, "#include \"Shared.xcconfig\"\n")
	sharedPath := filepath.Join(filepath.Dir(projectPath), "Configs", "Shared.xcconfig")
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}
	project, err := openSigningStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openSigningStructuredVersionProject() error = %v", err)
	}

	previousReader := signingXCConfigReadFileFn
	previousIdentity := signingXCConfigIdentityFn
	t.Cleanup(func() {
		signingXCConfigReadFileFn = previousReader
		signingXCConfigIdentityFn = previousIdentity
	})
	readCount := make(map[string]int)
	identityCount := make(map[string]int)
	signingXCConfigReadFileFn = func(path string, limit int64) ([]byte, error) {
		readCount[path]++
		return previousReader(path, limit)
	}
	signingXCConfigIdentityFn = func(path string) (os.FileInfo, error) {
		identityCount[path]++
		return os.Stat(path)
	}

	_, configFiles, _, _, _, _, _, _, _, err := project.signingXCConfigConsumersWithOptionalMissing(nil, false)
	if err != nil {
		t.Fatalf("signingXCConfigConsumersWithOptionalMissing() error = %v", err)
	}
	var appRoot string
	for _, configuration := range project.configurations {
		if configuration.target == "App" && configuration.baseReferenceID != "" {
			if appRoot == "" {
				appRoot = configFiles[configuration.id][0]
			}
			if len(configFiles[configuration.id]) != 2 {
				t.Fatalf("App %s files=%#v, want shared two-source graph", configuration.name, configFiles[configuration.id])
			}
		}
	}
	if appRoot == "" {
		t.Fatal("project fixture has no App xcconfig root")
	}
	for _, path := range []string{appRoot, sharedPath} {
		if readCount[path] != 1 || identityCount[path] != 2 {
			t.Fatalf("source %s read/identity counts = %d/%d, want 1/2", path, readCount[path], identityCount[path])
		}
	}
}

func TestXCConfigCollectorSharedBudgetSkipsAbsentOptionalIncludeAtLimit(t *testing.T) {
	rootDir := t.TempDir()
	root := filepath.Join(rootDir, "Root.xcconfig")
	child := filepath.Join(rootDir, "Child.xcconfig")
	missing := filepath.Join(rootDir, "Missing.xcconfig")
	contents := map[string][]byte{
		root:  []byte("#include \"Child.xcconfig\"\n#include? \"Missing.xcconfig\""),
		child: []byte("A = 1"),
	}
	readCount := make(map[string]int)
	probeCount := make(map[string]int)
	var optionalMissing string
	budget := &xcconfigSourceBudget{}
	collect := func() ([]string, error) {
		return collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
			root,
			func(path string) ([]byte, error) {
				readCount[path]++
				data, ok := contents[path]
				if !ok {
					return nil, os.ErrNotExist
				}
				return data, nil
			},
			nil,
			nil,
			nil,
			nil,
			func(path string) { optionalMissing = path },
			2,
			func(path string) (os.FileInfo, error) {
				probeCount[path]++
				return nil, os.ErrNotExist
			},
			budget,
		)
	}
	if files, err := collect(); err != nil || len(files) != 2 {
		t.Fatalf("first collection files=%#v error=%v, want two sources", files, err)
	}
	readCount = make(map[string]int)
	probeCount = make(map[string]int)
	optionalMissing = ""
	files, err := collect()
	if err != nil || len(files) != 2 || optionalMissing != missing {
		t.Fatalf("repeated collection files=%#v error=%v optionalMissing=%q, want absent optional include %q", files, err, optionalMissing, missing)
	}
	if readCount[missing] != 0 || probeCount[missing] != 1 {
		t.Fatalf("absent optional read/probe counts = %d/%d, want 0/1", readCount[missing], probeCount[missing])
	}
}

func TestXCConfigCollectorSkipsAbsentOptionalIncludeAtLimit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Root.xcconfig")
	left := filepath.Join(filepath.Dir(root), "Left.xcconfig")
	missing := filepath.Join(filepath.Dir(root), "Missing.xcconfig")
	contents := map[string][]byte{
		root: []byte("#include \"Left.xcconfig\"\n#include? \"Missing.xcconfig\""),
		left: []byte("A = 1"),
	}
	readCount := make(map[string]int)
	probeCount := make(map[string]int)
	var optionalMissing string
	files, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(
		root,
		func(path string) ([]byte, error) {
			readCount[path]++
			data, ok := contents[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return data, nil
		},
		nil,
		nil,
		nil,
		nil,
		func(path string) { optionalMissing = path },
		2,
		func(path string) (os.FileInfo, error) {
			probeCount[path]++
			if _, ok := contents[path]; !ok {
				return nil, os.ErrNotExist
			}
			return fakeXCConfigFileInfo{}, nil
		},
	)
	if err != nil {
		t.Fatalf("collector error = %v, want absent optional include to be ignored", err)
	}
	if len(files) != 2 || optionalMissing != missing {
		t.Fatalf("files=%#v optionalMissing=%q, want root/left and %q", files, optionalMissing, missing)
	}
	if readCount[missing] != 0 || probeCount[missing] != 1 {
		t.Fatalf("missing optional read/probe counts = %d/%d, want 0/1", readCount[missing], probeCount[missing])
	}
}

func TestXCConfigCollectorRejectsPresentOptionalIncludeAtLimitBeforeRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Root.xcconfig")
	left := filepath.Join(filepath.Dir(root), "Left.xcconfig")
	present := filepath.Join(filepath.Dir(root), "Present.xcconfig")
	contents := map[string][]byte{
		root:    []byte("#include \"Left.xcconfig\"\n#include? \"Present.xcconfig\""),
		left:    []byte("A = 1"),
		present: []byte("B = 2"),
	}
	readCount := make(map[string]int)
	probeCount := make(map[string]int)
	_, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimit(
		root,
		func(path string) ([]byte, error) {
			readCount[path]++
			return contents[path], nil
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		2,
		func(path string) (os.FileInfo, error) {
			probeCount[path]++
			return fakeXCConfigFileInfo{}, nil
		},
	)
	if err == nil || !isXCConfigSourceGraphLimitError(err) || !strings.Contains(err.Error(), "more than 2 files") {
		t.Fatalf("collector error = %v, want typed source-graph limit", err)
	}
	if readCount[present] != 0 || probeCount[present] != 1 {
		t.Fatalf("present optional read/probe counts = %d/%d, want 0/1", readCount[present], probeCount[present])
	}
}

// fakeXCConfigFileInfo is sufficient for the optional-probe contract: the
// bounded collector only needs a non-nil result to distinguish presence from
// os.ErrNotExist when the source budget is exhausted.
type fakeXCConfigFileInfo struct{}

func (fakeXCConfigFileInfo) Name() string       { return "xcconfig" }
func (fakeXCConfigFileInfo) Size() int64        { return 0 }
func (fakeXCConfigFileInfo) Mode() os.FileMode  { return 0 }
func (fakeXCConfigFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeXCConfigFileInfo) IsDir() bool        { return false }
func (fakeXCConfigFileInfo) Sys() any           { return nil }

func TestXCConfigStableCollectorAllowsLargeGraph(t *testing.T) {
	const count = signingPlanMaxFiles + 1
	dir := t.TempDir()
	paths := make([]string, count)
	contents := make(map[string][]byte, count)
	for i := range paths {
		paths[i] = filepath.Join(dir, fmt.Sprintf("source-%d.xcconfig", i))
		if i+1 < count {
			contents[paths[i]] = []byte(fmt.Sprintf(`#include "source-%d.xcconfig"`, i+1))
		} else {
			contents[paths[i]] = []byte("CODE_SIGN_STYLE = Manual")
		}
		if err := os.WriteFile(paths[i], contents[paths[i]], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := collectStableXCConfigFiles(paths[0])
	if err != nil || len(files) != count {
		t.Fatalf("stable collector err=%v files=%d, want %d files with no signing bound", err, len(files), count)
	}
}

func TestXCConfigEditorPreservesCommentsQuotesAndMissingFinalNewline(t *testing.T) {
	input := []byte("URL = \"https://example.com/path\" // URL comment\n/* MARKETING_VERSION = 9.9.9 */\nMARKETING_VERSION[sdk=iphoneos*] ?= 1.2.3 // keep me")
	updated, oldValues, changed, err := editXCConfig(input, marketingVersionSetting, "2.0.0")
	if err != nil {
		t.Fatalf("editXCConfig() error = %v", err)
	}
	if !changed || len(oldValues) != 1 || oldValues[0] != "1.2.3" {
		t.Fatalf("unexpected edit metadata changed=%v old=%#v", changed, oldValues)
	}
	got := string(updated)
	if !strings.Contains(got, "URL = \"https://example.com/path\" // URL comment") ||
		!strings.Contains(got, "/* MARKETING_VERSION = 9.9.9 */") ||
		!strings.HasSuffix(got, "MARKETING_VERSION[sdk=iphoneos*] = 2.0.0 // keep me") ||
		strings.HasSuffix(got, "\n") {
		t.Fatalf("lossless edit failed: %q", got)
	}
}

func TestXCConfigInheritedValueAndExactPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Values.xcconfig")
	contents := "OTHER = base\nOTHER = $(inherited)-child\nCURRENT_PROJECT_VERSION[sdk=iphoneos*] = 42\nCURRENT_PROJECT_VERSION = 42\nCURRENT_PROJECT_VERSION[sdk=macosx*] = 42\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	other, err := resolveXCConfigSetting(path, "OTHER")
	if err != nil || other.value != "base-child" {
		t.Fatalf("inherited resolution = %#v, err %v", other, err)
	}
	build, err := resolveXCConfigSetting(path, currentProjectSetting)
	if err != nil || build.value != "42" || !build.exact {
		t.Fatalf("exact resolution = %#v, err %v", build, err)
	}
}

func TestXCConfigResolverAcceptsSupersededSameSelectorConditional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Values.xcconfig")
	contents := "DEVELOPMENT_TEAM[sdk=iphoneos*] = OLDOLD1234\nDEVELOPMENT_TEAM[sdk=iphoneos*] = NEWNEW1234\nDEVELOPMENT_TEAM = NEWNEW1234\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSetting(path, "DEVELOPMENT_TEAM")
	if err != nil {
		t.Fatalf("resolveXCConfigSetting() error = %v, want later same-selector assignment to replace OLDOLD1234", err)
	}
	if resolved.value != "NEWNEW1234" || !resolved.exact {
		t.Fatalf("resolved = %#v, want NEWNEW1234 from the later same-selector assignment", resolved)
	}
}

func TestXCConfigResolverAcceptsSupersededReorderedSelectorConditional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Values.xcconfig")
	contents := "DEVELOPMENT_TEAM[sdk=iphoneos*][arch=arm64] = OLDOLD1234\nDEVELOPMENT_TEAM[arch=arm64][sdk=iphoneos*] = NEWNEW1234\nDEVELOPMENT_TEAM = NEWNEW1234\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveXCConfigSetting(path, "DEVELOPMENT_TEAM")
	if err != nil {
		t.Fatalf("resolveXCConfigSetting() error = %v, want reordered same-selector assignment to replace OLDOLD1234", err)
	}
	if resolved.value != "NEWNEW1234" || !resolved.exact {
		t.Fatalf("resolved = %#v, want NEWNEW1234 from the later reordered selector", resolved)
	}
}

func TestXCConfigResolverRejectsDivergentConditionalOverride(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
	}{
		{
			name:     "unconditional-before-conditional",
			contents: "CURRENT_PROJECT_VERSION = 42\nCURRENT_PROJECT_VERSION[sdk=iphoneos*] = 100\n",
		},
		{
			name:     "conditional-before-unconditional",
			contents: "CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 100\nCURRENT_PROJECT_VERSION = 42\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Values.xcconfig")
			if err := os.WriteFile(path, []byte(test.contents), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := resolveXCConfigSetting(path, currentProjectSetting)
			if err == nil || !strings.Contains(err.Error(), "differing conditional") {
				t.Fatalf("expected divergent conditional error, got %v", err)
			}
		})
	}
}

func TestXCConfigResolverIgnoresShadowedConditionalDefaults(t *testing.T) {
	for _, contents := range []string{
		"CODE_SIGN_ENTITLEMENTS = App.entitlements\nCODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= $(MISSING)\n",
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= $(MISSING)\nCODE_SIGN_ENTITLEMENTS = App.entitlements\n",
	} {
		path := filepath.Join(t.TempDir(), "Values.xcconfig")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("WriteFile(values) error = %v", err)
		}
		resolved, err := resolveXCConfigSetting(path, "CODE_SIGN_ENTITLEMENTS")
		if err != nil {
			t.Fatalf("resolveXCConfigSetting() error = %v", err)
		}
		if resolved.value != "App.entitlements" || !resolved.exact {
			t.Fatalf("resolved = %#v, want final concrete entitlement", resolved)
		}
	}
}

func TestXCConfigOperatorsQuotesAndIncludeOrder(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	childPath := filepath.Join(root, "Child.xcconfig")
	if err := os.WriteFile(rootPath, []byte("OTHER = base\n#include \"Child.xcconfig\"\nMARKETING_VERSION ?= \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(childPath, []byte("OTHER += child\nOTHER ?= ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	other, err := resolveXCConfigSetting(rootPath, "OTHER")
	if err != nil || other.value != "base child" {
		t.Fatalf("operator/include resolution = %#v, err %v", other, err)
	}
	version, err := resolveXCConfigSetting(rootPath, marketingVersionSetting)
	if err != nil || version.value != "1.2.3" {
		t.Fatalf("quoted/default resolution = %#v, err %v", version, err)
	}
}

func TestXCConfigEditorNormalizesOperatorsAndPreservesQuotes(t *testing.T) {
	for _, operator := range []string{"+=", "?="} {
		t.Run(operator, func(t *testing.T) {
			input := []byte("MARKETING_VERSION " + operator + " \"1.2.3\" // keep\n")
			updated, oldValues, changed, err := editXCConfig(input, marketingVersionSetting, "2.0.0")
			if err != nil {
				t.Fatalf("editXCConfig() error = %v", err)
			}
			if !changed || len(oldValues) != 1 || oldValues[0] != "1.2.3" {
				t.Fatalf("unexpected edit metadata changed=%v old=%#v", changed, oldValues)
			}
			if got := string(updated); got != "MARKETING_VERSION = \"2.0.0\" // keep\n" {
				t.Fatalf("operator/quote edit = %q", got)
			}
		})
	}
}

func TestXCConfigEditorEscapesTrailingBackslashInQuotedValue(t *testing.T) {
	input := []byte("MARKETING_VERSION = \"1.2.3\"\n")
	updated, oldValues, changed, err := editXCConfig(input, marketingVersionSetting, "2.0.0\\")
	if err != nil {
		t.Fatalf("editXCConfig() error = %v", err)
	}
	if !changed || len(oldValues) != 1 || oldValues[0] != "1.2.3" {
		t.Fatalf("unexpected edit metadata changed=%v old=%#v", changed, oldValues)
	}
	if got := string(updated); !strings.Contains(got, `MARKETING_VERSION = "2.0.0\\"`) {
		t.Fatalf("quoted value did not escape trailing backslash: %q", got)
	}
	if _, err := parseXCConfig(updated); err != nil {
		t.Fatalf("escaped quoted value is not parseable: %v", err)
	}
}

func TestXCConfigQuotedValueRoundTripsEscapedBackslashesAndQuotes(t *testing.T) {
	tests := []struct {
		name  string
		value string
		quote string
	}{
		{name: "unquoted backslashes", value: `Profiles\Team\`, quote: ""},
		{name: "double quoted backslashes", value: `Profiles\Team\`, quote: `"`},
		{name: "double quoted matching quote", value: `Profile "Preview"\Team\`, quote: `"`},
		{name: "single quoted matching quote", value: `Profile 'Preview'\Team\`, quote: `'`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := quoteXCConfigValue(test.value, test.quote)
			decoded, quote, err := parseXCConfigValue(encoded)
			if err != nil {
				t.Fatalf("parseXCConfigValue(%q) error = %v", encoded, err)
			}
			if decoded != test.value || quote != test.quote {
				t.Fatalf("quoted round-trip = value %q, quote %q; want value %q, quote %q", decoded, quote, test.value, test.quote)
			}
		})
	}
}

func TestXCConfigParserPreservesQuoteStateForContinuedBlockCommentDelimiters(t *testing.T) {
	document, err := parseXCConfig([]byte("HEADER_SEARCH_PATHS = \"Vendor\\\n/*SDK\"\n"))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v, want quoted continuation containing /* accepted", err)
	}
	if len(document.assignments) != 1 || !strings.Contains(document.assignments[0].value, "Vendor") || !strings.Contains(document.assignments[0].value, "/*SDK") {
		t.Fatalf("parseXCConfig() assignments = %#v, want quoted /* preserved across the continuation", document.assignments)
	}
}

func TestXCConfigParserPreservesQuoteStateAcrossContinuedLines(t *testing.T) {
	document, err := parseXCConfig([]byte("HEADER_SEARCH_PATHS = \"https:\\\n//host/path\"\n"))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v, want quoted URL continuation accepted", err)
	}
	if len(document.assignments) != 1 || !strings.Contains(document.assignments[0].value, "https://host/path") {
		t.Fatalf("parseXCConfig() assignments = %#v, want https://host/path from the continued quoted value", document.assignments)
	}
}

func TestXCConfigParserAcceptsQuotedLineContinuations(t *testing.T) {
	document, err := parseXCConfig([]byte("HEADER_SEARCH_PATHS = \"Vendor \\\n SDK\"\n"))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v, want quoted line continuation accepted", err)
	}
	if len(document.assignments) != 1 || document.assignments[0].key != "HEADER_SEARCH_PATHS" {
		t.Fatalf("parseXCConfig() assignments = %#v, want one HEADER_SEARCH_PATHS assignment", document.assignments)
	}
	if !strings.Contains(document.assignments[0].value, "Vendor") || !strings.Contains(document.assignments[0].value, "SDK") {
		t.Fatalf("parseXCConfig() value = %q, want Vendor and SDK from the continued quoted assignment", document.assignments[0].value)
	}
}

func TestXCConfigParserPreservesGenericQuotedEscapes(t *testing.T) {
	document, err := parseXCConfig([]byte(`HEADER_SEARCH_PATHS = "Vendor\ SDK"` + "\n"))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v, want generic quoted escape preserved", err)
	}
	if len(document.assignments) != 1 || document.assignments[0].value != `Vendor\ SDK` {
		t.Fatalf("parseXCConfig() assignments = %#v, want preserved backslash-space", document.assignments)
	}
}

func TestXCConfigParserRejectsMalformedQuotedEscapes(t *testing.T) {
	for _, raw := range []string{`"dangling\`} {
		if _, _, err := parseXCConfigValue(raw); err == nil {
			t.Fatalf("parseXCConfigValue(%q) error = nil, want malformed escape rejection", raw)
		}
	}
}

func TestXCConfigEditorPreservesStableVersionContinuationSupport(t *testing.T) {
	input := []byte("MARKETING_VERSION = 1.2.3\\\n 4\n")
	updated, oldValues, changed, err := editXCConfig(input, marketingVersionSetting, "2.0.0")
	if err != nil {
		t.Fatalf("editXCConfig() error = %v, want stable version edit compatibility", err)
	}
	if !changed || len(oldValues) != 1 || oldValues[0] != "1.2.3\\" {
		t.Fatalf("unexpected continuation edit metadata changed=%v old=%#v", changed, oldValues)
	}
	if got := string(updated); !strings.Contains(got, "MARKETING_VERSION = 2.0.0\n") {
		t.Fatalf("continuation assignment was not updated: %q", got)
	}
}

func TestXCConfigResolverRejectsConditionalOnlySetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Conditional.xcconfig")
	contents := "CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 41\nCURRENT_PROJECT_VERSION[sdk=macosx*] = 43\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveXCConfigSetting(path, currentProjectSetting)
	if err == nil || !strings.Contains(err.Error(), "conditional") {
		t.Fatalf("expected conditional-only setting error, got %v", err)
	}
}

func TestXCConfigParserRejectsUnterminatedBlockComment(t *testing.T) {
	_, err := parseXCConfig([]byte("MARKETING_VERSION = 1.0\n/* never closed"))
	if err == nil {
		t.Fatal("expected unterminated-comment error")
	}
}

func TestXCConfigParserPropagatesBlockCommentStateAcrossContinuation(t *testing.T) {
	data := []byte("OTHER = \"SDK\\\n\" /* comment\nMARKETING_VERSION = 9.9.9 */\nCURRENT_PROJECT_VERSION = 2\n")
	document, err := parseXCConfig(data)
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v", err)
	}
	if len(document.assignments) != 2 {
		t.Fatalf("assignments = %#v, want only OTHER and CURRENT_PROJECT_VERSION", document.assignments)
	}
	if document.assignments[0].key != "OTHER" || document.assignments[1].key != "CURRENT_PROJECT_VERSION" {
		t.Fatalf("assignments = %#v, want comment-contained line ignored", document.assignments)
	}
}

func TestXCConfigParserRejectsUnterminatedQuotedValues(t *testing.T) {
	for _, input := range []string{
		`DEVELOPMENT_TEAM = "BROKEN`,
		`DEVELOPMENT_TEAM = 'BROKEN`,
		`DEVELOPMENT_TEAM = "BROKEN\"`,
	} {
		_, err := parseXCConfig([]byte(input))
		if err == nil || !strings.Contains(err.Error(), "unterminated quote") {
			t.Fatalf("parseXCConfig(%q) error = %v, want unterminated quote", input, err)
		}
	}
}

func TestXCConfigParserStripsInlineBlockCommentsFromAssignmentValues(t *testing.T) {
	document, err := parseXCConfig([]byte("CODE_SIGN_IDENTITY = Apple /* note */ Development\n"))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v", err)
	}
	if len(document.assignments) != 1 {
		t.Fatalf("parseXCConfig() assignments = %#v, want one identity assignment", document.assignments)
	}
	if strings.Contains(document.assignments[0].value, "/*") || strings.Contains(document.assignments[0].value, "note") {
		t.Fatalf("parseXCConfig() value = %q, want comment stripped from the assignment value", document.assignments[0].value)
	}
	if !strings.Contains(document.assignments[0].value, "Apple") || !strings.Contains(document.assignments[0].value, "Development") {
		t.Fatalf("parseXCConfig() value = %q, want the surrounding identity tokens", document.assignments[0].value)
	}
}

func TestXCConfigEditorTreatsApostrophesInUnquotedValuesAsLiterals(t *testing.T) {
	input := "INFOPLIST_KEY_CFBundleDisplayName = Developer's App // keep this comment\nMARKETING_VERSION = 1.2.3\n"
	document, err := parseXCConfig([]byte(input))
	if err != nil {
		t.Fatalf("parseXCConfig() error = %v", err)
	}
	if len(document.assignments) != 2 || document.assignments[0].value != "Developer's App" {
		t.Fatalf("parseXCConfig() assignments = %#v", document.assignments)
	}

	updated, oldValues, changed, err := editXCConfig([]byte(input), marketingVersionSetting, "2.0.0")
	if err != nil {
		t.Fatalf("editXCConfig() error = %v", err)
	}
	if !changed || len(oldValues) != 1 || oldValues[0] != "1.2.3" {
		t.Fatalf("editXCConfig() changed=%t oldValues=%#v", changed, oldValues)
	}
	if got := string(updated); !strings.Contains(got, "INFOPLIST_KEY_CFBundleDisplayName = Developer's App // keep this comment\n") {
		t.Fatalf("editXCConfig() changed unquoted apostrophe value: %q", got)
	}
}

func TestXCConfigRequiredMissingIncludeFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Root.xcconfig")
	if err := os.WriteFile(path, []byte("#include \"Missing.xcconfig\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := collectXCConfigFiles(path)
	if err == nil {
		t.Fatal("expected required-include error")
	}
}

func TestXCConfigOptionalIncludePropagatesMissingRequiredDescendant(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	optionalPath := filepath.Join(root, "Optional.xcconfig")
	missingPath := filepath.Join(root, "Missing.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include? \"Optional.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	if err := os.WriteFile(optionalPath, []byte("#include \"Missing.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(optional) error = %v", err)
	}

	seen := make(map[string]bool)
	_, err := collectXCConfigFilesWithHooks(
		rootPath,
		os.ReadFile,
		nil,
		func(path string) { seen[filepath.Clean(path)] = true },
		nil,
	)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("collectXCConfigFilesWithHooks() error = %v, want descendant missing-include error", err)
	}
	for _, path := range []string{rootPath, optionalPath, missingPath} {
		if !seen[path] {
			t.Fatalf("collector did not retain lexical path %q: %#v", path, seen)
		}
	}
}

func TestXCConfigCollectorRecordsMissingIncludeBeforeAccess(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	missingPath := filepath.Join(root, "Missing.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include \"Missing.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}

	seen := make(map[string]bool)
	readPaths := make(map[string]bool)
	var errorsByPath map[string]error
	_, err := collectXCConfigFilesWithHooks(
		rootPath,
		func(path string) ([]byte, error) {
			readPaths[filepath.Clean(path)] = true
			return os.ReadFile(path)
		},
		func(path string) error {
			if path == missingPath && !seen[path] {
				t.Fatalf("missing include was authorized before path was recorded")
			}
			return nil
		},
		func(path string) { seen[filepath.Clean(path)] = true },
		func(path string, collectionErr error) {
			if errorsByPath == nil {
				errorsByPath = make(map[string]error)
			}
			errorsByPath[filepath.Clean(path)] = collectionErr
		},
	)
	if err == nil {
		t.Fatal("collector error = nil, want required missing include failure")
	}
	if !seen[missingPath] {
		t.Fatalf("missing include was not retained by path hook: %#v", seen)
	}
	if !readPaths[missingPath] {
		t.Fatalf("missing include did not use the authorized reader: %#v", readPaths)
	}
	if errorsByPath[missingPath] == nil {
		t.Fatalf("missing include was not retained by error hook: %#v", errorsByPath)
	}
}

func TestXCConfigCollectorRecordsOptionalMissingIncludeAbsence(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	missingPath := filepath.Join(root, "Missing.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include? \"Missing.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}

	var optionalMissing string
	readPaths := make(map[string]bool)
	_, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissing(
		rootPath,
		func(path string) ([]byte, error) {
			readPaths[filepath.Clean(path)] = true
			return os.ReadFile(path)
		},
		nil,
		nil,
		nil,
		nil,
		func(path string) { optionalMissing = filepath.Clean(path) },
	)
	if err != nil {
		t.Fatalf("collector error = %v, want optional absence to be ignored", err)
	}
	if optionalMissing != missingPath {
		t.Fatalf("optional missing path = %q, want %q", optionalMissing, missingPath)
	}
	if !readPaths[missingPath] {
		t.Fatalf("optional missing target did not use the authorized reader: %#v", readPaths)
	}
}

func TestXCConfigFileIdentityRequiresAuthorizedCollectionBeforeInspection(t *testing.T) {
	root := t.TempDir()
	externalPath := filepath.Join(root, "external.xcconfig")
	if err := os.WriteFile(externalPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(external) error = %v", err)
	}
	linkPath := filepath.Join(root, "unselected.xcconfig")
	if err := os.Symlink(externalPath, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	index := xcconfigFileIdentityIndex{}
	_, err := index.identity(linkPath, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "was not collected") {
		t.Fatalf("identity() error = %v, want collection-membership refusal", err)
	}
	if len(index.entries) != 0 {
		t.Fatalf("identity index entries = %#v, want empty after unauthorized path", index.entries)
	}
}

func TestXCConfigCollectorRejectsUnauthorizedSymlinkBeforeReader(t *testing.T) {
	root := t.TempDir()
	externalRoot := t.TempDir()
	externalPath := filepath.Join(externalRoot, "external.xcconfig")
	if err := os.WriteFile(externalPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(external) error = %v", err)
	}
	selectedPath := filepath.Join(root, "selected.xcconfig")
	if err := os.Symlink(externalPath, selectedPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	readCalls := 0
	paths := make(map[string]bool)
	_, err := collectXCConfigFilesWithHooks(
		selectedPath,
		func(path string) ([]byte, error) {
			readCalls++
			return os.ReadFile(path)
		},
		func(path string) error {
			if path != selectedPath {
				t.Fatalf("authorization path = %q, want %q", path, selectedPath)
			}
			return errors.New("external path not authorized")
		},
		func(path string) { paths[path] = true },
		nil,
	)
	if err == nil {
		t.Fatal("collector error = nil, want authorization refusal")
	}
	if readCalls != 0 {
		t.Fatalf("reader calls = %d, want zero before authorization", readCalls)
	}
	if !paths[selectedPath] {
		t.Fatalf("path hook did not retain unauthorized path: %#v", paths)
	}
}

func TestXCConfigCollectorContinuesSiblingIncludesAfterAuthorizationFailure(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	blockedPath := filepath.Join(root, "Blocked.xcconfig")
	allowedPath := filepath.Join(root, "Allowed.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include \"Blocked.xcconfig\"\n#include \"Allowed.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	if err := os.WriteFile(allowedPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(allowed) error = %v", err)
	}

	paths := make(map[string]bool)
	readPaths := make(map[string]bool)
	_, err := collectXCConfigFilesWithHooks(
		rootPath,
		func(path string) ([]byte, error) {
			readPaths[filepath.Clean(path)] = true
			return os.ReadFile(path)
		},
		func(path string) error {
			if filepath.Clean(path) == blockedPath {
				return errors.New("blocked include")
			}
			return nil
		},
		func(path string) { paths[filepath.Clean(path)] = true },
		nil,
	)
	if err == nil {
		t.Fatal("collector error = nil, want blocked include failure")
	}
	if !paths[rootPath] || !paths[blockedPath] || !paths[allowedPath] {
		t.Fatalf("paths = %#v, want root and both sibling includes", paths)
	}
	if readPaths[blockedPath] {
		t.Fatalf("blocked include was read despite authorization failure: %#v", readPaths)
	}
	if !readPaths[allowedPath] {
		t.Fatalf("allowed sibling include was not read after earlier failure: %#v", readPaths)
	}
}

func TestXCConfigCollectorUsesWindowsCaseInsensitiveTraversalKeys(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	rootPath := filepath.Join(root, "Config.xcconfig")
	caseVariantPath := filepath.Join(root, "config.xcconfig")

	readCalls := 0
	files, err := collectXCConfigFilesWithReader(rootPath, func(path string) ([]byte, error) {
		readCalls++
		switch filepath.Clean(path) {
		case rootPath:
			return []byte("#include \"config.xcconfig\"\n"), nil
		case caseVariantPath:
			return []byte("CODE_SIGN_STYLE = Manual\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}, nil)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithReader() error = %v", err)
	}
	if len(files) != 1 || files[0] != rootPath {
		t.Fatalf("files = %#v, want one case-insensitive traversal", files)
	}
	if readCalls != 1 {
		t.Fatalf("read calls = %d, want one for the same Windows path", readCalls)
	}
}

func TestXCConfigCollectorKeepsCaseDistinctFilesOnCaseSensitiveHost(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	rootPath := filepath.Join(root, "Config.xcconfig")
	caseVariantPath := filepath.Join(root, "config.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include \"config.xcconfig\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	if _, err := os.Lstat(caseVariantPath); err == nil {
		t.Skip("temporary filesystem is case-insensitive")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(case variant) error = %v", err)
	}
	if err := os.WriteFile(caseVariantPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(case variant) error = %v", err)
	}

	readCalls := 0
	files, err := collectXCConfigFilesWithReader(rootPath, func(path string) ([]byte, error) {
		readCalls++
		return os.ReadFile(path)
	}, nil)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithReader() error = %v", err)
	}
	if len(files) != 2 || files[0] != rootPath || files[1] != caseVariantPath {
		t.Fatalf("files = %#v, want both case-distinct files", files)
	}
	if readCalls != 2 {
		t.Fatalf("read calls = %d, want one per case-distinct file", readCalls)
	}
}

func TestIdentityAwareDarwinCollectorDeduplicatesCaseVariantSameFile(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "darwin"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootPath := filepath.Join(root, "Config.xcconfig")
	caseVariantPath := filepath.Join(root, "config.xcconfig")
	// The identity and reader seams model a case-insensitive volume even when
	// this test runs on a case-sensitive temporary filesystem.
	identityPath := filepath.Join(root, "identity")
	if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}
	identity, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("Stat(identity) error = %v", err)
	}
	readCalls := 0
	files, err := collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			readCalls++
			if filepath.Clean(path) == rootPath {
				return []byte("#include \"config.xcconfig\"\n"), nil
			}
			return []byte("CODE_SIGN_STYLE = Manual\n"), nil
		},
		func(string) (os.FileInfo, error) { return identity, nil },
	)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
	}
	if len(files) != 1 || files[0] != rootPath {
		t.Fatalf("files = %#v, want one identity-equivalent path", files)
	}
	if readCalls != 1 {
		t.Fatalf("read calls = %d, want one after identity deduplication", readCalls)
	}
	_ = caseVariantPath
}

func TestIdentityAwareLinuxCollectorDeduplicatesCaseVariantSameFile(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "linux"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootPath := filepath.Join(root, "Config.xcconfig")
	identityPath := filepath.Join(root, "identity")
	if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}
	identity, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("Stat(identity) error = %v", err)
	}
	readCalls := 0
	files, err := collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			readCalls++
			if filepath.Clean(path) == rootPath {
				return []byte("#include \"config.xcconfig\"\n"), nil
			}
			return []byte("CODE_SIGN_STYLE = Manual\nCODE_SIGN_STYLE += Extra\n"), nil
		},
		func(string) (os.FileInfo, error) { return identity, nil },
	)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
	}
	if len(files) != 1 || files[0] != rootPath {
		t.Fatalf("files = %#v, want one identity-equivalent path on Linux", files)
	}
	if readCalls != 1 {
		t.Fatalf("read calls = %d, want one after Linux identity deduplication", readCalls)
	}
	if !xcconfigUsesIdentityTraversal() {
		t.Fatal("linux should enable identity-aware xcconfig traversal")
	}
}

func TestIdentityAwareWindowsCollectorKeepsCaseDistinctFilesOnCaseSensitiveDirectory(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	firstPath := filepath.Join(root, "First.xcconfig")
	secondPath := filepath.Join(root, "first.xcconfig")
	identityPath := filepath.Join(root, "identity")
	if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}
	identity, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("Stat(identity) error = %v", err)
	}
	files, err := collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			switch filepath.Clean(path) {
			case rootPath:
				return []byte("#include \"First.xcconfig\"\n#include \"first.xcconfig\"\n"), nil
			case firstPath, secondPath:
				return []byte("CODE_SIGN_STYLE = Manual\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		func(string) (os.FileInfo, error) { return identity, nil },
	)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
	}
	if len(files) != 3 || files[0] != rootPath || files[1] != firstPath || files[2] != secondPath {
		t.Fatalf("files = %#v, want both case-distinct Windows includes", files)
	}
}

func TestIdentityAwareWindowsCollectorCoalescesCaseVariantOnCaseInsensitiveDirectory(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	firstPath := filepath.Join(root, "First.xcconfig")
	secondPath := filepath.Join(root, "first.xcconfig")
	identityPath := filepath.Join(root, "identity")
	if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}
	identity, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("Stat(identity) error = %v", err)
	}
	files, err := collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			switch filepath.Clean(path) {
			case rootPath:
				return []byte("#include \"First.xcconfig\"\n#include \"first.xcconfig\"\n"), nil
			case firstPath, secondPath:
				return []byte("CODE_SIGN_STYLE = Manual\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		func(string) (os.FileInfo, error) { return identity, nil },
	)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
	}
	if len(files) != 2 || files[0] != rootPath || files[1] != firstPath {
		t.Fatalf("files = %#v, want one case-insensitive Windows include", files)
	}
}

func TestSigningPathCaseEquivalentUsesWindowsDirectorySemantics(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	upper := filepath.Join(root, "Config.xcconfig")
	lower := filepath.Join(root, "config.xcconfig")
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	if signingPathCaseEquivalent(upper, lower) {
		t.Fatal("case-sensitive Windows directory collapsed case-distinct paths")
	}
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	if !signingPathCaseEquivalent(upper, lower) {
		t.Fatal("case-insensitive Windows directory did not coalesce case-variant paths")
	}
}

func TestIdentityAwareCollectorKeepsCaseDistinctHardlinkedIncludesOnCaseSensitiveHost(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	firstPath := filepath.Join(root, "First.xcconfig")
	secondPath := filepath.Join(root, "first.xcconfig")
	identityPath := filepath.Join(root, "identity")
	if err := os.WriteFile(identityPath, []byte("identity"), 0o600); err != nil {
		t.Fatalf("WriteFile(identity) error = %v", err)
	}
	identity, err := os.Stat(identityPath)
	if err != nil {
		t.Fatalf("Stat(identity) error = %v", err)
	}

	files, err := collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			switch filepath.Clean(path) {
			case rootPath:
				return []byte("#include \"First.xcconfig\"\n#include \"first.xcconfig\"\n"), nil
			case firstPath, secondPath:
				return []byte("CODE_SIGN_STYLE = Manual\n"), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		func(path string) (os.FileInfo, error) {
			return identity, nil
		},
	)
	if err != nil {
		t.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
	}
	if len(files) != 3 || files[0] != rootPath || files[1] != firstPath || files[2] != secondPath {
		t.Fatalf("files = %#v, want root and case-distinct include", files)
	}
}

func TestStableVersionResolverUsesLaterCaseDistinctInclude(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	appPath := filepath.Join(root, "App.xcconfig")
	basePath := filepath.Join(root, "Base.xcconfig")
	lowerBasePath := filepath.Join(root, "base.xcconfig")
	appBacking := filepath.Join(root, "app.backing")
	baseBacking := filepath.Join(root, "base.backing")
	lowerBaseBacking := filepath.Join(root, "lower-base.backing")
	for path, contents := range map[string]string{
		appBacking:       "#include \"Base.xcconfig\"\n",
		baseBacking:      "MARKETING_VERSION = 1.0.0\n#include \"base.xcconfig\"\n",
		lowerBaseBacking: "MARKETING_VERSION = 2.0.0\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}
	identities := make(map[string]os.FileInfo)
	for requested, backing := range map[string]string{
		appPath:       appBacking,
		basePath:      baseBacking,
		lowerBasePath: lowerBaseBacking,
	} {
		info, err := os.Stat(backing)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", backing, err)
		}
		identities[requested] = info
	}
	contents := map[string][]byte{
		appPath:       []byte("#include \"Base.xcconfig\"\n"),
		basePath:      []byte("MARKETING_VERSION = 1.0.0\n#include \"base.xcconfig\"\n"),
		lowerBasePath: []byte("MARKETING_VERSION = 2.0.0\n"),
	}
	read := func(path string) ([]byte, error) {
		data, ok := contents[filepath.Clean(path)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}
	stat := func(path string) (os.FileInfo, error) {
		info, ok := identities[filepath.Clean(path)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return info, nil
	}
	resolved, err := resolveXCConfigSettingWithBaseReaderAndIdentity(
		appPath,
		marketingVersionSetting,
		xcconfigResolvedValue{},
		read,
		stat,
		stat,
	)
	if err != nil {
		t.Fatalf("resolveXCConfigSettingWithBaseReaderAndIdentity() error = %v", err)
	}
	if resolved.value != "2.0.0" || resolved.path != lowerBasePath {
		t.Fatalf("resolved = %#v, want later case-distinct include", resolved)
	}
}

func TestSigningXCConfigConsumersKeepsWindowsCaseDistinctFilesByIdentity(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	projectPath := writeStructuredVersionProject(t, true)
	configDir := filepath.Join(filepath.Dir(projectPath), "Configs")
	rootPath := filepath.Join(configDir, "App.xcconfig")
	caseVariantPath := filepath.Join(configDir, "app.xcconfig")
	rootContents := mustReadVersionTestFile(t, rootPath)
	previousReader := signingXCConfigReadFileFn
	previousIdentity := signingXCConfigIdentityFn
	signingXCConfigReadFileFn = func(path string, _ int64) ([]byte, error) {
		switch filepath.Clean(path) {
		case rootPath:
			return []byte("#include \"app.xcconfig\"\n" + rootContents), nil
		case caseVariantPath:
			return []byte("CODE_SIGN_STYLE = Manual\n"), nil
		default:
			return previousReader(path, signingPlanMaxBytes)
		}
	}
	identityDir := t.TempDir()
	firstIdentityPath := filepath.Join(identityDir, "first")
	secondIdentityPath := filepath.Join(identityDir, "second")
	if err := os.WriteFile(firstIdentityPath, []byte("first"), 0o600); err != nil {
		t.Fatalf("WriteFile(first identity) error = %v", err)
	}
	if err := os.WriteFile(secondIdentityPath, []byte("second"), 0o600); err != nil {
		t.Fatalf("WriteFile(second identity) error = %v", err)
	}
	firstInfo, err := os.Stat(firstIdentityPath)
	if err != nil {
		t.Fatalf("Stat(first identity) error = %v", err)
	}
	secondInfo, err := os.Stat(secondIdentityPath)
	if err != nil {
		t.Fatalf("Stat(second identity) error = %v", err)
	}
	signingXCConfigIdentityFn = func(path string) (os.FileInfo, error) {
		switch filepath.Clean(path) {
		case rootPath:
			return firstInfo, nil
		case caseVariantPath:
			return secondInfo, nil
		default:
			return previousIdentity(path)
		}
	}
	t.Cleanup(func() {
		signingXCConfigReadFileFn = previousReader
		signingXCConfigIdentityFn = previousIdentity
	})

	project, err := openStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	_, configFiles, _, _, _, _, _, _, err := project.signingXCConfigConsumers(nil, false)
	if err != nil {
		t.Fatalf("signingXCConfigConsumers() error = %v", err)
	}
	for configurationID, files := range configFiles {
		if !containsString(files, caseVariantPath) {
			t.Fatalf("configuration %q files = %#v, want case-distinct include %q", configurationID, files, caseVariantPath)
		}
	}
}

func TestIdentityAwareWindowsCollectorReportsMissingRequiredCaseVariant(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	rootPath := filepath.Join(root, "Config.xcconfig")
	missingPath := filepath.Join(root, "config.xcconfig")
	if err := os.WriteFile(rootPath, []byte("#include \"config.xcconfig\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		t.Fatalf("Stat(root) error = %v", err)
	}
	readMissing := false
	_, err = collectXCConfigFilesWithHooksAndIdentity(
		rootPath,
		func(path string) ([]byte, error) {
			if filepath.Clean(path) == missingPath {
				readMissing = true
				return nil, os.ErrNotExist
			}
			return os.ReadFile(path)
		},
		func(path string) (os.FileInfo, error) {
			if filepath.Clean(path) == missingPath {
				return nil, os.ErrNotExist
			}
			return rootInfo, nil
		},
	)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("collector error = %v, want required include not-exist failure", err)
	}
	if !readMissing {
		t.Fatal("required missing case-variant include was mistaken for an existing-file cycle")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
