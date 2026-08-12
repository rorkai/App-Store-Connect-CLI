package cmdtest

import (
	"strings"
	"testing"
)

func TestResourceIDHelpShowsDiscoveryCommands(t *testing.T) {
	root := RootCommand("1.2.3")
	tests := []struct {
		name string
		path []string
		want []string
	}{
		{
			name: "in-app purchase versions list",
			path: []string{"iap", "versions", "list"},
			want: []string{
				"To find the in-app purchase ID",
				`asc iap list --app "APP_ID" --paginate --output json`,
			},
		},
		{
			name: "attach build",
			path: []string{"versions", "attach-build"},
			want: []string{
				"To find the version and build IDs",
				`asc versions list --app "APP_ID" --paginate --output json`,
				`asc builds list --app "APP_ID" --paginate --output json`,
			},
		},
		{
			name: "TestFlight distribution view",
			path: []string{"testflight", "distribution", "view"},
			want: []string{
				"To find the build ID",
				`asc builds list --app "APP_ID" --paginate --output json`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %v not found", test.path)
			}
			for _, want := range test.want {
				if !strings.Contains(command.LongHelp, want) {
					t.Fatalf("expected help for %v to contain %q, got %q", test.path, want, command.LongHelp)
				}
			}
		})
	}
}
