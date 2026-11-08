package xcode

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type archiveReplacement struct {
	parent *os.Root
	dir    string
	name   string
	path   string
}

func stageArchiveReplacement(destination string) (*archiveReplacement, error) {
	base, err := rootfs.New(filepath.Dir(destination))
	if err != nil {
		return nil, fmt.Errorf("resolve archive output directory: %w", err)
	}
	parent, err := base.OpenRoot()
	if err != nil {
		return nil, fmt.Errorf("open archive output directory: %w", err)
	}
	dir := ".asc-archive-stage-" + rand.Text()
	if err := parent.Mkdir(dir, 0o700); err != nil {
		_ = parent.Close()
		return nil, fmt.Errorf("stage archive replacement: %w", err)
	}
	name := filepath.Base(destination)
	return &archiveReplacement{
		parent: parent,
		dir:    dir,
		name:   name,
		path:   filepath.Join(base.Path(), dir, name),
	}, nil
}

// publish swaps the validated archive into place. The old archive stays in
// the staging directory until cleanup, even when both directories are nonempty.
func (r *archiveReplacement) publish() error {
	parentDir, err := r.parent.Open(".")
	if err != nil {
		return err
	}
	defer parentDir.Close()
	stageDir, err := r.parent.Open(r.dir)
	if err != nil {
		return err
	}
	defer stageDir.Close()
	return swapArchiveDirectories(stageDir, parentDir, r.name)
}

func (r *archiveReplacement) cleanup(log io.Writer) {
	defer r.parent.Close()
	if err := r.parent.RemoveAll(r.dir); err != nil && log != nil {
		fmt.Fprintf(log, "Warning: could not clean up archive staging directory: %v\n", err)
	}
}
