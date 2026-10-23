package ads

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestPlatformCommandInventoryMatchesIndependentFixture(t *testing.T) {
	want := platformFixtureCommandPaths(t)
	if got, expected := len(want), 99; got != expected {
		t.Fatalf("platform fixture command count = %d, want %d", got, expected)
	}

	// These executable leaves are intentionally outside the endpoint fixture:
	// the raw request escape hatch and the two status convenience workflows.
	knownNonEndpointLeaves := map[string]struct{}{
		"api request":      {},
		"campaigns pause":  {},
		"campaigns resume": {},
	}

	leaves := collectDirectPlatformCommandLeaves(AdsCommand())
	for path, count := range leaves {
		if count != 1 {
			t.Errorf("asc ads %s is registered %d times, want exactly once", path, count)
		}
	}

	for path := range want {
		if count := leaves[path]; count != 1 {
			t.Errorf("fixture command asc ads %s is registered %d times, want exactly once", path, count)
		}
	}
	for path := range knownNonEndpointLeaves {
		if count := leaves[path]; count != 1 {
			t.Errorf("known non-endpoint command asc ads %s is registered %d times, want exactly once", path, count)
		}
	}

	for path := range leaves {
		if _, ok := want[path]; ok {
			continue
		}
		if _, ok := knownNonEndpointLeaves[path]; ok {
			continue
		}
		t.Errorf("unexpected executable leaf asc ads %s", path)
	}

	if got, expected := len(leaves), len(want)+len(knownNonEndpointLeaves); got != expected {
		t.Fatalf("platform executable leaf count = %d, want %d", got, expected)
	}

	// Upload is represented by a dedicated multipart command rather than the
	// generated JSON endpoint command, but it must still occupy the fixture leaf.
	upload := findCommand(AdsCommand(), "assets", "upload")
	if upload == nil || upload.Exec == nil {
		t.Fatal("missing executable asc ads assets upload command")
	}
}

func platformFixtureCommandPaths(t *testing.T) map[string]struct{} {
	t.Helper()

	file, err := os.Open("../../appleads/testdata/platform_v1_endpoints.tsv")
	if err != nil {
		t.Fatalf("open platform endpoint fixture: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read platform endpoint fixture: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("platform endpoint fixture has %d records, want a header and rows", len(records))
	}

	commandColumn := -1
	for index, column := range records[0] {
		if column == "command" {
			commandColumn = index
			break
		}
	}
	if commandColumn < 0 {
		t.Fatalf("platform endpoint fixture has no command column: %v", records[0])
	}

	paths := make(map[string]struct{}, len(records)-1)
	for rowIndex, record := range records[1:] {
		if commandColumn >= len(record) {
			t.Fatalf("platform endpoint fixture row %d has %d columns, want command column %d: %v", rowIndex+2, len(record), commandColumn+1, record)
		}
		path := strings.TrimSpace(record[commandColumn])
		if path == "" {
			t.Fatalf("platform endpoint fixture row %d has an empty command path", rowIndex+2)
		}
		if _, exists := paths[path]; exists {
			t.Fatalf("platform endpoint fixture duplicates command path %q", path)
		}
		paths[path] = struct{}{}
	}
	return paths
}

func collectDirectPlatformCommandLeaves(root *ffcli.Command) map[string]int {
	leaves := map[string]int{}
	var walk func(*ffcli.Command, []string)
	walk = func(command *ffcli.Command, path []string) {
		if len(command.Subcommands) == 0 {
			if command.Exec != nil {
				leaves[strings.Join(path, " ")]++
			}
			return
		}
		for _, subcommand := range command.Subcommands {
			walk(subcommand, append(append([]string(nil), path...), subcommand.Name))
		}
	}
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "auth" {
			continue
		}
		walk(subcommand, []string{subcommand.Name})
	}
	return leaves
}
