package validate

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestIAPAttachmentEvidence(t *testing.T) {
	const versions = `{"data":[{"type":"inAppPurchaseVersions","id":"iap-v1","attributes":{"version":1,"state":"READY_FOR_REVIEW"}}],"links":{}}`
	const draft = `{"data":[{"type":"reviewSubmissions","id":"draft-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersionForReview":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}],"links":{}}`
	appItem := `{"type":"reviewSubmissionItems","id":"app-item","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}},"inAppPurchaseVersion":{"data":null}}}`
	iapItem := `{"type":"reviewSubmissionItems","id":"iap-item","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":null},"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iap-v1"}}}}`
	items := func(rows ...string) string { return `{"data":[` + strings.Join(rows, ",") + `],"links":{}}` }
	baseCatalog := []validation.IAP{{ID: "iap-1", ProductID: "com.example.item", Type: "NON_CONSUMABLE", State: "READY_TO_SUBMIT"}}
	tests := []struct {
		name, versions, drafts, items, later, appState, siblingVersions string
		catalog                                                         []validation.IAP
		want                                                            validation.DeepStatus
		command                                                         bool
	}{
		{name: "first prepare version can attach through web", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "PREPARE_FOR_SUBMISSION"), items: items(appItem), want: validation.DeepStatusBlocked, command: true},
		{name: "subsequent type prepare version can attach through web", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "approved", ProductID: "com.example.approved", Type: "CONSUMABLE", State: "APPROVED"}), versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "PREPARE_FOR_SUBMISSION"), items: items(appItem), want: validation.DeepStatusBlocked, command: true},
		{name: "later version approval", versions: strings.ReplaceAll(versions, `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?page=2"}`), later: strings.ReplaceAll(strings.ReplaceAll(versions, "iap-v1", "approved-v"), "READY_FOR_REVIEW", "APPROVED"), want: validation.DeepStatusPassed},
		{name: "malformed later version", versions: strings.ReplaceAll(versions, `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?page=2"}`), later: `{"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "same type submitted product satisfies unused ready", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "iap-submitted", ProductID: "com.example.submitted", Type: "NON_CONSUMABLE", State: "IN_REVIEW"}), items: items(appItem, strings.ReplaceAll(iapItem, "iap-v1", "submitted-v")), want: validation.DeepStatusPassed},
		{name: "terminal app version", appState: "READY_FOR_SALE", want: validation.DeepStatusNotApplicable},
		{name: "unknown app state", appState: "FUTURE_STATE", want: validation.DeepStatusUnverified},
		{name: "missing version number with membership", versions: strings.ReplaceAll(versions, `"version":1,`, ``), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "submissions forbidden", drafts: "API_ERROR", want: validation.DeepStatusUnverified},
		{name: "approval requires no submission access", drafts: "API_ERROR", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "approved", ProductID: "com.example.approved", Type: "NON_CONSUMABLE", State: "APPROVED"}), want: validation.DeepStatusPassed},
		{name: "multiple ready candidates manual choice", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "iap-extra", ProductID: "com.example.extra", Type: "NON_CONSUMABLE", State: "READY_TO_SUBMIT"}), items: items(appItem), want: validation.DeepStatusBlocked},
		{name: "unknown ready type cannot hide behind approved type", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "approved", ProductID: "com.example.approved", Type: "NON_CONSUMABLE", State: "APPROVED"}, validation.IAP{ID: "unknown", ProductID: "com.example.unknown", State: "READY_TO_SUBMIT"}), want: validation.DeepStatusUnverified},
		{name: "contradictory app version item", items: items(appItem, strings.ReplaceAll(strings.ReplaceAll(appItem, "app-item", "other-app-item"), "ver-1", "ver-other"), iapItem), want: validation.DeepStatusUnverified},
		{name: "one item with two content identities", items: items(strings.ReplaceAll(appItem, `"inAppPurchaseVersion":{"data":null}`, `"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iap-v1"}}`)), want: validation.DeepStatusUnverified},
		{name: "accepted same-type submitted product satisfies", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "ACCEPTED"), catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "iap-submitted", ProductID: "com.example.submitted", Type: "NON_CONSUMABLE", State: "PENDING_BINARY_APPROVAL"}), items: items(appItem, strings.ReplaceAll(iapItem, "iap-v1", "submitted-v")), want: validation.DeepStatusPassed},
		{name: "newer rejected version prevents stale pass", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"version":2,"state":"REJECTED"}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "conflicting duplicate item on later page", items: strings.ReplaceAll(items(appItem, iapItem), `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/draft-1/items?page=2"}`), later: items(strings.ReplaceAll(iapItem, "READY_FOR_REVIEW", "REMOVED")), want: validation.DeepStatusUnverified},
		{name: "identical duplicate item on later page", items: strings.ReplaceAll(items(appItem, iapItem), `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/draft-1/items?page=2"}`), later: items(iapItem), want: validation.DeepStatusPassed},
		{name: "later advertised item truncation", items: strings.ReplaceAll(items(appItem), `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/draft-1/items?page=2"}`), later: strings.ReplaceAll(items(iapItem), `"links":{}`, `"links":{},"meta":{"paging":{"total":3,"limit":200}}`), want: validation.DeepStatusUnverified},
		{name: "later advertised version truncation", versions: strings.ReplaceAll(versions, `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?page=2"}`), later: strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(versions, "iap-v1", "approved-v"), "READY_FOR_REVIEW", "APPROVED"), `"links":{}`, `"links":{},"meta":{"paging":{"total":3,"limit":200}}`), want: validation.DeepStatusUnverified},
		{name: "conflicting duplicate submission on later page", drafts: strings.ReplaceAll(draft, `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?page=2"}`), later: strings.ReplaceAll(draft, "ver-1", "ver-other"), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "later advertised submission truncation", drafts: strings.ReplaceAll(draft, `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?page=2"}`), later: strings.ReplaceAll(strings.ReplaceAll(draft, "draft-1", "draft-2"), `"links":{}`, `"links":{},"meta":{"paging":{"total":3,"limit":200}}`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "accepted alone cannot prove approval history", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "ACCEPTED"), want: validation.DeepStatusUnverified},
		{name: "newer unknown lifecycle invalidates stale membership", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"version":2,"state":"FUTURE_STATE"}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "newer missing state invalidates stale membership", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"version":2}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "missing version number invalidates stale membership", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "verified sibling satisfies despite unknown product history", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"version":2,"state":"FUTURE_STATE"}}],"links"`), siblingVersions: strings.ReplaceAll(strings.ReplaceAll(versions, "iap-v1", "submitted-v"), "READY_FOR_REVIEW", "IN_REVIEW"), catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "iap-submitted", ProductID: "com.example.submitted", Type: "NON_CONSUMABLE", State: "IN_REVIEW"}), items: items(appItem, strings.ReplaceAll(iapItem, "iap-v1", "submitted-v")), want: validation.DeepStatusPassed},
		{name: "known replaced history does not hide exact current version", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-old","attributes":{"version":1,"state":"REPLACED_WITH_NEW_VERSION"}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusPassed},
		{name: "missing with editable target", items: items(appItem), want: validation.DeepStatusBlocked, command: true},
		{name: "unrelated sparse item preserves exact membership", items: items(appItem, iapItem, `{"type":"reviewSubmissionItems","id":"event-item","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersion":{"data":null},"inAppPurchaseVersion":{"data":null}}}`), want: validation.DeepStatusPassed},
		{name: "exact membership", items: items(appItem, iapItem), want: validation.DeepStatusPassed},
		{name: "wrong IAP version", items: items(appItem, strings.ReplaceAll(iapItem, "iap-v1", "iap-old")), want: validation.DeepStatusBlocked, command: true},
		{name: "removed IAP item", items: items(appItem, strings.ReplaceAll(iapItem, "READY_FOR_REVIEW", "REMOVED")), want: validation.DeepStatusBlocked, command: true},
		{name: "removed app item", items: items(strings.ReplaceAll(appItem, "READY_FOR_REVIEW", "REMOVED"), iapItem), want: validation.DeepStatusBlocked},
		{name: "wrong draft app version", drafts: strings.ReplaceAll(draft, "ver-1", "other-version"), items: items(appItem, iapItem), want: validation.DeepStatusBlocked},
		{name: "wrong platform", drafts: strings.ReplaceAll(draft, "IOS", "MAC_OS"), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "no draft", drafts: `{"data":[],"links":{}}`, want: validation.DeepStatusBlocked},
		{name: "standalone draft explicit null", drafts: strings.ReplaceAll(draft, `{"type":"appStoreVersions","id":"ver-1"}`, `null`), want: validation.DeepStatusBlocked},
		{name: "draft linkage omitted", drafts: strings.ReplaceAll(draft, `"data":{"type":"appStoreVersions","id":"ver-1"}`, `"links":{}`), want: validation.DeepStatusUnverified},
		{name: "item linkage omitted", items: items(strings.ReplaceAll(appItem, `"inAppPurchaseVersion":{"data":null}`, `"inAppPurchaseVersion":{"links":{}}`)), want: validation.DeepStatusUnverified},
		{name: "ambiguous current versions", versions: strings.ReplaceAll(versions, `],"links"`, `,{"type":"inAppPurchaseVersions","id":"iap-v2","attributes":{"version":2,"state":"READY_FOR_REVIEW"}}],"links"`), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "old replaced is not current", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "REPLACED_WITH_NEW_VERSION"), items: items(appItem, iapItem), want: validation.DeepStatusUnverified},
		{name: "unknown version state", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "FUTURE_STATE"), want: validation.DeepStatusUnverified},
		{name: "missing versions data", versions: `{"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "missing version attributes", versions: `{"data":[{"type":"inAppPurchaseVersions","id":"iap-v1"}],"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "truncated advertised versions", versions: strings.ReplaceAll(versions, `"links":{}`, `"links":{},"meta":{"paging":{"total":2,"limit":200}}`), want: validation.DeepStatusUnverified},
		{name: "missing submission data", drafts: `{"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "missing item data", items: `{"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "later membership", items: strings.ReplaceAll(items(appItem), `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/draft-1/items?page=2"}`), later: items(iapItem), want: validation.DeepStatusPassed},
		{name: "malformed later membership", items: strings.ReplaceAll(items(appItem), `"links":{}`, `"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/draft-1/items?page=2"}`), later: `{"links":{}}`, want: validation.DeepStatusUnverified},
		{name: "prior same type approval", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "approved", ProductID: "com.example.approved", Type: "NON_CONSUMABLE", State: "APPROVED"}), want: validation.DeepStatusPassed},
		{name: "other type approval does not exempt", command: true, catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "approved", ProductID: "com.example.approved", Type: "CONSUMABLE", State: "APPROVED"}), want: validation.DeepStatusBlocked},
		{name: "removed history not proof", catalog: append(append([]validation.IAP{}, baseCatalog...), validation.IAP{ID: "removed", ProductID: "com.example.removed", Type: "NON_CONSUMABLE", State: "DEVELOPER_REMOVED_FROM_SALE"}), want: validation.DeepStatusUnverified},
		{name: "in review membership", versions: strings.ReplaceAll(versions, "READY_FOR_REVIEW", "IN_REVIEW"), drafts: strings.ReplaceAll(draft, "READY_FOR_REVIEW", "IN_REVIEW"), items: items(appItem, iapItem), appState: "IN_REVIEW", want: validation.DeepStatusPassed},
		{name: "in review missing avoids attach", items: items(appItem), appState: "IN_REVIEW", want: validation.DeepStatusBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.versions == "" {
				tt.versions = versions
			}
			if tt.drafts == "" {
				tt.drafts = draft
			}
			if tt.items == "" {
				tt.items = items(appItem)
			}
			if tt.catalog == nil {
				tt.catalog = baseCatalog
			}
			if tt.appState == "" {
				tt.appState = "PREPARE_FOR_SUBMISSION"
			}
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("validation wrote via %s", req.Method)
				}
				switch {
				case strings.HasSuffix(req.URL.Path, "/versions"):
					if req.URL.Query().Get("page") == "2" {
						return iapEvidenceResponse(tt.later)
					}
					if req.URL.Query().Get("filter[state]") != "" {
						t.Fatal("history filtered")
					}
					if strings.Contains(req.URL.Path, "iap-submitted") {
						if tt.siblingVersions != "" {
							return iapEvidenceResponse(tt.siblingVersions)
						}
						return iapEvidenceResponse(strings.ReplaceAll(tt.versions, "iap-v1", "submitted-v"))
					}
					return iapEvidenceResponse(tt.versions)
				case strings.HasSuffix(req.URL.Path, "/reviewSubmissions"):
					if req.URL.Query().Get("page") == "2" {
						return iapEvidenceResponse(tt.later)
					}
					if tt.drafts == "API_ERROR" {
						return buildsJSONResponse(403, `{"errors":[{"status":"403"}]}`)
					}
					if req.URL.Query().Get("include") != "appStoreVersionForReview" {
						t.Fatal("missing version include")
					}
					return iapEvidenceResponse(tt.drafts)
				case strings.HasSuffix(req.URL.Path, "/items"):
					if req.URL.Query().Get("page") != "2" && req.URL.Query().Get("include") != "appStoreVersion,inAppPurchaseVersion" {
						t.Fatal("item relationship linkage was not requested with include")
					}
					if req.URL.Query().Get("page") == "2" {
						return iapEvidenceResponse(tt.later)
					}
					return iapEvidenceResponse(tt.items)
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL)
				}
			}))
			report := validation.Report{AppID: "app-1", VersionID: "ver-1", Platform: "IOS", VersionState: tt.appState}
			check, finding := collectIAPAttachmentDeepCheck(context.Background(), client, report, tt.catalog, "")
			if check.Status != tt.want || check.Source != validation.DeepSourcePublicAPI {
				t.Fatalf("got %#v, want %s", check, tt.want)
			}
			hasCommand := check.Resolution != nil && len(check.Resolution.Commands) > 0
			if hasCommand != tt.command {
				t.Fatalf("remediation = %#v, want command %t", check.Resolution, tt.command)
			}
			if tt.command {
				wantCommand := `asc web review iaps attach --app "app-1" --iap-id "com.example.item" --submission "draft-1" --confirm`
				if check.Resolution.Fixability != validation.FixabilityWebFixable || len(check.Resolution.Commands) != 1 || check.Resolution.Commands[0] != wantCommand {
					t.Fatalf("recovery = %#v, want web-fixable with %q", check.Resolution, wantCommand)
				}
			} else if check.Resolution != nil && check.Resolution.Fixability != validation.FixabilityManual {
				t.Fatalf("unexpected actionable recovery: %#v", check.Resolution)
			}
			if tt.want == validation.DeepStatusBlocked || tt.want == validation.DeepStatusUnverified {
				if finding == nil {
					t.Fatal("missing finding")
				}
				report = validation.ApplyDeepValidation(report, validation.DeepReport{Checks: []validation.DeepCheck{check}}, []validation.CheckResult{*finding})
				if tt.want == validation.DeepStatusBlocked && report.Summary.Blocking != 1 {
					t.Fatalf("summary %#v", report.Summary)
				}
			} else if finding != nil {
				t.Fatalf("unexpected finding %#v", finding)
			}
		})
	}
}

func iapEvidenceResponse(body string) (*http.Response, error) {
	body = strings.ReplaceAll(body, `"links":{}`, `"links":{"self":"https://api.appstoreconnect.apple.com/test"}`)
	body = strings.ReplaceAll(body, `"links":{"next":`, `"links":{"self":"https://api.appstoreconnect.apple.com/test","next":`)
	return buildsJSONResponse(200, body)
}

func TestDeepIAPCatalogCompleteness(t *testing.T) {
	const row = `{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"com.example.item","inAppPurchaseType":"NON_CONSUMABLE","state":"READY_TO_SUBMIT"}}`
	for _, tc := range []struct {
		name, first, later string
		wantError          bool
	}{
		{name: "complete", first: `{"data":[` + row + `],"links":{}}`},
		{name: "missing data", first: `{"links":{}}`, wantError: true},
		{name: "advertised truncation", first: `{"data":[` + row + `],"links":{},"meta":{"paging":{"total":2,"limit":200}}`, wantError: true},
		{name: "later approval", first: `{"data":[` + row + `],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?page=2"}}`, later: `{"data":[{"type":"inAppPurchases","id":"iap-approved","attributes":{"productId":"com.example.approved","inAppPurchaseType":"NON_CONSUMABLE","state":"APPROVED"}}],"links":{}}`},
		{name: "later advertised catalog truncation", first: `{"data":[` + row + `],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?page=2"}}`, later: `{"data":[],"links":{},"meta":{"paging":{"total":3,"limit":200}}`, wantError: true},
		{name: "malformed later page", first: `{"data":[` + row + `],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?page=2"}}`, later: `{"links":{}}`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method %s", req.Method)
				}
				if req.URL.Query().Get("page") == "2" {
					return iapEvidenceResponse(tc.later)
				}
				return iapEvidenceResponse(tc.first)
			}))
			catalog, err := fetchIAPs(context.Background(), client, "app-1", true)
			if (err != nil) != tc.wantError {
				t.Fatalf("catalog=%#v error=%v", catalog, err)
			}
			if tc.wantError && catalog != nil {
				t.Fatalf("partial catalog returned: %#v", catalog)
			}
			if tc.name == "later approval" {
				if len(catalog) != 2 || catalog[0].State != "APPROVED" {
					t.Fatalf("lost approval %#v", catalog)
				}
			}
		})
	}
}
