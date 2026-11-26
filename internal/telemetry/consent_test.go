package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func enableTelemetryForTest(t *testing.T) {
	t.Helper()
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(endpointEnvVar, "https://telemetry.example.test/events")
	stubMaintenanceWorkerStart(t, func() error { return nil })
}

func captureNotice(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	original := noticeOutput
	noticeOutput = &buffer
	t.Cleanup(func() { noticeOutput = original })
	return &buffer
}

func TestBuildEventOmitsInstallIDAndStoresNothingWithoutOptIn(t *testing.T) {
	enableTelemetryForTest(t)

	ev, ok := BuildEventWithContext("asc builds list", "1.2.3", time.Second, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if !ok {
		t.Fatal("expected event")
	}
	if ev.InstallID != nil {
		t.Fatalf("InstallID = %q, want nil without opt-in", *ev.InstallID)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state file stat error = %v, want not exist", err)
	}
}

func TestBuildEventAttachesInstallIDAfterOptIn(t *testing.T) {
	enableTelemetryForTest(t)

	id, err := SetInstallIDEnabled(true)
	if err != nil {
		t.Fatalf("SetInstallIDEnabled(true) error = %v", err)
	}
	if id == "" {
		t.Fatal("expected install ID after opt-in")
	}

	ev, ok := BuildEventWithContext("asc builds list", "1.2.3", time.Second, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if !ok {
		t.Fatal("expected event")
	}
	if ev.InstallID == nil || *ev.InstallID != id {
		t.Fatalf("InstallID = %v, want %q", ev.InstallID, id)
	}
}

func TestInstallIDOptOutAndDisableDeleteStoredID(t *testing.T) {
	for _, tt := range []struct {
		name   string
		revoke func() error
	}{
		{name: "install ID opt-out", revoke: func() error { _, err := SetInstallIDEnabled(false); return err }},
		{name: "telemetry disable", revoke: func() error { return SetEnabled(false) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			enableTelemetryForTest(t)
			if _, err := SetInstallIDEnabled(true); err != nil {
				t.Fatalf("SetInstallIDEnabled(true) error = %v", err)
			}
			if err := tt.revoke(); err != nil {
				t.Fatalf("revoke error = %v", err)
			}
			st, err := loadCurrentState()
			if err != nil {
				t.Fatalf("loadCurrentState() error = %v", err)
			}
			if st.InstallID != "" || st.InstallIDOptIn {
				t.Fatalf("state = %+v, want no install ID and no opt-in", st)
			}
		})
	}
}

func TestResetInstallIDWithoutOptInStoresNoID(t *testing.T) {
	enableTelemetryForTest(t)

	id, err := ResetInstallID()
	if err != nil {
		t.Fatalf("ResetInstallID() error = %v", err)
	}
	if id != "" {
		t.Fatalf("ResetInstallID() = %q, want empty without opt-in", id)
	}
	st, err := loadCurrentState()
	if err != nil {
		t.Fatalf("loadCurrentState() error = %v", err)
	}
	if st.InstallID != "" {
		t.Fatalf("stored install ID = %q, want none", st.InstallID)
	}
}

func TestEmitPrintsNoticeOnceAndStillQueuesEvents(t *testing.T) {
	enableTelemetryForTest(t)
	notice := captureNotice(t)

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	first := notice.String()
	if !strings.Contains(first, "asc telemetry disable") || !strings.Contains(first, "DO_NOT_TRACK=1") || !strings.Contains(first, "--no-telemetry") {
		t.Fatalf("notice = %q, want every off switch named", first)
	}

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	if notice.String() != first {
		t.Fatalf("notice printed again: %q", notice.String())
	}
	if records := readDefaultSpool(t); len(records) != 2 {
		t.Fatalf("spool records = %d, want 2", len(records))
	}
}

func TestEmitNoticeRemovesInstallIDStoredWithoutOptIn(t *testing.T) {
	enableTelemetryForTest(t)
	captureNotice(t)
	if _, err := ensureInstallID(lockTimeout); err != nil {
		t.Fatalf("ensureInstallID() error = %v", err)
	}

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	st, err := loadCurrentState()
	if err != nil {
		t.Fatalf("loadCurrentState() error = %v", err)
	}
	if st.InstallID != "" || !st.NoticeShown {
		t.Fatalf("state = %+v, want notice recorded and legacy install ID removed", st)
	}
	records := readDefaultSpool(t)
	if len(records) != 1 || records[0].Event.InstallID != nil {
		t.Fatalf("spool = %+v, want one event without install ID", records)
	}
}

func TestEmitSendsNothingWhenNoticeCannotBeRecorded(t *testing.T) {
	enableTelemetryForTest(t)
	captureNotice(t)
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	unlock, err := lockState(path, lockTimeout)
	if err != nil {
		t.Fatalf("lockState() error = %v", err)
	}
	defer unlock()

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	if records := readDefaultSpool(t); len(records) != 0 {
		t.Fatalf("spool records = %d, want 0 before the notice is recorded", len(records))
	}
}

func TestEmitHonorsProcessOptOut(t *testing.T) {
	enableTelemetryForTest(t)
	notice := captureNotice(t)
	SetProcessOptOut(true)
	t.Cleanup(func() { SetProcessOptOut(false) })

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	if notice.Len() != 0 {
		t.Fatalf("notice = %q, want none", notice.String())
	}
	if records := readDefaultSpool(t); len(records) != 0 {
		t.Fatalf("spool records = %d, want 0", len(records))
	}
}

func TestReadStatusReportsInstallIDOnlyAfterOptIn(t *testing.T) {
	enableTelemetryForTest(t)
	if _, err := ensureInstallID(lockTimeout); err != nil {
		t.Fatalf("ensureInstallID() error = %v", err)
	}

	status, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if status.InstallIDEnabled || status.InstallID != "" {
		t.Fatalf("status = %+v, want no install ID without opt-in", status)
	}

	id, err := SetInstallIDEnabled(true)
	if err != nil {
		t.Fatalf("SetInstallIDEnabled(true) error = %v", err)
	}
	status, err = ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if !status.InstallIDEnabled || status.InstallID != id {
		t.Fatalf("status = %+v, want install ID %q", status, id)
	}
}

func spooledWire(t *testing.T, index int) map[string]any {
	t.Helper()
	records := readDefaultSpool(t)
	if len(records) <= index {
		t.Fatalf("spool records = %d, want more than %d", len(records), index)
	}
	encoded, err := json.Marshal(records[index].Event)
	if err != nil {
		t.Fatalf("marshal spooled event: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("decode spooled event: %v", err)
	}
	return wire
}

func TestEmitMarksOnlyTheFirstEventOfANewLocalInstall(t *testing.T) {
	enableTelemetryForTest(t)
	captureNotice(t)

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})
	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	first := spooledWire(t, 0)
	if first["schema_version"] != float64(8) || first["first_run"] != true {
		t.Fatalf("first event = %v, want schema 8 with first_run true", first)
	}
	for _, key := range []string{"upload_phase", "upload_size_bucket"} {
		if value, ok := first[key]; !ok || value != nil {
			t.Fatalf("first event %s = %v (present %v), want null", key, value, ok)
		}
	}
	if second := spooledWire(t, 1); second["first_run"] != false {
		t.Fatalf("second event first_run = %v, want false", second["first_run"])
	}
}

func TestEmitDoesNotMarkUpgradedInstallAsFirstRun(t *testing.T) {
	enableTelemetryForTest(t)
	notice := captureNotice(t)
	if _, err := ensureInstallID(lockTimeout); err != nil {
		t.Fatalf("ensureInstallID() error = %v", err)
	}

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	if notice.Len() == 0 {
		t.Fatal("expected the notice for an upgraded install")
	}
	if wire := spooledWire(t, 0); wire["first_run"] != false {
		t.Fatalf("upgraded install first_run = %v, want false", wire["first_run"])
	}
}

func TestEmitNeverMarksCIAsFirstRun(t *testing.T) {
	enableTelemetryForTest(t)
	notice := captureNotice(t)
	t.Setenv("CI", "true")

	EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

	if notice.Len() == 0 {
		t.Fatal("expected the notice in CI")
	}
	if wire := spooledWire(t, 0); wire["first_run"] != false {
		t.Fatalf("CI first_run = %v, want false", wire["first_run"])
	}
}

func TestEmitSkipsNoticeInRorkRuntimes(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"sandbox":  {"RORK_SANDBOX_ID": "sandbox-1"},
		"workflow": {"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "rorkai/user-workflows"},
	} {
		t.Run(name, func(t *testing.T) {
			enableTelemetryForTest(t)
			notice := captureNotice(t)
			for key, value := range env {
				t.Setenv(key, value)
			}

			EmitWithContext("asc builds list", "1.2.3", time.Millisecond, 0, EventContext{InvocationShape: InvocationShapeLeaf})

			if notice.Len() != 0 {
				t.Fatalf("notice = %q, want none in a Rork runtime", notice.String())
			}
			if wire := spooledWire(t, 0); wire["first_run"] != false {
				t.Fatalf("first_run = %v, want false", wire["first_run"])
			}
			path, err := StatePath()
			if err != nil {
				t.Fatalf("StatePath() error = %v", err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("state file stat error = %v, want not exist", err)
			}
		})
	}
}

func TestSchemaV6SpoolRecordOmitsSchemaV8Fields(t *testing.T) {
	event := Event{EventID: "event-v6", SchemaVersion: schemaVersionV6, CommandPath: "asc builds list"}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"first_run", "upload_phase", "upload_size_bucket"} {
		if _, ok := wire[key]; ok {
			t.Fatalf("schema v6 wire has %s: %s", key, encoded)
		}
	}
}
