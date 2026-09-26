package shared

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestResourceIDFromValue(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		resourceType string
		want         string
		wantErr      string
	}{
		{name: "empty", value: "", resourceType: "builds", want: ""},
		{name: "bare id", value: " 2f1a3c4d ", resourceType: "builds", want: "2f1a3c4d"},
		{name: "bundle identifier passes through", value: "com.example.app", resourceType: "bundleIds", want: "com.example.app"},
		{name: "app name passes through", value: "My App: Beta", resourceType: "apps", want: "My App: Beta"},
		{
			name:         "self link",
			value:        "https://api.appstoreconnect.apple.com/v1/builds/2f1a3c4d",
			resourceType: "builds",
			want:         "2f1a3c4d",
		},
		{
			name:         "self link with query",
			value:        "https://api.appstoreconnect.apple.com/v1/builds/2f1a3c4d?include=app",
			resourceType: "builds",
			want:         "2f1a3c4d",
		},
		{
			name:         "self link any type",
			value:        "https://api.appstoreconnect.apple.com/v2/appStoreVersions/abc",
			resourceType: "",
			want:         "abc",
		},
		{
			name:         "wrong type",
			value:        "https://api.appstoreconnect.apple.com/v1/appStoreVersions/abc",
			resourceType: "builds",
			wantErr:      "expected a self-link of type builds, got appStoreVersions",
		},
		{
			name:         "relationship path",
			value:        "https://api.appstoreconnect.apple.com/v1/builds/abc/relationships/betaGroups",
			resourceType: "builds",
			wantErr:      "/v1/<type>/<id>",
		},
		{
			name:         "related path",
			value:        "https://api.appstoreconnect.apple.com/v1/builds/abc/app",
			resourceType: "builds",
			wantErr:      "/v1/<type>/<id>",
		},
		{
			name:         "collection path",
			value:        "https://api.appstoreconnect.apple.com/v1/builds",
			resourceType: "builds",
			wantErr:      "/v1/<type>/<id>",
		},
		{
			name:         "missing version prefix",
			value:        "https://api.appstoreconnect.apple.com/builds/abc",
			resourceType: "builds",
			wantErr:      "/v1/<type>/<id>",
		},
		{
			name:         "other host",
			value:        "https://appstoreconnect.apple.com/v1/builds/abc",
			resourceType: "builds",
			wantErr:      "api.appstoreconnect.apple.com",
		},
		{
			name:         "empty id segment",
			value:        "https://api.appstoreconnect.apple.com/v1/builds/",
			resourceType: "builds",
			wantErr:      "/v1/<type>/<id>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResourceIDFromValue(test.value, test.resourceType)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got value %q", test.wantErr, got)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestBindResourceIDFlag(t *testing.T) {
	t.Run("bare id and self link", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		buildID := BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
		if *buildID != "" {
			t.Fatalf("default = %q, want empty", *buildID)
		}
		if err := fs.Parse([]string{"--build-id", "https://api.appstoreconnect.apple.com/v1/builds/abc"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if *buildID != "abc" {
			t.Fatalf("value = %q, want abc", *buildID)
		}
		if got := fs.Lookup("build-id").Value.String(); got != "abc" {
			t.Fatalf("String() = %q, want abc", got)
		}
		if got := fs.Lookup("build-id").DefValue; got != "" {
			t.Fatalf("DefValue = %q, want empty", got)
		}
		getter, ok := fs.Lookup("build-id").Value.(flag.Getter)
		if !ok {
			t.Fatal("flag value does not implement flag.Getter")
		}
		if got := getter.Get(); got != "abc" {
			t.Fatalf("Get() = %v, want abc", got)
		}
	})

	t.Run("wrong type is a parse error", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
		err := fs.Parse([]string{"--build-id", "https://api.appstoreconnect.apple.com/v1/apps/123"})
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "expected a self-link of type builds, got apps") {
			t.Fatalf("error = %q", err.Error())
		}
	})
}

func TestResolveAppIDAcceptsAppSelfLink(t *testing.T) {
	t.Setenv("ASC_APP_ID", "https://api.appstoreconnect.apple.com/v1/apps/env-app")
	if got := ResolveAppID(""); got != "env-app" {
		t.Fatalf("env app id = %q, want env-app", got)
	}
	if got := ResolveAppID("https://api.appstoreconnect.apple.com/v1/apps/flag-app"); got != "flag-app" {
		t.Fatalf("flag app id = %q, want flag-app", got)
	}
	if got := ResolveAppID("com.example.app"); got != "com.example.app" {
		t.Fatalf("bundle id = %q, want passthrough", got)
	}
	wrong := "https://api.appstoreconnect.apple.com/v1/builds/abc"
	if got := ResolveAppID(wrong); got != wrong {
		t.Fatalf("wrong-type link = %q, want unchanged", got)
	}
}
