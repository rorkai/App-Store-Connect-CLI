package xcode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

var (
	errArchiveSwapUnsupported = errors.New("atomic archive replacement is unsupported")
	swapArchiveDirectoriesFn  = swapArchiveDirectories
)

type archiveReplacement struct {
	parent        *os.Root
	dir           string
	name          string
	path          string
	retainStaging bool
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
	err = swapArchiveDirectoriesFn(stageDir, parentDir, r.name)
	if errors.Is(err, errArchiveSwapUnsupported) {
		return r.publishWithBackup()
	}
	return err
}

// publishWithBackup handles volumes without atomic directory exchange. Keep
// the prior archive in staging until publication succeeds or it is restored.
func (r *archiveReplacement) publishWithBackup() error {
	backup := filepath.Join(r.dir, "previous")
	if err := r.parent.Rename(r.name, backup); err != nil {
		return fmt.Errorf("back up existing archive: %w", err)
	}
	if err := r.renameToAvailableDestination(filepath.Join(r.dir, r.name)); err != nil {
		if restoreErr := r.renameToAvailableDestination(backup); restoreErr != nil {
			r.retainStaging = true
			return errors.Join(err, fmt.Errorf("could not restore previous archive retained at %q: %w", filepath.Join(filepath.Dir(r.path), "previous"), restoreErr))
		}
		return err
	}
	return nil
}

func (r *archiveReplacement) renameToAvailableDestination(source string) error {
	if _, err := r.parent.Lstat(r.name); err == nil {
		return fmt.Errorf("archive destination was created during replacement: %w", os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	info, err := r.parent.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		// A prior non-directory can be restored without replacing a file
		// created by a concurrent writer after the destination check.
		return r.parent.Link(source, r.name)
	}
	// A validated archive is a nonempty directory. A rename cannot
	// overwrite another nonempty directory or a file.
	return r.parent.Rename(source, r.name)
}

func (r *archiveReplacement) cleanup(log io.Writer) {
	defer r.parent.Close()
	if r.retainStaging {
		return
	}
	if err := r.parent.RemoveAll(r.dir); err != nil && log != nil {
		fmt.Fprintf(log, "Warning: could not clean up archive staging directory: %v\n", err)
	}
}
