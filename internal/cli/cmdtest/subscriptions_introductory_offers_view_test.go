package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionsIntroductoryOffersViewFindsOfferAcrossPages(t *testing.T) {
	setupAuth(t)

	const subscriptionID = "1234567890"
	nextURL := "https://api.appstoreconnect.apple.com/v1/subscriptions/" + subscriptionID + "/introductoryOffers?cursor=next"
	requestCount := 0
	useIntroductoryOffersViewServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/"+subscriptionID+"/introductoryOffers" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}

		switch requestCount {
		case 1:
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected first-page limit 200, got %q", got)
			}
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1"}],"links":{"next":"` + nextURL + `"}}`
			writeIntroductoryOffersViewJSON(w, body)
		case 2:
			if got := req.URL.Query().Get("cursor"); got != "next" {
				t.Fatalf("expected server-provided next URL, got %q", req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-2","attributes":{"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1}}],"links":{"next":""}}`
			writeIntroductoryOffersViewJSON(w, body)
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL.String())
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "view",
			"--subscription-id", subscriptionID,
			"--id", "offer-2",
			"--output", "json",
		}); err != nil {
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
	if requestCount != 2 {
		t.Fatalf("expected two requests, got %d", requestCount)
	}

	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&response); err != nil {
		t.Fatalf("decode output: %v\nstdout: %s", err, stdout)
	}
	if response.Data.ID != "offer-2" {
		t.Fatalf("expected offer-2, got %q", response.Data.ID)
	}
}

func TestSubscriptionsIntroductoryOffersViewResolvesSubscriptionSelector(t *testing.T) {
	setupAuth(t)

	requestCount := 0
	useIntroductoryOffersViewServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/1/subscriptionGroups":
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.monthly" {
				t.Fatalf("expected product ID filter, got %q", got)
			}
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/introductoryOffers":
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1"}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "view",
			"--app", "1",
			"--subscription-id", "com.example.monthly",
			"--id", "offer-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("expected three requests, got %d", requestCount)
	}
	if !strings.Contains(stdout, `"id":"offer-1"`) {
		t.Fatalf("expected selected offer output, got %q", stdout)
	}
}

func TestSubscriptionsIntroductoryOffersViewResolvesExactSubscriptionName(t *testing.T) {
	setupAuth(t)

	groupRequestCount := 0
	subscriptionRequestCount := 0
	offerRequestCount := 0
	useIntroductoryOffersViewServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/1/subscriptionGroups":
			groupRequestCount++
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			subscriptionRequestCount++
			switch subscriptionRequestCount {
			case 1:
				if got := req.URL.Query().Get("filter[productId]"); got != "Monthly" {
					t.Fatalf("expected product ID lookup before name lookup, got %q", got)
				}
				writeIntroductoryOffersViewJSON(w, `{"data":[],"links":{"next":""}}`)
			case 2:
				if got := req.URL.Query().Get("filter[name]"); got != "Monthly" {
					t.Fatalf("expected exact name filter, got %q", got)
				}
				writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{"next":""}}`)
			default:
				t.Fatalf("unexpected subscription lookup request %d: %s", subscriptionRequestCount, req.URL.String())
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/introductoryOffers":
			offerRequestCount++
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1"}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "view",
			"--app", "1",
			"--subscription-id", "Monthly",
			"--id", "offer-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if groupRequestCount != 2 || subscriptionRequestCount != 2 || offerRequestCount != 1 {
		t.Fatalf(
			"expected two group requests, two subscription requests, and one offer request; got groups=%d subscriptions=%d offers=%d",
			groupRequestCount,
			subscriptionRequestCount,
			offerRequestCount,
		)
	}
	if !strings.Contains(stdout, `"id":"offer-1"`) {
		t.Fatalf("expected selected offer output, got %q", stdout)
	}
}

func TestSubscriptionsIntroductoryOffersViewReturnsNotFoundAfterAllPages(t *testing.T) {
	setupAuth(t)

	const subscriptionID = "1234567890"
	nextURL := "https://api.appstoreconnect.apple.com/v1/subscriptions/" + subscriptionID + "/introductoryOffers?cursor=next"
	requestCount := 0
	useIntroductoryOffersViewServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/"+subscriptionID+"/introductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		switch requestCount {
		case 1:
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-other-1"}],"links":{"next":"` + nextURL + `"}}`
			writeIntroductoryOffersViewJSON(w, body)
		case 2:
			if got := req.URL.Query().Get("cursor"); got != "next" {
				t.Fatalf("expected server-provided next URL, got %q", req.URL.String())
			}
			writeIntroductoryOffersViewJSON(w, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-other-2"}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL.String())
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "view",
			"--subscription-id", subscriptionID,
			"--id", "offer-missing",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", rootcmd.ExitNotFound, got)
	}
	if !strings.Contains(runErr.Error(), `introductory offer "offer-missing" not found for subscription "1234567890"`) {
		t.Fatalf("expected contextual not-found error, got %v", runErr)
	}
	if requestCount != 2 {
		t.Fatalf("expected two requests before not-found, got %d", requestCount)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no process output before error reporting, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSubscriptionsIntroductoryOffersViewRejectsRepeatedPaginationURL(t *testing.T) {
	setupAuth(t)

	const subscriptionID = "1234567890"
	nextURL := "https://api.appstoreconnect.apple.com/v1/subscriptions/" + subscriptionID + "/introductoryOffers?cursor=repeated"
	requestCount := 0
	useIntroductoryOffersViewServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if requestCount > 2 {
			t.Fatal("unexpected third request")
		}
		body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-other"}],"links":{"next":"` + nextURL + `"}}`
		writeIntroductoryOffersViewJSON(w, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "offers", "introductory", "view",
		"--subscription-id", subscriptionID,
		"--id", "offer-missing",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if !errors.Is(runErr, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", runErr)
	}
	if requestCount != 2 {
		t.Fatalf("expected two requests before repeated URL detection, got %d", requestCount)
	}
}

func useIntroductoryOffersViewServer(t *testing.T, handler http.Handler) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create introductory offers view test client: %v", err)
	}
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}

func writeIntroductoryOffersViewJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}
