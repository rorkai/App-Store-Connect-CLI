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
		`{"type":"subscriptions","id":"sub-3","attributes":{"name":"Weekly","productId":"com.example.weekly","state":"READY_TO_SUBMIT","groupLevel":3}},` +
		`{"type":"subscriptions","id":"sub-4","attributes":{"name":"Legacy","productId":"com.example.legacy","state":"APPROVED","groupLevel":4}}` +
		`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"},"meta":{"paging":{"total":4,"limit":200}}}`
	// attachFirstSubsBody is an app whose subscriptions were never approved, so
	// its first subscription must travel with an app version.
	attachFirstSubsBody = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"READY_TO_SUBMIT","groupLevel":1}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"},"meta":{"paging":{"total":1,"limit":200}}}`
	attachNoItemsBody   = `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/rs-1/items"},"meta":{"paging":{"total":0,"limit":200}}}`
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
	return attachItemsBodyOf(submissionID, itemID, "subscriptionVersion", "subscriptionVersions", versionID)
}

func attachItemsBodyOf(submissionID, itemID, relationship, resourceType, resourceID string) string {
	return `{"data":[{"type":"reviewSubmissionItems","id":"` + itemID + `","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":null},"subscriptionGroupVersion":{"data":null},"subscriptionVersion":{"data":null},"` + relationship + `":{"data":{"type":"` + resourceType + `","id":"` + resourceID + `"}}}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/` + submissionID + `/items"},"meta":{"paging":{"total":1,"limit":200}}}`
}

func attachTypedItemPost(relationship, resourceType, resourceID string) asctest.Matcher {
	const submissionID = "rs-1"
	return asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"` + submissionID + `"}},"` + relationship + `":{"data":{"type":"` + resourceType + `","id":"` + resourceID + `"}}}}}`)
}

func registerFirstAttachLookups(reg *asctest.Registry) {
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, attachGroupsBody))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, attachFirstSubsBody))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-1", "PREPARE_FOR_SUBMISSION")))
	reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
}

func attachGroupVersionsBody(id, state string) string {
	return `{"data":[{"type":"subscriptionGroupVersions","id":"` + id + `","attributes":{"version":1,"state":"` + state + `"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/versions"},"meta":{"paging":{"total":1,"limit":200}}}`
}

func attachAppVersionsBody(ids ...string) string {
	rows := make([]string, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, `{"type":"appStoreVersions","id":"`+id+`","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}`)
	}
	return `{"data":[` + strings.Join(rows, ",") + `],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions"}}`
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
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-1", "APPROVED")))
}

func registerAttachDrafts(reg *asctest.Registry, items map[string]string, ids ...string) {
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[state]=READY_FOR_REVIEW", "limit=200"), asctest.JSON(http.StatusOK, attachDraftsBody(ids...)))
	for _, id := range ids {
		body, ok := items[id]
		if !ok {
			body = strings.ReplaceAll(attachNoItemsBody, "rs-1", id)
		}
		reg.Register(asctest.GET("/v1/reviewSubmissions/"+id+"/items", "include=appStoreVersion,subscriptionGroupVersion,subscriptionVersion", "limit=200"), asctest.JSON(http.StatusOK, body))
	}
}

func TestWebReviewSubscriptionsAttachReviewItems(t *testing.T) {
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
		// webFlow leaves out --review-items.
		webFlow bool
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
			wantStdout:  []string{`"operation":"attach"`, `"changed":true`, `"reviewSubmissionId":"rs-1"`, `"reviewSubmissionCreated":false`, `"items":[{"itemType":"subscriptionVersions","resourceId":"ver-1","reviewSubmissionItemId":"item-1","subscriptionId":"sub-1","added":true}]`, `"id":"sub-1"`},
			rejectInOut: []string{`"submissionId"`, `submitWithNextAppStoreVersion`, `isAppStoreReviewInProgress`},
		},
		{
			name: "reports an already attached version without a POST",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "READY_FOR_REVIEW")))
				registerAttachDrafts(reg, map[string]string{"rs-2": attachItemsBody("rs-2", "item-1", "ver-1")}, "rs-1", "rs-2")
			},
			wantStdout: []string{`"changed":false`, `"reviewSubmissionId":"rs-2"`, `"items":[{"itemType":"subscriptionVersions","resourceId":"ver-1","reviewSubmissionItemId":"item-1","subscriptionId":"sub-1","added":false}]`},
		},
		{
			name: "creates a draft for --platform when the app has none",
			args: []string{"attach", "--subscription-id", "sub-1", "--platform", "MAC_OS"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil)
				reg.Register(asctest.POST("/v1/reviewSubmissions").JSONBody(`{"data":{"type":"reviewSubmissions","attributes":{"platform":"MAC_OS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"rs-new","attributes":{"platform":"MAC_OS","state":"READY_FOR_REVIEW"}}}`))
				reg.Register(attachItemPost("rs-new"), attachItemCreated("item-new"))
			},
			wantStdout: []string{`"changed":true`, `"reviewSubmissionId":"rs-new"`, `"reviewSubmissionCreated":true`},
			wantStderr: []string{"Created review submission rs-new (MAC_OS) for app app-1; it is a draft and is not submitted for review."},
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
			wantErr:        []string{"--review-items` uses the App Store Connect API key, not the web session", "asc auth login"},
		},
		{
			name: "first subscription travels with the app version and the group version",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerFirstAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED", "limit=200"), asctest.JSON(http.StatusOK, attachAppVersionsBody("app-v1")))
				reg.Register(attachTypedItemPost("appStoreVersion", "appStoreVersions", "app-v1"), attachItemCreated("item-app"))
				reg.Register(attachTypedItemPost("subscriptionGroupVersion", "subscriptionGroupVersions", "gv-1"), attachItemCreated("item-group"))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout: []string{
				`{"itemType":"appStoreVersions","resourceId":"app-v1","reviewSubmissionItemId":"item-app","added":true}`,
				`{"itemType":"subscriptionGroupVersions","resourceId":"gv-1","reviewSubmissionItemId":"item-group","added":true}`,
				`{"itemType":"subscriptionVersions","resourceId":"ver-1","reviewSubmissionItemId":"item-1","subscriptionId":"sub-1","added":true}`,
			},
			wantStderr: []string{"first subscription"},
		},
		{
			name: "first subscription joins the draft that holds the app version",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerFirstAttachLookups(reg)
				twoItems := `{"data":[` +
					`{"type":"reviewSubmissionItems","id":"item-app","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"app-v1"}}}},` +
					`{"type":"reviewSubmissionItems","id":"item-group","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"gv-1"}}}}` +
					`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/rs-2/items"},"meta":{"paging":{"total":2,"limit":200}}}`
				registerAttachDrafts(reg, map[string]string{"rs-2": twoItems}, "rs-1", "rs-2")
				reg.Register(attachItemPost("rs-2"), attachItemCreated("item-1"))
			},
			wantStdout: []string{`"reviewSubmissionId":"rs-2"`, `{"itemType":"appStoreVersions","resourceId":"app-v1","reviewSubmissionItemId":"item-app","added":false}`, `{"itemType":"subscriptionGroupVersions","resourceId":"gv-1","reviewSubmissionItemId":"item-group","added":false}`},
		},
		{
			name: "a never-approved group travels with its subscription even when the app has an approved one",
			args: []string{"attach", "--subscription-id", "sub-9"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}},{"type":"subscriptionGroups","id":"group-2","attributes":{"referenceName":"New"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-4","attributes":{"name":"Legacy","productId":"com.example.legacy","state":"APPROVED"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-2/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-9","attributes":{"name":"Fresh","productId":"com.example.fresh","state":"READY_TO_SUBMIT"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-2/subscriptions"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-1", "APPROVED")))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-2/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-2", "PREPARE_FOR_SUBMISSION")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-9/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(attachTypedItemPost("subscriptionGroupVersion", "subscriptionGroupVersions", "gv-2"), attachItemCreated("item-group"))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout:  []string{`{"itemType":"subscriptionGroupVersions","resourceId":"gv-2","reviewSubmissionItemId":"item-group","added":true}`, `"subscriptionId":"sub-9"`},
			rejectInOut: []string{`appStoreVersions`},
		},
		{
			name: "an approved group version alone stops the app counting as first",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, attachGroupsBody))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"READY_TO_SUBMIT"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-1", "APPROVED")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout:  []string{`"items":[{"itemType":"subscriptionVersions","resourceId":"ver-1","reviewSubmissionItemId":"item-1","subscriptionId":"sub-1","added":true}]`},
			rejectInOut: []string{`appStoreVersions`, `subscriptionGroupVersions`},
		},
		{
			name: "a subscription approved once and now removed from sale alone stops the app counting as first",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, attachGroupsBody))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","state":"READY_TO_SUBMIT"}},{"type":"subscriptions","id":"sub-5","attributes":{"name":"Old","productId":"com.example.old","state":"DEVELOPER_REMOVED_FROM_SALE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachGroupVersionsBody("gv-1", "PREPARE_FOR_SUBMISSION")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(attachTypedItemPost("subscriptionGroupVersion", "subscriptionGroupVersions", "gv-1"), attachItemCreated("item-group"))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout:  []string{`{"itemType":"subscriptionGroupVersions","resourceId":"gv-1","reviewSubmissionItemId":"item-group","added":true}`, `"subscriptionId":"sub-1"`},
			rejectInOut: []string{`appStoreVersions`},
		},
		{
			name: "rejects --platform that does not match the draft already holding the version",
			args: []string{"attach", "--subscription-id", "sub-1", "--platform", "MAC_OS"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "READY_FOR_REVIEW")))
				registerAttachDrafts(reg, map[string]string{"rs-2": attachItemsBody("rs-2", "item-1", "ver-1")}, "rs-2")
			},
			wantUsage:  true,
			wantStderr: []string{"subscription \"sub-1\" is already on review submission rs-2 (IOS), not on --platform MAC_OS"},
		},
		{
			name:       "attach-group: --submission without --review-items is a usage error",
			args:       []string{"attach-group", "--group-id", "group-1", "--submission", "rs-1"},
			register:   func(reg *asctest.Registry) {},
			webFlow:    true,
			wantUsage:  true,
			wantStderr: []string{"--submission and --platform require --review-items"},
		},
		{
			name: "first subscription removes what it added when a later item fails",
			args: []string{"attach", "--subscription-id", "sub-1", "--submission", "rs-1"},
			register: func(reg *asctest.Registry) {
				registerFirstAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED", "limit=200"), asctest.JSON(http.StatusOK, attachAppVersionsBody("app-v1")))
				reg.Register(attachTypedItemPost("appStoreVersion", "appStoreVersions", "app-v1"), attachItemCreated("item-app"))
				reg.Register(attachTypedItemPost("subscriptionGroupVersion", "subscriptionGroupVersions", "gv-1"), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"subscriptionGroupVersions with id 'gv-1' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why."}]}`))
				reg.Register(asctest.DELETE("/v1/reviewSubmissionItems/item-app"), asctest.JSON(http.StatusNoContent, ``))
			},
			wantErr: []string{"subscriptionGroupVersions with id 'gv-1' is not in valid state", "removed review item item-app (appStoreVersions app-v1), which this run added"},
		},
		{
			name: "first subscription without an editable app version is not ready",
			args: []string{"attach", "--subscription-id", "sub-1"},
			register: func(reg *asctest.Registry) {
				registerFirstAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED", "limit=200"), asctest.JSON(http.StatusOK, attachAppVersionsBody()))
			},
			wantErr:        []string{"app-1 has no approved subscription yet, so App Store Connect reviews this one with an app version", "asc versions create"},
			wantDiagnostic: "state_not_ready",
		},
		{
			name:       "--submission without --review-items is a usage error",
			args:       []string{"attach", "--subscription-id", "sub-1", "--submission", "rs-1"},
			register:   func(reg *asctest.Registry) {},
			webFlow:    true,
			wantUsage:  true,
			wantStderr: []string{"--submission and --platform require --review-items"},
		},
		{
			name:       "--review-items refuses web session flags",
			args:       []string{"attach", "--subscription-id", "sub-1", "--apple-id", "dev@example.com"},
			register:   func(reg *asctest.Registry) {},
			wantUsage:  true,
			wantStderr: []string{"--apple-id belongs to the web-session flow; drop it with --review-items"},
		},
		{
			name: "attach-group removes what it added when a later subscription fails",
			args: []string{"attach-group", "--group-id", "group-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-3/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-3", "PREPARE_FOR_SUBMISSION")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-4/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-4", "APPROVED")))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
				reg.Register(attachTypedItemPost("subscriptionVersion", "subscriptionVersions", "ver-3"), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"subscriptionVersions with id 'ver-3' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why."}]}`))
				reg.Register(asctest.DELETE("/v1/reviewSubmissionItems/item-1"), asctest.JSON(http.StatusNoContent, ``))
			},
			wantErr: []string{"subscriptionVersions with id 'ver-3' is not in valid state", "removed review item item-1 (subscriptionVersions ver-1), which this run added"},
		},
		{
			name: "rejects a --submission that is not a draft of the app",
			args: []string{"attach", "--subscription-id", "sub-1", "--submission", "rs-other"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "PREPARE_FOR_SUBMISSION")))
				registerAttachDrafts(reg, nil, "rs-1")
			},
			wantUsage:  true,
			wantStderr: []string{"--submission must identify a READY_FOR_REVIEW review submission of --app"},
		},
		{
			name: "attach-group reports a group with nothing to add as not ready",
			args: []string{"attach-group", "--group-id", "group-1"},
			register: func(reg *asctest.Registry) {
				registerAttachLookups(reg)
				registerAttachDrafts(reg, nil, "rs-1")
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-1", "WAITING_FOR_REVIEW")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-3/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-3", "IN_REVIEW")))
				reg.Register(asctest.GET("/v1/subscriptions/sub-4/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-4", "APPROVED")))
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
				reg.Register(asctest.GET("/v1/subscriptions/sub-4/versions", "limit=200"), asctest.JSON(http.StatusOK, attachVersionsBody("ver-4", "APPROVED")))
				reg.Register(attachItemPost("rs-1"), attachItemCreated("item-1"))
			},
			wantStdout:  []string{`"operation":"attach-group"`, `"reviewSubmissionId":"rs-1"`, `"changedCount":1`, `"skippedCount":3`, `"reason":"state is MISSING_METADATA`, `"reason":"already on review submission rs-1"`, `"reason":"no version that can be submitted (version states: APPROVED)"`},
			rejectInOut: []string{`"submissionId"`},
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
			if !tt.webFlow {
				args = append(args, "--review-items")
			}
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
			for _, reject := range tt.rejectInOut {
				if strings.Contains(stdout, reject) {
					t.Fatalf("expected stdout not to contain %q, got %s", reject, stdout)
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
