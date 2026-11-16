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

// TestFlagAliasesBehaveAsTheirCanonicalFlag runs every alias bound with
// shared.BindFlagAlias against its canonical flag: the two spellings must
// print, exit, request and record telemetry identically, the alias must stay
// out of help, and both spellings with different values must be a usage error
// naming both.
func TestFlagAliasesBehaveAsTheirCanonicalFlag(t *testing.T) {
	type flagAlias struct {
		path            []string
		name, canonical string
	}
	var aliases []flagAlias
	walkCommandTree(RootCommand("1.0.0"), func(path string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		command.FlagSet.VisitAll(func(item *flag.Flag) {
			if canonical, ok := shared.FlagAliasTarget(item); ok {
				aliases = append(aliases, flagAlias{path: strings.Fields(path)[1:], name: item.Name, canonical: canonical})
			}
		})
	})
	if len(aliases) == 0 {
		t.Fatal("no flag aliases are bound")
	}

	for _, alias := range aliases {
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
