package metadata

import (
	"strings"
	"testing"
)

func TestMetadataPullCommand_AppInfoFlagDefined(t *testing.T) {
	cmd := MetadataPullCommand()
	if cmd.FlagSet.Lookup("app-info") == nil {
		t.Fatal("expected --app-info flag to be defined on metadata pull")
	}
}

func TestMetadataPullCommand_AppInfoFlagDefault(t *testing.T) {
	cmd := MetadataPullCommand()
	f := cmd.FlagSet.Lookup("app-info")
	if f == nil {
		t.Fatal("expected --app-info flag to be defined")
	}
	if f.DefValue != "" {
		t.Fatalf("expected --app-info default to be empty, got %q", f.DefValue)
	}
}

func TestBuildMetadataAppInfoExample(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		appID    string
		version  string
		platform string
		dir      string
		want     string
	}{
		{
			name:    "pull basic",
			command: "pull",
			appID:   "123",
			version: "1.0",
			dir:     "./metadata",
			want:    `asc metadata pull --app "123" --version "1.0" --dir "./metadata" --app-info "info-1"`,
		},
		{
			name:    "push basic",
			command: "push",
			appID:   "123",
			version: "1.0",
			dir:     "./metadata",
			want:    `asc metadata push --app "123" --version "1.0" --dir "./metadata" --app-info "info-1"`,
		},
		{
			name:     "with platform",
			command:  "pull",
			appID:    "123",
			version:  "1.0",
			platform: "IOS",
			dir:      "./metadata",
			want:     `asc metadata pull --app "123" --version "1.0" --platform IOS --dir "./metadata" --app-info "info-1"`,
		},
		{
			name:    "empty dir uses default",
			command: "pull",
			appID:   "123",
			version: "1.0",
			dir:     "",
			want:    `asc metadata pull --app "123" --version "1.0" --dir "./metadata" --app-info "info-1"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildMetadataAppInfoExample(tc.command, tc.appID, tc.version, tc.platform, tc.dir, "info-1")
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMetadataPullCommand_ShortUsageMentionsAppInfo(t *testing.T) {
	cmd := MetadataPullCommand()
	if !strings.Contains(cmd.ShortUsage, "--app-info") {
		t.Fatalf("expected ShortUsage to mention --app-info, got %q", cmd.ShortUsage)
	}
}
