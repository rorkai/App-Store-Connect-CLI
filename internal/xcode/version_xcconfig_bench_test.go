package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// deepXCConfigChainIdentity is a distinct identity per source path. Synthetic
// FileInfo values never satisfy os.SameFile, which is the worst case for the
// identity-aware ancestor scans the signing collector and resolver perform on
// every supported platform.
type deepXCConfigChainIdentity struct {
	fakeXCConfigFileInfo
	name string
}

func (identity deepXCConfigChainIdentity) Name() string { return identity.name }

// deepXCConfigIncludeChain builds an in-memory include chain of the requested
// depth and returns its root together with matching reader and stat/identity
// hooks. Signing callers always supply an identity hook, so the measurement
// uses one too. Faking the filesystem keeps the measurement on the include
// traversal itself, which is where a per-level copy of the ancestor stack
// turned a deep chain into quadratic work.
func deepXCConfigIncludeChain(depth int) (string, func(string) ([]byte, error), func(string) (os.FileInfo, error)) {
	dir := filepath.Join(os.TempDir(), "asc-xcconfig-include-chain")
	contents := make(map[string][]byte, depth)
	for index := 0; index < depth; index++ {
		path := filepath.Join(dir, fmt.Sprintf("Chain-%05d.xcconfig", index))
		if index+1 < depth {
			contents[path] = []byte(fmt.Sprintf("#include \"Chain-%05d.xcconfig\"\n", index+1))
			continue
		}
		contents[path] = []byte("CODE_SIGN_STYLE = Manual\n")
	}
	read := func(path string) ([]byte, error) {
		data, ok := contents[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}
	stat := func(path string) (os.FileInfo, error) {
		if _, ok := contents[path]; !ok {
			return nil, os.ErrNotExist
		}
		return deepXCConfigChainIdentity{name: filepath.Base(path)}, nil
	}
	return filepath.Join(dir, "Chain-00000.xcconfig"), read, stat
}

func BenchmarkCollectXCConfigFilesDeepIncludeChain(b *testing.B) {
	for _, depth := range []int{256, 512, 1024} {
		root, read, identify := deepXCConfigIncludeChain(depth)
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				files, err := collectXCConfigFilesWithHooksAndIdentity(root, read, identify)
				if err != nil {
					b.Fatalf("collectXCConfigFilesWithHooksAndIdentity() error = %v", err)
				}
				if len(files) != depth {
					b.Fatalf("files = %d, want %d", len(files), depth)
				}
			}
		})
	}
}

func BenchmarkResolveXCConfigSettingDeepIncludeChain(b *testing.B) {
	for _, depth := range []int{256, 512, 1024} {
		root, read, stat := deepXCConfigIncludeChain(depth)
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				resolved, _, err := resolveXCConfigSettingStateWithReaderAndIdentity(
					root, "CODE_SIGN_STYLE", xcconfigResolvedValue{}, read, stat, stat, nil, nil,
				)
				if err != nil {
					b.Fatalf("resolveXCConfigSettingStateWithReaderAndIdentity() error = %v", err)
				}
				if resolved.value != "Manual" {
					b.Fatalf("resolved value = %q, want Manual", resolved.value)
				}
			}
		})
	}
}
