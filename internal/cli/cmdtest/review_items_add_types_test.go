package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestRunReviewItemsAddSupportsGameCenterChallengeVersions(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}

		var payload asc.ReviewSubmissionItemCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("unmarshal request body: %w", err)
		}

		if payload.Data.Relationships.ReviewSubmission == nil {
			return nil, fmt.Errorf("expected reviewSubmission relationship")
		}
		if payload.Data.Relationships.ReviewSubmission.Data.ID != "submission-1" {
			return nil, fmt.Errorf("expected submission id submission-1, got %q", payload.Data.Relationships.ReviewSubmission.Data.ID)
		}
		if payload.Data.Relationships.GameCenterChallengeVersion == nil {
			return nil, fmt.Errorf("expected gameCenterChallengeVersion relationship")
		}
		if payload.Data.Relationships.GameCenterChallengeVersion.Data.Type != asc.ResourceTypeGameCenterChallengeVersions {
			return nil, fmt.Errorf("expected gameCenterChallengeVersions type, got %q", payload.Data.Relationships.GameCenterChallengeVersion.Data.Type)
		}
		if payload.Data.Relationships.GameCenterChallengeVersion.Data.ID != "version-1" {
			return nil, fmt.Errorf("expected item id version-1, got %q", payload.Data.Relationships.GameCenterChallengeVersion.Data.ID)
		}

		return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"review", "items-add",
			"--submission", "submission-1",
			"--item-type", "gameCenterChallengeVersions",
			"--item-id", "version-1",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if got := stripDeprecatedCommandWarnings(stderr); strings.TrimSpace(got) != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var resp asc.ReviewSubmissionItemResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if resp.Data.ID != "item-1" {
		t.Fatalf("expected item id item-1, got %q", resp.Data.ID)
	}
}
