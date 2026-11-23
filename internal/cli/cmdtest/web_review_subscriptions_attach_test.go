package cmdtest

import (
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Response shapes below are trimmed from live App Store Connect API answers.
const (
	attachGroupsBody = `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"},"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups"},"meta":{"paging":{"total":1,"limit":200}}}`
	attachSubsBody   = `{"data":[` +
		`{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"READY_TO_SUBMIT","subscriptionPeriod":"ONE_MONTH","groupLevel":1}},` +
		`{"type":"subscriptions","id":"sub-2","attributes":{"name":"Yearly","productId":"com.example.yearly","state":"MISSING_METADATA","groupLevel":2}},` +
		`{"type":"subscriptions","id":"sub-3","attributes":{"name":"Weekly","productId":"com.example.weekly","state":"READY_TO_SUBMIT","groupLevel":3}}` +
		`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"},"meta":{"paging":{"total":3,"limit":200}}}`
	attachNoItemsBody = `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/rs-1/items"},"meta":{"paging":{"total":0,"limit":200}}}`
)

func attachVersionsBody(id, state string) string {
	return `{"data":[{"type":"subscriptionVersions","id":"` + id + `","attributes":{"version":1,"state":"` + state + `"},"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionVersions/` + id + `"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub/versions"},"meta":{"paging":{"total":1,"limit":200}}}`
}

func attachDraftsBody(ids ...string) string {
	rows := make([]string, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, `{"type":"reviewSubmissions","id":"`+id+`","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW","submittedDate":null}}`)
	}
	return `{"data":[` + strings.Join(rows, ",") + `],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"},"meta":{"paging":{"total":` + string(rune('0'+len(ids))) + `,"limit":200}}}`
}

func attachItemsBody(submissionID, itemID, versionID string) string {
	return `{"data":[{"type":"reviewSubmissionItems","id":"` + itemID + `","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"` + versionID + `"}}}}],"included":[{"type":"subscriptionVersions","id":"` + versionID + `","attributes":{"version":1,"state":"READY_FOR_REVIEW"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/` + submissionID + `/items"},"meta":{"paging":{"total":1,"limit":200}}}`
}

func attachItemPost(submissionID string) asctest.Matcher {
	const versionID = "ver-1"
	return asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"` + submissionID + `"}},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"` + versionID + `"}}}}}`)
}

func attachItemCreated(itemID string) asctest.Responder {
	return asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"`+itemID+`","attributes":{"state":"READY_FOR_REVIEW"}}}`)
}

func registerAttachLookups(reg *asctest.Registry) {
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, attachGroupsBody))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, attachSubsBody))
}

func registerAttachDrafts(reg *asctest.Registry, items map[string]string, ids ...string) {
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[state]=READY_FOR_REVIEW", "limit=200"), asctest.JSON(http.StatusOK, attachDraftsBody(ids...)))
	for _, id := range ids {
		body, ok := items[id]
		if !ok {
			body = strings.ReplaceAll(attachNoItemsBody, "rs-1", id)
		}
		reg.Register(asctest.GET("/v1/reviewSubmissions/"+id+"/items", "include=subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, body))
	}
}

func TestWebReviewSubscriptionsAttachUsesReviewSubmissionItems(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		register    func(reg *asctest.Registry)
		wantUsage   bool
		wantErr     []string
		wantStdout  []string
		wantStderr  []string
		rejectInOut []string
		// wantDiagnostic is the diagnostic code the error must carry.
		wantDiagnostic string
		// webSessionOnly runs without API key auth, as a caller that only has
		// the web session the command used before.
		webSessionOnly bool
	}{
		{
			name: "adds the version to the only draft by product ID",
			args: []string{"attach", "--subscription-id", "com.example.monthly"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout: []string{`"operation":"attach"`, `"changed":true`, `"submissionId":"rs-1"`, `"reviewSubmissionItemId":"item-1"`, `"subscriptionVersionId":"ver-1"`, `"id":"sub-1"`, `"submitWithNextAppStoreVersion":true`},
		},
		{
			name: "reports an already attached version without a POST",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "READY_FOR_REVIEW")))
				registerAttachDrafts(reg, map[string]string{"rs-2": attachItemsBody("rs-2", "item-1", "ver-1")}, "rs-1", "rs-2")
			},
			wantStdout: []string{`"changed":false`, `"submissionId":"rs-2"`, `"reviewSubmissionItemId":"item-1"`},
		},
		{
			name: "creates a draft for --platform when the app has none",
			args: []string{"attach", "--subscription-id", "sub-1", "--platform", "MAC_OS", "--apple-id", "dev@example.com"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil)
				reg.Register(asctest.POST("/v1/reviewSubmissions").JSONBody(`{"data":{"type":"reviewSubmissions","attributes":{"platform":"MAC_OS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"platform":"MAC_OS","state":"READY_FOR_REVIEW"}}}`))
				reg.Register(attachItemPost("rs-new"), attachItemCreated("item-new"))
			},
			wantStdout: []string{`"changed":true`, `"submissionId":"rs-new"`},
			wantStderr: []string{
				"Created review submission rs-new (MAC_OS) for app app-1; it is a draft and is not submitted for review.",
				"Warning: --apple-id is deprecated for `asc web review subscriptions attach` and has no effect",
			},
		},
		{
			name: "asks for --submission when the app has several drafts",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1", "rs-2")
			},
			wantUsage:  true,
			wantStderr: []string{"multiple READY_FOR_REVIEW review submissions; select one with --submission: rs-1, rs-2"},
		},
		{
			name: "names the next step when no version can be submitted",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "WAITING_FOR_REVIEW")))
			},
			wantErr:        []string{`subscription "sub-1" has no version that can be added to a review submission (version states: WAITING_FOR_REVIEW)`, `asc subscriptions versions create --subscription-id "sub-1"`},
			wantDiagnostic: "state_not_ready",
		},
		{
			name: "refuses a MISSING_METADATA subscription before any write",
			args: []string{"attach", "--subscription-id", "sub-2"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
			},
			wantErr:        []string{`subscription "sub-2" is MISSING_METADATA`, `asc validate subscriptions --app "app-1"`},
			wantDiagnostic: "state_not_ready",
		},
		{
			name: "cancels the draft it created when Apple refuses the item",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil)
				reg.Register(asctest.POST("/v1/reviewSubmissions"), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}}`))
				reg.Register(attachItemPost("rs-new"), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"subscriptionVersions with id 'ver-1' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why.","meta":{"associatedErrors":{"/v1/subscriptionVersions/ver-1":[{"code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED","detail":"You must upload a screenshot before submitting a subscription for review."}]}}}]}`))
				reg.Register(asctest.PATCH("/v1/reviewSubmissions/rs-new").JSONBody(`{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"canceled":true}}}`),
					asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"platform":"IOS","state":"CANCELING"}}}`))
			},
			wantErr: []string{"not in valid state", "canceled review submission rs-new, which this run created"},
		},
		{
			name:       "rejects --platform together with --submission before any request",
			args:       []string{"attach", "--subscription-id", "sub-1", "--submission", "rs-1", "--platform", "MAC_OS"},
			register:   func(reg *asctest.Registry) {},
			wantUsage:  true,
			wantStderr: []string{"--platform only applies when a new review submission is created; drop it when --submission is set"},
		},
		{
			name: "rejects --platform that does not match the app's draft",
			args: []string{"attach", "--subscription-id", "sub-1", "--platform", "MAC_OS"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
			},
			wantUsage:  true,
			wantStderr: []string{"--platform MAC_OS does not match the app's READY_FOR_REVIEW review submission rs-1 (IOS)"},
		},
		{
			name: "refuses to add a version that is already on another draft",
			args: []string{"attach", "--subscription-id", "sub-1", "--submission", "rs-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "READY_FOR_REVIEW")))
				registerAttachDrafts(reg, map[string]string{"rs-2": attachItemsBody("rs-2", "item-1", "ver-1")}, "rs-1", "rs-2")
			},
			wantUsage:  true,
			wantStderr: []string{"subscription \"sub-1\" is already on review submission rs-2"},
		},
		{
			name:           "tells a web-session-only caller that attach needs the API key",
			args:           []string{"attach", "--subscription-id", "sub-1"},
			register:       func(reg *asctest.Registry) {},
			webSessionOnly: true,
			wantErr:        []string{"now uses the App Store Connect API key, not the web session", "asc auth login"},
		},
		{
			name: "attach-group reports a group with nothing to add as not ready",
			args: []string{"attach-group", "--group-id", "group-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "WAITING_FOR_REVIEW")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-3/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-3", "IN_REVIEW")))
			},
			wantErr:        []string{`group "group-1" has no subscription that can be added to a review submission`},
			wantDiagnostic: "state_not_ready",
		},
		{
			name: "attach-group adds attachable subscriptions and skips the rest",
			args: []string{"attach-group", "--group-id", "Premium", "--submission", "rs-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				registerAttachDrafts(reg, map[string]string{"rs-1": attachItemsBody("rs-1", "item-3", "ver-3")}, "rs-1")
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-3/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-3", "READY_FOR_REVIEW")))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout: []string{`"operation":"attach-group"`, `"submissionId":"rs-1"`, `"changedCount":1`, `"skippedCount":2`, `"reason":"state is MISSING_METADATA`, `"reason":"already on review submission rs-1"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			if tt.webSessionOnly {
				for _, name := range []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_BEARER_TOKEN", "ASC_BEARER_TOKEN_COMMAND"} {
					t.Setenv(name, "")
				}
				t.Setenv("ASC_WEB_SESSION", "session-placeholder")
			}
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Install()
			tt.register(reg)

			args := append([]string{"web", "review", "subscriptions"}, tt.args...)
			args = append(args, "--app", "app-1", "--confirm", "--output", "json")
			stdout, stderr, err := runCommand(t, args)

			switch {
			case tt.wantUsage:
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			case len(tt.wantErr) > 0:
				if err == nil {
					t.Fatal("expected error")
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) && !strings.Contains(errorHint(err), want) {
						t.Fatalf("expected error or hint to contain %q, got %v (hint %q)", want, err, errorHint(err))
					}
				}
			case err != nil:
				t.Fatalf("run error: %v\nstderr=%s", err, stderr)
			}
			if tt.wantDiagnostic != "" {
				diagnostic, ok := shared.DiagnosticFromError(err)
				if !ok || string(diagnostic.Code) != tt.wantDiagnostic {
					t.Fatalf("diagnostic = %+v (found %t), want %s", diagnostic, ok, tt.wantDiagnostic)
				}
			}
			for _, want := range tt.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %s", want, stdout)
				}
			}
			for _, want := range tt.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
			for _, req := range reg.Requests() {
				if strings.Contains(req.URL.Path, "/iris/") || strings.Contains(req.URL.Path, "subscriptionSubmissions") {
					t.Fatalf("attach must not use the web session flow, sent %s %s", req.Method, req.URL.Path)
				}
			}
		})
	}
}

func errorHint(err error) string {
	if hinted, ok := errors.AsType[*shared.HintedError](err); ok {
		return hinted.Hint
	}
	return ""
}
