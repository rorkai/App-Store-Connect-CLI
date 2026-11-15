package cmdtest

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Recorded App Store Connect response for a PATCH that asks to cancel a
// review submission in a state that does not allow it.
const reviewSubmissionNotCancellableBody = `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`

func reviewSubmissionStateBody(id, state string) string {
	return `{"data":{"type":"reviewSubmissions","id":"` + id + `","attributes":{"platform":"IOS","state":"` + state + `"}}}`
}

type reviewCancelStateRun struct {
	requests []string
	stdout   string
	stderr   string
	err      error
}

// runReviewCancelStateCommand runs args against a client whose PATCH returns
// patchStatus/patchBody and whose submission GET returns getStatus/getBody.
func runReviewCancelStateCommand(t *testing.T, args []string, patchStatus int, patchBody string, getStatus int, getBody string) reviewCancelStateRun {
	t.Helper()

	var run reviewCancelStateRun
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		run.requests = append(run.requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/sub-1":
			return jsonResponse(patchStatus, patchBody)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/sub-1":
			if getStatus == 0 {
				return nil, errors.New("simulated network failure")
			}
			return jsonResponse(getStatus, getBody)
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		return nil, nil
	}))
	run.stdout, run.stderr, run.err = runRootWithDeps(t, func() (*asc.Client, error) {
		return client, nil
	}, args...)
	return run
}

func TestReviewSubmissionsCancelExplainsNonCancellableState(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		state string
		want  []string
	}{
		{
			name:  "cancel draft",
			args:  []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			state: "READY_FOR_REVIEW",
			want: []string{
				"review submissions-cancel: review submission sub-1 cannot be canceled in state READY_FOR_REVIEW",
				"unsubmitted draft",
				"asc review items-remove --id ITEM_ID --confirm",
				"asc review submissions-submit --id sub-1 --confirm",
			},
		},
		{
			name:  "cancel unresolved issues",
			args:  []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			state: "UNRESOLVED_ISSUES",
			want: []string{
				"cannot be canceled in state UNRESOLVED_ISSUES",
				"Resolution Center",
				"asc review items-update --id ITEM_ID --resolved true",
			},
		},
		{
			name:  "cancel already canceling",
			args:  []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			state: "CANCELING",
			want:  []string{"cannot be canceled in state CANCELING", "already being canceled"},
		},
		{
			name:  "cancel complete",
			args:  []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			state: "COMPLETE",
			want:  []string{"cannot be canceled in state COMPLETE", "nothing left to cancel"},
		},
		{
			name:  "update canceled draft",
			args:  []string{"review", "submissions-update", "--id", "sub-1", "--canceled=true", "--confirm"},
			state: "READY_FOR_REVIEW",
			want: []string{
				"review submissions-update: review submission sub-1 cannot be canceled in state READY_FOR_REVIEW",
				"unsubmitted draft",
			},
		},
		{
			name:  "update canceled in review",
			args:  []string{"review", "submissions-update", "--id", "sub-1", "--canceled=true", "--confirm"},
			state: "IN_REVIEW",
			want:  []string{"cannot be canceled in state IN_REVIEW", "asc review submissions-get --id sub-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := runReviewCancelStateCommand(t, test.args, http.StatusConflict, reviewSubmissionNotCancellableBody, http.StatusOK, reviewSubmissionStateBody("sub-1", test.state))

			if run.err == nil {
				t.Fatal("expected error, got nil")
			}
			message := run.err.Error()
			for _, want := range append(test.want, "Resource is not in cancellable state") {
				if !strings.Contains(message, want) {
					t.Fatalf("error %q does not contain %q", message, want)
				}
			}
			if !errors.Is(run.err, asc.ErrConflict) {
				t.Fatalf("error %v no longer wraps the App Store Connect conflict", run.err)
			}
			wantRequests := []string{"PATCH /v1/reviewSubmissions/sub-1", "GET /v1/reviewSubmissions/sub-1"}
			if !reflect.DeepEqual(run.requests, wantRequests) {
				t.Fatalf("requests = %v, want %v", run.requests, wantRequests)
			}
			if run.stdout != "" || run.stderr != "" {
				t.Fatalf("stdout = %q, stderr = %q, want no command output", run.stdout, run.stderr)
			}
		})
	}
}

func TestReviewSubmissionsCancelKeepsOriginalErrorWhenStateReadFails(t *testing.T) {
	tests := []struct {
		name      string
		getStatus int
		getBody   string
	}{
		{name: "network failure", getStatus: 0},
		{name: "forbidden", getStatus: http.StatusForbidden, getBody: `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"Not allowed"}]}`},
		{name: "missing state", getStatus: http.StatusOK, getBody: `{"data":{"type":"reviewSubmissions","id":"sub-1","attributes":{}}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := runReviewCancelStateCommand(t, []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"}, http.StatusConflict, reviewSubmissionNotCancellableBody, test.getStatus, test.getBody)

			want := "review submissions-cancel: Resource state is invalid.: Resource is not in cancellable state"
			if run.err == nil || run.err.Error() != want {
				t.Fatalf("error = %v, want %q", run.err, want)
			}
			if !errors.Is(run.err, asc.ErrConflict) {
				t.Fatalf("error %v no longer wraps the App Store Connect conflict", run.err)
			}
		})
	}
}

func TestReviewSubmissionsCancelSkipsStateReadForOtherErrors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		patchStatus int
		patchBody   string
	}{
		{
			name:        "cancel not found",
			args:        []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			patchStatus: http.StatusNotFound,
			patchBody:   `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found","detail":"No resource"}]}`,
		},
		{
			name:        "cancel with a different state conflict",
			args:        []string{"review", "submissions-cancel", "--id", "sub-1", "--confirm"},
			patchStatus: http.StatusConflict,
			patchBody:   `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is locked"}]}`,
		},
		{
			name:        "update submitted with state conflict",
			args:        []string{"review", "submissions-update", "--id", "sub-1", "--submitted=true", "--confirm"},
			patchStatus: http.StatusConflict,
			patchBody:   `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in submittable state"}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := runReviewCancelStateCommand(t, test.args, test.patchStatus, test.patchBody, http.StatusOK, reviewSubmissionStateBody("sub-1", "READY_FOR_REVIEW"))

			if run.err == nil {
				t.Fatal("expected error, got nil")
			}
			if strings.Contains(run.err.Error(), "cannot be canceled") {
				t.Fatalf("error %q explained a cancel the command did not attempt", run.err)
			}
			wantRequests := []string{"PATCH /v1/reviewSubmissions/sub-1"}
			if !reflect.DeepEqual(run.requests, wantRequests) {
				t.Fatalf("requests = %v, want %v", run.requests, wantRequests)
			}
		})
	}
}
