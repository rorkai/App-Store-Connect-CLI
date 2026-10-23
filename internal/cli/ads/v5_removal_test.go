package ads

import (
	"regexp"
	"strings"
	"testing"
)

func TestAdsV5RemovalGuidanceNamesRegisteredCommands(t *testing.T) {
	root := AdsCommand()
	if findCommand(root, adsV5RemovedCommand) != nil {
		t.Fatal("asc ads v5 must stay unregistered so the removal error is reachable")
	}
	commandPattern := regexp.MustCompile("`asc ads ([a-z0-9 -]+?)(?: --[^`]*)?`")
	for path, guidance := range adsV5Replacements {
		for _, match := range commandPattern.FindAllStringSubmatch(guidance, -1) {
			command := strings.Fields(match[1])
			if findCommand(root, command...) == nil {
				t.Fatalf("guidance for asc ads v5 %s names unregistered command asc ads %s", path, match[1])
			}
		}
	}
}
