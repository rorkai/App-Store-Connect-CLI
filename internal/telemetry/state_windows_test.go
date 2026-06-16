//go:build windows

package telemetry

import "testing"

func TestStateMutationSucceedsWhileStateReaderIsOpen(t *testing.T) {
	setTelemetryTestHome(t)

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() error = %v", err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	reader, err := openStateFileForRead(path)
	if err != nil {
		t.Fatalf("openStateFileForRead() error = %v", err)
	}
	defer reader.Close()

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) with reader open error = %v", err)
	}
	status, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if status.Enabled || status.Reason != "state" {
		t.Fatalf("expected persisted opt-out, got %+v", status)
	}
}
