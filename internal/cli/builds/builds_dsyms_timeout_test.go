package builds

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsDsymsCommandGivesEachDownloadFreshTimeout(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TIMEOUT", "2s")

	apiCalls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected App Store Connect path: %s", req.URL.Path)
		}
		apiCalls++
		if apiCalls == 1 {
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}}
			}`)
		}
		if apiCalls == 2 {
			return buildsWaitJSONResponse(http.StatusOK, `{
				"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}},
				"included":[
					{"type":"buildBundles","id":"bundle-1","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example/one.zip"}},
					{"type":"buildBundles","id":"bundle-2","attributes":{"bundleId":"com.example.extension","dSYMUrl":"https://downloads.example/two.zip"}}
				]
			}`)
		}
		return nil, fmt.Errorf("unexpected App Store Connect request %d", apiCalls)
	})

	restoreASCClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restoreASCClient)

	var downloadDeadlines []time.Time
	restoreDownloadClient := SetDSYMHTTPClient(&http.Client{
		Transport: buildsWaitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				return nil, fmt.Errorf("dSYM download has no deadline")
			}
			downloadDeadlines = append(downloadDeadlines, deadline)
			if len(downloadDeadlines) == 1 {
				time.Sleep(15 * time.Millisecond)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("symbols")),
				Request:    req,
			}, nil
		}),
	})
	t.Cleanup(restoreDownloadClient)

	cmd := BuildsDsymsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--build-id", "build-1",
		"--output-dir", t.TempDir(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := cmd.Exec(t.Context(), nil); err != nil {
		t.Fatalf("BuildsDsymsCommand.Exec() error: %v", err)
	}

	if len(downloadDeadlines) != 2 {
		t.Fatalf("download requests = %d, want 2", len(downloadDeadlines))
	}
	if delta := downloadDeadlines[1].Sub(downloadDeadlines[0]); delta < 10*time.Millisecond {
		t.Fatalf("second download deadline advanced by %v, want at least 10ms from a fresh timeout", delta)
	}
}
