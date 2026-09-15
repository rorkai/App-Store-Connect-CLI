package cmdtest

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// A guess nothing can place gets the curated task map, while a typo and a
// guessable verb keep the suggestions they already had. All of them keep the
// error first and the help pointer last on stderr. Tokens the suggester now
// answers (`builds latest`, `testflight groups invite`) are covered in
// unknown_child_suggestions_test.go.
func TestRunUnknownSubcommandTaskHints(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	tests := []struct {
		name         string
		args         []string
		wantOrder    []string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:      "unplaceable guess on a curated group",
			args:      []string{"builds", "qqqqq"},
			wantOrder: []string{"Error: unknown command `asc builds qqqqq`", "Common tasks:", "For help:"},
			wantContains: []string{
				"  list builds          asc builds list --app APP_ID\n",
				// The canonical latest-build lookup, not a sorted single-result list.
				"  latest build         asc builds info --app APP_ID --latest\n",
				// The first-class command, not arithmetic over a build listing.
				"  next build number    asc builds next-build-number --app APP_ID\n",
				"  wait for processing  asc builds wait --app APP_ID --latest\n",
				"  asc builds --help\n",
			},
			// Angle brackets would be shell redirections in a pasted hint.
			wantAbsent: []string{"Try:", "--sort -uploadedDate", "<", ">"},
		},
		{
			name:      "unplaceable guess on a curated nested group",
			args:      []string{"testflight", "groups", "qqqqq"},
			wantOrder: []string{"Error: unknown command `asc testflight groups qqqqq`", "Common tasks:", "For help:"},
			wantContains: []string{
				"  add testers     asc testflight groups add-testers --group GROUP_ID --email EMAIL\n",
			},
			wantAbsent: []string{"Try:", "<", ">"},
		},
		{
			name:         "typo keeps the nearest-match suggestion",
			args:         []string{"builds", "lsit"},
			wantOrder:    []string{"Error: unknown command `asc builds lsit`", "Try:", "For help:"},
			wantContains: []string{"  asc builds list\n"},
			wantAbsent:   []string{"Common tasks:"},
		},
		{
			name:       "group without curated hints is unchanged",
			args:       []string{"xcode-cloud", "workflows", "qqqqq"},
			wantOrder:  []string{"Error: unknown command `asc xcode-cloud workflows qqqqq`", "For help:"},
			wantAbsent: []string{"Common tasks:", "Try:"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			previous := -1
			for _, marker := range test.wantOrder {
				index := strings.Index(stderr, marker)
				if index < 0 {
					t.Fatalf("stderr missing %q: %q", marker, stderr)
				}
				if index <= previous {
					t.Fatalf("stderr has %q out of order: %q", marker, stderr)
				}
				previous = index
			}
			for _, fragment := range test.wantContains {
				if !strings.Contains(stderr, fragment) {
					t.Fatalf("stderr missing %q: %q", fragment, stderr)
				}
			}
			for _, fragment := range test.wantAbsent {
				if strings.Contains(stderr, fragment) {
					t.Fatalf("stderr unexpectedly contains %q: %q", fragment, stderr)
				}
			}
		})
	}
}
