package cmdtest

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestProductPagesUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "CustomPagesListRequiresApp",
			args:    []string{"product-pages", "custom-pages", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "CustomPagesDeleteRequiresConfirm",
			args:    []string{"product-pages", "custom-pages", "delete", "--custom-page-id", "page-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "ExperimentsCreateRequiresVersionID",
			args:    []string{"product-pages", "experiments", "create", "--name", "Icon Test", "--traffic-proportion", "50"},
			wantErr: "--version-id is required",
		},
		{
			name:    "ExperimentsCreateRequiresAppForV2",
			args:    []string{"product-pages", "experiments", "create", "--v2", "--platform", "IOS", "--name", "Icon Test", "--traffic-proportion", "50"},
			wantErr: "--app is required",
		},
		{
			name:    "ExperimentsDeleteRequiresConfirm",
			args:    []string{"product-pages", "experiments", "delete", "--experiment-id", "exp-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "CustomPagesLocalizationsSearchKeywordsListRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "search-keywords", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsSearchKeywordsAddRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--keywords", "kw-1"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsSearchKeywordsAddRequiresKeywords",
			args:    []string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--localization-id", "loc-1"},
			wantErr: "--keywords is required",
		},
		{
			name:    "CustomPagesLocalizationsSearchKeywordsDeleteRequiresConfirm",
			args:    []string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--keywords", "kw-1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "CustomPagesLocalizationsSearchKeywordsDeleteRequiresKeywords",
			args:    []string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--confirm"},
			wantErr: "--keywords is required",
		},
		{
			name:    "CustomPagesLocalizationsPreviewSetsListRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsScreenshotSetsListRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "ExperimentsTreatmentsLocalizationsPreviewSetsListRequiresLocalizationID",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "ExperimentsTreatmentsLocalizationsScreenshotSetsListRequiresLocalizationID",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsPreviewSetsUploadRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "upload", "--path", "./previews", "--device-type", "IPHONE_65"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsPreviewSetsSyncRequiresConfirm",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "sync", "--localization-id", "loc-1", "--path", "./previews", "--device-type", "IPHONE_65"},
			wantErr: "--confirm is required to sync",
		},
		{
			name:    "CustomPagesLocalizationsScreenshotSetsUploadRequiresLocalizationID",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload", "--path", "./screenshots", "--device-type", "IPHONE_65"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "CustomPagesLocalizationsScreenshotSetsSyncRequiresConfirm",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "sync", "--localization-id", "loc-1", "--path", "./screenshots", "--device-type", "IPHONE_65"},
			wantErr: "--confirm is required to sync",
		},
	})
}

func TestProductPagesListPaginationUsageErrors(t *testing.T) {
	runExactUsageExitCases(t, []usageExitCase{
		{
			name:    "custom-pages list limit -1",
			args:    []string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--limit", "-1"},
			wantErr: "custom-pages list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages list limit 201",
			args:    []string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--limit", "201"},
			wantErr: "custom-pages list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages list invalid next",
			args:    []string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--next", "not-a-url"},
			wantErr: "custom-pages list: --next must be an App Store Connect URL",
		},
		{
			name:    "custom-pages localizations preview-sets list limit -1",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--limit", "-1"},
			wantErr: "custom-pages localizations preview-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages localizations preview-sets list limit 201",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--limit", "201"},
			wantErr: "custom-pages localizations preview-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages localizations preview-sets list invalid next",
			args:    []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"},
			wantErr: "custom-pages localizations preview-sets list: --next must be an App Store Connect URL",
		},
		{
			name:    "custom-pages localizations screenshot-sets list limit -1",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--limit", "-1"},
			wantErr: "custom-pages localizations screenshot-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages localizations screenshot-sets list limit 201",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--limit", "201"},
			wantErr: "custom-pages localizations screenshot-sets list: --limit must be between 1 and 200",
		},
		{
			name:    "custom-pages localizations screenshot-sets list invalid next",
			args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"},
			wantErr: "custom-pages localizations screenshot-sets list: --next must be an App Store Connect URL",
		},
	})
}

func TestProductPagesScreenshotSetIncludeScreenshotsRequiresFullLocalizationList(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets?cursor=next"

	cases := []struct {
		name           string
		path           []string
		localizationID string
		next           string
		wantStderr     string
	}{
		{
			name:       "custom pages requires localization id for expansion",
			path:       []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			wantStderr: "Error: --localization-id is required",
		},
		{
			name:           "custom pages requires paginate",
			path:           []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			wantStderr:     "custom-pages localizations screenshot-sets list: --include-screenshots requires --paginate",
		},
		{
			name:           "custom pages rejects next",
			path:           []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			next:           nextURL,
			wantStderr:     "custom-pages localizations screenshot-sets list: --include-screenshots cannot be combined with --next",
		},
		{
			name:       "treatment requires localization id for expansion",
			path:       []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			wantStderr: "Error: --localization-id is required",
		},
		{
			name:           "treatment requires paginate",
			path:           []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			wantStderr:     "experiments treatments localizations screenshot-sets list: --include-screenshots requires --paginate",
		},
		{
			name:           "treatment rejects next",
			path:           []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			next:           nextURL,
			wantStderr:     "experiments treatments localizations screenshot-sets list: --include-screenshots cannot be combined with --next",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, test.path...), "--include-screenshots")
			if test.localizationID != "" {
				args = append(args, "--localization-id", test.localizationID)
			}
			if test.next != "" {
				args = append(args, "--paginate", "--next", test.next)
			}

			assertUsageExit(t, args, test.wantStderr)
		})
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsUploadRejectsInvalidDeviceType(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload",
			"--localization-id", "loc-1",
			"--path", "./screenshots",
			"--device-type", "not-a-device",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsUploadRejectsInvalidDeviceType(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "custom-pages", "localizations", "preview-sets", "upload",
			"--localization-id", "loc-1",
			"--path", "./previews",
			"--device-type", "not-a-device",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
