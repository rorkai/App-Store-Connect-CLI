package certificates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyCSRFilesystemRelation_MountAliases(t *testing.T) {
	sharedParent := t.TempDir()
	sharedInfo, err := os.Stat(sharedParent)
	if err != nil {
		t.Fatalf("Stat(sharedParent) error: %v", err)
	}
	aliasRoot := t.TempDir()
	leftParent := filepath.Join(aliasRoot, "first-mount")
	rightParent := filepath.Join(aliasRoot, "different", "depth", "second-mount")
	stat := func(path string) (os.FileInfo, error) {
		switch filepath.Clean(path) {
		case leftParent, rightParent:
			return sharedInfo, nil
		default:
			if strings.HasPrefix(path, leftParent+string(filepath.Separator)) ||
				strings.HasPrefix(path, rightParent+string(filepath.Separator)) {
				return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
			}
			return os.Stat(path)
		}
	}

	t.Run("same output", func(t *testing.T) {
		same, nested, err := classifyCSRFilesystemRelationWithStat(
			filepath.Join(leftParent, "pair"),
			filepath.Join(rightParent, "pair"),
			stat,
		)
		if err != nil {
			t.Fatalf("classifyCSRFilesystemRelationWithStat() error: %v", err)
		}
		if !same || nested {
			t.Fatalf("relation = same %t, nested %t; want same output", same, nested)
		}
	})

	t.Run("nested output", func(t *testing.T) {
		same, nested, err := classifyCSRFilesystemRelationWithStat(
			filepath.Join(leftParent, "pair"),
			filepath.Join(rightParent, "pair", "request.csr"),
			stat,
		)
		if err != nil {
			t.Fatalf("classifyCSRFilesystemRelationWithStat() error: %v", err)
		}
		if same || !nested {
			t.Fatalf("relation = same %t, nested %t; want nested output", same, nested)
		}
	})

	t.Run("reverse nested output", func(t *testing.T) {
		same, nested, err := classifyCSRFilesystemRelationWithStat(
			filepath.Join(leftParent, "pair", "request.key"),
			filepath.Join(rightParent, "pair"),
			stat,
		)
		if err != nil {
			t.Fatalf("classifyCSRFilesystemRelationWithStat() error: %v", err)
		}
		if same || !nested {
			t.Fatalf("relation = same %t, nested %t; want nested output", same, nested)
		}
	})

	t.Run("distinct leaves", func(t *testing.T) {
		same, nested, err := classifyCSRFilesystemRelationWithStat(
			filepath.Join(leftParent, "pair.key"),
			filepath.Join(rightParent, "pair.csr"),
			stat,
		)
		if err != nil {
			t.Fatalf("classifyCSRFilesystemRelationWithStat() error: %v", err)
		}
		if same || nested {
			t.Fatalf("relation = same %t, nested %t; want distinct outputs", same, nested)
		}
	})
}

func TestClassifyCSRFilesystemRelation_DistinctAncestors(t *testing.T) {
	leftParent := t.TempDir()
	rightParent := t.TempDir()

	same, nested, err := classifyCSRFilesystemRelation(
		filepath.Join(leftParent, "pair"),
		filepath.Join(rightParent, "pair"),
	)
	if err != nil {
		t.Fatalf("classifyCSRFilesystemRelation() error: %v", err)
	}
	if same || nested {
		t.Fatalf("relation = same %t, nested %t; want distinct outputs", same, nested)
	}
}
