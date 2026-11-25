package assets

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func verifiedScreenshotTestScope(t *testing.T, ctx context.Context, client *asc.Client) context.Context {
	t.Helper()
	target := assetsScreenshotResumeTarget()
	scoped, err := prepareScreenshotResumeScope(asc.WithNoRedirectReads(ctx), newAssetsUploadTestDeps(), client, screenshotUploadFailureArtifact{
		VersionLocalizationID: target.LocalizationID, DisplayType: target.DisplayType,
		SetID: "set-1", OrderedIDs: []string{"existing"},
	}, target)
	if err != nil {
		t.Fatal(err)
	}
	return scoped
}

func TestResumedScreenshotPollReadsSelectedSet(t *testing.T) {
	const members = "/v1/appScreenshotSets/set-1/appScreenshots"
	const asset = `{"type":"appScreenshots","id":"existing","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"abc"}}`
	var trace []string
	present := true
	client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		trace = append(trace, req.Method+" "+req.URL.Path)
		switch req.URL.Path {
		case members:
			if !present {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[]}`)
				return
			}
			writeAssetsTestJSON(w, 200, `{"data":[`+asset+`]}`)
		case "/v1/appScreenshots/existing":
			writeAssetsTestJSON(w, 200, `{"data":`+asset+`}`)
		default:
			payload, ok := screenshotResumeScopeTestPayload(t, req, []string{"existing"})
			if !ok {
				t.Fatalf("unexpected request: %s", req.URL)
			}
			writeAssetsTestJSON(w, 200, payload)
		}
	}))
	ctx := verifiedScreenshotTestScope(t, context.Background(), client)
	trace = nil
	result, err := waitForScreenshotSettlement(ctx, nil, client, "existing", "abc")
	if err != nil || result.ID != "existing" {
		t.Fatalf("result=%v error=%v", result.ID, err)
	}
	if !slices.Equal(trace, []string{"GET " + members}) {
		t.Fatalf("one settled-state poll issued %d requests: %v", len(trace), trace)
	}
	present, trace = false, nil
	_, err = waitForScreenshotSettlement(ctx, nil, client, "existing", "abc")
	if err == nil || !slices.Equal(trace, []string{"GET " + members}) {
		t.Fatalf("missing screenshot: err=%v requests=%v", err, trace)
	}
}

func TestResumedScreenshotWaitsForReservationMembership(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		firstInventory, firstOrder []string
		settles, rejectImmediately bool
	}{
		{"both views lag", []string{"existing"}, []string{"existing"}, true, false},
		{"resource collection first", []string{"existing", "new"}, []string{"existing"}, true, false},
		{"order collection first", []string{"existing"}, []string{"existing", "new"}, true, false},
		{"foreign arrival", []string{"existing", "foreign"}, []string{"existing", "foreign"}, false, true},
		{"previous member missing", nil, nil, false, true},
		{"reservation never visible", []string{"existing"}, []string{"existing"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Handler transport never uses sockets, so synctest drives all poll timers.
				const inventory = "/v1/appScreenshotSets/set-1/appScreenshots"
				const order = "/v1/appScreenshotSets/set-1/relationships/appScreenshots"
				var posted bool
				var invReads, orderReads int
				var writes []string
				client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method != http.MethodGet {
						writes = append(writes, req.Method+" "+req.URL.Path)
					}
					members := []string{"existing"}
					switch {
					case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
						posted = true
						writeAssetsTestJSON(w, 201, `{"data":{"type":"appScreenshots","id":"new"}}`)
						return
					case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new":
						writeAssetsTestJSON(w, 200, `{"data":{"type":"appScreenshots","id":"new","attributes":{"uploaded":true}}}`)
						return
					case posted && req.URL.Path == inventory:
						invReads++
						members = tc.firstInventory
						if tc.settles && invReads > 1 {
							members = []string{"existing", "new"}
						}
					case posted && req.URL.Path == order:
						orderReads++
						members = tc.firstOrder
						if tc.settles && orderReads > 1 {
							members = []string{"existing", "new"}
						}
					}
					if invReads > 8 || orderReads > 8 {
						t.Fatal("unbounded visibility reads")
					}
					payload, ok := screenshotResumeScopeTestPayload(t, req, members)
					if !ok {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					writeAssetsTestJSON(w, 200, payload)
				}))
				ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
				defer cancel()
				ctx = verifiedScreenshotTestScope(t, ctx, client)
				created, err := createScopedAppScreenshot(ctx, client, "set-1", "new.png", 42)
				if err != nil || created.Data.ID != "new" {
					t.Fatalf("reservation: %v", err)
				}
				_, err = commitScopedAppScreenshot(ctx, client, "new", "abc")
				wanted := []string{"POST /v1/appScreenshots"}
				if tc.settles {
					wanted = append(wanted, "PATCH /v1/appScreenshots/new")
				}
				if (err == nil) != tc.settles || !slices.Equal(writes, wanted) {
					t.Fatalf("commit=%v writes=%v inventory=%d order=%d", err, writes, invReads, orderReads)
				}
				if tc.rejectImmediately && (invReads > 1 || orderReads > 1) {
					t.Fatalf("retried real drift: inventory=%d order=%d", invReads, orderReads)
				}
				if !tc.rejectImmediately && invReads < 2 {
					t.Fatalf("failed to recheck only the newly acknowledged reservation: %v (inventory=%d)", err, invReads)
				}
				if ctx.Err() != nil {
					t.Fatalf("visibility checks exhausted the full upload deadline: %v", ctx.Err())
				}
			})
		})
	}
}
