package cmdtest

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// webLeafExclusions lists asc web leaves that are session plumbing rather than
// product capabilities. Each reason is part of the inventory contract: a new
// web leaf must gain a capability row or an explicit reason here.
var webLeafExclusions = map[string]string{
	"asc web auth login":         "Session authentication plumbing for other web commands.",
	"asc web auth status":        "Session status check, not a product workflow.",
	"asc web auth logout":        "Session cache cleanup, not a product workflow.",
	"asc web auth export":        "Session cache transfer plumbing for other machines and CI.",
	"asc web auth import":        "Session cache transfer plumbing for other machines and CI.",
	"asc web auth messages-code": "2FA code helper for web auth login --two-factor-code-command, not a product workflow.",
}

func TestRun_CapabilitiesInventoryCoversWebLeaves(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	root := RootCommand("1.0.0")
	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("expected registered asc web command")
	}

	leaves := collectCommandLeaves(webCmd, []string{"asc"})
	if len(leaves) < 20 {
		t.Fatalf("expected a populated web command tree, got %d leaves: %v", len(leaves), leaves)
	}

	stdout, stderr, code := runWithDeps(t, nil, "capabilities", "--output", "json")
	if code != cmd.ExitSuccess {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}

	var resp capabilitiesTestResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected JSON output, got error %v and stdout %s", err, stdout)
	}

	var uncovered []string
	seen := make(map[string]struct{}, len(leaves))
	for _, leaf := range leaves {
		seen[leaf] = struct{}{}
		if reason, excluded := webLeafExclusions[leaf]; excluded {
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("exclusion %q has an empty reason", leaf)
			}
			continue
		}
		if !inventoryCoversWebLeaf(resp, leaf) {
			uncovered = append(uncovered, leaf)
		}
	}

	if len(uncovered) > 0 {
		t.Fatalf("web leaves missing a capability entry or explicit exclusion:\n%s", strings.Join(uncovered, "\n"))
	}

	var stale []string
	for leaf := range webLeafExclusions {
		if _, ok := seen[leaf]; !ok {
			stale = append(stale, leaf)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("exclusions that are not current web leaves: %s", strings.Join(stale, ", "))
	}
}

func collectCommandLeaves(cmd *ffcli.Command, prefix []string) []string {
	path := append(append([]string{}, prefix...), cmd.Name)
	if len(cmd.Subcommands) == 0 {
		return []string{strings.Join(path, " ")}
	}
	leaves := make([]string, 0)
	for _, sub := range cmd.Subcommands {
		leaves = append(leaves, collectCommandLeaves(sub, path)...)
	}
	return leaves
}

func inventoryCoversWebLeaf(resp capabilitiesTestResponse, leaf string) bool {
	for _, entry := range resp.Capabilities {
		for _, command := range entry.Commands {
			if commandCoversWebLeaf(command, leaf) {
				return true
			}
		}
	}
	return false
}

func commandCoversWebLeaf(command, leaf string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	if command == leaf {
		return true
	}
	return strings.HasPrefix(leaf, command+" ")
}
