package subscriptions

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type relationshipLimitCommandCase struct {
	name      string
	command   func(*shared.Deps) *ffcli.Command
	baseArgs  []string
	canonical []string
	removed   []string
}

func relationshipLimitCommandCases() []relationshipLimitCommandCase {
	return []relationshipLimitCommandCase{
		{
			name: "subscriptions list", command: newSubscriptionsListCommand,
			baseArgs:  []string{"--group-id", "group-1"},
			canonical: []string{"versions-limit"}, removed: []string{"version-limit"},
		},
		{
			name: "subscriptions view", command: newSubscriptionsGetCommand,
			baseArgs:  []string{"--id", "sub-1"},
			canonical: []string{"versions-limit"}, removed: []string{"version-limit"},
		},
		{
			name: "subscription versions list", command: newSubscriptionsVersionsListCommand,
			baseArgs:  []string{"--subscription-id", "sub-1"},
			canonical: []string{"images-limit", "localizations-limit"}, removed: []string{"image-limit", "localization-limit"},
		},
		{
			name: "subscription versions view", command: newSubscriptionsVersionsViewCommand,
			baseArgs:  []string{"--id", "version-1"},
			canonical: []string{"images-limit", "localizations-limit"}, removed: []string{"image-limit", "localization-limit"},
		},
	}
}

// relationshipLimitDeps returns deps whose client fails, and the deps' stderr.
func relationshipLimitDeps() (*shared.Deps, *bytes.Buffer) {
	d, stderr := newSubscriptionsTestDeps()
	d.Client = func() (*asc.Client, error) { return nil, errors.New("no client in this test") }
	return d, stderr
}

func captureRelationshipLimitStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = old }()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		_ = reader.Close()
		done <- buf.String()
	}()

	fn()
	_ = writer.Close()
	return <-done
}

// The singular relationship-limit spellings were removed in 5.0.0. Only the
// plural canonical flags remain bound, so the removed spellings fall through to
// the generic unknown-flag parse error.
func TestSubscriptionRelationshipLimitsExposeOnlyPluralCanonicalFlags(t *testing.T) {
	for _, test := range relationshipLimitCommandCases() {
		t.Run(test.name, func(t *testing.T) {
			d, _ := relationshipLimitDeps()
			cmd := test.command(d)
			usage := shared.DefaultUsageFunc(cmd)
			for index, canonical := range test.canonical {
				removed := test.removed[index]
				if cmd.FlagSet.Lookup(canonical) == nil {
					t.Fatalf("expected --%s", canonical)
				}
				if !strings.Contains(usage, "--"+canonical) {
					t.Fatalf("canonical help does not contain --%s: %q", canonical, usage)
				}
				if strings.Contains(usage, "--"+removed) {
					t.Fatalf("canonical help still mentions removed --%s: %q", removed, usage)
				}
			}
		})
	}
}

func TestSubscriptionRelationshipLimitsCanonicalFlagsReachClientSetup(t *testing.T) {
	isolateSubscriptionsAuthEnv(t)
	for _, test := range relationshipLimitCommandCases() {
		t.Run(test.name, func(t *testing.T) {
			d, depsStderr := relationshipLimitDeps()
			cmd := test.command(d)
			args := append([]string{}, test.baseArgs...)
			for index, canonical := range test.canonical {
				args = append(args, "--"+canonical, string(rune('7'+index)))
			}
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			stderr := captureRelationshipLimitStderr(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if err == nil {
					t.Fatal("expected isolated client setup to fail")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("canonical flags failed usage validation: %v", err)
				}
			})
			stderr += depsStderr.String()
			if strings.Contains(stderr, "deprecated") {
				t.Fatalf("canonical flags emitted a deprecation warning: %q", stderr)
			}
		})
	}
}

func TestSubscriptionRelationshipLimitsRejectInvalidCanonicalValues(t *testing.T) {
	for _, test := range relationshipLimitCommandCases() {
		for _, canonical := range test.canonical {
			t.Run(test.name+" "+canonical, func(t *testing.T) {
				d, _ := relationshipLimitDeps()
				cmd := test.command(d)
				args := append(append([]string{}, test.baseArgs...), "--"+canonical, "51")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--"+canonical+" must be between 1 and 50") {
					t.Fatalf("Exec() error = %v, want canonical range usage error", err)
				}
			})
		}
	}
}

func TestSubscriptionRelationshipLimitsConflictWithOpaqueNext(t *testing.T) {
	tests := []struct {
		name      string
		command   func(*shared.Deps) *ffcli.Command
		canonical string
	}{
		{name: "subscription versions", command: newSubscriptionsListCommand, canonical: "versions-limit"},
		{name: "version images", command: newSubscriptionsVersionsListCommand, canonical: "images-limit"},
		{name: "version localizations", command: newSubscriptionsVersionsListCommand, canonical: "localizations-limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, depsStderr := relationshipLimitDeps()
			cmd := test.command(d)
			if err := cmd.FlagSet.Parse([]string{"--next", "https://api.appstoreconnect.apple.com/v1/example?cursor=next", "--" + test.canonical, "7"}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			stderr := captureRelationshipLimitStderr(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--next cannot be combined with --"+test.canonical) {
					t.Fatalf("Exec() error = %v, want canonical opaque-next conflict", err)
				}
			})
			stderr += depsStderr.String()
			if strings.Contains(stderr, "Warning:") {
				t.Fatalf("unexpected warning: %q", stderr)
			}
		})
	}
}
