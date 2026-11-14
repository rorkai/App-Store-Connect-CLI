package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Apple's live 409 on POST /v1/betaGroups/{id}/relationships/betaTesters when
// the membership cannot be created as requested. Quoted verbatim from Apple
// Developer Forums thread 745785, which reports it for a tester the group
// already appears to contain. The code alone is therefore not decisive: the
// same code is returned when the tester genuinely cannot be assigned, so the
// membership read-back decides which case this is.
const betaGroupAddTestersStateConflictBody = `{
  "errors" : [ {
    "id" : "7d7f28bc-60e5-438e-9a29-f17e14b533bc",
    "status" : "409",
    "code" : "STATE_ERROR",
    "title" : "The request cannot be fulfilled because of the state of another resource.",
    "detail" : "Tester(s) cannot be assigned"
  } ]
}`

// The relationship-rejection 409 this repository already records for the
// sibling POST /v1/betaTesters/{id}/relationships/betaGroups conflict (see
// betaTesterGroupConflictAlreadySatisfied in internal/cli/testflight).
const betaGroupAddTestersRelationshipConflictBody = `{"errors":[{"id":"29f5cf4e-6a41-4d03-9c4f-4c8d2b6a1d55","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'betaTesters' includes a value that is already related to this resource."}]}`

// stubAddTestersTransport stubs the POST adding testerIDs to group-1 and, when
// readBackStatus is not zero, the membership read-back in chunks of 50 tester
// IDs. membership maps a tester ID to the group IDs it belongs to; each
// read-back answers with the requested testers that list group-1.
func stubAddTestersTransport(
	t *testing.T,
	testerIDs []string,
	postStatus int,
	postBody string,
	membership map[string][]string,
	readBackStatus int,
) *asctest.Registry {
	t.Helper()

	reg := asctest.NewRegistry(t)
	reg.Install()

	testers := make([]string, 0, len(testerIDs))
	for _, testerID := range testerIDs {
		testers = append(testers, `{"type":"betaTesters","id":"`+testerID+`"}`)
	}
	reg.Register(asctest.POST("/v1/betaGroups/group-1/relationships/betaTesters").JSONBody(`{"data":[`+strings.Join(testers, ",")+`]}`),
		asctest.JSON(postStatus, postBody))
	if readBackStatus == 0 {
		return reg
	}

	for start := 0; start < len(testerIDs); start += 50 {
		requested := testerIDs[start:min(start+50, len(testerIDs))]
		readBack := asctest.GET("/v1/betaTesters", "filter[betaGroups]=group-1", "filter[id]="+strings.Join(requested, ","), "limit=200")
		if readBackStatus != http.StatusOK {
			reg.Register(readBack, asctest.JSON(readBackStatus, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"Request failed."}]}`))
			return reg
		}

		matches := make([]string, 0, len(requested))
		for _, testerID := range requested {
			for _, groupID := range membership[testerID] {
				if groupID == "group-1" {
					matches = append(matches, `{"type":"betaTesters","id":"`+testerID+`"}`)
					break
				}
			}
		}
		reg.Register(readBack, asctest.JSON(http.StatusOK, `{"data":[`+strings.Join(matches, ",")+`],"links":{}}`))
	}
	return reg
}

type betaGroupAddTestersReceipt struct {
	GroupID        string   `json:"groupId"`
	TesterIDs      []string `json:"testerIds"`
	Action         string   `json:"action"`
	AlreadyPresent bool     `json:"alreadyPresent"`
}

func runAddTesters(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestBetaGroupsAddTestersAlreadyMemberIsExpectedNegative(t *testing.T) {
	for _, conflict := range []struct {
		name string
		body string
	}{
		{name: "state error", body: betaGroupAddTestersStateConflictBody},
		{name: "relationship invalid", body: betaGroupAddTestersRelationshipConflictBody},
	} {
		t.Run(conflict.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := stubAddTestersTransport(t, []string{"tester-1"}, http.StatusConflict, conflict.body, map[string][]string{
				"tester-1": {"group-0", "group-1"},
			}, http.StatusOK)

			stdout, stderr, err := runAddTesters(
				t,
				"testflight", "groups", "add-testers",
				"--group", "group-1",
				"--tester", "tester-1",
				"--output", "json",
			)
			if err != nil {
				t.Fatalf("expected an already-present tester to succeed, got %v", err)
			}

			var receipt betaGroupAddTestersReceipt
			if jsonErr := json.Unmarshal([]byte(stdout), &receipt); jsonErr != nil {
				t.Fatalf("parse stdout JSON: %v; stdout=%q", jsonErr, stdout)
			}
			if receipt.GroupID != "group-1" {
				t.Fatalf("receipt groupId = %q, want group-1", receipt.GroupID)
			}
			if len(receipt.TesterIDs) != 1 || receipt.TesterIDs[0] != "tester-1" {
				t.Fatalf("receipt testerIds = %v, want [tester-1]", receipt.TesterIDs)
			}
			if receipt.Action != "skipped" {
				t.Fatalf("receipt action = %q, want skipped", receipt.Action)
			}
			if !receipt.AlreadyPresent {
				t.Fatalf("receipt alreadyPresent = false, want true; stdout=%q", stdout)
			}
			if !strings.Contains(stderr, "already in group group-1") {
				t.Fatalf("expected stderr to report the already-present tester, got %q", stderr)
			}
			if got := requests.Requests(); len(got) != 2 || got[0].Method != http.MethodPost || got[1].Method != http.MethodGet {
				t.Fatalf("expected one POST then one membership read-back, got %v", got)
			}
		})
	}
}

func TestBetaGroupsAddTestersConflictWithoutMembershipStillFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := stubAddTestersTransport(t, []string{"tester-1"}, http.StatusConflict, betaGroupAddTestersStateConflictBody, map[string][]string{
		"tester-1": {"group-0"},
	}, http.StatusOK)

	stdout, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1",
		"--output", "json",
	)
	if err == nil {
		t.Fatal("expected a conflict whose read-back finds no membership to fail")
	}
	if !strings.Contains(err.Error(), "Tester(s) cannot be assigned") {
		t.Fatalf("expected Apple's conflict detail to survive, got %v", err)
	}
	if !strings.Contains(err.Error(), "not in group group-1: tester-1") {
		t.Fatalf("expected the read-back result in the diagnostic, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected no receipt for a failed add, got %q", stdout)
	}
	if got := requests.Requests(); len(got) != 2 {
		t.Fatalf("expected one POST and one read-back, got %v", got)
	}
}

func TestBetaGroupsAddTestersPartialMembershipStillFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := stubAddTestersTransport(t, []string{"tester-1", "tester-2"}, http.StatusConflict, betaGroupAddTestersStateConflictBody, map[string][]string{
		"tester-1": {"group-1"},
		"tester-2": {},
	}, http.StatusOK)

	_, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1,tester-2",
		"--output", "json",
	)
	if err == nil {
		t.Fatal("expected a partially satisfied conflict to fail")
	}
	if !strings.Contains(err.Error(), "already in group group-1: tester-1") ||
		!strings.Contains(err.Error(), "not in group group-1: tester-2") {
		t.Fatalf("expected the diagnostic to name both sides, got %v", err)
	}
	// One read-back resolves the whole batch: the requested testers are
	// filtered server-side rather than read one membership at a time.
	if got := requests.Requests(); len(got) != 2 {
		t.Fatalf("expected one POST and one batched read-back, got %v", got)
	}
}

func TestBetaGroupsAddTestersConflictReadBackChunksTesterIDs(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	testerIDs := make([]string, 51)
	membership := make(map[string][]string, len(testerIDs))
	for index := range testerIDs {
		testerIDs[index] = fmt.Sprintf("tester-%02d", index+1)
		membership[testerIDs[index]] = []string{"group-1"}
	}
	requests := stubAddTestersTransport(t, testerIDs, http.StatusConflict, betaGroupAddTestersStateConflictBody, membership, http.StatusOK)

	stdout, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", strings.Join(testerIDs, ","),
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("expected the fully satisfied conflict to succeed, got %v", err)
	}
	var receipt betaGroupAddTestersReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("parse stdout JSON: %v; stdout=%q", err, stdout)
	}
	if len(receipt.TesterIDs) != len(testerIDs) {
		t.Fatalf("receipt testerIds count = %d, want %d", len(receipt.TesterIDs), len(testerIDs))
	}
	if got := requests.Requests(); len(got) != 3 || got[0].Method != http.MethodPost || got[1].Method != http.MethodGet || got[2].Method != http.MethodGet {
		t.Fatalf("expected one POST and two chunked read-backs, got %v", got)
	}
}

func TestBetaGroupsAddTestersConflictReadBackPaginates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/betaGroups/group-1/relationships/betaTesters").JSONBody(`{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}`),
		asctest.JSON(http.StatusConflict, betaGroupAddTestersStateConflictBody))
	reg.Register(asctest.GET("/v1/betaTesters", "filter[betaGroups]=group-1", "filter[id]=tester-1,tester-2", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=2"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "cursor=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-2"}],"links":{}}`))

	stdout, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1,tester-2",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("expected membership across both pages to satisfy the conflict, got %v", err)
	}
	var receipt betaGroupAddTestersReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("parse stdout JSON: %v; stdout=%q", err, stdout)
	}
	if receipt.Action != "skipped" || !receipt.AlreadyPresent || len(receipt.TesterIDs) != 2 {
		t.Fatalf("unexpected skip receipt: %+v", receipt)
	}
	if requests := reg.Requests(); len(requests) != 3 || requests[0].Method != http.MethodPost || requests[1].Method != http.MethodGet || requests[2].Method != http.MethodGet {
		t.Fatalf("expected one POST and two paginated read-backs, got %v", requests)
	}
}

func TestBetaGroupsAddTestersNonConflictFailureSkipsReadBack(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := stubAddTestersTransport(t, []string{"tester-1"}, http.StatusForbidden,
		`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"This request is forbidden for security reasons","detail":"The API key in use does not allow this request."}]}`,
		nil, 0)

	_, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1",
	)
	if err == nil {
		t.Fatal("expected a non-conflict failure to fail")
	}
	if got := requests.Requests(); len(got) != 1 || got[0].Method != http.MethodPost {
		t.Fatalf("expected only the POST for a non-conflict failure, got %v", got)
	}
}

func TestBetaGroupsAddTestersConflictCodeWithoutHTTP409SkipsReadBack(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := stubAddTestersTransport(t, []string{"tester-1"}, http.StatusUnprocessableEntity,
		`{"errors":[{"status":"422","code":"CONFLICT","title":"The request cannot be fulfilled","detail":"The build is not eligible for this tester."}]}`,
		nil, 0)

	_, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1",
	)
	if err == nil {
		t.Fatal("expected a non-409 API error to fail even when its code is CONFLICT")
	}
	if !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("expected Apple's error detail to survive, got %v", err)
	}
	if got := requests.Requests(); len(got) != 1 || got[0].Method != http.MethodPost {
		t.Fatalf("expected only the POST for a non-409 failure, got %v", got)
	}
}

func TestBetaGroupsAddTestersReadBackFailureReportsBothErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	stubAddTestersTransport(t, []string{"tester-1"}, http.StatusConflict, betaGroupAddTestersStateConflictBody,
		map[string][]string{"tester-1": {"group-1"}}, http.StatusInternalServerError)

	_, _, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1",
	)
	if err == nil {
		t.Fatal("expected a failed read-back to keep the add failing")
	}
	if !strings.Contains(err.Error(), "Tester(s) cannot be assigned") {
		t.Fatalf("expected the original conflict in the error, got %v", err)
	}
	if !strings.Contains(err.Error(), "read-back") {
		t.Fatalf("expected the read-back failure in the error, got %v", err)
	}
}

func TestBetaGroupsAddTestersSuccessPrintsCreatedReceipt(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := stubAddTestersTransport(t, []string{"tester-1", "tester-2"}, http.StatusNoContent, "", nil, 0)

	stdout, stderr, err := runAddTesters(
		t,
		"testflight", "groups", "add-testers",
		"--group", "group-1",
		"--tester", "tester-1,tester-2",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("expected a successful add, got %v", err)
	}

	var receipt betaGroupAddTestersReceipt
	if jsonErr := json.Unmarshal([]byte(stdout), &receipt); jsonErr != nil {
		t.Fatalf("parse stdout JSON: %v; stdout=%q", jsonErr, stdout)
	}
	if receipt.Action != "added" {
		t.Fatalf("receipt action = %q, want added", receipt.Action)
	}
	if receipt.AlreadyPresent {
		t.Fatalf("receipt alreadyPresent = true, want false; stdout=%q", stdout)
	}
	if len(receipt.TesterIDs) != 2 {
		t.Fatalf("receipt testerIds = %v, want both requested testers", receipt.TesterIDs)
	}
	if !strings.Contains(stderr, "Successfully added 2 tester(s) to group group-1") {
		t.Fatalf("expected the success diagnostic, got %q", stderr)
	}
	if got := requests.Requests(); len(got) != 1 || got[0].Method != http.MethodPost {
		t.Fatalf("expected only the POST on the success path, got %v", got)
	}
}
