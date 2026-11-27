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
	command, alias string
	// canonical is the flag the alias stands for, or for --build the flag a
	// build ID and the flag a build number resolve to, joined by "|".
	canonical string
}

const (
	aliasBuildID       = "8e2f6a3c-41b7-4c55-9a0d-2f1c7b6e9d40"
	aliasOtherBuildID  = "1A2B3C4D-5E6F-4A7B-8C9D-0E1F2A3B4C5D"
	aliasBuildResource = `{"data":{"type":"builds","id":"8e2f6a3c-41b7-4c55-9a0d-2f1c7b6e9d40","attributes":{"version":"42","processingState":"VALID"}}}`
)

// flagAliases lists every accepted alias: removing one is a visible change,
// so the test fails until it is taken out of this table too.
var flagAliases = []flagAliasCase{
	{"analytics sales", "output-file", "output"},
	{"analytics sales", "output-path", "output"},
	{"api", "data", "body"},
	{"api", "url-params", "query"},
	{"apps app-encryption-declarations list", "build", "build-id"},
	{"apps info relationships primary-category", "id", "info-id"},
	{"apps info relationships primary-subcategory-one", "id", "info-id"},
	{"apps info relationships primary-subcategory-two", "id", "info-id"},
	{"apps info relationships secondary-category", "id", "info-id"},
	{"apps info relationships secondary-subcategory-one", "id", "info-id"},
	{"apps info relationships secondary-subcategory-two", "id", "info-id"},
	{"apps info relationships view", "id", "info-id"},
	{"apps info territory-age-ratings list", "id", "info-id"},
	{"apps info view", "app-id", "app"},
	{"apps info view", "app-info", "info-id"},
	{"apps info view", "id", "info-id"},
	{"apps public descriptions", "id", "app"},
	{"apps public prices", "id", "app"},
	{"apps public view", "id", "app"},
	{"apps view", "app", "id"},
	{"apps view", "app-id", "id"},
	{"build-bundles list", "build", "build-id"},
	{"build-localizations create", "build", "build-id"},
	{"build-localizations list", "build", "build-id"},
	{"builds add-groups", "build", "build-id|build-number"},
	{"builds add-groups", "group-id", "group"},
	{"builds app view", "build", "build-id|build-number"},
	{"builds app view", "id", "build-id"},
	{"builds app-encryption-declaration view", "build", "build-id|build-number"},
	{"builds app-encryption-declaration view", "id", "build-id"},
	{"builds beta-app-review-submission view", "build", "build-id|build-number"},
	{"builds beta-app-review-submission view", "id", "build-id"},
	{"builds build-beta-detail view", "build", "build-id|build-number"},
	{"builds build-beta-detail view", "id", "build-id"},
	{"builds dsyms", "build", "build-id|build-number"},
	{"builds expire", "build", "build-id|build-number"},
	{"builds groups list", "build", "build-id"},
	{"builds icons list", "build", "build-id|build-number"},
	{"builds icons list", "id", "build-id"},
	{"builds individual-testers add", "build", "build-id|build-number"},
	{"builds individual-testers list", "build", "build-id|build-number"},
	{"builds individual-testers remove", "build", "build-id|build-number"},
	{"builds info", "build", "build-id|build-number"},
	{"builds info", "id", "build-id"},
	{"builds links view", "build", "build-id|build-number"},
	{"builds list", "app-id", "app"},
	{"builds metrics beta-usages", "build", "build-id|build-number"},
	{"builds pre-release-version view", "build", "build-id|build-number"},
	{"builds pre-release-version view", "id", "build-id"},
	{"builds remove-groups", "build", "build-id|build-number"},
	{"builds test-notes create", "build", "build-id|build-number"},
	{"builds test-notes delete", "build", "build-id|build-number"},
	{"builds test-notes delete", "id", "localization-id"},
	{"builds test-notes list", "build", "build-id|build-number"},
	{"builds test-notes update", "build", "build-id|build-number"},
	{"builds test-notes update", "id", "localization-id"},
	{"builds test-notes view", "build", "build-id|build-number"},
	{"builds test-notes view", "id", "localization-id"},
	{"builds update", "build", "build-id|build-number"},
	{"builds wait", "build", "build-id|build-number"},
	{"builds wait", "newest", "latest"},
	{"bundle-ids capabilities list", "bundle-id", "bundle"},
	{"encryption declarations assign-builds", "build", "build-id"},
	{"encryption declarations list", "build", "build-id"},
	{"iap versions localizations update", "id", "localization-id"},
	{"iap versions view", "id", "version-id"},
	{"iap view", "iap-id", "id"},
	{"ipa-info", "ipa", "path"},
	{"localizations list", "version-id", "version"},
	{"localizations update", "localization-id", "id"},
	{"localizations update", "version-id", "version"},
	{"metadata pull", "version-id", "version"},
	{"performance diagnostics list", "build", "build-id"},
	{"performance download", "build", "build-id"},
	{"performance metrics view", "build", "build-id"},
	{"product-pages custom-pages localizations list", "version-id", "custom-page-version-id"},
	{"profiles download", "output-path", "output"},
	{"publish testflight", "build", "build-id|build-number"},
	{"release stage", "build", "build-id"},
	{"review items list", "submission-id", "submission"},
	{"review items-list", "submission-id", "submission"},
	{"review submit", "build", "build-id|build-number"},
	{"screenshots list", "localization-id", "version-localization"},
	{"subscriptions list", "group", "group-id"},
	{"subscriptions list", "subscription-group", "group-id"},
	{"subscriptions list", "subscription-group-id", "group-id"},
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
	{"testflight review submissions list", "app-id", "app"},
	{"testflight review submissions list", "build", "build-id"},
	{"testflight review submit", "build", "build-id"},
	{"testflight testers add-builds", "build", "build-id"},
	{"testflight testers export", "build", "build-id"},
	{"testflight testers list", "build", "build-id"},
	{"testflight testers remove-builds", "build", "build-id"},
	{"validate testflight", "build", "build-id"},
	{"versions attach-build", "build", "build-id|build-number"},
	{"versions list", "app-id", "app"},
	{"versions update", "id", "version-id"},
	{"versions view", "id", "version-id"},
	{"xcode-cloud status", "id", "run-id"},
}

// TestFlagAliasesBehaveAsTheirCanonicalFlag checks that flagAliases is exactly
// the set of aliases bound with shared.BindFlagAlias, then runs each against
// its canonical flag: the two spellings must print, exit, request and record
// telemetry identically, the alias must stay out of help, and both spellings
// with different values must be a usage error naming both, unless the flag is
// repeatable.
func TestFlagAliasesBehaveAsTheirCanonicalFlag(t *testing.T) {
	var bound []flagAliasCase
	walkCommandTree(RootCommand("1.0.0"), func(path string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		command.FlagSet.VisitAll(func(item *flag.Flag) {
			if _, _, implied := shared.FlagAliasImpliedValue(item); implied {
				return
			}
			canonical, ok := shared.FlagAliasTarget(item, aliasBuildID)
			if !ok {
				return
			}
			if byNumber, _ := shared.FlagAliasTarget(item, "42"); byNumber != canonical {
				canonical += "|" + byNumber
			}
			bound = append(bound, flagAliasCase{command: strings.TrimPrefix(path, "asc "), alias: item.Name, canonical: canonical})
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
		targets := strings.Split(test.canonical, "|")
		for _, target := range targets {
			value1, value2 := "value-1", "value-2"
			if len(targets) > 1 {
				value1, value2 = "42", "43"
				if target == "build-id" {
					value1, value2 = aliasBuildID, aliasOtherBuildID
				}
			}
			alias := struct {
				path            []string
				name, canonical string
			}{strings.Fields(test.command), test.alias, target}
			t.Run(strings.Join(alias.path, " ")+" --"+alias.name+" as --"+target, func(t *testing.T) {
				testFlagAliasAgainstCanonical(t, alias.path, alias.name, alias.canonical, value1, value2)
			})
		}
	}
}

// testFlagAliasAgainstCanonical runs one alias against the flag that value1
// and value2 resolve it to.
func testFlagAliasAgainstCanonical(t *testing.T, path []string, aliasName, canonicalName, value1, value2 string) {
	t.Helper()
	alias := struct {
		path            []string
		name, canonical string
	}{path, aliasName, canonicalName}
	invoke := func(args ...string) aliasInvocationResult {
		return runAliasInvocation(t, append(slices.Clone(alias.path), args...))
	}

	canonical := invoke("--"+alias.canonical, value1)
	aliased := invoke("--"+alias.name, value1)
	if !reflect.DeepEqual(aliased, canonical) {
		t.Fatalf("--%s ran differently from --%s:\nalias:     %+v\ncanonical: %+v", alias.name, alias.canonical, aliased, canonical)
	}
	if shared.IsRepeatableFlag(commandFlagSet(t, RootCommand("1.0.0"), strings.Join(alias.path, " ")).Lookup(alias.canonical)) {
		// Every value of a repeatable flag counts, under either spelling.
		mixed := invoke("--"+alias.name, value1, "--"+alias.canonical, value2)
		if twice := invoke("--"+alias.canonical, value1, "--"+alias.canonical, value2); !reflect.DeepEqual(mixed, twice) {
			t.Fatalf("--%s and --%s ran differently from --%s twice:\nmixed: %+v\ntwice: %+v", alias.name, alias.canonical, alias.canonical, mixed, twice)
		}
		return
	}
	if both := invoke("--"+alias.name, value1, "--"+alias.canonical, value1); !reflect.DeepEqual(both, canonical) {
		t.Fatalf("--%s and --%s with one value ran differently from --%s:\nboth:      %+v\ncanonical: %+v", alias.name, alias.canonical, alias.canonical, both, canonical)
	}

	conflict := invoke("--"+alias.name, value1, "--"+alias.canonical, value2)
	wantStderr := fmt.Sprintf("Error: `--%s` and `--%s` are the same flag; pass one value\n", alias.name, alias.canonical)
	if conflict.exit != ExitUsage || conflict.stderr != wantStderr || conflict.stdout != "" || len(conflict.requests) != 0 {
		t.Fatalf("conflicting values = %+v, want exit %d and stderr %q", conflict, ExitUsage, wantStderr)
	}
	if conflict.telemetry.ErrorKind != telemetry.ErrorKindInvalidValue || conflict.telemetry.FailureParameter != "--"+alias.canonical {
		t.Fatalf("conflicting values telemetry = %+v", conflict.telemetry)
	}

	if spaced := invoke("--"+alias.name, value1, "--"+alias.canonical, " "+value1); spaced.exit != ExitUsage || len(spaced.requests) != 0 {
		t.Fatalf("values differing only in whitespace = %+v, want a conflict (exit %d)", spaced, ExitUsage)
	}
	lastWins := invoke("--"+alias.canonical, value1, "--"+alias.canonical, value2, "--"+alias.name, value1)
	wantStderr = fmt.Sprintf("Error: `--%s` and `--%s` are the same flag; pass one value\n", alias.canonical, alias.name)
	if lastWins.exit != ExitUsage || lastWins.stderr != wantStderr || len(lastWins.requests) != 0 {
		t.Fatalf("alias matching an overridden canonical value = %+v, want exit %d and stderr %q", lastWins, ExitUsage, wantStderr)
	}
	repeated := invoke("--"+alias.name, value1, "--"+alias.name, value2)
	if agreeing := invoke("--"+alias.name, value1, "--"+alias.name, value2, "--"+alias.canonical, value2); !reflect.DeepEqual(agreeing, repeated) {
		t.Fatalf("canonical matching the last alias value ran differently from the alias alone:\nboth:  %+v\nalias: %+v", agreeing, repeated)
	}

	help := invoke("--help")
	if regexp.MustCompile(`(?m)^\s+--` + regexp.QuoteMeta(alias.name) + `\s`).MatchString(help.stdout) {
		t.Fatalf("help lists --%s:\n%s", alias.name, help.stdout)
	}
}

// An alias of a flag that never resolves @env:/@file: keeps that flag's rule:
// the token stays a literal value under either spelling.
func TestFlagAliasKeepsCanonicalIndirectionExclusion(t *testing.T) {
	t.Setenv("ALIAS_TEST_UNSET", "")
	args := []string{"profiles", "download", "--id", "PROFILE_ID"}
	aliased := runAliasInvocation(t, append(slices.Clone(args), "--output-path", "@env:ALIAS_TEST_UNSET"))
	canonical := runAliasInvocation(t, append(slices.Clone(args), "--output", "@env:ALIAS_TEST_UNSET"))
	if !reflect.DeepEqual(aliased, canonical) {
		t.Fatalf("--output-path @env:… ran differently from --output @env:…:\nalias:     %+v\ncanonical: %+v", aliased, canonical)
	}
}

// TestBuildAliasReadsItsValueByShape runs --build on a read, a write and a
// wait command: a UUID must behave exactly as --build-id and any other value
// exactly as --build-number, and a second flag the value conflicts with exits 2.
func TestBuildAliasReadsItsValueByShape(t *testing.T) {
	appSelector := []string{"--app", "123456789", "--platform", "IOS"}
	tests := []struct {
		command  []string
		byID     []string
		byNumber []string
	}{
		{command: []string{"builds", "info"}, byNumber: appSelector},
		{command: []string{"builds", "expire"}, byID: []string{"--confirm"}, byNumber: append(slices.Clone(appSelector), "--confirm")},
		{command: []string{"builds", "wait"}, byNumber: appSelector},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.command, " "), func(t *testing.T) {
			invoke := func(base []string, args ...string) aliasInvocationResult {
				return runAliasInvocation(t, append(append(slices.Clone(test.command), base...), args...))
			}
			same := func(name string, got, want aliasInvocationResult) {
				t.Helper()
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s ran differently:\n--build: %+v\nwant:    %+v", name, got, want)
				}
				if len(want.requests) == 0 {
					t.Fatalf("%s sent no request, so the comparison proves nothing: %+v", name, want)
				}
			}
			same("--build <uuid>", invoke(test.byID, "--build", aliasBuildID), invoke(test.byID, "--build-id", aliasBuildID))
			same("--build 42", invoke(test.byNumber, "--build", "42"), invoke(test.byNumber, "--build-number", "42"))
			same("--build 1.2.3", invoke(test.byNumber, "--build", "1.2.3"), invoke(test.byNumber, "--build-number", "1.2.3"))
			selfLink := "https://api.appstoreconnect.apple.com/v1/builds/" + aliasBuildID
			same("--build <self-link>", invoke(test.byID, "--build", selfLink), invoke(test.byID, "--build-id", selfLink))
			t.Setenv("ALIAS_TEST_BUILD_ID", aliasBuildID)
			same("--build @env:<uuid>", invoke(test.byID, "--build", "@env:ALIAS_TEST_BUILD_ID"), invoke(test.byID, "--build-id", aliasBuildID))

			for _, conflict := range [][]string{
				{"--build", aliasBuildID, "--build-id", aliasOtherBuildID},
				{"--build", "42", "--build-number", "43"},
			} {
				if got := invoke(test.byNumber, conflict...); got.exit != ExitUsage || len(got.requests) != 0 {
					t.Fatalf("%v = %+v, want exit %d before any request", conflict, got, ExitUsage)
				}
			}
			mixed := invoke(test.byNumber, "--build", aliasBuildID, "--build-number", "42")
			if want := invoke(test.byNumber, "--build-id", aliasBuildID, "--build-number", "42"); !reflect.DeepEqual(mixed, want) || mixed.exit != ExitUsage {
				t.Fatalf("--build <uuid> --build-number 42 = %+v, want what --build-id <uuid> --build-number 42 gives (exit %d): %+v", mixed, ExitUsage, want)
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
		switch version := req.URL.Query().Get("filter[version]"); {
		case req.URL.Path == "/v1/builds/"+aliasBuildID:
			fmt.Fprint(w, aliasBuildResource)
		case strings.HasSuffix(req.URL.Path, "/builds") && version != "":
			fmt.Fprintf(w, `{"data":[{"type":"builds","id":%q,"attributes":{"version":%q,"processingState":"VALID"}}],"links":{}}`, aliasBuildID, version)
		default:
			fmt.Fprint(w, `{"data":[]}`)
		}
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
