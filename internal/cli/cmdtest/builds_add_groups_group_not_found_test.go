package cmdtest

import (
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// A --group that the build's app does not have is reported with that app's ID
// and groups, so a typo, a group of another app, and a group that was never
// created read differently. Nothing is posted.
func TestBuildsAddGroupsGroupNotFoundNamesTheAppAndItsGroups(t *testing.T) {
	tests := []struct {
		name   string
		group  string
		groups string
		want   string
	}{
		{
			name:   "group of another app",
			group:  "db83ece7-dcec-4f6d-b9a1-7dce7c15d13b",
			groups: `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Team (Expo)","isInternalGroup":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"Public Beta","isInternalGroup":false}}],"links":{}}`,
			want:   `builds add-groups: beta group "db83ece7-dcec-4f6d-b9a1-7dce7c15d13b" not found in app app-1. Its groups: "Team (Expo)" (group-internal, internal), "Public Beta" (group-external, external). Pass one of those names or IDs, or create the group with: asc testflight groups create --app "app-1" --name "NAME"`,
		},
		{
			name:   "app without groups",
			group:  "Public Beta",
			groups: `{"data":[],"links":{}}`,
			want:   `builds add-groups: beta group "Public Beta" not found: app app-1 has no beta groups. Create one with: asc testflight groups create --app "app-1" --name "NAME"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			registerAddGroupsLookups(reg, test.groups)

			_, _, runErr := runCommand(t, []string{"builds", "add-groups", "--build-id", "build-1", "--group", test.group})
			if runErr == nil || runErr.Error() != test.want {
				t.Fatalf("error =\n%v\nwant\n%s", runErr, test.want)
			}
		})
	}
}
