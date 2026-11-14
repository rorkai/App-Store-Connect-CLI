package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const dsymErrorPrefix = "builds dsyms: "

func TestBuildsDSYMNoLiveVersionErrorNamesTheCommandOnce(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,PREORDER_READY_FOR_SALE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"builds", "dsyms", "--app", "123456789", "--version", "live", "--output-dir", t.TempDir()}, "1.2.3")
	})
	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if want := "Error: builds dsyms: no live App Store version\n"; stderr != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr, want)
	}
}

// TestBuildsDSYMErrorsNameTheCommandOnce covers every failure path that
// crosses the command's error wrapping, including errors raised after an inner
// step already named the command and errors from helpers that do not.
func TestBuildsDSYMErrorsNameTheCommandOnce(t *testing.T) {
	forbidden := asctest.JSON(http.StatusForbidden, apiErrorJSONForStatus(http.StatusForbidden))
	buildsList := asctest.GET("/v1/builds", "filter[app]=123456789", "include=preReleaseVersion", "limit=200", "sort=-uploadedDate")
	buildFound := asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"7"}}}`)

	tests := []struct {
		name      string
		args      []string
		register  func(reg *asctest.Registry)
		wantExact string
	}{
		{
			name: "no builds match a range",
			args: []string{"--app", "123456789", "--min-version", "9.0"},
			register: func(reg *asctest.Registry) {
				reg.Register(buildsList, asctest.JSON(http.StatusOK, `{"data":[],"included":[],"links":{}}`))
			},
			wantExact: "builds dsyms: no builds matched",
		},
		{
			name: "live version lookup fails",
			args: []string{"--app", "123456789", "--version", "live"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,PREORDER_READY_FOR_SALE", "limit=200"), forbidden)
			},
		},
		{
			name: "build listing fails",
			args: []string{"--app", "123456789", "--min-version", "1.0"},
			register: func(reg *asctest.Registry) {
				reg.Register(buildsList, forbidden)
			},
		},
		{
			name: "build lookup fails",
			args: []string{"--build-id", "build-1"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/builds/build-1"), forbidden)
			},
		},
		{
			name: "bundle lookup fails",
			args: []string{"--build-id", "build-1"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/builds/build-1"), buildFound)
				reg.Register(asctest.GET("/v1/builds/build-1", "include=buildBundles"), forbidden)
			},
		},
		{
			name: "bundle lookup fails while waiting",
			args: []string{"--build-id", "build-1", "--wait", "--poll-interval", "1ms", "--timeout", "5s"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/builds/build-1"), buildFound)
				reg.Register(asctest.GET("/v1/builds/build-1", "include=buildBundles"), forbidden)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			test.register(reg)

			args := append([]string{"builds", "dsyms"}, test.args...)
			args = append(args, "--output-dir", filepath.Join(t.TempDir(), "dsyms"), "--output", "json")
			_, _, err := runDSYMErr(t, args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			message := err.Error()
			if !strings.HasPrefix(message, dsymErrorPrefix) || strings.Count(message, dsymErrorPrefix) != 1 {
				t.Fatalf("error = %q, want exactly one %q prefix", message, dsymErrorPrefix)
			}
			if test.wantExact != "" && message != test.wantExact {
				t.Fatalf("error = %q, want %q", message, test.wantExact)
			}
		})
	}
}
