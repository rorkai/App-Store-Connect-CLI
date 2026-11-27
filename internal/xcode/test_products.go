package xcode

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type testProductsSnapshot struct {
	path      string
	root      *rootfs.Root
	directory *os.File
	previous  []os.FileInfo
	files     []*os.File
	ready     bool
}

// Retaining descriptors prevents deleted files from having their identities
// reused during the build. Contents and timestamps do not prove a fresh output.
func snapshotTestProducts(args []string) *testProductsSnapshot {
	var snapshot *testProductsSnapshot
	for _, arg := range args {
		if strings.HasPrefix(arg, "SYMROOT[") {
			return &testProductsSnapshot{}
		}
		if value, ok := strings.CutPrefix(arg, "SYMROOT="); ok {
			if snapshot != nil {
				return &testProductsSnapshot{}
			}
			snapshot = &testProductsSnapshot{path: value}
		}
	}
	if snapshot == nil || !filepath.IsAbs(snapshot.path) || strings.Contains(snapshot.path, "$(") || strings.Contains(snapshot.path, "${") {
		return snapshot
	}
	snapshot.path = filepath.Clean(snapshot.path)
	root, err := rootfs.New(filepath.Dir(snapshot.path))
	if err != nil {
		return snapshot
	}
	snapshot.root = &root
	parent, err := root.OpenDir(".")
	if err != nil {
		return snapshot
	}
	_ = parent.Close()
	snapshot.directory, err = root.OpenDir(filepath.Base(snapshot.path))
	if errors.Is(err, os.ErrNotExist) {
		snapshot.ready = true
		return snapshot
	}
	if err != nil {
		return snapshot
	}
	entries, err := snapshot.directory.ReadDir(-1)
	if err != nil {
		return snapshot
	}
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".xctestrun") {
			continue
		}
		// Bound retained descriptors even for an unexpectedly large products dir.
		if len(snapshot.previous) >= 128 {
			return snapshot
		}
		file, err := root.OpenFile(filepath.Join(filepath.Base(snapshot.path), entry.Name()))
		if err != nil {
			return snapshot
		}
		snapshot.files = append(snapshot.files, file)
		info, err := file.Stat()
		if err != nil {
			return snapshot
		}
		snapshot.previous = append(snapshot.previous, info)
	}
	snapshot.ready = true
	return snapshot
}

func (s *testProductsSnapshot) close() {
	for _, file := range s.files {
		_ = file.Close()
	}
	if s.directory != nil {
		_ = s.directory.Close()
	}
	if s.root != nil {
		_ = s.root.Close()
	}
}

func (s *testProductsSnapshot) xctestrunPath() string {
	if !s.ready {
		return ""
	}
	base := filepath.Base(s.path)
	directory, err := s.root.OpenDir(base)
	if err != nil {
		return ""
	}
	defer directory.Close()
	if s.directory != nil && !sameTestProductFile(s.directory, directory) {
		return ""
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return ""
	}
	var candidate *os.File
	var name string
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".xctestrun") {
			continue
		}
		file, err := s.root.OpenFile(filepath.Join(base, entry.Name()))
		if err != nil {
			return ""
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return ""
		}
		if slices.ContainsFunc(s.previous, func(previous os.FileInfo) bool { return os.SameFile(previous, info) }) {
			_ = file.Close()
			continue
		}
		defer file.Close()
		if candidate != nil {
			return ""
		}
		candidate, name = file, entry.Name()
	}
	if candidate == nil {
		return ""
	}
	currentDirectory, err := s.root.OpenDir(base)
	if err != nil {
		return ""
	}
	defer currentDirectory.Close()
	if !sameTestProductFile(directory, currentDirectory) {
		return ""
	}
	path := filepath.Join(s.path, name)
	if contained, err := s.root.ContainsPath(path); err != nil || !contained {
		return ""
	}
	current, err := s.root.OpenFile(filepath.Join(base, name))
	if err != nil {
		return ""
	}
	defer current.Close()
	if !sameTestProductFile(candidate, current) {
		return ""
	}
	return path
}

func sameTestProductFile(a, b *os.File) bool {
	aInfo, aErr := a.Stat()
	bInfo, bErr := b.Stat()
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
}
