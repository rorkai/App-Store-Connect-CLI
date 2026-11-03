package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestListAppBundlesJoinsReviewStateAcrossPages(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/appBundles" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != "appBundleVersions" {
			t.Errorf("include = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			_, _ = w.Write([]byte(`{
				"data": [{"type": "appBundles", "id": "1501593771",
					"attributes": {"name": "Colorway", "platforms": ["IOS", "MAC_OS"]},
					"relationships": {"appBundleVersions": {"data": [{"type": "appBundleVersions", "id": "v-1"}]}}}],
				"included": [{"type": "appBundleVersions", "id": "v-1", "attributes": {"bundleReviewState": "DEVELOPER_REMOVED_FROM_SALE"}}],
				"links": {"next": "` + server.URL + `/appBundles?include=appBundleVersions&limit=300&cursor=abc"}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{"data": [{"type": "appBundles", "id": "2", "attributes": {"name": "No Versions"}}]}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).ListAppBundles(context.Background())
	if err != nil {
		t.Fatalf("ListAppBundles() error = %v", err)
	}
	want := []AppBundle{
		{
			ID: "1501593771", Name: "Colorway", Platforms: []string{"IOS", "MAC_OS"}, Status: "DEVELOPER_REMOVED_FROM_SALE",
			Versions: []AppBundleVersion{{ID: "v-1", ReviewState: "DEVELOPER_REMOVED_FROM_SALE"}},
		},
		{ID: "2", Name: "No Versions"},
	}
	if !reflect.DeepEqual(got.Data, want) {
		t.Fatalf("bundles = %+v, want %+v", got.Data, want)
	}
}

func TestGetAppBundleKeepsMemberOrderAndRenamesReviewFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/WebObjects/iTunesConnect.woa/ra/appbundles/metadetail/1501593771" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {
			"state": "Developer Removed from Sale", "stateKey": "developerRemovedFromSale", "inFlightState": null,
			"primaryLocaleCode": "en-GB", "priceTierStem": "530",
			"canSubmit": false, "canDevReject": true, "clearedForSale": false, "clearedForSaleEditable": true,
			"memberApps": [
				{"adamId": 1500196580, "name": "Second Listed First", "appType": "ios", "priceTier": "Free"},
				{"adamId": 1479784361, "name": "Gradient Game", "appType": "ios", "priceTier": "Free"}
			]
		}, "messages": {"warn": null, "error": null, "info": null}, "statusCode": "SUCCESS"}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppBundle(context.Background(), " 1501593771 ")
	if err != nil {
		t.Fatalf("GetAppBundle() error = %v", err)
	}
	if got.ID != "1501593771" || got.StateKey != "developerRemovedFromSale" || got.PriceTier != "530" || !got.CanRemoveFromReview || !got.ClearedForSaleEditable {
		t.Fatalf("unexpected detail: %+v", got)
	}
	if len(got.Apps) != 2 || got.Apps[0].ID != "1500196580" || got.Apps[1].ID != "1479784361" {
		t.Fatalf("apps must keep Apple's order: %+v", got.Apps)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fields["canRemoveFromReview"] != true || fields["clearedForSale"] != false {
		t.Fatalf("false booleans must stay in the JSON contract: %s", encoded)
	}
}

func TestGetAppBundleReportsLegacyFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"error on HTTP 200", http.StatusOK, `{"data": null, "messages": {"error": ["Bundle unavailable"]}, "statusCode": "ERROR"}`, "Bundle unavailable"},
		{"error with HTML", http.StatusOK, `{"data": null, "messages": {"error": ["Each app must offer a subscription.<ul><a href=\"/ra/ng/app/1\">Quoting</a><br><a href=\"/ra/ng/app/2\">Tips &amp; More</a></ul>"]}, "statusCode": "ERROR"}`, "subscription. Quoting Tips & More"},
		{"status without message", http.StatusOK, `{"data": null, "messages": {"error": null}, "statusCode": "ERROR"}`, `"ERROR"`},
		{"success without data", http.StatusOK, `{"data": null, "messages": {"error": null}, "statusCode": "SUCCESS"}`, "missing data"},
		{"unknown bundle", http.StatusNotFound, `{"data": null, "messages": {"error": ["Not Found"]}, "statusCode": "ERROR"}`, "404"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := testWebClient(server).GetAppBundle(context.Background(), "1")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

type appBundleSaveServer struct {
	t         *testing.T
	stateKey  string
	flags     string
	saveError string
	members   []string
	saves     []map[string]any
	headers   []http.Header
	requests  []string
}

func (s *appBundleSaveServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.requests = append(s.requests, r.Method+" "+strings.TrimPrefix(r.URL.Path, "/WebObjects/iTunesConnect.woa/ra"))
	switch r.Method + " " + r.URL.Path {
	case "GET /WebObjects/iTunesConnect.woa/ra/appbundles/metadetail/1501593771":
		apps := make([]string, 0, len(s.members))
		for _, id := range s.members {
			apps = append(apps, `{"adamId": `+id+`, "name": "App `+id+`", "appType": "ios", "priceTier": "Free", "iconUrl": "x", "versions": []}`)
		}
		_, _ = w.Write([]byte(`{"data": {
			"adamId": 1501593771, "vendorIdentifier": "COLORWAY", "primaryLocaleCode": "en-GB", "priceTierStem": "530",
			"clearedForSale": false, "stateKey": "` + s.stateKey + `", ` + s.flags + `
			"details": [{"name": "Colorway", "localeCode": "en-GB", "approvedDetail": null, "empty": false,
				"marketingText": {"value": "Colour games", "isEditable": true}, "marketingUrl": {"value": null, "isEditable": true}}],
			"userBundleIcon": {"value": {"assetToken": "token"}, "isEditable": true},
			"memberApps": [` + strings.Join(apps, ",") + `]
		}, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
	case "GET /WebObjects/iTunesConnect.woa/ra/apps/approvedForBundles":
		if got := r.URL.Query().Get("platform"); got != "IOS,OSX" {
			s.t.Errorf("platform = %q", got)
		}
		_, _ = w.Write([]byte(`{"data": [{"adamId": 3, "name": "Three", "bundleId": null, "vendorId": null, "appType": null, "iconUrl": "x", "priceTier": null, "versionSets": [{"platformString": "IOS"}], "isAAG": false}], "messages": {"error": null}, "statusCode": "SUCCESS"}`))
	case "POST /WebObjects/iTunesConnect.woa/ra/appbundles/createOrUpdate":
		var body map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&body); err != nil {
			s.t.Errorf("decode save: %v", err)
		}
		s.saves = append(s.saves, body)
		s.headers = append(s.headers, r.Header.Clone())
		if s.saveError != "" {
			_, _ = w.Write([]byte(`{"data": null, "messages": {"error": ["` + s.saveError + `"]}, "statusCode": "ERROR"}`))
			return
		}
		s.members = s.members[:0]
		for _, app := range body["memberApps"].([]any) {
			s.members = append(s.members, app.(map[string]any)["adamId"].(json.Number).String())
		}
		_, _ = w.Write([]byte(`{"data": null, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSetAppBundleAppsReorderSendsStoredBundleBack(t *testing.T) {
	handler := &appBundleSaveServer{t: t, stateKey: "developerRemovedFromSale", members: []string{"1", "2"}}
	server := httptest.NewServer(handler)
	defer server.Close()

	got, err := testWebClient(server).SetAppBundleApps(context.Background(), "1501593771", []string{"2", "1"})
	if err != nil {
		t.Fatalf("SetAppBundleApps() error = %v", err)
	}
	if !got.Changed || !reflect.DeepEqual(got.PreviousApps, []string{"1", "2"}) || !reflect.DeepEqual(got.Apps, []string{"2", "1"}) {
		t.Fatalf("unexpected receipt: %+v", got)
	}
	if len(handler.saves) != 1 {
		t.Fatalf("saves = %d, want 1", len(handler.saves))
	}
	if csrf := handler.headers[0].Get("X-Csrf-Itc"); csrf != "itc" {
		t.Fatalf("X-Csrf-Itc = %q", csrf)
	}
	encoded, err := json.Marshal(handler.saves[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"adamId":1501593771,"clearedForSale":false,` +
		`"details":[{"localeCode":"en-GB","marketingText":"Colour games","marketingUrl":null,"name":"Colorway"}],` +
		`"memberApps":[{"adamId":2,"priceTier":"Free"},{"adamId":1,"priceTier":"Free"}],` +
		`"priceTierStem":"530","primaryLocaleCode":"en-GB",` +
		`"userBundleIcon":{"isEditable":true,"value":{"assetToken":"token"}},"vendorIdentifier":"COLORWAY"}`
	if string(encoded) != want {
		t.Fatalf("save body =\n%s\nwant\n%s", encoded, want)
	}
}

func TestSetAppBundleAppsGuards(t *testing.T) {
	tests := []struct {
		name      string
		stateKey  string
		flags     string
		apps      []string
		saveError string
		wantErr   string
		wantSaves int
		eligible  bool
	}{
		{name: "matching list sends nothing", stateKey: "readyForSale", apps: []string{"1", "2"}},
		{name: "add after approval", stateKey: "readyForSale", apps: []string{"1", "2", "3"}, wantErr: "only allows reordering"},
		{name: "remove after approval", stateKey: "developerRemovedFromSale", apps: []string{"1"}, wantErr: "only allows reordering"},
		{name: "add while released but pending a contract", stateKey: "pendingContract", apps: []string{"1", "2", "3"}, wantErr: "only allows reordering"},
		{name: "remove after Apple took it off sale", stateKey: "removedFromSale", apps: []string{"1"}, wantErr: "only allows reordering"},
		{name: "swap after approval", stateKey: "readyForSale", apps: []string{"1", "3"}, wantErr: "only allows reordering"},
		{name: "reorder in review", stateKey: "inReview", apps: []string{"2", "1"}, wantErr: "remove it from review"},
		{name: "reorder while a newer version waits for review", stateKey: "readyForSale", flags: `"inFlightState": "Waiting for Review",`, apps: []string{"2", "1"}, wantErr: "remove it from review"},
		{name: "reorder while Apple offers withdrawal", stateKey: "readyForSale", flags: `"canDevReject": true,`, apps: []string{"2", "1"}, wantErr: "remove it from review"},
		{name: "reorder after a rejected version", stateKey: "readyForSale", flags: `"inFlightState": "Rejected",`, apps: []string{"2", "1"}, wantSaves: 1},
		{name: "add ineligible app", stateKey: "prepareForSubmission", apps: []string{"1", "2", "9"}, wantErr: "9 is not eligible", eligible: true},
		{name: "add eligible app", stateKey: "prepareForSubmission", apps: []string{"3", "2", "1"}, wantSaves: 1, eligible: true},
		{name: "remove before approval", stateKey: "prepareForSubmission", apps: []string{"2"}, wantSaves: 1},
		{name: "apple refuses", stateKey: "prepareForSubmission", apps: []string{"2", "1"}, saveError: "Bundle is invalid", wantErr: "Bundle is invalid", wantSaves: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &appBundleSaveServer{t: t, stateKey: test.stateKey, flags: test.flags, saveError: test.saveError, members: []string{"1", "2"}}
			server := httptest.NewServer(handler)
			defer server.Close()

			_, err := testWebClient(server).SetAppBundleApps(context.Background(), "1501593771", test.apps)
			if test.wantErr == "" && err != nil {
				t.Fatalf("SetAppBundleApps() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if len(handler.saves) != test.wantSaves {
				t.Fatalf("saves = %d, want %d (requests %v)", len(handler.saves), test.wantSaves, handler.requests)
			}
			if asked := slices.Contains(handler.requests, "GET /apps/approvedForBundles"); asked != test.eligible {
				t.Fatalf("eligibility lookup = %v, want %v", asked, test.eligible)
			}
		})
	}
}

func TestSetAppBundleAppsSendsAddedAppLikeThePage(t *testing.T) {
	handler := &appBundleSaveServer{t: t, stateKey: "prepareForSubmission", members: []string{"1", "2"}}
	server := httptest.NewServer(handler)
	defer server.Close()

	if _, err := testWebClient(server).SetAppBundleApps(context.Background(), "1501593771", []string{"1", "2", "3"}); err != nil {
		t.Fatalf("SetAppBundleApps() error = %v", err)
	}
	encoded, err := json.Marshal(handler.saves[0]["memberApps"].([]any)[2])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"adamId":3,"isAAG":false,"priceTier":null,"versionSets":[{"platformString":"IOS"}]}`
	if string(encoded) != want {
		t.Fatalf("added app = %s, want %s", encoded, want)
	}
}

func TestSetAppBundleAppsRejectsReadbackMismatch(t *testing.T) {
	handler := &appBundleSaveServer{t: t, stateKey: "prepareForSubmission", members: []string{"1", "2"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := slices.Clone(handler.members)
		handler.ServeHTTP(w, r)
		if r.Method == http.MethodPost {
			handler.members = before
		}
	}))
	defer server.Close()

	result, err := testWebClient(server).SetAppBundleApps(context.Background(), "1501593771", []string{"2", "1"})
	if err == nil || !strings.Contains(err.Error(), "readback") || result != nil {
		t.Fatalf("result = %+v, error = %v, want readback error", result, err)
	}
}

func TestAppBundleReviewActions(t *testing.T) {
	type call func(*Client, context.Context, string) (*asc.WebAppBundleReviewResult, error)
	submit, remove := call((*Client).SubmitAppBundle), call((*Client).RemoveAppBundleFromReview)
	tests := []struct {
		name         string
		run          call
		flags        string
		afterFlags   string
		before       string
		after        string
		postBody     string
		wantPath     string
		wantErr      string
		wantState    string
		wantInFlight string
		wantNoPost   bool
	}{
		{name: "submit", run: submit, flags: `"canSubmit": true`, before: "prepareForSubmission", after: "waitingForReview", wantPath: "/submit/1501593771", wantState: "waitingForReview"},
		{name: "remove", run: remove, flags: `"canDevReject": true`, before: "waitingForReview", after: "developerRejected", afterFlags: `"canSubmit": true`, wantPath: "/devreject/1501593771", wantState: "developerRejected"},
		{name: "remove still in review", run: remove, flags: `"canDevReject": true`, before: "waitingForReview", after: "inReview", afterFlags: `"canDevReject": false`, wantPath: "/devreject/1501593771", wantErr: "readback"},
		{name: "submit ends somewhere unrelated", run: submit, flags: `"canSubmit": true`, before: "prepareForSubmission", after: "readyForSale", wantPath: "/submit/1501593771", wantErr: "readback"},
		{name: "submit a new version of a released bundle", run: submit, flags: `"canSubmit": true`, before: "readyForSale", after: "readyForSale", afterFlags: `"inFlightState": "Waiting for Review"`, wantPath: "/submit/1501593771", wantState: "readyForSale", wantInFlight: "Waiting for Review"},
		{name: "submit not offered", run: submit, flags: `"canDevReject": true`, before: "developerRemovedFromSale", wantErr: "cannot be submitted", wantNoPost: true},
		{name: "remove not offered", run: remove, flags: `"canSubmit": true`, before: "prepareForSubmission", wantErr: "cannot be removed", wantNoPost: true},
		{name: "apple refuses", run: submit, flags: `"canSubmit": true`, before: "prepareForSubmission", postBody: `{"data": null, "messages": {"error": ["Missing metadata"]}, "statusCode": "ERROR"}`, wantPath: "/submit/1501593771", wantErr: "Missing metadata"},
		{name: "state did not change", run: submit, flags: `"canSubmit": true`, before: "prepareForSubmission", after: "prepareForSubmission", wantPath: "/submit/1501593771", wantErr: "readback"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var posts []string
			var bodySizes []int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					posts = append(posts, strings.TrimPrefix(r.URL.Path, "/WebObjects/iTunesConnect.woa/ra/appbundles"))
					bodySizes = append(bodySizes, r.ContentLength)
					if csrf := r.Header.Get("X-Csrf-Itc"); csrf != "itc" {
						t.Errorf("X-Csrf-Itc = %q", csrf)
					}
					if test.postBody != "" {
						_, _ = w.Write([]byte(test.postBody))
						return
					}
					_, _ = w.Write([]byte(`{"data": null, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
					return
				}
				state, flags := test.before, test.flags
				if len(posts) > 0 {
					state, flags = test.after, test.afterFlags
					if flags == "" {
						flags = `"canSubmit": false`
					}
				}
				_, _ = w.Write([]byte(`{"data": {"stateKey": "` + state + `", ` + flags + `}, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
			}))
			defer server.Close()

			got, err := test.run(testWebClient(server), context.Background(), "1501593771")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil || got.State != test.wantState || got.PreviousState != test.before || got.InFlightState != test.wantInFlight {
				t.Fatalf("result = %+v, error = %v", got, err)
			}
			if test.wantNoPost {
				if len(posts) != 0 {
					t.Fatalf("posts = %v, want none", posts)
				}
				return
			}
			if len(posts) != 1 || posts[0] != test.wantPath || bodySizes[0] > 0 {
				t.Fatalf("posts = %v sizes %v, want one bodyless %s", posts, bodySizes, test.wantPath)
			}
		})
	}
}

func TestCreateAppBundleSendsPageBodyAndReadsBack(t *testing.T) {
	tests := []struct {
		name      string
		priceTier string
		readback  string
		wantTier  string
		wantErr   string
	}{
		{name: "defaults to the first compatible tier", readback: "3,4", wantTier: "10000"},
		{name: "keeps a chosen tier", priceTier: "10003", readback: "3,4", wantTier: "10003"},
		{name: "readback mismatch", readback: "4,3", wantTier: "10000", wantErr: "created app bundle 6830005835, but its apps read back as 4,3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var saves []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + strings.TrimPrefix(r.URL.Path, "/WebObjects/iTunesConnect.woa/ra") {
				case "GET /apps/approvedForBundles":
					_, _ = w.Write([]byte(`{"data": [
						{"adamId": 3, "name": "Three", "iconUrl": "x", "priceTier": null, "isAAG": false},
						{"adamId": 4, "name": "Four", "iconUrl": "x", "priceTier": null, "isAAG": false}
					], "messages": {"error": null}, "statusCode": "SUCCESS"}`))
				case "GET /appbundles/metareference":
					if got := r.URL.Query().Get("adamIds"); got != "3,4" {
						t.Errorf("adamIds = %q", got)
					}
					_, _ = w.Write([]byte(`{"data": {"primaryLocaleCode": "en-GB",
						"locales": [{"name": "UK English", "code": "en-GB"}, {"name": "French", "code": "fr-FR"}],
						"defaultDetails": [{"name": null, "localeCode": "en-GB", "marketingText": {"value": "Quotes"}, "marketingUrl": {"value": null}}],
						"validPriceTiers": [{"tierStem": "10000"}], "invalidPriceTiers": [{"tierStem": "10001"}]
					}, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
				case "POST /appbundles/createOrUpdate":
					if csrf := r.Header.Get("X-Csrf-Itc"); csrf != "itc" {
						t.Errorf("X-Csrf-Itc = %q", csrf)
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode save: %v", err)
					}
					encoded, _ := json.Marshal(body)
					saves = append(saves, string(encoded))
					_, _ = w.Write([]byte(`{"data": {"adamId": 6830005835}, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
				case "GET /appbundles/metadetail/6830005835":
					apps := make([]string, 0, 2)
					for id := range strings.SplitSeq(test.readback, ",") {
						apps = append(apps, `{"adamId": `+id+`}`)
					}
					_, _ = w.Write([]byte(`{"data": {"stateKey": "prepareForSubmission", "canSubmit": true, "memberApps": [` + strings.Join(apps, ",") + `]}, "messages": {"error": null}, "statusCode": "SUCCESS"}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			got, err := testWebClient(server).CreateAppBundle(context.Background(), " Test Bundle ", "TEST_BUNDLE", []string{"3", "4"}, test.priceTier)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil || got.ID != "6830005835" || !got.CanSubmit {
				t.Fatalf("CreateAppBundle() = %+v, %v", got, err)
			}
			want := `{"clearedForSale":false,` +
				`"details":[{"localeCode":"en-GB","marketingText":"Quotes","marketingUrl":null,"name":"Test Bundle"},` +
				`{"localeCode":"fr-FR","marketingText":null,"marketingUrl":"","name":""}],` +
				`"memberApps":[{"adamId":3,"isAAG":false,"priceTier":null},{"adamId":4,"isAAG":false,"priceTier":null}],` +
				`"priceTierStem":"` + test.wantTier + `","primaryLocaleCode":"en-GB","userBundleIcon":{"value":null},"vendorIdentifier":"TEST_BUNDLE"}`
			if len(saves) != 1 || saves[0] != want {
				t.Fatalf("saves =\n%v\nwant\n%s", saves, want)
			}
		})
	}
}
