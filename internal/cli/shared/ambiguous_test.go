package shared

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAmbiguousErrorListsCandidatesAndFlag(t *testing.T) {
	err := AmbiguousError("app", "--app", "Outslept", []AmbiguousCandidate{
		{ID: "6759231657", Label: "Outslept", Extra: "com.rudrank.outslept"},
		{ID: "12", Label: "Outslept Lite"},
	})
	want := strings.Join([]string{
		`2 apps match "Outslept"; pass --app with one of:`,
		"  6759231657  Outslept       com.rudrank.outslept",
		"  12          Outslept Lite",
	}, "\n")
	if got := err.Error(); got != want {
		t.Fatalf("unexpected message:\n got: %q\nwant: %q", got, want)
	}

	var ambiguous *AmbiguousSelectionError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected *AmbiguousSelectionError, got %T", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("plain ambiguous error must not be a usage error")
	}
}

func TestAmbiguousErrorPluralizesKinds(t *testing.T) {
	cases := map[string]string{
		"app store version": "app store versions",
		"app info":          "app infos",
		"beta group":        "beta groups",
		"build":             "builds",
		"App Clip":          "App Clips",
	}
	for kind, want := range cases {
		msg := AmbiguousError(kind, "--x", "v", []AmbiguousCandidate{{ID: "1"}, {ID: "2"}}).Error()
		if !strings.HasPrefix(msg, "2 "+want+" match") {
			t.Errorf("kind %q: got %q, want prefix %q", kind, msg, "2 "+want+" match")
		}
	}
}

func TestAmbiguousErrorBoundsCandidateList(t *testing.T) {
	candidates := make([]AmbiguousCandidate, 0, 14)
	for i := 0; i < 14; i++ {
		candidates = append(candidates, AmbiguousCandidate{ID: fmt.Sprintf("id-%02d", i)})
	}
	msg := AmbiguousError("build", "--build-id", "42", candidates).Error()
	lines := strings.Split(msg, "\n")
	if len(lines) != 1+AmbiguousCandidateLimit+1 {
		t.Fatalf("expected header, %d candidates, and a summary line; got %d lines:\n%s", AmbiguousCandidateLimit, len(lines), msg)
	}
	if !strings.HasPrefix(lines[0], `14 builds match "42"`) {
		t.Fatalf("header must report the full count, got %q", lines[0])
	}
	if lines[len(lines)-1] != "  ... and 4 more" {
		t.Fatalf("unexpected summary line %q", lines[len(lines)-1])
	}
	if strings.Contains(msg, "id-10") {
		t.Fatalf("candidate beyond the limit must not be listed:\n%s", msg)
	}
}

func TestAmbiguousErrorWithoutFlagAndWithHint(t *testing.T) {
	err := &AmbiguousSelectionError{
		Kind:        "version localization",
		Description: `locale "en-US"`,
		Candidates:  []AmbiguousCandidate{{ID: "loc-1"}, {ID: "loc-2"}},
		Hint:        "Inspect them with `asc localizations list`.",
	}
	want := strings.Join([]string{
		`2 version localizations match locale "en-US":`,
		"  loc-1",
		"  loc-2",
		"Inspect them with `asc localizations list`.",
	}, "\n")
	if got := err.Error(); got != want {
		t.Fatalf("unexpected message:\n got: %q\nwant: %q", got, want)
	}
}

func TestAmbiguousErrorSanitizesCandidateText(t *testing.T) {
	err := AmbiguousError("app", "--app", "x", []AmbiguousCandidate{
		{ID: "1", Label: "bad\nname\x1b[31m"},
		{ID: "2", Label: "ok"},
	})
	msg := err.Error()
	if strings.Contains(msg, "\x1b") || strings.Count(msg, "\n") != 2 {
		t.Fatalf("candidate text must be sanitized:\n%q", msg)
	}
}

func TestAmbiguousUsageErrorPrintsEveryLineAndKeepsUsageExit(t *testing.T) {
	stderr := captureStderr(t, func() {
		err := AmbiguousUsageError(AmbiguousError("app store version", "--platform", "1.2.3", []AmbiguousCandidate{
			{ID: "v-ios", Label: "IOS"},
			{ID: "v-mac", Label: "MAC_OS"},
		}))
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage-class error, got %T %v", err, err)
		}
		if !strings.Contains(err.Error(), "v-ios") || !strings.Contains(err.Error(), "v-mac") {
			t.Fatalf("usage-class error must keep every candidate, got %q", err.Error())
		}
		if ClassifyUsageError(err) != UsageErrorOther {
			t.Fatalf("unexpected usage kind %q", ClassifyUsageError(err))
		}
	})
	want := "Error: 2 app store versions match \"1.2.3\"; pass --platform with one of:\n  v-ios  IOS\n  v-mac  MAC_OS\n"
	if stderr != want {
		t.Fatalf("unexpected stderr:\n got: %q\nwant: %q", stderr, want)
	}
}

func TestIsAmbiguousSelectionDetectsWrappedErrors(t *testing.T) {
	ambiguous := AmbiguousError("app", "--app", "x", []AmbiguousCandidate{{ID: "1"}, {ID: "2"}})
	if !IsAmbiguousSelection(ambiguous) {
		t.Fatal("expected a bare ambiguous error to be detected")
	}
	if !IsAmbiguousSelection(fmt.Errorf("lookup failed: %w", ambiguous)) {
		t.Fatal("expected a wrapped ambiguous error to be detected")
	}
	if IsAmbiguousSelection(errors.New("boom")) {
		t.Fatal("unrelated errors must not be reported as ambiguous")
	}
	if IsAmbiguousSelection(nil) {
		t.Fatal("nil must not be reported as ambiguous")
	}
}

func TestAmbiguousAppStoreVersionErrorOffersPlatformOnlyWhenItSelectsOne(t *testing.T) {
	version := func(id, platform string) asc.Resource[asc.AppStoreVersionAttributes] {
		return asc.Resource[asc.AppStoreVersionAttributes]{
			ID:         id,
			Attributes: asc.AppStoreVersionAttributes{VersionString: "1.2.3", Platform: asc.Platform(platform)},
		}
	}

	unique := AmbiguousAppStoreVersionError("1.2.3", "", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios", "IOS"),
		version("version-mac", "MAC_OS"),
	}, "--platform", "").Error()
	if !strings.Contains(unique, "pass --platform with one of:") || !strings.Contains(unique, "IOS") {
		t.Fatalf("expected platform candidates, got %q", unique)
	}

	duplicated := AmbiguousAppStoreVersionError("1.2.3", "", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios-1", "IOS"),
		version("version-ios-2", "IOS"),
		version("version-mac", "MAC_OS"),
	}, "--platform", "").Error()
	if strings.Contains(duplicated, "pass --platform with one of:") {
		t.Fatalf("--platform must not be offered when a platform matches several versions: %q", duplicated)
	}
	for _, want := range []string{"version-ios-1", "version-ios-2", "version-mac", "cannot select one on its own"} {
		if !strings.Contains(duplicated, want) {
			t.Fatalf("expected %q in %q", want, duplicated)
		}
	}

	withVersionFlag := AmbiguousAppStoreVersionError("1.2.3", "", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios-1", "IOS"),
		version("version-ios-2", "IOS"),
		version("version-mac", "MAC_OS"),
	}, "--platform", "--version-id").Error()
	if !strings.Contains(withVersionFlag, "pass --version-id with one of:") {
		t.Fatalf("expected --version-id fallback, got %q", withVersionFlag)
	}
}

func TestIsAmbiguousSelectionDetectsStableSelectorAmbiguity(t *testing.T) {
	err := selectorAmbiguousError{
		resourceName: "subscription",
		fieldName:    "id",
		selector:     "42",
		matches: []ExactSelectorCandidate{
			{ID: "sub-1", ProductID: "pro.monthly"},
			{ID: "sub-2", ProductID: "pro.yearly"},
		},
	}
	if !IsAmbiguousSelection(err) {
		t.Fatal("stable selector ambiguity must be detected as an ambiguous selection")
	}
	var ambiguous *AmbiguousSelectionError
	if !errors.As(error(err), &ambiguous) || len(ambiguous.Candidates) != 2 {
		t.Fatalf("expected the candidates to be reachable, got %#v", ambiguous)
	}
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatal("unwrapping must not break the existing sentinel")
	}
}
