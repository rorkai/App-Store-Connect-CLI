package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlushSpoolRemovesSuccessfullyDeliveredEvents(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), spoolFileName))
	for _, id := range []string{"event-01", "event-02"} {
		if err := store.append(testSpoolRecord(id)); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}

	var delivered []string
	err := flushSpool(store, func(event Event, endpoint string) error {
		delivered = append(delivered, event.EventID+"@"+endpoint)
		return nil
	})
	if err != nil {
		t.Fatalf("flushSpool() error = %v", err)
	}
	if len(delivered) != 2 {
		t.Fatalf("delivered events = %v, want 2", delivered)
	}
	assertSpoolEventIDs(t, store)
}

func TestFlushSpoolRetainsOnlyFailedEvents(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), spoolFileName))
	for _, id := range []string{"event-01", "event-02", "event-03"} {
		if err := store.append(testSpoolRecord(id)); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}

	err := flushSpool(store, func(event Event, _ string) error {
		if event.EventID == "event-02" {
			return errors.New("endpoint unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("flushSpool() error = %v", err)
	}
	assertSpoolEventIDs(t, store, "event-02")
}

func TestMaintenanceWorkerDeduplicatesActiveFlushes(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	store := testSpoolStore(filepath.Join(t.TempDir(), spoolFileName))
	if err := store.append(testSpoolRecord("event-01")); err != nil {
		t.Fatalf("append event: %v", err)
	}
	workerPath := filepath.Join(t.TempDir(), "telemetry-worker")

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	var deliveries atomic.Int32
	deliver := func(event Event, _ string) error {
		if event.EventID == "" {
			return errors.New("missing event ID")
		}
		if deliveries.Add(1) == 1 {
			close(entered)
		}
		<-release
		return nil
	}
	go func() {
		firstDone <- runMaintenanceWorker(workerPath, store, deliver)
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first worker did not begin delivery")
	}
	if err := runMaintenanceWorker(workerPath, store, deliver); err != nil {
		t.Fatalf("duplicate worker error = %v", err)
	}
	if got := deliveries.Load(); got != 1 {
		t.Fatalf("deliveries while first worker active = %d, want 1", got)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first worker error = %v", err)
	}
}

func TestMaintenanceWorkerWaitsForPreviousWorkerHandoff(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	store := testSpoolStore(filepath.Join(t.TempDir(), spoolFileName))
	if err := store.append(testSpoolRecord("event-01")); err != nil {
		t.Fatalf("append event: %v", err)
	}
	workerPath := filepath.Join(t.TempDir(), "telemetry-worker")
	unlock, err := lockTelemetryFile(workerPath, 0, "worker")
	if err != nil {
		t.Fatalf("lock active worker: %v", err)
	}

	delivered := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- runMaintenanceWorker(workerPath, store, func(Event, string) error {
			delivered <- struct{}{}
			return nil
		})
	}()
	select {
	case <-delivered:
		unlock()
		t.Fatal("handoff worker delivered before active worker released lock")
	case <-time.After(20 * time.Millisecond):
	}
	unlock()

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("handoff worker did not flush after active worker released lock")
	}
	if err := <-done; err != nil {
		t.Fatalf("handoff worker error = %v", err)
	}
}

func TestMaintenanceWorkerPurgesQueueWhenEnvironmentOptsOut(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	store, err := defaultSpoolStore()
	if err != nil {
		t.Fatalf("defaultSpoolStore() error = %v", err)
	}
	if err := store.append(testSpoolRecord("event-01")); err != nil {
		t.Fatalf("append event: %v", err)
	}
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")

	err = runMaintenanceWorker(filepath.Join(filepath.Dir(store.path), "telemetry-worker"), store, func(Event, string) error {
		t.Fatal("worker delivered an event after opt-out")
		return nil
	})
	if err != nil {
		t.Fatalf("runMaintenanceWorker() error = %v", err)
	}
	if _, err := os.Stat(store.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spool stat error = %v, want not exist after opt-out", err)
	}
}

func TestSetEnabledFalsePurgesQueuedEvents(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	store, err := defaultSpoolStore()
	if err != nil {
		t.Fatalf("defaultSpoolStore() error = %v", err)
	}
	if err := store.append(testSpoolRecord("event-01")); err != nil {
		t.Fatalf("append event: %v", err)
	}
	crashTempPath := store.path + ".crash.tmp"
	if err := os.WriteFile(crashTempPath, []byte("queued telemetry"), 0o600); err != nil {
		t.Fatalf("write abandoned temp file: %v", err)
	}

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if _, err := os.Stat(store.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spool stat error = %v, want not exist after disable", err)
	}
	if _, err := os.Stat(crashTempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("crash temp stat error = %v, want not exist after disable", err)
	}
}

func TestShouldStartMaintenanceWorker(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		spooledRecords int
		hasMarker      bool
		markerAge      time.Duration
		want           bool
	}{
		{name: "missing marker spawns", spooledRecords: 1, hasMarker: false, want: true},
		{name: "fresh marker skips", spooledRecords: 1, hasMarker: true, markerAge: time.Minute, want: false},
		{name: "cooldown boundary spawns", spooledRecords: 1, hasMarker: true, markerAge: workerSpawnCooldown, want: true},
		{name: "expired cooldown spawns", spooledRecords: 1, hasMarker: true, markerAge: workerSpawnCooldown + time.Hour, want: true},
		{name: "future marker spawns", spooledRecords: 1, hasMarker: true, markerAge: -time.Minute, want: true},
		{name: "backlog at threshold overrides cooldown", spooledRecords: workerSpawnSpoolThreshold, hasMarker: true, markerAge: time.Minute, want: true},
		{name: "backlog above threshold respects cooldown", spooledRecords: workerSpawnSpoolThreshold + 1, hasMarker: true, markerAge: time.Minute, want: false},
		{name: "backlog below threshold respects cooldown", spooledRecords: workerSpawnSpoolThreshold - 1, hasMarker: true, markerAge: time.Minute, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markerPath := filepath.Join(t.TempDir(), workerSpawnMarkerName)
			if tt.hasMarker {
				markMaintenanceWorkerSpawn(markerPath, now.Add(-tt.markerAge))
				if _, err := os.Stat(markerPath); err != nil {
					t.Fatalf("stat spawn marker: %v", err)
				}
			}
			if got := shouldStartMaintenanceWorker(markerPath, tt.spooledRecords, now); got != tt.want {
				t.Fatalf("shouldStartMaintenanceWorker(records=%d, age=%s) = %t, want %t",
					tt.spooledRecords, tt.markerAge, got, tt.want)
			}
		})
	}
}

func TestMarkMaintenanceWorkerSpawnRecordsInjectedTimestamp(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), workerSpawnMarkerName)
	first := time.Now().Add(-time.Hour).Truncate(time.Second)
	markMaintenanceWorkerSpawn(markerPath, first)
	info, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("stat spawn marker: %v", err)
	}
	if !info.ModTime().Equal(first) {
		t.Fatalf("marker mtime = %s, want %s", info.ModTime(), first)
	}

	second := first.Add(30 * time.Minute)
	markMaintenanceWorkerSpawn(markerPath, second)
	info, err = os.Stat(markerPath)
	if err != nil {
		t.Fatalf("stat refreshed spawn marker: %v", err)
	}
	if !info.ModTime().Equal(second) {
		t.Fatalf("refreshed marker mtime = %s, want %s", info.ModTime(), second)
	}
}

func TestInternalWorkerInvocationRequiresMarker(t *testing.T) {
	t.Setenv(internalWorkerEnvVar, "")
	if isMaintenanceWorkerInvocation([]string{internalWorkerArg}) {
		t.Fatal("worker argument was accepted without internal marker")
	}
	t.Setenv(internalWorkerEnvVar, "1")
	if !isMaintenanceWorkerInvocation([]string{internalWorkerArg}) {
		t.Fatal("worker argument and marker were not recognized")
	}
	if isMaintenanceWorkerInvocation([]string{internalWorkerArg, "extra"}) {
		t.Fatal("worker invocation accepted extra arguments")
	}
}
