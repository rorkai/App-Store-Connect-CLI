package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestRemovedFlagAliasesAreUnknownFlags locks the 5.0.0 removal of the
// compatibility spellings that 4.x accepted behind a deprecation warning. Each
// removed alias now fails with migration guidance before any HTTP request, and
// the canonical spelling stays registered. TestRemovedFlagAliasesNameTheirReplacement
// pins the full diagnostic for a representative set.
func TestRemovedFlagAliasesAreUnknownFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		path      []string
		alias     string
		canonical string
		args      []string
	}{
		{path: []string{"versions", "update"}, alias: "id", canonical: "version-id", args: []string{"--id", "version-1", "--copyright", "2026 Example"}},
		{path: []string{"versions", "attach-build"}, alias: "build", canonical: "build-id", args: []string{"--version-id", "version-1", "--build", "build-1"}},
		{path: []string{"apps", "app-encryption-declarations", "list"}, alias: "build", canonical: "build-id", args: []string{"--id", "app-1", "--build", "build-1"}},
		{path: []string{"encryption", "declarations", "list"}, alias: "build", canonical: "build-id", args: []string{"--app", "app-1", "--build", "build-1"}},
		{path: []string{"encryption", "declarations", "assign-builds"}, alias: "build", canonical: "build-id", args: []string{"--id", "decl-1", "--build", "build-1"}},
		{path: []string{"bundle-ids", "capabilities", "list"}, alias: "bundle-id", canonical: "bundle", args: []string{"--bundle-id", "bundle-1"}},
		{path: []string{"localizations", "update"}, alias: "version-id", canonical: "version", args: []string{"--version-id", "version-1", "--locale", "en-US", "--description", "Updated"}},
		{path: []string{"screenshots", "list"}, alias: "localization-id", canonical: "version-localization", args: []string{"--localization-id", "loc-1"}},
		{path: []string{"apps", "info", "view"}, alias: "app-info", canonical: "info-id", args: []string{"--app-info", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-category"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-subcategory-one"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-subcategory-two"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-category"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-subcategory-one"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-subcategory-two"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
		{path: []string{"apps", "info", "territory-age-ratings", "list"}, alias: "id", canonical: "info-id", args: []string{"--id", "info-1"}},
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed aliases must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	root := RootCommand("1.2.3")
	for _, test := range tests {
		commandPath := strings.Join(test.path, " ")
		t.Run(commandPath+" --"+test.alias, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", commandPath)
			}
			if command.FlagSet.Lookup(test.canonical) == nil {
				t.Fatalf("canonical flag --%s not found on %q", test.canonical, commandPath)
			}

			args := append(append([]string{}, test.path...), test.args...)
			assertUsageExit(t, args, "Error: `--"+test.alias+"` was removed in 5.0.0; use `--"+test.canonical+"`")
		})
	}
}
