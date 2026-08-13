package cmdtest

import "testing"

func TestWebAppsCompatibilityCommandSurface(t *testing.T) {
	root := RootCommand("1.2.3")

	group := findSubcommand(root, "web", "apps", "compatibility")
	if group == nil {
		t.Fatal("expected web apps compatibility command")
	}
	if findSubcommand(root, "web", "apps", "compatibility", "view") == nil {
		t.Fatal("expected web apps compatibility view command")
	}
	if findSubcommand(root, "web", "apps", "compatibility", "edit") == nil {
		t.Fatal("expected web apps compatibility edit command")
	}
}

func TestWebAppsCompatibilityInvalidBooleanExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "web mac flag",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-mac=maybe",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-mac: must be true or false`,
		},
		{
			name: "web vision pro flag",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-vision-pro=maybe",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-vision-pro: must be true or false`,
		},
		{
			name: "flag value matches subcommand name",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-mac=edit",
			},
			wantStderr: `invalid value "edit" for flag -ios-app-on-mac: must be true or false`,
		},
		{
			name: "mixed flag order",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--ios-app-on-vision-pro=maybe",
				"--app", "app-1",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-vision-pro: must be true or false`,
		},
		{
			name: "flag before subcommands",
			args: []string{
				"--ios-app-on-mac=maybe",
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
			},
			wantStderr: "Error: unknown flag `--ios-app-on-mac` for `asc`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantStderr)
		})
	}
}
