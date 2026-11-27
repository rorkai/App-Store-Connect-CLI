package shared

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgressFileWriteFailureWarnsOnce(t *testing.T) {
	var warnings bytes.Buffer
	d := &Deps{IO: &IO{ErrOut: &warnings}}
	progress, err := d.OpenProgressFile(filepath.Join(t.TempDir(), "progress.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := progress.file.Close(); err != nil {
		t.Fatal(err)
	}
	progress.Emit(map[string]string{"event": "started"})
	progress.Emit(map[string]string{"event": "finished"})
	progress.Close()
	if strings.Count(warnings.String(), "--progress-file output is incomplete:") != 1 || !strings.Contains(warnings.String(), "file already closed") {
		t.Fatalf("warning = %q, want one closed-file warning", warnings.String())
	}
}
