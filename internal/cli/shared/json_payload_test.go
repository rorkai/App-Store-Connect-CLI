package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadJSONFilePayload(t *testing.T) {
	t.Run("valid payload", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "payload.json")
		if err := os.WriteFile(path, []byte(`{"name":"demo"}`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		payload, err := ReadJSONFilePayload(path)
		if err != nil {
			t.Fatalf("ReadJSONFilePayload unexpected error: %v", err)
		}
		if string(payload) != `{"name":"demo"}` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("symlink payload", func(t *testing.T) {
		dir := t.TempDir()
		targetPath := filepath.Join(dir, "target.json")
		linkPath := filepath.Join(dir, "payload-link.json")
		if err := os.WriteFile(targetPath, []byte(`{"name":"linked"}`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Skipf("symlink not available in this environment: %v", err)
		}

		payload, err := ReadJSONFilePayload(linkPath)
		if err != nil {
			t.Fatalf("ReadJSONFilePayload unexpected error for symlink: %v", err)
		}
		if string(payload) != `{"name":"linked"}` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("directory path", func(t *testing.T) {
		dir := t.TempDir()
		_, err := ReadJSONFilePayload(dir)
		if err == nil {
			t.Fatal("expected error for directory payload path")
		}
		if !strings.Contains(err.Error(), "payload path must be a file") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.json")
		if err := os.WriteFile(path, []byte(" \n\t"), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		_, err := ReadJSONFilePayload(path)
		if err == nil {
			t.Fatal("expected error for empty payload")
		}
		if !strings.Contains(err.Error(), "payload file is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "invalid.json")
		if err := os.WriteFile(path, []byte(`{"name"`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		_, err := ReadJSONFilePayload(path)
		if err == nil {
			t.Fatal("expected error for invalid payload")
		}
		if !strings.Contains(err.Error(), "invalid JSON:") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("object helper rejects array", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "array.json")
		if err := os.WriteFile(path, []byte(`[1,2,3]`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		_, err := ReadJSONFilePayload(path)
		if err == nil {
			t.Fatal("expected object payload error")
		}
		if !strings.Contains(err.Error(), "payload must be a JSON object") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestReadJSONFilePayloadKind(t *testing.T) {
	t.Run("array payload", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "array.json")
		if err := os.WriteFile(path, []byte(`[{"name":"demo"}]`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		payload, err := ReadJSONFilePayloadKind(path, JSONPayloadArray)
		if err != nil {
			t.Fatalf("ReadJSONFilePayloadKind unexpected error: %v", err)
		}
		if string(payload) != `[{"name":"demo"}]` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("array mode rejects object", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "object.json")
		if err := os.WriteFile(path, []byte(`{"name":"demo"}`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		_, err := ReadJSONFilePayloadKind(path, JSONPayloadArray)
		if err == nil {
			t.Fatal("expected array payload error")
		}
		if !strings.Contains(err.Error(), "payload must be a JSON array") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("any payload accepts scalar", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "scalar.json")
		if err := os.WriteFile(path, []byte(`true`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		payload, err := ReadJSONFilePayloadKind(path, JSONPayloadAny)
		if err != nil {
			t.Fatalf("ReadJSONFilePayloadKind unexpected error: %v", err)
		}
		if string(payload) != `true` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("unsupported kind", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "payload.json")
		if err := os.WriteFile(path, []byte(`{"name":"demo"}`), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}

		_, err := ReadJSONFilePayloadKind(path, JSONPayloadKind("document"))
		if err == nil {
			t.Fatal("expected unsupported kind error")
		}
		if !strings.Contains(err.Error(), "unsupported JSON payload kind: document") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func setStdinPayload(t *testing.T, contents string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	previous := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		reader.Close()
	})
	go func() {
		defer writer.Close()
		_, _ = writer.WriteString(contents)
	}()
}

func TestReadJSONFilePayloadKindReadsStdin(t *testing.T) {
	t.Run("object payload", func(t *testing.T) {
		setStdinPayload(t, `{"name":"demo"}`)

		payload, err := ReadJSONFilePayloadKind("-", JSONPayloadObject)
		if err != nil {
			t.Fatalf("ReadJSONFilePayloadKind unexpected error: %v", err)
		}
		if string(payload) != `{"name":"demo"}` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("array payload", func(t *testing.T) {
		setStdinPayload(t, `[{"id":"1"}]`)

		payload, err := ReadJSONFilePayloadKind("-", JSONPayloadArray)
		if err != nil {
			t.Fatalf("ReadJSONFilePayloadKind unexpected error: %v", err)
		}
		if string(payload) != `[{"id":"1"}]` {
			t.Fatalf("unexpected payload: %q", string(payload))
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		setStdinPayload(t, `{"name"`)

		_, err := ReadJSONFilePayloadKind("-", JSONPayloadObject)
		if err == nil {
			t.Fatal("expected error for invalid stdin payload")
		}
		if !strings.Contains(err.Error(), "invalid JSON:") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty stdin", func(t *testing.T) {
		setStdinPayload(t, " \n")

		_, err := ReadJSONFilePayloadKind("-", JSONPayloadObject)
		if err == nil {
			t.Fatal("expected error for empty stdin payload")
		}
		if !strings.Contains(err.Error(), "stdin payload is empty") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("kind mismatch", func(t *testing.T) {
		setStdinPayload(t, `[1,2,3]`)

		_, err := ReadJSONFilePayloadKind("-", JSONPayloadObject)
		if err == nil {
			t.Fatal("expected object payload error")
		}
		if !strings.Contains(err.Error(), "payload must be a JSON object") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("terminal stdin rejected", func(t *testing.T) {
		previous := isTerminal
		isTerminal = func(int) bool { return true }
		t.Cleanup(func() {
			isTerminal = previous
		})

		_, err := ReadJSONFilePayloadKind("-", JSONPayloadObject)
		if err == nil {
			t.Fatal("expected error for terminal stdin")
		}
		if !strings.Contains(err.Error(), "stdin is a terminal") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
