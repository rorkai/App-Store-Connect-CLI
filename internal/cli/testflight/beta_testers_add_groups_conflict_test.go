package testflight

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Apple's 409 on POST /v1/betaTesters/{id}/relationships/betaGroups when a
// requested relationship already exists. The code alone is not decisive: the
// same status is returned when the tester cannot be assigned at all, so the
// membership read-back decides which case this is.
const betaTesterAddGroupsRelationshipConflictBody = `{"errors":[{"id":"29f5cf4e-6a41-4d03-9c4f-4c8d2b6a1d55","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'betaGroups' includes a value that is already related to this resource."}]}`

type betaTesterAddGroupsRequest struct {
	method string
	path   string
}

// newBetaTesterAddGroupsServer replays the POST conflict followed by the
// membership read-back that reports memberships as beta group linkages.
func newBetaTesterAddGroupsServer(
	t *testing.T,
	postStatus int,
	postBody string,
	memberships []string,
	readBackStatus int,
) (*httptest.Server, *[]betaTesterAddGroupsRequest) {
	t.Helper()

	requests := make([]betaTesterAddGroupsRequest, 0, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, betaTesterAddGroupsRequest{method: req.Method, path: req.URL.Path})
		if req.URL.Path != "/v1/betaTesters/tester-1/relationships/betaGroups" {
			t.Errorf("unexpected request path %q", req.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodPost {
			w.WriteHeader(postStatus)
			if postBody != "" {
				if _, err := io.WriteString(w, postBody); err != nil {
					t.Errorf("write conflict body: %v", err)
				}
			}
			return
		}
		if req.Method != http.MethodGet {
			t.Errorf("unexpected %s request", req.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}

		if readBackStatus != 0 && readBackStatus != http.StatusOK {
			w.WriteHeader(readBackStatus)
			if _, err := io.WriteString(w, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"Request failed."}]}`); err != nil {
				t.Errorf("write read-back error body: %v", err)
			}
			return
		}

		data := make([]map[string]string, 0, len(memberships))
		for _, groupID := range memberships {
			data = append(data, map[string]string{"type": "betaGroups", "id": groupID})
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data":  data,
			"links": map[string]string{},
		}); err != nil {
			t.Errorf("marshal read-back response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func runBetaTestersAddGroups(t *testing.T, server *httptest.Server, args ...string) (string, string, error) {
	t.Helper()

	isolateTestFlightAuthEnvForAddTests(t)
	client := newBetaTesterCSVConflictClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	cmd := BetaTestersAddGroupsCommand()
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := captureBetaTesterAddGroupsOutput(t, func() {
		runErr = cmd.Exec(context.Background(), []string{})
	})
	return stdout, stderr, runErr
}

func captureBetaTesterAddGroupsOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	originalStdout, originalStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	stdoutResult := make(chan string, 1)
	stderrResult := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stdoutReader)
		stdoutResult <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(stderrReader)
		stderrResult <- string(data)
	}()

	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	fn()
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stdout, os.Stderr = originalStdout, originalStderr

	stdout, stderr := <-stdoutResult, <-stderrResult
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return stdout, stderr
}

func TestBetaTestersAddGroupsAlreadyPresentIsSkipped(t *testing.T) {
	server, requests := newBetaTesterAddGroupsServer(
		t,
		http.StatusConflict,
		betaTesterAddGroupsRelationshipConflictBody,
		[]string{"group-1", "group-2"},
		http.StatusOK,
	)

	stdout, stderr, err := runBetaTestersAddGroups(
		t, server,
		"--id", "tester-1",
		"--group", "group-1,group-2",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("expected an already-present membership to succeed, got %v", err)
	}

	var result asc.BetaTesterGroupsUpdateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("parse stdout JSON: %v; stdout=%q", jsonErr, stdout)
	}
	if result.Action != asc.BetaGroupTestersActionSkipped {
		t.Fatalf("receipt action = %q, want %q", result.Action, asc.BetaGroupTestersActionSkipped)
	}
	if result.TesterID != "tester-1" || len(result.GroupIDs) != 2 {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if !strings.Contains(stderr, "already in 2 group(s)") {
		t.Fatalf("expected stderr to report the skip, got %q", stderr)
	}
	if got := *requests; len(got) != 2 || got[0].method != http.MethodPost || got[1].method != http.MethodGet {
		t.Fatalf("expected one POST then one membership read-back, got %v", got)
	}
}

func TestBetaTestersAddGroupsConflictWithoutMembershipStillFails(t *testing.T) {
	server, requests := newBetaTesterAddGroupsServer(
		t,
		http.StatusConflict,
		betaTesterAddGroupsRelationshipConflictBody,
		[]string{"group-1"},
		http.StatusOK,
	)

	stdout, _, err := runBetaTestersAddGroups(
		t, server,
		"--id", "tester-1",
		"--group", "group-1,group-2",
		"--output", "json",
	)
	if err == nil {
		t.Fatal("expected a conflict whose read-back finds a missing group to fail")
	}
	if !strings.Contains(err.Error(), "beta-testers add-groups: failed to add groups") {
		t.Fatalf("expected the add-groups failure to survive, got %v", err)
	}
	if !strings.Contains(err.Error(), "already related to this resource") {
		t.Fatalf("expected Apple's conflict detail to survive, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt for a failed add, got %q", stdout)
	}
	if got := *requests; len(got) != 2 {
		t.Fatalf("expected one POST and one read-back, got %v", got)
	}
}

func TestBetaTestersAddGroupsReadBackFailureReportsBothErrors(t *testing.T) {
	server, _ := newBetaTesterAddGroupsServer(
		t,
		http.StatusConflict,
		betaTesterAddGroupsRelationshipConflictBody,
		[]string{"group-1"},
		http.StatusInternalServerError,
	)

	_, _, err := runBetaTestersAddGroups(
		t, server,
		"--id", "tester-1",
		"--group", "group-1",
		"--output", "json",
	)
	if err == nil {
		t.Fatal("expected a failed read-back to keep the add failing")
	}
	if !strings.Contains(err.Error(), "already related to this resource") {
		t.Fatalf("expected the original conflict in the error, got %v", err)
	}
	if !strings.Contains(err.Error(), "failed to verify beta group membership") {
		t.Fatalf("expected the read-back failure in the error, got %v", err)
	}
}

func TestBetaTestersAddGroupsNonConflictFailureSkipsReadBack(t *testing.T) {
	server, requests := newBetaTesterAddGroupsServer(
		t,
		http.StatusForbidden,
		`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"This request is forbidden for security reasons","detail":"The API key in use does not allow this request."}]}`,
		[]string{"group-1"},
		http.StatusOK,
	)

	_, _, err := runBetaTestersAddGroups(
		t, server,
		"--id", "tester-1",
		"--group", "group-1",
		"--output", "json",
	)
	if err == nil {
		t.Fatal("expected a non-conflict failure to fail")
	}
	if got := *requests; len(got) != 1 || got[0].method != http.MethodPost {
		t.Fatalf("expected only the POST for a non-conflict failure, got %v", got)
	}
}
