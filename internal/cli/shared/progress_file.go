package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ProgressFile streams events to an exclusively created regular file. Later
// write failures warn once without changing the outcome of the observed work.
type ProgressFile struct {
	mu       sync.Mutex
	file     *os.File
	warnings io.Writer
	failed   bool
}

// OpenProgressFile opens a new file beneath its selected parent directory.
// An empty path disables progress output.
func (d *Deps) OpenProgressFile(path string) (*ProgressFile, error) {
	if path == "" {
		return nil, nil
	}
	absolute, err := d.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	opened, err := root.OpenRoot()
	if err != nil {
		return nil, err
	}
	defer opened.Close()
	file, err := opened.OpenFile(filepath.Base(absolute), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &ProgressFile{file: file, warnings: d.IO.ErrOut}, nil
}

// Emit serializes each event as one JSON line, including concurrent observers.
func (p *ProgressFile) Emit(event any) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.failed {
		p.warn(json.NewEncoder(p.file).Encode(event))
	}
}

// Close releases the stream and reports a final write failure if necessary.
func (p *ProgressFile) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.warn(p.file.Close())
}

func (p *ProgressFile) warn(err error) {
	if err != nil && !p.failed {
		p.failed = true
		fmt.Fprintf(p.warnings, "Warning: --progress-file output is incomplete: %s\n", SanitizeTerminal(err.Error()))
	}
}
