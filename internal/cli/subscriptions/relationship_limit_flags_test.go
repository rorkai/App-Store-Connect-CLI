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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type relationshipLimitCommandCase struct {
	name      string
	command   func() *ffcli.Command
	baseArgs  []string
	canonical []string
	legacy    []string
}

func relationshipLimitCommandCases() []relationshipLimitCommandCase {
	return []relationshipLimitCommandCase{
		{
			name: "subscriptions list", command: SubscriptionsListCommand,
			baseArgs:  []string{"--group-id", "group-1"},
			canonical: []string{"versions-limit"}, legacy: []string{"version-limit"},
		},
		{
			name: "subscriptions view", command: SubscriptionsGetCommand,
			baseArgs:  []string{"--id", "sub-1"},
			canonical: []string{"versions-limit"}, legacy: []string{"version-limit"},
		},
		{
			name: "subscription versions list", command: SubscriptionsVersionsListCommand,
			baseArgs:  []string{"--subscription-id", "sub-1"},
			canonical: []string{"images-limit", "localizations-limit"}, legacy: []string{"image-limit", "localization-limit"},
		},
		{
			name: "subscription versions view", command: SubscriptionsVersionsViewCommand,
			baseArgs:  []string{"--id", "version-1"},
			canonical: []string{"images-limit", "localizations-limit"}, legacy: []string{"image-limit", "localization-limit"},
		},
	}
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

func TestSubscriptionRelationshipLimitsExposePluralCanonicalFlags(t *testing.T) {
	for _, test := range relationshipLimitCommandCases() {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			usage := shared.DefaultUsageFunc(cmd)
			for index, canonical := range test.canonical {
				legacy := test.legacy[index]
				if cmd.FlagSet.Lookup(canonical) == nil {
					t.Fatalf("expected --%s", canonical)
				}
				if !strings.Contains(usage, "--"+canonical) {
					t.Fatalf("canonical help does not contain --%s: %q", canonical, usage)
				}
				if cmd.FlagSet.Lookup(legacy) == nil {
					t.Fatalf("expected hidden compatibility --%s", legacy)
				}
				if strings.Contains(usage, "--"+legacy) {
					t.Fatalf("canonical help exposes deprecated --%s: %q", legacy, usage)
				}
			}
		})
	}
}

func TestSubscriptionRelationshipLimitsCanonicalFlagsReachClientSetup(t *testing.T) {
	isolateSubscriptionsAuthEnv(t)
	for _, test := range relationshipLimitCommandCases() {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
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
			if strings.Contains(stderr, "deprecated") {
				t.Fatalf("canonical flags emitted a deprecation warning: %q", stderr)
			}
		})
	}
}

func TestSubscriptionRelationshipLimitAliasesWarnOnce(t *testing.T) {
	isolateSubscriptionsAuthEnv(t)
	for _, test := range relationshipLimitCommandCases() {
		for index, legacy := range test.legacy {
			canonical := test.canonical[index]
			t.Run(test.name+" "+legacy, func(t *testing.T) {
				cmd := test.command()
				args := append(append([]string{}, test.baseArgs...), "--"+legacy, "7")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				stderr := captureRelationshipLimitStderr(t, func() {
					err := cmd.Exec(context.Background(), nil)
					if err == nil {
						t.Fatal("expected isolated client setup to fail")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("deprecated alias failed transition validation: %v", err)
					}
				})
				warning := "Warning: `--" + legacy + "` is deprecated. Use `--" + canonical + "`."
				if got := strings.Count(stderr, warning); got != 1 {
					t.Fatalf("warning count = %d, want 1; stderr=%q", got, stderr)
				}
			})
		}
	}
}

func TestSubscriptionRelationshipLimitAliasesConflictWithCanonicalFlags(t *testing.T) {
	for _, test := range relationshipLimitCommandCases() {
		for index, legacy := range test.legacy {
			canonical := test.canonical[index]
			t.Run(test.name+" "+legacy, func(t *testing.T) {
				cmd := test.command()
				args := append(append([]string{}, test.baseArgs...), "--"+canonical, "7", "--"+legacy, "7")
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				stderr := captureRelationshipLimitStderr(t, func() {
					err := cmd.Exec(context.Background(), nil)
					if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--"+legacy+" conflicts with --"+canonical) {
						t.Fatalf("Exec() error = %v, want alias conflict usage error", err)
					}
				})
				if got := strings.Count(stderr, "Warning: `--"+legacy+"` is deprecated."); got != 1 {
					t.Fatalf("warning count = %d, want 1; stderr=%q", got, stderr)
				}
			})
		}
	}
}

func TestSubscriptionRelationshipLimitsRejectInvalidCanonicalValues(t *testing.T) {
	for _, test := range relationshipLimitCommandCases() {
		for _, canonical := range test.canonical {
			t.Run(test.name+" "+canonical, func(t *testing.T) {
				cmd := test.command()
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

func TestSubscriptionRelationshipLimitAliasesConflictWithOpaqueNext(t *testing.T) {
	tests := []struct {
		name      string
		command   func() *ffcli.Command
		legacy    string
		canonical string
	}{
		{name: "subscription versions", command: SubscriptionsListCommand, legacy: "version-limit", canonical: "versions-limit"},
		{name: "version images", command: SubscriptionsVersionsListCommand, legacy: "image-limit", canonical: "images-limit"},
		{name: "version localizations", command: SubscriptionsVersionsListCommand, legacy: "localization-limit", canonical: "localizations-limit"},
	}
	for _, test := range tests {
		for _, spelling := range []struct {
			name         string
			flag         string
			warningCount int
		}{
			{name: "canonical", flag: test.canonical},
			{name: "deprecated alias", flag: test.legacy, warningCount: 1},
		} {
			t.Run(test.name+" "+spelling.name, func(t *testing.T) {
				cmd := test.command()
				if err := cmd.FlagSet.Parse([]string{"--next", "https://api.appstoreconnect.apple.com/v1/example?cursor=next", "--" + spelling.flag, "7"}); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				stderr := captureRelationshipLimitStderr(t, func() {
					err := cmd.Exec(context.Background(), nil)
					if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--next cannot be combined with --"+test.canonical) {
						t.Fatalf("Exec() error = %v, want canonical opaque-next conflict", err)
					}
				})
				if got := strings.Count(stderr, "Warning: `--"+test.legacy+"` is deprecated."); got != spelling.warningCount {
					t.Fatalf("warning count = %d, want %d; stderr=%q", got, spelling.warningCount, stderr)
				}
			})
		}
	}
}
