package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func TestSubscriptionsIntroductoryOffersCreateNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		var payload struct {
			Data struct {
				Relationships struct {
					Subscription struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"subscription"`
					Territory struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"territory"`
					SubscriptionPricePoint struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"subscriptionPricePoint"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request payload: %v", err)
		}
		if got := payload.Data.Relationships.Territory.Data.ID; got != "USA" {
			t.Fatalf("expected normalized territory USA, got %q", got)
		}
		if got := payload.Data.Relationships.Subscription.Data.ID; got != "8000000001" {
			t.Fatalf("expected subscription relationship 8000000001, got %q", got)
		}
		if got := payload.Data.Relationships.SubscriptionPricePoint.Data.ID; got != "price-1" {
			t.Fatalf("expected price point relationship price-1, got %q", got)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-1","attributes":{"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "offers", "introductory", "create",
		"--subscription-id", "8000000001",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
		"--territory", "US",
		"--price-point", "price-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestSubscriptionsIntroductoryOffersCreateRequiresExactlyOneTerritorySelectorBeforeAuth(t *testing.T) {
	isolateIntroductoryOfferCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("validation must happen before HTTP: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	baseArgs := []string{
		"subscriptions", "offers", "introductory", "create",
		"--subscription-id", "8000000001",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
	}
	tests := []struct {
		name       string
		additional []string
		wantErr    string
	}{
		{
			name:    "missing selector",
			wantErr: "exactly one of --territory or --all-territories is required",
		},
		{
			name:       "blank territory",
			additional: []string{"--territory", "   "},
			wantErr:    "invalid value for --territory: cannot be empty",
		},
		{
			name:       "both selectors",
			additional: []string{"--territory", "USA", "--all-territories"},
			wantErr:    "exactly one of --territory or --all-territories is required",
		},
		{
			name:       "deprecated all alias and canonical selector",
			additional: []string{"--territory", "ALL", "--all-territories"},
			wantErr:    "exactly one of --territory or --all-territories is required",
		},
		{
			name:       "invalid territory",
			additional: []string{"--territory", "Atlantis"},
			wantErr:    `territory "Atlantis" could not be mapped to an App Store Connect territory ID`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, baseArgs...), test.additional...)
			stdout, stderr, runErr := runRootCommand(t, args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			wantStderr := "Error: " + test.wantErr + "\n" + subscriptionIntroductoryOfferCreateSelectorGuidanceForTest + "\n"
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if strings.Contains(stderr, "missing authentication") {
				t.Fatalf("validation must happen before authentication, got %q", stderr)
			}
			if strings.Contains(stderr, "DESCRIPTION") || strings.Contains(stderr, "FLAGS") {
				t.Fatalf("selector failure must not dump full help, got %q", stderr)
			}
		})
	}
}

func isolateIntroductoryOfferCreateAuth(t *testing.T) {
	t.Helper()

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	for _, key := range []string{
		"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
}

func TestSubscriptionsIntroductoryOffersCreateSelectorFailuresAreConciseAtTopLevel(t *testing.T) {
	isolateIntroductoryOfferCreateAuth(t)

	baseArgs := []string{
		"subscriptions", "offers", "introductory", "create",
		"--subscription-id", "8000000001",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
	}
	tests := []struct {
		name       string
		additional []string
		wantErr    string
	}{
		{name: "missing selector", wantErr: "exactly one of --territory or --all-territories is required"},
		{name: "blank territory", additional: []string{"--territory", "   "}, wantErr: "invalid value for --territory: cannot be empty"},
		{name: "both selectors", additional: []string{"--territory", "USA", "--all-territories"}, wantErr: "exactly one of --territory or --all-territories is required"},
		{name: "invalid territory", additional: []string{"--territory", "Atlantis"}, wantErr: `territory "Atlantis" could not be mapped to an App Store Connect territory ID`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, baseArgs...), test.additional...)
			var exitCode int
			stdout, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(args, "1.2.3")
			})
			if exitCode != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", exitCode, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			wantStderr := "Error: " + test.wantErr + "\n" + subscriptionIntroductoryOfferCreateSelectorGuidanceForTest + "\n"
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersCreateNonSelectorValidationKeepsFullHelp(t *testing.T) {
	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--territory", "USA",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
		}, "1.2.3")
	})
	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: --offer-duration is required\n") ||
		!strings.Contains(stderr, "DESCRIPTION\n") || !strings.Contains(stderr, "FLAGS\n") {
		t.Fatalf("expected non-selector validation to retain full help, got %q", stderr)
	}
}

const subscriptionIntroductoryOfferCreateSelectorGuidanceForTest = `Try:
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --territory "USA" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
  asc subscriptions offers introductory create --subscription-id "SUB_ID" --all-territories --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1
For help:
  asc subscriptions offers introductory create --help
`

func TestSubscriptionsIntroductoryOffersCreateSelectorAndPostShareOperationTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var lookupDeadline time.Time
	var postDeadlineDelta time.Duration
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			var ok bool
			lookupDeadline, ok = req.Context().Deadline()
			if !ok {
				t.Fatal("expected selector lookup deadline")
			}
			time.Sleep(250 * time.Millisecond)
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.monthly" {
				t.Fatalf("expected product-id selector filter, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			postDeadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected create request deadline")
			}
			postDeadlineDelta = postDeadline.Sub(lookupDeadline)
			if postDeadlineDelta > 100*time.Millisecond {
				return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	started := time.Now()
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--app", "app-1",
			"--subscription-id", "com.example.monthly",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatalf("expected create operation timeout, got nil; selector-to-POST deadline delta=%v", postDeadlineDelta)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
		t.Fatalf("expected timeout exit code %d, got %d", rootcmd.ExitError, got)
	}
	if !strings.Contains(runErr.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no output before single-create timeout, got stdout=%q stderr=%q", stdout, stderr)
	}
	if postDeadlineDelta < 0 || postDeadlineDelta > 100*time.Millisecond {
		t.Fatalf("expected selector and POST to share one operation deadline, delta=%v", postDeadlineDelta)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("expected explicit 1s timeout to cap the operation, elapsed=%v", elapsed)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesDryRunSummarizesAvailability(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	seen := make([]string, 0, 3)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories" && req.URL.Query().Get("cursor") == "":
			body := `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=2"}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories" && req.URL.Query().Get("cursor") == "2":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"GBR"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"eyJpIjoiVVMifQ"}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var summary struct {
		SubscriptionID string `json:"subscriptionId"`
		AllTerritories bool   `json:"allTerritories"`
		DryRun         bool   `json:"dryRun"`
		Total          int    `json:"total"`
		Created        int    `json:"created"`
		Skipped        int    `json:"skipped"`
		Failed         int    `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
			"--dry-run",
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
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.SubscriptionID != "8000000001" || !summary.AllTerritories || !summary.DryRun {
		t.Fatalf("unexpected summary identity: %+v", summary)
	}
	if summary.Total != 3 || summary.Created != 2 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	for _, request := range seen {
		if strings.HasPrefix(request, http.MethodPost+" ") {
			t.Fatalf("dry-run should not POST, saw requests: %v", seen)
		}
	}
}

func TestSubscriptionsIntroductoryOffersCreateDeprecatedAllAliasCreatesPerAvailabilityTerritory(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	postedTerritories := make([]string, 0, 2)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			var payload struct {
				Data struct {
					Attributes struct {
						Duration        string `json:"duration"`
						OfferMode       string `json:"offerMode"`
						NumberOfPeriods int    `json:"numberOfPeriods"`
					} `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.Attributes.Duration != "ONE_MONTH" || payload.Data.Attributes.OfferMode != "FREE_TRIAL" || payload.Data.Attributes.NumberOfPeriods != 1 {
				t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
			}
			postedTerritories = append(postedTerritories, payload.Data.Relationships.Territory.Data.ID)
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var summary struct {
		Total   int `json:"total"`
		Created int `json:"created"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--territory", "ALL",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	wantWarning := "Warning: `--territory ALL` is deprecated. Use `--all-territories`.\n"
	if stderr != wantWarning {
		t.Fatalf("expected stderr %q, got %q", wantWarning, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if got := strings.Join(postedTerritories, ","); got != "CAN,USA" {
		t.Fatalf("expected POSTs for CAN,USA in availability order, got %s", got)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesTimeoutStopsContinueOnError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var operationDeadline time.Time
	postedTerritories := make([]string, 0, 3)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			var ok bool
			operationDeadline, ok = req.Context().Deadline()
			if !ok {
				t.Fatal("expected availability request deadline")
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"},{"type":"territories","id":"GBR"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			var payload struct {
				Data struct {
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			postedTerritories = append(postedTerritories, payload.Data.Relationships.Territory.Data.ID)
			if len(postedTerritories) == 1 {
				if wait := time.Until(operationDeadline.Add(25 * time.Millisecond)); wait > 0 {
					time.Sleep(wait)
				}
				return nil, context.DeadlineExceeded
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	var summary struct {
		Total    int `json:"total"`
		Created  int `json:"created"`
		Failed   int `json:"failed"`
		Failures []struct {
			Territory string `json:"territory"`
			Error     string `json:"error"`
		} `json:"failures"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
			"--continue-on-error=true",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected operation timeout, got nil")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
		t.Fatalf("expected timeout exit code %d, got %d", rootcmd.ExitError, got)
	}
	if !strings.Contains(runErr.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline error, got %v; stdout=%q; POSTs=%v", runErr, stdout, postedTerritories)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Total != 3 || summary.Created != 0 || summary.Failed != 1 {
		t.Fatalf("unexpected timeout summary: %+v", summary)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Territory != "USA" || !strings.Contains(summary.Failures[0].Error, "context deadline exceeded") {
		t.Fatalf("expected deterministic USA deadline failure, got %+v", summary.Failures)
	}
	if got := strings.Join(postedTerritories, ","); got != "USA" {
		t.Fatalf("expected timeout to stop before CAN even with --continue-on-error=true, got POSTs for %s", got)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesSkipsExistingBeforeCancellationFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	t.Cleanup(cancelRun)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			resp := jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"intro-existing","relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`)
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancelRun}
			return resp, nil
		case req.Method == http.MethodPost:
			t.Fatalf("canceled operation should not create another offer: %s", req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	var summary struct {
		Total    int                                        `json:"total"`
		Skipped  int                                        `json:"skipped"`
		Failed   int                                        `json:"failed"`
		Skips    []subscriptionIntroductoryOfferSummaryItem `json:"skips"`
		Failures []subscriptionIntroductoryOfferSummaryItem `json:"failures"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(runCtx)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "context canceled") {
		t.Fatalf("expected canceled operation error, got %v", runErr)
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Total != 2 || summary.Skipped != 1 || summary.Failed != 1 {
		t.Fatalf("unexpected canceled summary: %+v", summary)
	}
	if len(summary.Skips) != 1 || summary.Skips[0].Territory != "USA" {
		t.Fatalf("expected existing USA offer to remain skipped, got %+v", summary.Skips)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Territory != "CAN" || !strings.Contains(summary.Failures[0].Error, "context canceled") {
		t.Fatalf("expected CAN cancellation failure, got %+v", summary.Failures)
	}
}

type subscriptionIntroductoryOfferSummaryItem struct {
	Territory string `json:"territory"`
	Error     string `json:"error"`
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesPartialFailureReturnsReportedError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			var payload struct {
				Data struct {
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.Relationships.Territory.Data.ID == "CAN" {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","detail":"duplicate territory"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary struct {
		Created  int `json:"created"`
		Failed   int `json:"failed"`
		Failures []struct {
			Territory string `json:"territory"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Created != 1 || summary.Failed != 1 || len(summary.Failures) != 1 || summary.Failures[0].Territory != "CAN" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesRejectsConcreteTerritoryAndPricePoint(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "all territories and concrete territory",
			args: []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--all-territories",
				"--territory", "USA",
			},
			wantErr: "Error: exactly one of --territory or --all-territories is required",
		},
		{
			name: "all territories and price point",
			args: []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--all-territories",
				"--price-point", "price-1",
			},
			wantErr: "Error: --price-point cannot be used with --all-territories or --territory ALL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
