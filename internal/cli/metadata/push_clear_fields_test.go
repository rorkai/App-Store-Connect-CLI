package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMetadataPatchFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "en-US.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestReadAppInfoLocalizationPatchClearsNullableFields(t *testing.T) {
	path := writeMetadataPatchFile(t, `{"name":"New Name","subtitle":null,"privacyPolicyUrl":null}`)

	patch, err := readAppInfoLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readAppInfoLocalizationPatchFromFile() error: %v", err)
	}
	if len(patch.setFields) != 1 || patch.setFields["name"] != "New Name" {
		t.Fatalf("setFields = %+v, want only name", patch.setFields)
	}
	for _, field := range []string{"subtitle", "privacyPolicyUrl"} {
		if _, ok := patch.clearFields[field]; !ok {
			t.Fatalf("clearFields = %+v, want %q", patch.clearFields, field)
		}
	}
	if len(patch.clearFields) != 2 {
		t.Fatalf("clearFields = %+v, want exactly two entries", patch.clearFields)
	}
}

func TestReadAppInfoLocalizationPatchAcceptsClearOnlyPatch(t *testing.T) {
	path := writeMetadataPatchFile(t, `{"subtitle":null}`)

	patch, err := readAppInfoLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readAppInfoLocalizationPatchFromFile() error: %v", err)
	}
	if len(patch.setFields) != 0 {
		t.Fatalf("setFields = %+v, want empty", patch.setFields)
	}
	if _, ok := patch.clearFields["subtitle"]; !ok {
		t.Fatalf("clearFields = %+v, want subtitle", patch.clearFields)
	}
}

func TestReadVersionLocalizationPatchClearsPromotionalText(t *testing.T) {
	path := writeMetadataPatchFile(t, `{"description":"New Description","promotionalText":null}`)

	patch, err := readVersionLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readVersionLocalizationPatchFromFile() error: %v", err)
	}
	if len(patch.setFields) != 1 || patch.setFields["description"] != "New Description" {
		t.Fatalf("setFields = %+v, want only description", patch.setFields)
	}
	if _, ok := patch.clearFields["promotionalText"]; !ok {
		t.Fatalf("clearFields = %+v, want promotionalText", patch.clearFields)
	}
}

func TestReadLocalizationPatchLeavesOmittedKeysUnchanged(t *testing.T) {
	appInfoPath := writeMetadataPatchFile(t, `{"name":"New Name"}`)
	appInfoPatch, err := readAppInfoLocalizationPatchFromFile(appInfoPath)
	if err != nil {
		t.Fatalf("readAppInfoLocalizationPatchFromFile() error: %v", err)
	}
	if _, ok := appInfoPatch.setFields["subtitle"]; ok {
		t.Fatalf("setFields = %+v, want no subtitle", appInfoPatch.setFields)
	}
	if _, ok := appInfoPatch.clearFields["subtitle"]; ok {
		t.Fatalf("clearFields = %+v, want no subtitle", appInfoPatch.clearFields)
	}

	versionPath := writeMetadataPatchFile(t, `{"description":"New Description"}`)
	versionPatch, err := readVersionLocalizationPatchFromFile(versionPath)
	if err != nil {
		t.Fatalf("readVersionLocalizationPatchFromFile() error: %v", err)
	}
	if _, ok := versionPatch.setFields["promotionalText"]; ok {
		t.Fatalf("setFields = %+v, want no promotionalText", versionPatch.setFields)
	}
	if _, ok := versionPatch.clearFields["promotionalText"]; ok {
		t.Fatalf("clearFields = %+v, want no promotionalText", versionPatch.clearFields)
	}
}

func TestReadLocalizationPatchRejectsEmptyStringForClearableFields(t *testing.T) {
	appInfoPath := writeMetadataPatchFile(t, `{"name":"New Name","subtitle":""}`)
	if _, err := readAppInfoLocalizationPatchFromFile(appInfoPath); err == nil {
		t.Fatal("expected empty-string rejection for subtitle")
	} else if !strings.Contains(err.Error(), `field "subtitle" cannot be empty`) {
		t.Fatalf("unexpected error: %v", err)
	}

	versionPath := writeMetadataPatchFile(t, `{"promotionalText":""}`)
	if _, err := readVersionLocalizationPatchFromFile(versionPath); err == nil {
		t.Fatal("expected empty-string rejection for promotionalText")
	} else if !strings.Contains(err.Error(), `field "promotionalText" cannot be empty`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadLocalizationPatchRejectsNullForFieldsWithoutClearSupport(t *testing.T) {
	appInfoPath := writeMetadataPatchFile(t, `{"name":null}`)
	if _, err := readAppInfoLocalizationPatchFromFile(appInfoPath); err == nil {
		t.Fatal("expected null rejection for name")
	} else if !strings.Contains(err.Error(), `field "name" cannot be null`) {
		t.Fatalf("unexpected error: %v", err)
	}

	versionPath := writeMetadataPatchFile(t, `{"description":null}`)
	if _, err := readVersionLocalizationPatchFromFile(versionPath); err == nil {
		t.Fatal("expected null rejection for description")
	} else if !strings.Contains(err.Error(), `field "description" cannot be null`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
