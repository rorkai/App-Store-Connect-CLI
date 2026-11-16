package shared

import (
	"os"
	"slices"
	"testing"
)

func TestDepsResolveAppIDMatchesProcessVariant(t *testing.T) {
	envApp := func(value string) *string { return &value }
	tests := []struct {
		name      string
		flag      string
		envApp    *string
		configApp string
		want      string
	}{
		{name: "flag wins over ASC_APP_ID", flag: "flag-app", envApp: envApp("env-app"), want: "flag-app"},
		{name: "flag self-link gives its app ID", flag: "https://api.appstoreconnect.apple.com/v1/apps/flag-app", want: "flag-app"},
		{name: "ASC_APP_ID wins over config and is trimmed", envApp: envApp("  env-app\n"), configApp: "config-app", want: "env-app"},
		{name: "ASC_APP_ID self-link gives its app ID", envApp: envApp("https://api.appstoreconnect.apple.com/v1/apps/env-app"), want: "env-app"},
		{name: "ASC_APP_ID of another resource type is kept", envApp: envApp("https://api.appstoreconnect.apple.com/v1/builds/abc"), want: "https://api.appstoreconnect.apple.com/v1/builds/abc"},
		{name: "ASC_APP_ID that is not an ID is kept", envApp: envApp("not an app"), want: "not an app"},
		{name: "empty ASC_APP_ID skips the config", envApp: envApp(""), configApp: "config-app", want: ""},
		{name: "config supplies the app when ASC_APP_ID is unset", configApp: "config-app", want: "config-app"},
		{name: "nothing set", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBuilderTestAppEnv(t, tt.envApp, tt.configApp)
			if got := ResolveAppID(tt.flag); got != tt.want {
				t.Fatalf("ResolveAppID(%q) = %q, want %q", tt.flag, got, tt.want)
			}
			if got := DefaultDeps().ResolveAppID(tt.flag); got != tt.want {
				t.Fatalf("DefaultDeps().ResolveAppID(%q) = %q, want %q", tt.flag, got, tt.want)
			}
			processValue, _ := os.LookupEnv("ASC_APP_ID")
			if got := DefaultDeps().Getenv("ASC_APP_ID"); got != processValue {
				t.Fatalf("DefaultDeps().Getenv(ASC_APP_ID) = %q, want the process value %q", got, processValue)
			}

			// A nil LookupEnv reads as an empty environment: only the flag and
			// the config remain.
			wantEmptyEnvironment := tt.want
			if tt.flag == "" {
				wantEmptyEnvironment = tt.configApp
			}
			empty := &Deps{}
			if got := empty.ResolveAppID(tt.flag); got != wantEmptyEnvironment {
				t.Fatalf("ResolveAppID(%q) with a nil LookupEnv = %q, want %q", tt.flag, got, wantEmptyEnvironment)
			}
			if got := empty.Getenv("ASC_APP_ID"); got != "" {
				t.Fatalf("Getenv(ASC_APP_ID) with a nil LookupEnv = %q, want empty", got)
			}

			// The process environment now names another app; the injected
			// form must read its own.
			setBuilderTestAppEnv(t, envApp("process-app"), tt.configApp)
			var lookups []string
			d := &Deps{LookupEnv: func(name string) (string, bool) {
				lookups = append(lookups, name)
				if tt.envApp == nil {
					return "", false
				}
				return *tt.envApp, true
			}}
			if got := d.ResolveAppID(tt.flag); got != tt.want {
				t.Fatalf("injected ResolveAppID(%q) = %q, want %q", tt.flag, got, tt.want)
			}
			var wantLookups []string
			if tt.flag == "" {
				wantLookups = []string{"ASC_APP_ID"}
			}
			if !slices.Equal(lookups, wantLookups) {
				t.Fatalf("LookupEnv calls = %q, want %q", lookups, wantLookups)
			}
		})
	}
}
