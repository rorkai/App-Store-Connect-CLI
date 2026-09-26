package iap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestSnapshotImageFileUnlinksSnapshotPath(t *testing.T) {
	probe, err := os.CreateTemp(t.TempDir(), "unlink-probe-*")
	if err != nil {
		t.Fatalf("create unlink probe: %v", err)
	}
	probePath := probe.Name()
	canUnlinkOpenFile := os.Remove(probePath) == nil
	if err := probe.Close(); err != nil {
		t.Fatalf("close unlink probe: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(probePath) })
	if !canUnlinkOpenFile {
		if _, err := os.Stat(probePath); err != nil {
			t.Fatalf("unlink probe path stat error = %v, want path to remain until close", err)
		}
	}

	source, err := os.CreateTemp(t.TempDir(), "source-*.png")
	if err != nil {
		t.Fatalf("create source image: %v", err)
	}
	defer source.Close()

	want := []byte("image bytes")
	if _, err := source.Write(want); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	snapshot, cleanup, err := snapshotImageFile(source, int64(len(want)))
	if err != nil {
		t.Fatalf("snapshot image: %v", err)
	}
	defer cleanup()

	snapshotPath := snapshot.Name()
	if _, err := os.Stat(snapshotPath); canUnlinkOpenFile {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot path stat error = %v, want not-exist when unlinking open files is supported", err)
		}
	} else if err != nil {
		t.Fatalf("snapshot path stat error = %v, want path to remain until cleanup", err)
	}
	got, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot bytes = %q, want %q", got, want)
	}

	cleanup()
	if _, err := os.Stat(snapshotPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot path stat after cleanup = %v, want not-exist", err)
	}
}

func TestRelationshipResourceID(t *testing.T) {
	relationships := json.RawMessage(`{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"iap-1"}}}`)

	id, err := relationshipResourceID(relationships, "inAppPurchase")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != "iap-1" {
		t.Fatalf("expected id iap-1, got %s", id)
	}
}

func TestRelationshipResourceIDMissingID(t *testing.T) {
	relationships := json.RawMessage(`{"inAppPurchase":{"data":{"type":"inAppPurchases"}}}`)

	_, err := relationshipResourceID(relationships, "inAppPurchase")
	if err == nil {
		t.Fatalf("expected error for missing relationship id")
	}
}

func TestNormalizeIAPListEnumDeduplicatesAfterNormalization(t *testing.T) {
	got, err := normalizeIAPListEnum("ready_to_submit,READY_TO_SUBMIT", "--state", iapListStates)
	if err != nil {
		t.Fatalf("unexpected normalization error: %v", err)
	}
	if len(got) != 1 || got[0] != "READY_TO_SUBMIT" {
		t.Fatalf("normalized states = %v, want [READY_TO_SUBMIT]", got)
	}
}
