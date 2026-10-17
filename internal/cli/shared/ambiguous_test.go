package shared

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestAmbiguousErrorSampleWithoutFlagLabelsCandidatesAsSample(t *testing.T) {
	err := MarkAmbiguousSelectionSample(AmbiguousError("source app store version", "", `version "1.2.3"`, []AmbiguousCandidate{{ID: "version-1"}}))
	if !strings.Contains(err.Error(), "; these are sample matches:") {
		t.Fatalf("expected sample wording without a disambiguating flag, got %q", err)
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

func TestAmbiguousErrorBoundsProviderTextAndPreservesRecoveryID(t *testing.T) {
	recoveryID := "iap-recovery-id-" + strings.Repeat("9", AmbiguousDiagnosticTextLimit+32)
	providerText := strings.Repeat("界", AmbiguousDiagnosticTextLimit)
	err := &AmbiguousSelectionError{
		Kind:             "in-app purchase",
		Description:      providerText + "\nselector-tail",
		Flag:             "--iap-id",
		DisplayTextLimit: AmbiguousDiagnosticTextLimit,
		Candidates: []AmbiguousCandidate{{
			ID:    recoveryID,
			Label: providerText + "\x1b[31m-label-tail",
			Extra: providerText + "\u202e-extra-tail",
		}},
		Hint: providerText + "\x00-hint-tail",
	}

	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity message must remain valid UTF-8: %q", message)
	}
	if strings.Contains(message, "selector-tail") || strings.Contains(message, "-label-tail") || strings.Contains(message, "-extra-tail") || strings.Contains(message, "-hint-tail") {
		t.Fatalf("provider text beyond the field bound must not be rendered: %q", message)
	}
	if strings.Contains(message, recoveryID) {
		t.Fatalf("displayed recovery ID should be bounded: %q", message)
	}
	if !strings.Contains(message, recoveryID[:len("iap-recovery-id-")]) {
		t.Fatalf("bounded recovery ID should retain useful context: %q", message)
	}
	if strings.ContainsAny(message, "\x00\x1b\u202e") {
		t.Fatalf("provider terminal controls must not be rendered: %q", message)
	}

	for _, field := range []string{sanitizeAmbiguousText(err.Description, err.DisplayTextLimit), sanitizeAmbiguousText(err.Candidates[0].Label, err.DisplayTextLimit), sanitizeAmbiguousText(err.Candidates[0].Extra, err.DisplayTextLimit), sanitizeAmbiguousText(err.Hint, err.DisplayTextLimit)} {
		if len(field) > AmbiguousDiagnosticTextLimit {
			t.Fatalf("sanitized provider field is %d bytes, want <= %d: %q", len(field), AmbiguousDiagnosticTextLimit, field)
		}
		if !utf8.ValidString(field) {
			t.Fatalf("sanitized provider field must remain valid UTF-8: %q", field)
		}
	}
}

func TestSanitizeAmbiguousTextReplacesInvalidUTF8(t *testing.T) {
	got := sanitizeAmbiguousText(string([]byte{'o', 'k', 0xff, 0xfe, ' ', 't', 'a', 'i', 'l'}))
	if !utf8.ValidString(got) {
		t.Fatalf("sanitized text must remain valid UTF-8: %q", got)
	}
	if got != "ok�� tail" {
		t.Fatalf("invalid UTF-8 must be replaced without dropping surrounding context: %q", got)
	}
}

func TestAmbiguousUsageErrorPrintsEveryLineAndKeepsUsageExit(t *testing.T) {
	original := &AmbiguousSelectionError{
		Kind:        "app store version",
		Description: `"1.2.3"`,
		Flag:        "--platform",
		Candidates: []AmbiguousCandidate{
			{ID: "v-ios", Label: "IOS"},
			{ID: "v-mac", Label: "MAC_OS"},
		},
	}
	stderr := captureStderr(t, func() {
		err := AmbiguousUsageError(original)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage-class error, got %T %v", err, err)
		}
		var recovered *AmbiguousSelectionError
		if !errors.As(err, &recovered) || recovered != original {
			t.Fatalf("expected original ambiguity cause, got %#v", recovered)
		}
		if !IsAmbiguousSelection(err) {
			t.Fatal("usage wrapper must preserve ambiguity classification")
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

func TestAmbiguousUsageErrorWithKindPreservesRequestedClassification(t *testing.T) {
	original := AmbiguousError("app store version", "--platform", "1.2.3", []AmbiguousCandidate{{ID: "version-ios"}})
	stderr := captureStderr(t, func() {
		err := AmbiguousUsageErrorWithKind(original, UsageErrorMissingRequired)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage-class error, got %T %v", err, err)
		}
		if !IsAmbiguousSelection(err) {
			t.Fatal("usage wrapper must preserve ambiguity classification")
		}
		if ClassifyUsageError(err) != UsageErrorMissingRequired {
			t.Fatalf("unexpected usage kind %q", ClassifyUsageError(err))
		}
	})
	if !strings.HasPrefix(stderr, "Error: 1 app store versions match") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestMarkAmbiguousSelectionSample(t *testing.T) {
	err := AmbiguousError("app", "--app", "Example", []AmbiguousCandidate{{ID: "app-1"}})
	if got := MarkAmbiguousSelectionSample(err); !errors.Is(got, err) {
		t.Fatalf("expected marker to preserve error identity")
	}
	if !strings.HasPrefix(err.Error(), `multiple apps match "Example"; pass --app with one of these sample matches:`) {
		t.Fatalf("expected sample ambiguity wording, got %q", err)
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

	samePlatform := AmbiguousAppStoreVersionError("1.2.3", "", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios-1", "IOS"),
		version("version-ios-2", "IOS"),
	}, "--platform", "").Error()
	for _, want := range []string{"version-ios-1", "version-ios-2", "cannot select between duplicate App Store version records"} {
		if !strings.Contains(samePlatform, want) {
			t.Fatalf("expected %q in same-platform ambiguity %q", want, samePlatform)
		}
	}
	if strings.Contains(samePlatform, "pass --platform") {
		t.Fatalf("platform must not be offered for same-platform duplicates: %q", samePlatform)
	}

	platformAlreadySelected := AmbiguousAppStoreVersionError("1.2.3", "IOS", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios-1", "IOS"),
		version("version-ios-2", "IOS"),
	}, "--platform", "").Error()
	if !strings.Contains(platformAlreadySelected, "cannot select between duplicate App Store version records") {
		t.Fatalf("expected honest same-platform recovery hint, got %q", platformAlreadySelected)
	}

	noSelectorFlags := AmbiguousAppStoreVersionError("1.2.3", "", []asc.Resource[asc.AppStoreVersionAttributes]{
		version("version-ios-1", "IOS"),
		version("version-ios-2", "IOS"),
	}, "", "").Error()
	if !strings.Contains(noSelectorFlags, "cannot select between duplicate App Store version records") || strings.Contains(noSelectorFlags, "pass --") {
		t.Fatalf("expected honest no-selector recovery hint, got %q", noSelectorFlags)
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

func TestBetaTesterCandidatesExposeOnlyDisambiguatingIDs(t *testing.T) {
	testers := []asc.Resource[asc.BetaTesterAttributes]{
		{
			ID: "tester-1",
			Attributes: asc.BetaTesterAttributes{
				Email:     "private@example.com",
				FirstName: "Private",
				LastName:  "Person",
			},
		},
	}

	candidates := BetaTesterCandidates(testers)
	if len(candidates) != 1 || candidates[0].ID != "tester-1" {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}
	if candidates[0].Label != "" || candidates[0].Extra != "" {
		t.Fatalf("tester candidates must not repeat personal data: %#v", candidates[0])
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

func TestAmbiguousSelectionsAreValidationFailures(t *testing.T) {
	defaultVersion := fmt.Errorf("localizations list: %w", &AmbiguousDefaultAppStoreVersionError{
		AppID:      "app-1",
		Source:     "editable",
		Candidates: []DefaultAppStoreVersion{{ID: "ver-ios", Platform: "IOS"}, {ID: "ver-mac", Platform: "MAC_OS"}},
	})
	if !IsValidationError(defaultVersion) {
		t.Fatalf("IsValidationError(%v) = false, want true", defaultVersion)
	}
}
