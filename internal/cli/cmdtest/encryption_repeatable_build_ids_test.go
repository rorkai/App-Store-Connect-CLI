package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestEncryptionAssignBuildsRepeatedBuildIDRejectedBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("repeated --build-id made %d HTTP requests, want 0", requestCount)
	}
}

func TestEncryptionAssignBuildsCSVPreservesDuplicatesAndSkipsEmptyFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appEncryptionDeclarations/DECL_ID/relationships/builds").JSONBody(`{"data":[{"type":"builds","id":"BUILD_A"},{"type":"builds","id":"BUILD_A"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

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

	requests := reg.Requests()
	if requestCount := len(requests); requestCount != 1 {
		t.Fatalf("HTTP request count = %d, want 1", requestCount)
	}
	var payload struct {
		Data []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(requests[0].Body, &payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	var requestIDs []string
	for _, item := range payload.Data {
		if item.Type != "builds" {
			t.Fatalf("request relationship type = %q, want builds", item.Type)
		}
		requestIDs = append(requestIDs, item.ID)
	}
	if want := []string{"BUILD_A", "BUILD_A"}; !reflect.DeepEqual(requestIDs, want) {
		t.Fatalf("request build IDs = %#v, want %#v", requestIDs, want)
	}
	if !strings.Contains(stdout, `"buildIds":["BUILD_A","BUILD_A"]`) {
		t.Fatalf("stdout = %q, want duplicate build IDs in receipt", stdout)
	}
	if !strings.Contains(stderr, "Successfully assigned 2 build(s) to declaration DECL_ID") {
		t.Fatalf("stderr = %q, want success diagnostic", stderr)
	}
}

func TestEncryptionAssignBuildsExplicitEmptyValueKeepsRequiredValidation(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("empty --build-id made %d HTTP requests, want 0", requestCount)
	}
}

func TestEncryptionAssignBuildsLegacyBuildFlagIsUnknownBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("removed --build alias made %d HTTP requests, want 0", requestCount)
	}
}
