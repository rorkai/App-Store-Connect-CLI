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

type reorderListItem struct {
	ID         string
	Name       string
	ProductID  string
	GroupLevel int
}

func TestSubscriptionsReorderBeforeSendsComputedGroupLevelAndVerifiesOrder(t *testing.T) {
	testSubscriptionsReorderPlacement(t, subscriptionsReorderFixture{
		args:               []string{"subscriptions", "reorder", "--id", "sub-yearly", "--before", "sub-monthly"},
		sourceID:           "sub-yearly",
		groupID:            "group-1",
		initialOrder:       []reorderListItem{{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3}},
		verifiedOrder:      []reorderListItem{{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 2}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 3}},
		expectedGroupLevel: 2,
		expectedPlacement:  "before",
		expectedAnchorID:   "sub-monthly",
	})
}

func TestSubscriptionsReorderAfterSendsComputedGroupLevelAndVerifiesOrder(t *testing.T) {
	testSubscriptionsReorderPlacement(t, subscriptionsReorderFixture{
		args:               []string{"subscriptions", "reorder", "--id", "sub-weekly", "--after", "sub-monthly"},
		sourceID:           "sub-weekly",
		groupID:            "group-1",
		initialOrder:       []reorderListItem{{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3}},
		verifiedOrder:      []reorderListItem{{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 1}, {ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 2}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3}},
		expectedGroupLevel: 2,
		expectedPlacement:  "after",
		expectedAnchorID:   "sub-monthly",
	})
}

func TestSubscriptionsReorderTopSendsComputedGroupLevelAndVerifiesOrder(t *testing.T) {
	testSubscriptionsReorderPlacement(t, subscriptionsReorderFixture{
		args:               []string{"subscriptions", "reorder", "--id", "sub-yearly", "--top"},
		sourceID:           "sub-yearly",
		groupID:            "group-1",
		initialOrder:       []reorderListItem{{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3}},
		verifiedOrder:      []reorderListItem{{ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 1}, {ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 2}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 3}},
		expectedGroupLevel: 1,
		expectedPlacement:  "top",
	})
}

func TestSubscriptionsReorderBottomSendsComputedGroupLevelAndVerifiesOrder(t *testing.T) {
	testSubscriptionsReorderPlacement(t, subscriptionsReorderFixture{
		args:               []string{"subscriptions", "reorder", "--id", "sub-weekly", "--bottom"},
		sourceID:           "sub-weekly",
		groupID:            "group-1",
		initialOrder:       []reorderListItem{{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1}, {ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3}},
		verifiedOrder:      []reorderListItem{{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 1}, {ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 2}, {ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 3}},
		expectedGroupLevel: 3,
		expectedPlacement:  "bottom",
	})
}

func TestSubscriptionsReorderRejectsAnchorOutsideGroup(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-yearly":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionDetailBody("sub-yearly", "group-1", 3))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody([]reorderListItem{
				{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
				{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
				{ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
			}))
		default:
			return subscriptionsReorderJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "reorder", "--id", "sub-yearly", "--before", "sub-missing"}); err != nil {
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
	if runErr == nil || !strings.Contains(runErr.Error(), `subscription "sub-missing" not found in the subscription group`) {
		t.Fatalf("expected missing anchor error, got %v", runErr)
	}
}

func TestSubscriptionsReorderFailsWhenVerificationDoesNotMatch(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	listCallCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-yearly":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionDetailBody("sub-yearly", "group-1", 3))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			listCallCount++
			if listCallCount == 1 {
				return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody([]reorderListItem{
					{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
					{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
					{ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
				}))
			}
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody([]reorderListItem{
				{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
				{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
				{ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
			}))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-yearly":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionPatchResponseBody("sub-yearly", "Yearly", "com.example.yearly", 2))
		default:
			return subscriptionsReorderJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "reorder", "--id", "sub-yearly", "--before", "sub-monthly"}); err != nil {
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
	if runErr == nil || !strings.Contains(runErr.Error(), "verification failed") {
		t.Fatalf("expected verification failure, got %v", runErr)
	}
}

func TestSubscriptionsReorderTopNoOpSkipsPatch(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	patchCalled := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-weekly":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionDetailBody("sub-weekly", "group-1", 1))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody([]reorderListItem{
				{ID: "sub-weekly", Name: "Weekly", ProductID: "com.example.weekly", GroupLevel: 1},
				{ID: "sub-monthly", Name: "Monthly", ProductID: "com.example.monthly", GroupLevel: 2},
				{ID: "sub-yearly", Name: "Yearly", ProductID: "com.example.yearly", GroupLevel: 3},
			}))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-weekly":
			patchCalled = true
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionPatchResponseBody("sub-weekly", "Weekly", "com.example.weekly", 1))
		default:
			return subscriptionsReorderJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "reorder", "--id", "sub-weekly", "--top"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if patchCalled {
		t.Fatal("expected no PATCH request for a no-op reorder")
	}
	if !strings.Contains(stdout, `"changed":false`) {
		t.Fatalf("expected changed=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"verified":true`) {
		t.Fatalf("expected verified=true in output, got %q", stdout)
	}
}

type subscriptionsReorderFixture struct {
	args               []string
	sourceID           string
	groupID            string
	initialOrder       []reorderListItem
	verifiedOrder      []reorderListItem
	expectedGroupLevel int
	expectedPlacement  string
	expectedAnchorID   string
}

func testSubscriptionsReorderPlacement(t *testing.T, fixture subscriptionsReorderFixture) {
	t.Helper()
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	listCallCount := 0
	var capturedBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+fixture.sourceID:
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionDetailBody(fixture.sourceID, fixture.groupID, sourceGroupLevel(fixture.initialOrder, fixture.sourceID)))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/"+fixture.groupID+"/subscriptions":
			listCallCount++
			if listCallCount == 1 {
				return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody(fixture.initialOrder))
			}
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionsListBody(fixture.verifiedOrder))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/"+fixture.sourceID:
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			source := findReorderItem(fixture.verifiedOrder, fixture.sourceID)
			return subscriptionsReorderJSONResponse(http.StatusOK, subscriptionPatchResponseBody(source.ID, source.Name, source.ProductID, fixture.expectedGroupLevel))
		default:
			return subscriptionsReorderJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(fixture.args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var req asc.SubscriptionUpdateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if req.Data.Attributes.GroupLevel == nil || *req.Data.Attributes.GroupLevel != fixture.expectedGroupLevel {
		t.Fatalf("expected groupLevel=%d in request, got %+v", fixture.expectedGroupLevel, req.Data.Attributes)
	}

	if !strings.Contains(stdout, fmt.Sprintf(`"toGroupLevel":%d`, fixture.expectedGroupLevel)) {
		t.Fatalf("expected toGroupLevel in output, got %q", stdout)
	}
	if !strings.Contains(stdout, fmt.Sprintf(`"placement":"%s"`, fixture.expectedPlacement)) {
		t.Fatalf("expected placement in output, got %q", stdout)
	}
	if fixture.expectedAnchorID != "" && !strings.Contains(stdout, fmt.Sprintf(`"anchorSubscriptionId":"%s"`, fixture.expectedAnchorID)) {
		t.Fatalf("expected anchorSubscriptionId in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"verified":true`) {
		t.Fatalf("expected verified=true in output, got %q", stdout)
	}
}

func subscriptionsReorderJSONResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func subscriptionDetailBody(id, groupID string, groupLevel int) string {
	return fmt.Sprintf(`{"data":{"type":"subscriptions","id":"%s","attributes":{"name":"Subscription %s","productId":"com.example.%s","groupLevel":%d},"relationships":{"group":{"data":{"type":"subscriptionGroups","id":"%s"}}}}}`, id, id, id, groupLevel, groupID)
}

func subscriptionsListBody(items []reorderListItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf(`{"type":"subscriptions","id":"%s","attributes":{"name":"%s","productId":"%s","groupLevel":%d}}`, item.ID, item.Name, item.ProductID, item.GroupLevel))
	}
	return fmt.Sprintf(`{"data":[%s],"links":{}}`, strings.Join(parts, ","))
}

func subscriptionPatchResponseBody(id, name, productID string, groupLevel int) string {
	return fmt.Sprintf(`{"data":{"type":"subscriptions","id":"%s","attributes":{"name":"%s","productId":"%s","groupLevel":%d}}}`, id, name, productID, groupLevel)
}

func sourceGroupLevel(items []reorderListItem, sourceID string) int {
	return findReorderItem(items, sourceID).GroupLevel
}

func findReorderItem(items []reorderListItem, id string) reorderListItem {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return reorderListItem{}
}
