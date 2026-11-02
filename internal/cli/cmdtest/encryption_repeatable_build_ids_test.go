package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestEncryptionAssignBuildsRepeatedBuildIDRejectedBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return jsonHTTPResponse(http.StatusNoContent, ""), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build-id", "BUILD_A",
			"--build-id", "BUILD_B",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	wantMessage := repeatedCSVFlagMessage("build-id", "BUILD_A", "BUILD_B")
	if !strings.Contains(stderr, wantMessage) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, wantMessage)
	}
	if requestCount != 0 {
		t.Fatalf("repeated --build-id made %d HTTP requests, want 0", requestCount)
	}
}

func TestEncryptionAssignBuildsCSVPreservesDuplicatesAndSkipsEmptyFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var requestIDs []string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestIDs = append(requestIDs, decodeBuildDeclarationPatch(t, req, "DECL_ID"))
		return jsonHTTPResponse(http.StatusNoContent, ""), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build-id", "BUILD_A,, BUILD_A, ",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})

	if want := []string{"BUILD_A", "BUILD_A"}; !reflect.DeepEqual(requestIDs, want) {
		t.Fatalf("request build IDs = %#v, want %#v", requestIDs, want)
	}
	if want := `{"declarationId":"DECL_ID","buildIds":["BUILD_A","BUILD_A"],"action":"assigned"}`; strings.TrimSpace(stdout) != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, "Successfully assigned 2 build(s) to declaration DECL_ID") {
		t.Fatalf("stderr = %q, want success diagnostic", stderr)
	}
}

// decodeBuildDeclarationPatch asserts req is the non-deprecated per-build
// relationship update and returns the build ID it targets.
func decodeBuildDeclarationPatch(t *testing.T, req *http.Request, wantDeclarationID string) string {
	t.Helper()
	buildID, ok := strings.CutPrefix(req.URL.Path, "/v1/builds/")
	if ok {
		buildID, ok = strings.CutSuffix(buildID, "/relationships/appEncryptionDeclaration")
	}
	if req.Method != http.MethodPatch || !ok {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
	}
	var payload struct {
		Data struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if payload.Data.Type != "appEncryptionDeclarations" || payload.Data.ID != wantDeclarationID {
		t.Fatalf("request linkage = %+v, want appEncryptionDeclarations/%s", payload.Data, wantDeclarationID)
	}
	return buildID
}

const buildNotFoundBody = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found","detail":"no such build"}]}`

func TestEncryptionAssignBuildsPartialFailureReportsEachBuild(t *testing.T) {
	tests := []struct {
		output     string
		wantStdout []string
	}{
		{
			output: "json",
			wantStdout: []string{
				`"buildIds":["BUILD_A","BUILD_C"]`,
				`"failures":[{"buildId":"BUILD_B","error":`,
			},
		},
		{
			output:     "table",
			wantStdout: []string{"BUILD_A,BUILD_C", "BUILD_B: "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			var requestIDs []string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				buildID := decodeBuildDeclarationPatch(t, req, "DECL_ID")
				requestIDs = append(requestIDs, buildID)
				if buildID == "BUILD_B" {
					return jsonHTTPResponse(http.StatusNotFound, buildNotFoundBody), nil
				}
				return jsonHTTPResponse(http.StatusNoContent, ""), nil
			}))

			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run([]string{
					"encryption", "declarations", "assign-builds",
					"--id", "DECL_ID",
					"--build-id", "BUILD_A,BUILD_B,BUILD_C",
					"--output", tt.output,
				}, "1.2.3")
				if code != rootcmd.ExitNotFound {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitNotFound)
				}
			})

			if want := []string{"BUILD_A", "BUILD_B", "BUILD_C"}; !reflect.DeepEqual(requestIDs, want) {
				t.Fatalf("request build IDs = %#v, want %#v", requestIDs, want)
			}
			for _, want := range tt.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want it to contain %q", stdout, want)
				}
			}
			for _, want := range []string{"assigned 2 of 3 build(s)", "build BUILD_B: "} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
		})
	}
}

func TestEncryptionAssignBuildsTotalFailureKeepsStdoutEmpty(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		decodeBuildDeclarationPatch(t, req, "DECL_ID")
		return jsonHTTPResponse(http.StatusNotFound, buildNotFoundBody), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build-id", "BUILD_A,BUILD_B",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitNotFound {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitNotFound)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"failed to assign builds", "build BUILD_A: ", "build BUILD_B: "} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestEncryptionAssignBuildsRejectsPathUnsafeBuildIDBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return jsonHTTPResponse(http.StatusNoContent, ""), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build-id", "BUILD_A,BUILD_B/relationships/betaGroups",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "encryption declarations assign-builds: --build-id:") {
		t.Fatalf("stderr = %q, want --build-id validation error", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("path-unsafe --build-id made %d HTTP requests, want 0", requestCount)
	}
}

func TestEncryptionAssignBuildsExplicitEmptyValueKeepsRequiredValidation(t *testing.T) {
	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return jsonHTTPResponse(http.StatusNoContent, ""), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build-id", "",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --build-id is required") {
		t.Fatalf("stderr = %q, want required build-id error", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("empty --build-id made %d HTTP requests, want 0", requestCount)
	}
}

func TestEncryptionAssignBuildsLegacyBuildFlagIsUnknownBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		return jsonHTTPResponse(http.StatusNoContent, ""), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"encryption", "declarations", "assign-builds",
			"--id", "DECL_ID",
			"--build", "BUILD_B",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: `--build` was removed in 5.0.0; use `--build-id`") {
		t.Fatalf("stderr = %q, want removed-flag guidance", stderr)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Fatalf("stderr = %q, want no deprecation guidance for the removed alias", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("removed --build alias made %d HTTP requests, want 0", requestCount)
	}
}
