package builds

import (
	"errors"
	"flag"
	"testing"
)

func TestDsymsCommandShape(t *testing.T) {
	cmd := BuildsDsymsCommand()
	if cmd == nil {
		t.Fatal("expected dsyms command")
	}
	if cmd.Name != "dsyms" {
		t.Errorf("expected name dsyms, got %s", cmd.Name)
	}

	flagNames := []string{"build", "app", "version", "build-number", "platform", "latest", "output-dir", "output"}
	for _, name := range flagNames {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Errorf("expected flag --%s to be registered", name)
		}
	}
}

func TestDsymsRequiresBuildOrApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := BuildsDsymsCommand()
	err := cmd.Exec(t.Context(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
	}
}

func TestFilterBundlesWithDSYM(t *testing.T) {
	s := func(v string) *string { return &v }

	tests := []struct {
		name     string
		bundles  []dsymBundleInfo
		wantURLs int
	}{
		{
			name:     "no bundles",
			bundles:  nil,
			wantURLs: 0,
		},
		{
			name: "bundle with dsym url",
			bundles: []dsymBundleInfo{
				{BundleID: "com.example.app", DSYMURL: s("https://example.com/dsym.zip")},
			},
			wantURLs: 1,
		},
		{
			name: "bundle without dsym url",
			bundles: []dsymBundleInfo{
				{BundleID: "com.example.app", DSYMURL: nil},
			},
			wantURLs: 0,
		},
		{
			name: "mixed bundles",
			bundles: []dsymBundleInfo{
				{BundleID: "com.example.app", DSYMURL: s("https://example.com/app.zip")},
				{BundleID: "com.example.clip", DSYMURL: nil},
				{BundleID: "com.example.ext", DSYMURL: s("https://example.com/ext.zip")},
			},
			wantURLs: 2,
		},
		{
			name: "empty dsym url string",
			bundles: []dsymBundleInfo{
				{BundleID: "com.example.app", DSYMURL: s("")},
			},
			wantURLs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBundlesWithDSYM(tt.bundles)
			if len(got) != tt.wantURLs {
				t.Errorf("got %d bundles with dsym URLs, want %d", len(got), tt.wantURLs)
			}
		})
	}
}

func TestDsymFileName(t *testing.T) {
	tests := []struct {
		name         string
		bundleID     string
		appVersion   string
		buildVersion string
		buildID      string
		index        int
		want         string
	}{
		{
			name:         "full info",
			bundleID:     "com.example.app",
			appVersion:   "1.2.3",
			buildVersion: "42",
			buildID:      "build-1",
			want:         "com.example.app-1.2.3-42.dSYM.zip",
		},
		{
			name:         "bundle id with build version",
			bundleID:     "com.example.app",
			buildVersion: "42",
			buildID:      "build-1",
			want:         "com.example.app-42.dSYM.zip",
		},
		{
			name:     "bundle id only fallback to build id",
			bundleID: "com.example.app",
			buildID:  "build-1",
			want:     "com.example.app-build-1.dSYM.zip",
		},
		{
			name:    "fallback to build id",
			buildID: "build-1",
			index:   0,
			want:    "build-1_0.dSYM.zip",
		},
		{
			name:    "fallback with index",
			buildID: "build-1",
			index:   2,
			want:    "build-1_2.dSYM.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsymFileName(tt.bundleID, tt.appVersion, tt.buildVersion, tt.buildID, tt.index)
			if got != tt.want {
				t.Errorf("dsymFileName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveBuildOptions_RequiresInput(t *testing.T) {
	_, err := ResolveBuild(t.Context(), nil, ResolveBuildOptions{})
	if err == nil {
		t.Fatal("expected error for empty options")
	}
}

func TestResolveBuildOptions_RejectsConflictingSelectors(t *testing.T) {
	t.Run("latest + build-number", func(t *testing.T) {
		_, err := ResolveBuild(t.Context(), nil, ResolveBuildOptions{
			AppID:       "app-1",
			Latest:      true,
			BuildNumber: "42",
		})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp (usage error), got: %v", err)
		}
	})

	t.Run("build + app selectors", func(t *testing.T) {
		_, err := ResolveBuild(t.Context(), nil, ResolveBuildOptions{
			BuildID: "build-1",
			AppID:   "app-1",
			Latest:  true,
		})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp (usage error), got: %v", err)
		}
	})
}

func TestValidateResolveBuildOptions_BuildIgnoresDefaultAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "default-app")

	err := validateResolveBuildOptions(ResolveBuildOptions{
		BuildID: "build-1",
	})
	if err != nil {
		t.Fatalf("expected direct --build selection to ignore ASC_APP_ID fallback, got %v", err)
	}
}
