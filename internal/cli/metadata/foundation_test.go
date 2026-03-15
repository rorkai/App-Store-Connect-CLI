package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDecodeAppInfoLocalizationRejectsUnknownKeys(t *testing.T) {
	_, err := DecodeAppInfoLocalization([]byte(`{"name":"App Name","unknown":"x"}`))
	if err == nil {
		t.Fatal("expected unknown-key error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestDecodeVersionLocalizationRejectsTrailingJSON(t *testing.T) {
	_, err := DecodeVersionLocalization([]byte(`{"description":"Hello"}{"description":"Again"}`))
	if err == nil {
		t.Fatal("expected trailing data error")
	}
	if !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("expected trailing data error, got %v", err)
	}
}

func TestEncodeVersionLocalizationDeterministicJSON(t *testing.T) {
	got, err := EncodeVersionLocalization(VersionLocalization{
		Description: " Desc ",
		Keywords:    " one,two ",
	})
	if err != nil {
		t.Fatalf("EncodeVersionLocalization() error: %v", err)
	}
	want := `{"description":"Desc","keywords":"one,two"}`
	if string(got) != want {
		t.Fatalf("expected %q, got %q", want, string(got))
	}
}

func TestEncodeAppInfoLocalizationDoesNotEscapeHTML(t *testing.T) {
	got, err := EncodeAppInfoLocalization(AppInfoLocalization{
		Name:             "App",
		PrivacyPolicyURL: "https://example.com/privacy?a=1&b=2",
	})
	if err != nil {
		t.Fatalf("EncodeAppInfoLocalization() error: %v", err)
	}
	want := `{"name":"App","privacyPolicyUrl":"https://example.com/privacy?a=1&b=2"}`
	if string(got) != want {
		t.Fatalf("expected %q, got %q", want, string(got))
	}
}

func TestEncodeVersionLocalizationDoesNotEscapeHTML(t *testing.T) {
	got, err := EncodeVersionLocalization(VersionLocalization{
		Description: "A & B",
		SupportURL:  "https://example.com/support?a=1&b=2",
	})
	if err != nil {
		t.Fatalf("EncodeVersionLocalization() error: %v", err)
	}
	want := `{"description":"A & B","supportUrl":"https://example.com/support?a=1&b=2"}`
	if string(got) != want {
		t.Fatalf("expected %q, got %q", want, string(got))
	}
}

func TestBuildWritePlansSortsPathsDeterministically(t *testing.T) {
	plans, err := BuildWritePlans(
		"/tmp/metadata",
		map[string]AppInfoLocalization{
			"ja":    {Name: "App JP"},
			"en-US": {Name: "App EN"},
		},
		map[string]map[string]VersionLocalization{
			"2.0.0": {
				"ja":    {Description: "D2 ja"},
				"en-US": {Description: "D2 en"},
			},
			"1.0.0": {
				"en-US": {Description: "D1 en"},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildWritePlans() error: %v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("expected 5 plans, got %d", len(plans))
	}
	paths := make([]string, 0, len(plans))
	for _, plan := range plans {
		paths = append(paths, plan.Path)
	}
	sorted := append([]string(nil), paths...)
	slices.Sort(sorted)
	if !slices.Equal(paths, sorted) {
		t.Fatalf("expected deterministic sorted paths, got %v", paths)
	}
}

func TestLocalizationFilePathRejectsTraversal(t *testing.T) {
	_, err := AppInfoLocalizationFilePath("/tmp/metadata", "../en-US")
	if err == nil {
		t.Fatal("expected traversal error for locale")
	}

	_, err = VersionLocalizationFilePath("/tmp/metadata", "../1.0.0", "en-US")
	if err == nil {
		t.Fatal("expected traversal error for version")
	}
}

func TestApplyWritePlansRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"name":"target"}`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	link := filepath.Join(dir, "en-US.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	err := ApplyWritePlans([]WritePlan{
		{
			Path:     link,
			Contents: []byte(`{"name":"app"}`),
		},
	})
	if err == nil {
		t.Fatal("expected symlink write rejection")
	}
}

func TestReadAppInfoLocalizationFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"name":"target"}`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	link := filepath.Join(dir, "en-US.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := ReadAppInfoLocalizationFile(link)
	if err == nil {
		t.Fatal("expected symlink read rejection")
	}
}

func TestValidateAppInfoLocalizationRequireName(t *testing.T) {
	issues := ValidateAppInfoLocalization(AppInfoLocalization{Subtitle: "Only subtitle"}, ValidationOptions{
		RequireName: true,
	})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Field != "name" {
		t.Fatalf("expected name issue, got %q", issues[0].Field)
	}
}

func TestValidateVersionLocalizationRequiresAtLeastOneField(t *testing.T) {
	issues := ValidateVersionLocalization(VersionLocalization{})
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %d", len(issues))
	}
	if issues[0].Field != "metadata" {
		t.Fatalf("expected metadata issue, got %q", issues[0].Field)
	}
}

func TestValidateLocaleCanonicalizesKnownLocale(t *testing.T) {
	got, err := validateLocale("EN-us")
	if err != nil {
		t.Fatalf("validateLocale() error: %v", err)
	}
	if got != "en-US" {
		t.Fatalf("expected en-US, got %q", got)
	}
}

func TestValidateLocaleCanonicalizesCommonAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "en_US", want: "en-US"},
		{input: "de", want: "de-DE"},
		{input: "english-us", want: "en-US"},
		{input: "czech-cs", want: "cs"},
		{input: "zh_TW", want: "zh-Hant"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := validateLocale(test.input)
			if err != nil {
				t.Fatalf("validateLocale() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestValidateLocaleRejectsAmbiguousLanguageWithSuggestions(t *testing.T) {
	_, err := validateLocale("english")
	if err == nil {
		t.Fatal("expected ambiguous locale error")
	}
	for _, want := range []string{`ambiguous locale "english"`, "en-AU", "en-CA", "en-GB", "en-US"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func TestValidateLocaleRejectsUnsupportedLocale(t *testing.T) {
	_, err := validateLocale("en-ZZ")
	if err == nil {
		t.Fatal("expected unsupported locale error")
	}
	for _, want := range []string{`unsupported locale "en-ZZ"`, "use one of", "en-AU", "en-US"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}
