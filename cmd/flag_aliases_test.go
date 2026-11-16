package cmd

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

type aliasInvocationResult struct {
	stdout    string
	stderr    string
	exit      int
	requests  []string
	telemetry telemetry.EventContext
}

type flagAliasCase struct {
	command, alias, canonical string
}

// flagAliases lists every accepted alias: removing one is a visible change,
// so the test fails until it is taken out of this table too.
var flagAliases = []flagAliasCase{
	{"analytics sales", "output-file", "output"},
	{"analytics sales", "output-path", "output"},
	{"apps app-encryption-declarations list", "build", "build-id"},
	{"apps info relationships primary-category", "id", "info-id"},
	{"apps info relationships primary-subcategory-one", "id", "info-id"},
	{"apps info relationships primary-subcategory-two", "id", "info-id"},
	{"apps info relationships secondary-category", "id", "info-id"},
	{"apps info relationships secondary-subcategory-one", "id", "info-id"},
	{"apps info relationships secondary-subcategory-two", "id", "info-id"},
	{"apps info territory-age-ratings list", "id", "info-id"},
	{"apps info view", "app-info", "info-id"},
	{"apps public descriptions", "id", "app"},
	{"apps public prices", "id", "app"},
	{"apps public view", "id", "app"},
	{"apps view", "app", "id"},
	{"build-bundles list", "build", "build-id"},
	{"build-localizations create", "build", "build-id"},
	{"build-localizations list", "build", "build-id"},
	{"builds app view", "id", "build-id"},
	{"builds app-encryption-declaration view", "id", "build-id"},
	{"builds beta-app-review-submission view", "id", "build-id"},
	{"builds build-beta-detail view", "id", "build-id"},
	{"builds icons list", "id", "build-id"},
	{"builds list", "app-id", "app"},
	{"builds pre-release-version view", "id", "build-id"},
	{"builds test-notes delete", "id", "localization-id"},
	{"builds test-notes update", "id", "localization-id"},
	{"builds test-notes view", "id", "localization-id"},
	{"builds wait", "newest", "latest"},
	{"bundle-ids capabilities list", "bundle-id", "bundle"},
	{"encryption declarations assign-builds", "build", "build-id"},
	{"encryption declarations list", "build", "build-id"},
	{"iap view", "iap-id", "id"},
	{"ipa-info", "ipa", "path"},
	{"localizations list", "version-id", "version"},
	{"localizations update", "localization-id", "id"},
	{"localizations update", "version-id", "version"},
	{"performance diagnostics list", "build", "build-id"},
	{"performance download", "build", "build-id"},
	{"performance metrics view", "build", "build-id"},
	{"product-pages custom-pages localizations list", "version-id", "custom-page-version-id"},
	{"profiles download", "output-path", "output"},
	{"release stage", "build", "build-id"},
	{"review items list", "submission-id", "submission"},
	{"review items-list", "submission-id", "submission"},
	{"screenshots list", "localization-id", "version-localization"},
	{"subscriptions list", "group", "group-id"},
	{"subscriptions list", "version-limit", "versions-limit"},
	{"subscriptions offers introductory delete", "offer-id", "id"},
	{"subscriptions review screenshots delete", "id", "screenshot-id"},
	{"subscriptions versions list", "image-limit", "images-limit"},
	{"subscriptions versions list", "localization-limit", "localizations-limit"},
	{"subscriptions versions view", "image-limit", "images-limit"},
	{"subscriptions versions view", "localization-limit", "localizations-limit"},
	{"subscriptions versions view", "version-id", "id"},
	{"subscriptions view", "subscription-id", "id"},
	{"subscriptions view", "version-limit", "versions-limit"},
	{"testflight config export", "build", "build-id"},
	{"testflight crashes list", "build", "build-id"},
	{"testflight distribution view", "build", "build-id"},
	{"testflight feedback list", "build", "build-id"},
	{"testflight groups builds list", "group", "group-id"},
	{"testflight groups testers list", "group", "group-id"},
	{"testflight groups view", "group-id", "id"},
	{"testflight notifications send", "build", "build-id"},
	{"testflight review submissions list", "build", "build-id"},
	{"testflight review submit", "build", "build-id"},
	{"testflight testers add-builds", "build", "build-id"},
	{"testflight testers export", "build", "build-id"},
	{"testflight testers list", "build", "build-id"},
	{"testflight testers remove-builds", "build", "build-id"},
	{"validate testflight", "build", "build-id"},
	{"versions update", "id", "version-id"},
	{"versions view", "id", "version-id"},
	{"xcode-cloud status", "id", "run-id"},
}

// TestFlagAliasesBehaveAsTheirCanonicalFlag checks that flagAliases is exactly
// the set of aliases bound with shared.BindFlagAlias, then runs each against
// its canonical flag: the two spellings must print, exit, request and record
// telemetry identically, the alias must stay out of help, and both spellings
// with different values must be a usage error naming both.
func TestFlagAliasesBehaveAsTheirCanonicalFlag(t *testing.T) {
	var bound []flagAliasCase
	walkCommandTree(RootCommand("1.0.0"), func(path string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		command.FlagSet.VisitAll(func(item *flag.Flag) {
			if canonical, ok := shared.FlagAliasTarget(item); ok {
				bound = append(bound, flagAliasCase{command: strings.TrimPrefix(path, "asc "), alias: item.Name, canonical: canonical})
			}
		})
	})
	byKey := func(a, b flagAliasCase) int {
		return strings.Compare(a.command+" --"+a.alias, b.command+" --"+b.alias)
	}
	want := slices.SortedFunc(slices.Values(flagAliases), byKey)
	if got := slices.SortedFunc(slices.Values(bound), byKey); !slices.Equal(got, want) {
		t.Fatalf("bound aliases differ from flagAliases:\nbound: %v\nwant:  %v", got, want)
	}

	for _, test := range flagAliases {
		alias := struct {
			path            []string
			name, canonical string
		}{strings.Fields(test.command), test.alias, test.canonical}
		t.Run(strings.Join(alias.path, " ")+" --"+alias.name, func(t *testing.T) {
			invoke := func(args ...string) aliasInvocationResult {
				return runAliasInvocation(t, append(slices.Clone(alias.path), args...))
			}

			canonical := invoke("--"+alias.canonical, "value-1")
			aliased := invoke("--"+alias.name, "value-1")
			if !reflect.DeepEqual(aliased, canonical) {
				t.Fatalf("--%s ran differently from --%s:\nalias:     %+v\ncanonical: %+v", alias.name, alias.canonical, aliased, canonical)
			}
			if both := invoke("--"+alias.name, "value-1", "--"+alias.canonical, "value-1"); !reflect.DeepEqual(both, canonical) {
				t.Fatalf("--%s and --%s with one value ran differently from --%s:\nboth:      %+v\ncanonical: %+v", alias.name, alias.canonical, alias.canonical, both, canonical)
			}

			conflict := invoke("--"+alias.name, "value-1", "--"+alias.canonical, "value-2")
			wantStderr := fmt.Sprintf("Error: `--%s` and `--%s` are the same flag; pass one value\n", alias.name, alias.canonical)
			if conflict.exit != ExitUsage || conflict.stderr != wantStderr || conflict.stdout != "" || len(conflict.requests) != 0 {
				t.Fatalf("conflicting values = %+v, want exit %d and stderr %q", conflict, ExitUsage, wantStderr)
			}
			if conflict.telemetry.ErrorKind != telemetry.ErrorKindInvalidValue || conflict.telemetry.FailureParameter != "--"+alias.canonical {
				t.Fatalf("conflicting values telemetry = %+v", conflict.telemetry)
			}

			if spaced := invoke("--"+alias.name, "value-1", "--"+alias.canonical, " value-1"); spaced.exit != ExitUsage || len(spaced.requests) != 0 {
				t.Fatalf("values differing only in whitespace = %+v, want a conflict (exit %d)", spaced, ExitUsage)
			}
			lastWins := invoke("--"+alias.canonical, "value-1", "--"+alias.canonical, "value-2", "--"+alias.name, "value-1")
			wantStderr = fmt.Sprintf("Error: `--%s` and `--%s` are the same flag; pass one value\n", alias.canonical, alias.name)
			if lastWins.exit != ExitUsage || lastWins.stderr != wantStderr || len(lastWins.requests) != 0 {
				t.Fatalf("alias matching an overridden canonical value = %+v, want exit %d and stderr %q", lastWins, ExitUsage, wantStderr)
			}
			repeated := invoke("--"+alias.name, "value-1", "--"+alias.name, "value-2")
			if agreeing := invoke("--"+alias.name, "value-1", "--"+alias.name, "value-2", "--"+alias.canonical, "value-2"); !reflect.DeepEqual(agreeing, repeated) {
				t.Fatalf("canonical matching the last alias value ran differently from the alias alone:\nboth:  %+v\nalias: %+v", agreeing, repeated)
			}

			help := invoke("--help")
			if regexp.MustCompile(`(?m)^\s+--` + regexp.QuoteMeta(alias.name) + `\s`).MatchString(help.stdout) {
				t.Fatalf("help lists --%s:\n%s", alias.name, help.stdout)
			}
		})
	}
}

func runAliasInvocation(t *testing.T, args []string) aliasInvocationResult {
	t.Helper()
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	t.Chdir(t.TempDir())

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		mu.Lock()
		requests = append(requests, req.Method+" "+req.URL.RequestURI()+" "+string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) { return client, nil }
	d.ClientWithTimeout = func(time.Duration) (*asc.Client, error) { return client, nil }

	original := emitTelemetry
	t.Cleanup(func() { emitTelemetry = original })
	var result aliasInvocationResult
	emitTelemetry = func(_, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		result.telemetry = eventContext
	}

	result.stdout, result.stderr = captureCommandOutput(t, func() {
		result.exit = RunWithDeps(args, "1.0.0", d)
	})
	mu.Lock()
	result.requests = requests
	mu.Unlock()
	return result
}
