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

func TestSubscriptionsUpdateWithCopiesGroupLevelFromPeer(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedBody string
	sawSourceInclude := false
	sawPeerInclude := false

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-source":
			if req.URL.Query().Get("include") == "group" {
				sawSourceInclude = true
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-source", "group-1", 3))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-peer":
			if req.URL.Query().Get("include") == "group" {
				sawPeerInclude = true
			}
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithDetailBody("sub-peer", "group-1", 1))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-source":
			bodyBytes, _ := io.ReadAll(req.Body)
			capturedBody = string(bodyBytes)
			return subscriptionUpdateWithJSONResponse(http.StatusOK, subscriptionUpdateWithPatchBody("sub-source", 1))
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

	var req asc.SubscriptionUpdateRequest
	if err := json.Unmarshal([]byte(capturedBody), &req); err != nil {
		t.Fatalf("failed to parse request body: %v\nbody: %s", err, capturedBody)
	}
	if req.Data.Attributes.GroupLevel == nil || *req.Data.Attributes.GroupLevel != 1 {
		t.Fatalf("expected groupLevel=1 in request, got %+v", req.Data.Attributes)
	}
	if req.Data.Attributes.Name != nil {
		t.Fatalf("expected Name to be nil when only --with is passed, got %q", *req.Data.Attributes.Name)
	}
	if req.Data.Attributes.SubscriptionPeriod != nil {
		t.Fatalf("expected SubscriptionPeriod to be nil when only --with is passed, got %q", *req.Data.Attributes.SubscriptionPeriod)
	}
	if req.Data.Attributes.FamilySharable != nil {
		t.Fatalf("expected FamilySharable to be nil when only --with is passed, got %t", *req.Data.Attributes.FamilySharable)
	}

	if !strings.Contains(stdout, `"groupLevel":1`) {
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
