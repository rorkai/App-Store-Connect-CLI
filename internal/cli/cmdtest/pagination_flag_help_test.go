package cmdtest

import (
	"flag"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// paginationFlagHelpExceptions are --limit, --next and --paginate help lines
// that say something the shared lines do not, keyed by flag and help line.
var paginationFlagHelpExceptions = map[string]string{
	"--limit Maximum groups per page (1-200)":                                          "Game Center metrics pages hold groups of data points",
	"--limit Maximum results per page (1-200; 0 uses server default)":                  "states the --limit 0 behaviour",
	"--limit With --set-id: maximum results per page (1-200; 0 uses server default)":   "applies only with --set-id",
	"--limit Maximum schemes to return":                                                "caps the total; the command has no pages",
	"--limit Maximum results to return":                                                "Apple Ads search pages by --offset, not --next",
	"--limit Maximum field changes per page":                                           "counts field changes inside each change-history entry",
	"--limit Maximum suggestions to return":                                            "caps the total; the command has no pages",
	"--limit Limit number of logs (max 200)":                                           "caps the logs of one signature; the command has no pages",
	"--limit Limit number of logs (max 200, diagnostic logs only)":                     "applies to diagnostic log downloads only",
	"--limit Maximum number of apps to include (1-200)":                                "caps the total; the command has no pages",
	"--limit Maximum results (1-200)":                                                  "caps the total; the command has no pages",
	"--limit Maximum included localizations (1-200)":                                   "counts included resources",
	"--limit Maximum included build bundles (1-50)":                                    "counts included resources",
	"--limit Maximum number of results to display (0 = all)":                           "caps local display; 0 shows all",
	"--limit Maximum number of results to return":                                      "caps the total; the command has no pages",
	"--next Start from a links.next URL":                                               "the command always continues to the last page",
	"--paginate Paginate all reports (recommended with --processing-date or --next)":   "recommends flags to combine with it",
	"--paginate Fetch all pages (always on; kept for compatibility)":                   "has no effect",
	"--paginate With --set-id: fetch all pages and aggregate the collection":           "applies only with --set-id",
	"--paginate Fetch all app pages (default: first page only)":                        "pages the apps being audited, not the output",
	"--paginate Follow links.next and print one merged collection envelope (GET only)": "works for GET requests only",
}

var limitFlagUsagePattern = regexp.MustCompile(`^Maximum results per page \(1-(\d+)\)$`)

func TestPaginationFlagsUseSharedHelp(t *testing.T) {
	root := RootCommand("1.2.3")
	seen := map[string]bool{}
	var mismatched []string
	var walk func(path string, command *ffcli.Command)
	walk = func(path string, command *ffcli.Command) {
		if command.FlagSet != nil {
			command.FlagSet.VisitAll(func(f *flag.Flag) {
				if f.Name != "limit" && f.Name != "next" && f.Name != "paginate" {
					return
				}
				if isSharedPaginationFlagUsage(f) {
					return
				}
				key := "--" + f.Name + " " + f.Usage
				if _, ok := paginationFlagHelpExceptions[key]; ok {
					seen[key] = true
					return
				}
				mismatched = append(mismatched, path+" "+key)
			})
		}
		for _, sub := range command.Subcommands {
			walk(path+" "+sub.Name, sub)
		}
	}
	walk("asc", root)

	if len(mismatched) > 0 {
		sort.Strings(mismatched)
		t.Fatalf("pagination flags whose help is neither shared.LimitFlagUsage, shared.NextFlagUsage or shared.PaginateFlagUsage nor a reasoned exception:\n%s", strings.Join(mismatched, "\n"))
	}
	var stale []string
	for key := range paginationFlagHelpExceptions {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("stale pagination help exceptions (no command uses them):\n%s", strings.Join(stale, "\n"))
	}
}

func isSharedPaginationFlagUsage(f *flag.Flag) bool {
	switch f.Name {
	case "next":
		return f.Usage == shared.NextFlagUsage
	case "paginate":
		return f.Usage == shared.PaginateFlagUsage
	}
	match := limitFlagUsagePattern.FindStringSubmatch(f.Usage)
	if match == nil {
		return false
	}
	limitMax, err := strconv.Atoi(match[1])
	return err == nil && f.Usage == shared.LimitFlagUsage(limitMax)
}
