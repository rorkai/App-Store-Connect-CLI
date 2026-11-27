package cmdtest

import (
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppEventsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing app",
			args:    []string{"app-events", "list"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "create missing app",
			args:    []string{"app-events", "create", "--name", "Launch", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "create missing name",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --name is required",
		},
		{
			name:    "create missing end time",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z"},
			wantErr: "Error: --end is required",
		},
		{
			name:    "create schedule missing territories",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --territories is required with a schedule: pass territory codes such as USA,GBR, or all for every territory the app is available in\n",
		},
		{
			name:    "create territories all mixed with codes",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z", "--territories", "all,USA"},
			wantErr: "Error: --territories all cannot be combined with territory codes\n",
		},
		{
			name:    "update schedule missing territories",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --territories is required with a schedule: pass territory codes such as USA,GBR\n",
		},
		{
			name:    "update territories all",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z", "--territories", "all"},
			wantErr: "Error: --territories all is supported only by app-events create; pass territory codes such as USA,GBR\n",
		},
		{
			name:    "create event longer than 31 days",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--start", "2026-11-29T17:00:00Z", "--end", "2027-01-03T17:00:00Z", "--territories", "USA"},
			wantErr: "Error: --end must be at most 31 days after --start (latest: 2026-12-30T17:00:00Z)\n",
		},
		{
			name:    "create event shorter than 15 minutes",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--start", "2026-11-29T17:00:00Z", "--end", "2026-11-29T17:10:00Z", "--territories", "USA"},
			wantErr: "Error: --end must be at least 15 minutes after --start\n",
		},
		{
			name:    "create publish more than 14 days before start",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--publish-start", "2026-11-01T00:00:00Z", "--start", "2026-11-29T17:00:00Z", "--end", "2026-12-30T17:00:00Z", "--territories", "USA"},
			wantErr: "Error: --publish-start must be at most 14 days before --start (earliest: 2026-11-15T17:00:00Z)\n",
		},
		{
			name:    "update event longer than 31 days",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID", "--start", "2026-11-29T17:00:00Z", "--end", "2027-01-03T17:00:00Z", "--territories", "USA"},
			wantErr: "Error: --end must be at most 31 days after --start (latest: 2026-12-30T17:00:00Z)\n",
		},
		{
			name:    "update missing event-id",
			args:    []string{"app-events", "update", "--name", "Launch"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "update missing update fields",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"app-events", "delete", "--event-id", "EVENT_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "localizations list missing event-id",
			args:    []string{"app-events", "localizations", "list"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "localizations create missing locale",
			args:    []string{"app-events", "localizations", "create", "--event-id", "EVENT_ID"},
			wantErr: "Error: --locale is required",
		},
		{
			name:    "localizations update missing localization-id",
			args:    []string{"app-events", "localizations", "update", "--name", "New Name"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations update missing fields",
			args:    []string{"app-events", "localizations", "update", "--localization-id", "LOC_ID"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "localizations delete missing confirm",
			args:    []string{"app-events", "localizations", "delete", "--localization-id", "LOC_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "screenshots list missing localization",
			args:    []string{"app-events", "screenshots", "list"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "screenshots create missing path",
			args:    []string{"app-events", "screenshots", "create", "--event-id", "EVENT_ID", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --path is required",
		},
		{
			name:    "screenshots create missing localization",
			args:    []string{"app-events", "screenshots", "create", "--path", "./event.png", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "screenshots delete missing confirm",
			args:    []string{"app-events", "screenshots", "delete", "--screenshot-id", "SHOT_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "video-clips list missing localization",
			args:    []string{"app-events", "video-clips", "list"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "video-clips create missing path",
			args:    []string{"app-events", "video-clips", "create", "--event-id", "EVENT_ID", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --path is required",
		},
		{
			name:    "video-clips create missing localization",
			args:    []string{"app-events", "video-clips", "create", "--path", "./clip.mov", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "video-clips delete missing confirm",
			args:    []string{"app-events", "video-clips", "delete", "--clip-id", "CLIP_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "submit missing confirm",
			args:    []string{"app-events", "submit", "--event-id", "EVENT_ID", "--app", "APP_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "submit missing event-id",
			args:    []string{"app-events", "submit", "--app", "APP_ID", "--confirm"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "submit missing app",
			args:    []string{"app-events", "submit", "--event-id", "EVENT_ID", "--confirm"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "screenshots relationships missing event or localization id",
			args:    []string{"app-events", "screenshots", "links"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "video-clips relationships missing event or localization id",
			args:    []string{"app-events", "video-clips", "links"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "localizations screenshots list missing localization id",
			args:    []string{"app-events", "localizations", "screenshots", "list"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations video-clips list missing localization id",
			args:    []string{"app-events", "localizations", "video-clips", "list"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations screenshots relationships missing localization id",
			args:    []string{"app-events", "localizations", "screenshots-links"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations video-clips relationships missing localization id",
			args:    []string{"app-events", "localizations", "video-clips-links"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "relationships missing event id",
			args:    []string{"app-events", "links"},
			wantErr: "Error: --event-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, nil, test.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
