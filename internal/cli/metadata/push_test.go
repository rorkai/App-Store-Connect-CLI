package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestExecutePushPrefixesLocalMetadataReadErrors(t *testing.T) {
	for _, commandName := range []string{"push", "apply"} {
		t.Run(commandName, func(t *testing.T) {
			dir := t.TempDir()
			appInfoPath := filepath.Join(dir, appInfoDirName)
			if err := os.WriteFile(appInfoPath, []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("write app-info file: %v", err)
			}

			_, err := ExecutePush(context.Background(), PushExecutionOptions{
				CommandName: commandName,
				AppID:       "123456789",
				Version:     "1.2.3",
				Dir:         dir,
			})
			if err == nil {
				t.Fatal("expected local metadata read failure")
			}

			want := "metadata " + commandName + ": failed to read " + appInfoPath
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %q in error, got %v", want, err)
			}
		})
	}
}

func TestRepresentativeMetadataMutationErrorPrefersHTTPStatusWithinFirstAction(t *testing.T) {
	mutationErr := errors.New("connection reset")
	readbackErr := &asc.APIError{StatusCode: 503, Code: "SERVICE_UNAVAILABLE"}
	laterActionErr := &asc.APIError{StatusCode: 404, Code: "NOT_FOUND"}
	firstActionErr := fmt.Errorf("mutation and readback failed: %w", errors.Join(mutationErr, readbackErr))

	got := representativeMetadataMutationError(errors.Join(
		fmt.Errorf("update version localization fr-FR: %w", firstActionErr),
		fmt.Errorf("update version localization ja: %w", laterActionErr),
	))
	if !errors.Is(got, readbackErr) {
		t.Fatalf("representative error = %T %v, want first action HTTP cause %v", got, got, readbackErr)
	}
}

func TestLoadLocalMetadataTreatsDefaultLocaleCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "Default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "DeFaUlT.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	bundle, err := loadLocalMetadata(dir, version)
	if err != nil {
		t.Fatalf("loadLocalMetadata() error: %v", err)
	}
	if bundle.defaultAppInfo == nil {
		t.Fatal("expected default app-info localization")
	}
	if bundle.defaultVersion == nil {
		t.Fatal("expected default version localization")
	}
	if bundle.defaultAppInfo.localization.Name != "Default App Name" {
		t.Fatalf("expected default app-info name, got %q", bundle.defaultAppInfo.localization.Name)
	}
	if bundle.defaultVersion.localization.Description != "Default description" {
		t.Fatalf("expected default version description, got %q", bundle.defaultVersion.localization.Description)
	}
	if len(bundle.appInfo) != 0 {
		t.Fatalf("expected no explicit app-info locales, got %+v", bundle.appInfo)
	}
	if len(bundle.version) != 0 {
		t.Fatalf("expected no explicit version locales, got %+v", bundle.version)
	}
}

func TestLoadLocalMetadataRejectsVersionPathTraversal(t *testing.T) {
	dir := t.TempDir()

	_, err := loadLocalMetadata(dir, "../../secret")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error for invalid version, got %v", err)
	}
}

func TestBuildScopePlanTreatsMissingLocalFieldsAsNoOp(t *testing.T) {
	local := map[string]localPlanFields{
		"en-US": {
			setFields: map[string]string{
				"name": "Local Name",
			},
		},
	}
	remote := map[string]map[string]string{
		"en-US": {
			"name":     "Remote Name",
			"subtitle": "Remote subtitle",
		},
	}

	adds, updates, deletes, calls := buildScopePlan(
		appInfoDirName,
		"",
		appInfoPlanFields,
		local,
		remote,
	)

	if len(adds) != 0 {
		t.Fatalf("expected no adds, got %+v", adds)
	}
	if len(updates) != 1 {
		t.Fatalf("expected one field update, got %+v", updates)
	}
	if len(deletes) != 0 {
		t.Fatalf("expected no field deletes, got %+v", deletes)
	}
	if calls.create != 0 || calls.delete != 0 || calls.update != 1 {
		t.Fatalf("unexpected call counts: %+v", calls)
	}
}

func TestApplyAppInfoChangesIgnoresRemoteOnlyEmptyLocalization(t *testing.T) {
	remote := []asc.Resource[asc.AppInfoLocalizationAttributes]{
		{
			ID: "loc-empty",
			Attributes: asc.AppInfoLocalizationAttributes{
				Locale: "en-US",
			},
		},
	}
	for _, allowDeletes := range []bool{false, true} {
		actions, err := applyAppInfoChanges(context.Background(), nil, "appinfo-1", map[string]appInfoLocalPatch{}, remote, allowDeletes)
		if err != nil || len(actions) != 0 {
			t.Fatalf("allowDeletes=%t: expected empty remote locale no-op, actions=%+v err=%v", allowDeletes, actions, err)
		}
	}
}

func TestApplyDefaultFallbackSkipsRemoteLocalesWhenDeletesAllowed(t *testing.T) {
	defaultAppInfo := appInfoLocalPatch{
		localization: AppInfoLocalization{Name: "Default Name"},
		setFields: map[string]string{
			"name": "Default Name",
		},
	}
	appInfo := applyDefaultAppInfoFallback(
		map[string]appInfoLocalPatch{},
		&defaultAppInfo,
		map[string]AppInfoLocalization{
			"fr": {Name: "Remote Name"},
		},
		true,
	)
	if len(appInfo) != 0 {
		t.Fatalf("expected no app-info fallback locales when deletes are allowed, got %+v", appInfo)
	}

	defaultVersion := versionLocalPatch{
		localization: VersionLocalization{Description: "Default Description"},
		setFields: map[string]string{
			"description": "Default Description",
		},
	}
	version := applyDefaultVersionFallback(
		map[string]versionLocalPatch{},
		&defaultVersion,
		map[string]VersionLocalization{
			"fr": {Description: "Remote Description"},
		},
		true,
	)
	if len(version) != 0 {
		t.Fatalf("expected no version fallback locales when deletes are allowed, got %+v", version)
	}
}

func TestReadAppInfoLocalizationPatchTracksExplicitFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en-US.json")
	if err := os.WriteFile(path, []byte(`{"name":"New Name","subtitle":"New Subtitle"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	patch, err := readAppInfoLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readAppInfoLocalizationPatchFromFile() error: %v", err)
	}
	if patch.localization.Name != "New Name" {
		t.Fatalf("expected name set, got %+v", patch.localization)
	}
	if patch.localization.Subtitle != "New Subtitle" {
		t.Fatalf("expected subtitle set, got %+v", patch.localization)
	}
}

func TestReadAppInfoLocalizationPatchAcceptsCaseInsensitiveKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en-US.json")
	if err := os.WriteFile(path, []byte(`{"Name":"New Name","SubTitle":"New Subtitle"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	patch, err := readAppInfoLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readAppInfoLocalizationPatchFromFile() error: %v", err)
	}
	if patch.localization.Name != "New Name" {
		t.Fatalf("expected name set, got %+v", patch.localization)
	}
	if patch.localization.Subtitle != "New Subtitle" {
		t.Fatalf("expected subtitle set, got %+v", patch.localization)
	}
	if patch.setFields["name"] != "New Name" {
		t.Fatalf("expected canonical field key name in setFields, got %+v", patch.setFields)
	}
	if patch.setFields["subtitle"] != "New Subtitle" {
		t.Fatalf("expected canonical field key subtitle in setFields, got %+v", patch.setFields)
	}
}

func TestReadAppInfoLocalizationPatchRejectsLegacyClearToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en-US.json")
	if err := os.WriteFile(path, []byte(`{"name":"New Name","subtitle":"__ASC_DELETE__"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := readAppInfoLocalizationPatchFromFile(path)
	if err == nil {
		t.Fatal("expected error for legacy clear token")
	}
	if !strings.Contains(err.Error(), "unsupported clear token __ASC_DELETE__") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadVersionLocalizationPatchAcceptsCaseInsensitiveKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en-US.json")
	if err := os.WriteFile(path, []byte(`{"Description":"New Description","Whatsnew":"New What's New"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	patch, err := readVersionLocalizationPatchFromFile(path)
	if err != nil {
		t.Fatalf("readVersionLocalizationPatchFromFile() error: %v", err)
	}
	if patch.localization.Description != "New Description" {
		t.Fatalf("expected description set, got %+v", patch.localization)
	}
	if patch.localization.WhatsNew != "New What's New" {
		t.Fatalf("expected whatsNew set, got %+v", patch.localization)
	}
	if patch.setFields["description"] != "New Description" {
		t.Fatalf("expected canonical field key description in setFields, got %+v", patch.setFields)
	}
	if patch.setFields["whatsNew"] != "New What's New" {
		t.Fatalf("expected canonical field key whatsNew in setFields, got %+v", patch.setFields)
	}
}

func TestReadVersionLocalizationPatchRejectsOverLimitKeywordCharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ja.json")
	body := `{"keywords":"` + strings.Repeat("語", 101) + `"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := readVersionLocalizationPatchFromFile(path)
	if err == nil {
		t.Fatal("expected keyword limit error")
	}
	if !strings.Contains(err.Error(), "keywords exceed 100 characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}
