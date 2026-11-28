package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDeveloperCapabilitySettings(t *testing.T) {
	const metadataJSON = `{"settings":[{"key":"PROTECTION","minInstances":1,"allowedInstances":"SINGLE","options":[{"key":"COMPLETE","enabledByDefault":true},{"key":"UNTIL_AUTH"}]},{"key":"FEATURES","minInstances":0,"allowedInstances":"MULTIPLE","options":[{"key":"A"},{"key":"B"}]}]}`
	var metadata developerCapabilityMetadataAttributes
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		settings  string
		wantError string
	}{
		{name: "preserve existing", settings: `null`},
		{name: "configured", settings: `[{"key":"PROTECTION","options":[{"key":"COMPLETE","enabled":true}]},{"key":"FEATURES","options":[{"key":"A","enabled":true},{"key":"B","enabled":true}]}]`},
		{name: "required setting missing", settings: `[]`, wantError: `setting "PROTECTION" requires at least 1 enabled option`},
		{name: "required option disabled", settings: `[{"key":"PROTECTION","options":[{"key":"COMPLETE","enabled":false}]}]`, wantError: `setting "PROTECTION" requires at least 1 enabled option`},
		{name: "single choice", settings: `[{"key":"PROTECTION","options":[{"key":"COMPLETE","enabled":true},{"key":"UNTIL_AUTH","enabled":true}]}]`, wantError: `setting "PROTECTION" allows at most one enabled option`},
		{name: "unknown setting", settings: `[{"key":"TYPO"}]`, wantError: `unknown capability setting "TYPO"`},
		{name: "unknown option", settings: `[{"key":"PROTECTION","options":[{"key":"TYPO","enabled":true}]}]`, wantError: `unknown option "TYPO" for capability setting "PROTECTION"`},
		{name: "duplicate setting", settings: `[{"key":"PROTECTION"},{"key":"PROTECTION"}]`, wantError: `duplicate capability setting "PROTECTION"`},
		{name: "duplicate option", settings: `[{"key":"PROTECTION","options":[{"key":"COMPLETE","enabled":true},{"key":"COMPLETE","enabled":true}]}]`, wantError: `duplicate option "COMPLETE" for capability setting "PROTECTION"`},
		{name: "explicit option state", settings: `[{"key":"PROTECTION","options":[{"key":"COMPLETE"}]}]`, wantError: `option "COMPLETE" for capability setting "PROTECTION" requires an enabled value`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var settings []BundleIDCapabilitySetting
			if err := json.Unmarshal([]byte(test.settings), &settings); err != nil {
				t.Fatal(err)
			}
			err := validateDeveloperCapabilitySettings(metadata, settings)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("valid settings rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}
