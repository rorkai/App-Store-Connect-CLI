package docs_test

import (
	_ "embed"
	"flag"
	"sort"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

//go:embed templates/ASC.md
var embeddedTemplate string

func TestASCTemplateIncludesAllRootSubcommands(t *testing.T) {
	section := sectionBetween(t, embeddedTemplate, "## Command Groups", "## Global Flags")
	templateCommands := parseBacktickBullets(section)

	root := cmd.RootCommand("test")
	rootCommands := make([]string, 0, len(root.Subcommands))
	for _, sub := range root.Subcommands {
		if strings.HasPrefix(strings.TrimSpace(sub.ShortHelp), "DEPRECATED:") {
			continue
		}
		rootCommands = append(rootCommands, sub.Name)
	}

	missing := difference(rootCommands, templateCommands)
	extra := difference(templateCommands, rootCommands)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("template command groups are out of sync: missing=%v extra=%v", missing, extra)
	}
}

func TestASCTemplateIncludesAllRootFlags(t *testing.T) {
	section := sectionBetween(t, embeddedTemplate, "## Global Flags", "## Environment Variables")
	templateFlags := parseBacktickBullets(section)

	root := cmd.RootCommand("test")
	rootFlags := []string{}
	root.FlagSet.VisitAll(func(f *flag.Flag) {
		rootFlags = append(rootFlags, "--"+f.Name)
	})

	missing := difference(rootFlags, templateFlags)
	extra := difference(templateFlags, rootFlags)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("template global flags are out of sync: missing=%v extra=%v", missing, extra)
	}
}

func sectionBetween(t *testing.T, content, startHeading, endHeading string) string {
	t.Helper()

	start := strings.Index(content, startHeading)
	if start == -1 {
		t.Fatalf("missing heading %q", startHeading)
	}

	rest := content[start:]
	endRel := strings.Index(rest, endHeading)
	if endRel == -1 {
		t.Fatalf("missing heading %q", endHeading)
	}

	return rest[:endRel]
}

func parseBacktickBullets(section string) []string {
	values := []string{}
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- `") {
			continue
		}

		rest := strings.TrimPrefix(trimmed, "- `")
		end := strings.Index(rest, "`")
		if end <= 0 {
			continue
		}

		values = append(values, rest[:end])
	}

	return uniqueSorted(values)
}

func difference(expected, actual []string) []string {
	actualSet := map[string]struct{}{}
	for _, v := range actual {
		actualSet[v] = struct{}{}
	}

	missing := []string{}
	for _, v := range expected {
		if _, ok := actualSet[v]; !ok {
			missing = append(missing, v)
		}
	}

	return uniqueSorted(missing)
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		set[v] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
