package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type subscriptionUpdateWithListItem struct {
	ID         string
	Name       string
	ProductID  string
	GroupLevel int
}

func TestSubscriptionsUpdateWithMovesSourceIntoPeerSlotAndShiftsSiblings(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	initialOrder := []subscriptionUpdateWithListItem{
		{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
		{ID: "sub-peer", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
		{ID: "sub-source", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
	}
	verifiedOrder := []subscriptionUpdateWithListItem{
		{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
		{ID: "sub-source", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 2},
		{ID: "sub-peer", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 3},
	}

	sawSourceInclude := false
	sawPeerInclude := false
	listCallCount := 0
	patchHistory := map[string][]asc.SubscriptionUpdateRequest{}

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-source":
			if req.URL.Query().Get("include") == "group" {
				sawSourceInclude = true
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-source", "group-1", subscriptionUpdateWithFindItem(initialOrder, "sub-source").GroupLevel))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-peer":
			if req.URL.Query().Get("include") == "group" {
				sawPeerInclude = true
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-peer", "group-1", subscriptionUpdateWithFindItem(initialOrder, "sub-peer").GroupLevel))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			listCallCount++
			if listCallCount == 1 {
				return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithListBody(initialOrder))
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithListBody(verifiedOrder))
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/subscriptions/"):
			bodyBytes, _ := io.ReadAll(req.Body)
			var updateReq asc.SubscriptionUpdateRequest
			if err := json.Unmarshal(bodyBytes, &updateReq); err != nil {
				t.Fatalf("failed to parse request body: %v\nbody: %s", err, string(bodyBytes))
			}
			subID := strings.TrimPrefix(req.URL.Path, "/v1/subscriptions/")
			patchHistory[subID] = append(patchHistory[subID], updateReq)
			level := 0
			if updateReq.Data.Attributes.GroupLevel != nil {
				level = *updateReq.Data.Attributes.GroupLevel
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithPatchBody(subID, level))
		default:
			return subscriptionUpdateWithJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "update", "--id", "sub-source", "--with", "sub-peer"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !sawSourceInclude || !sawPeerInclude {
		t.Fatalf("expected both subscription detail fetches to include group, saw source=%t peer=%t", sawSourceInclude, sawPeerInclude)
	}
	if listCallCount < 2 {
		t.Fatalf("expected list subscriptions call before and after reorder, got %d", listCallCount)
	}
	if len(patchHistory["sub-peer"]) == 0 {
		t.Fatal("expected sibling updates when --with moves into a peer slot")
	}
	if got := subscriptionUpdateWithLastPatchedGroupLevel(t, patchHistory["sub-source"]); got != 2 {
		t.Fatalf("expected final source groupLevel=2, got %d", got)
	}
	if got := subscriptionUpdateWithLastPatchedGroupLevel(t, patchHistory["sub-peer"]); got != 3 {
		t.Fatalf("expected peer to shift to groupLevel=3, got %d", got)
	}

	for subID, requests := range patchHistory {
		for _, req := range requests {
			if req.Data.Attributes.Name != nil {
				t.Fatalf("expected Name to be nil for %s when only --with is passed, got %q", subID, *req.Data.Attributes.Name)
			}
			if req.Data.Attributes.SubscriptionPeriod != nil {
				t.Fatalf("expected SubscriptionPeriod to be nil for %s when only --with is passed, got %q", subID, *req.Data.Attributes.SubscriptionPeriod)
			}
			if req.Data.Attributes.FamilySharable != nil {
				t.Fatalf("expected FamilySharable to be nil for %s when only --with is passed, got %t", subID, *req.Data.Attributes.FamilySharable)
			}
		}
	}

	if !strings.Contains(stdout, `"groupLevel":2`) {
		t.Fatalf("expected groupLevel in output, got %q", stdout)
	}
}

func TestSubscriptionsUpdateWithRejectsDifferentSubscriptionGroups(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	patchCalled := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-source":
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-source", "group-1", 3))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-peer":
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-peer", "group-2", 1))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-source":
			patchCalled = true
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithPatchBody("sub-source", 1))
		default:
			return subscriptionUpdateWithJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "update", "--id", "sub-source", "--with", "sub-peer"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "same subscription group") {
		t.Fatalf("expected same-group validation error, got %v", runErr)
	}
	if patchCalled {
		t.Fatal("expected no PATCH request when --with references a different subscription group")
	}
}

func TestSubscriptionsUpdateWithKeepsSourceOnlyAttributesOnFinalSourcePatch(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	initialOrder := []subscriptionUpdateWithListItem{
		{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
		{ID: "sub-peer", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
		{ID: "sub-source", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
	}
	verifiedOrder := []subscriptionUpdateWithListItem{
		{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
		{ID: "sub-source", Name: "Renamed", ProductID: "com.example.yearly", GroupLevel: 2},
		{ID: "sub-peer", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 3},
	}

	patchHistory := map[string][]asc.SubscriptionUpdateRequest{}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-source":
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-source", "group-1", subscriptionUpdateWithFindItem(initialOrder, "sub-source").GroupLevel))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-peer":
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-peer", "group-1", subscriptionUpdateWithFindItem(initialOrder, "sub-peer").GroupLevel))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if len(patchHistory) == 0 {
				return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithListBody(initialOrder))
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithListBody(verifiedOrder))
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/subscriptions/"):
			bodyBytes, _ := io.ReadAll(req.Body)
			var updateReq asc.SubscriptionUpdateRequest
			if err := json.Unmarshal(bodyBytes, &updateReq); err != nil {
				t.Fatalf("failed to parse request body: %v\nbody: %s", err, string(bodyBytes))
			}
			subID := strings.TrimPrefix(req.URL.Path, "/v1/subscriptions/")
			patchHistory[subID] = append(patchHistory[subID], updateReq)
			level := 0
			if updateReq.Data.Attributes.GroupLevel != nil {
				level = *updateReq.Data.Attributes.GroupLevel
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithPatchBody(subID, level))
		default:
			return subscriptionUpdateWithJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "update", "--id", "sub-source", "--with", "sub-peer", "--reference-name", "Renamed"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if got := subscriptionUpdateWithLastPatchedGroupLevel(t, patchHistory["sub-source"]); got != 2 {
		t.Fatalf("expected final source groupLevel=2, got %d", got)
	}

	sourceFinal := subscriptionUpdateWithLastPatchedRequest(t, patchHistory["sub-source"])
	if sourceFinal.Data.Attributes.Name == nil || *sourceFinal.Data.Attributes.Name != "Renamed" {
		t.Fatalf("expected final source patch to include reference name, got %+v", sourceFinal.Data.Attributes)
	}
	for _, req := range patchHistory["sub-peer"] {
		if req.Data.Attributes.Name != nil {
			t.Fatalf("expected sibling patches to omit source-only name changes, got %+v", req.Data.Attributes)
		}
	}
	if !strings.Contains(stdout, `"groupLevel":2`) {
		t.Fatalf("expected groupLevel in output, got %q", stdout)
	}
}

func subscriptionUpdateWithJSONResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func subscriptionUpdateWithDetailBody(id, groupID string, groupLevel int) string {
	return fmt.Sprintf(`{"data":{"type":"subscriptions","id":"%s","attributes":{"name":"Subscription %s","productId":"com.example.%s","groupLevel":%d},"relationships":{"group":{"data":{"type":"subscriptionGroups","id":"%s"}}}}}`, id, id, id, groupLevel, groupID)
}

func subscriptionUpdateWithPatchBody(id string, groupLevel int) string {
	return fmt.Sprintf(`{"data":{"type":"subscriptions","id":"%s","attributes":{"name":"Subscription %s","productId":"com.example.%s","groupLevel":%d}}}`, id, id, id, groupLevel)
}

func subscriptionUpdateWithListBody(items []subscriptionUpdateWithListItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf(`{"type":"subscriptions","id":"%s","attributes":{"name":"%s","productId":"%s","groupLevel":%d}}`, item.ID, item.Name, item.ProductID, item.GroupLevel))
	}
	return fmt.Sprintf(`{"data":[%s],"links":{}}`, strings.Join(parts, ","))
}

func subscriptionUpdateWithFindItem(items []subscriptionUpdateWithListItem, id string) subscriptionUpdateWithListItem {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return subscriptionUpdateWithListItem{}
}

func subscriptionUpdateWithLastPatchedGroupLevel(t *testing.T, requests []asc.SubscriptionUpdateRequest) int {
	t.Helper()
	last := subscriptionUpdateWithLastPatchedRequest(t, requests)
	if last.Data.Attributes.GroupLevel == nil {
		t.Fatalf("expected final patch request to include groupLevel, got %+v", last.Data.Attributes)
	}
	return *last.Data.Attributes.GroupLevel
}

func subscriptionUpdateWithLastPatchedRequest(t *testing.T, requests []asc.SubscriptionUpdateRequest) asc.SubscriptionUpdateRequest {
	t.Helper()
	if len(requests) == 0 {
		t.Fatal("expected at least one patch request")
	}
	return requests[len(requests)-1]
}
