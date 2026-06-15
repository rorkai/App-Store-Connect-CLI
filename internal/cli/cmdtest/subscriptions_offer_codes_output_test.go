package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSubscriptionsOfferCodesCreateNormalizesValuesAndBuildsPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes, got %s", req.URL.Path)
		}

		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		if attrs["name"] != "Spring Promo" {
			t.Fatalf("expected name Spring Promo, got %#v", attrs["name"])
		}
		if attrs["offerEligibility"] != "REPLACE_INTRO_OFFERS" {
			t.Fatalf("expected normalized offerEligibility REPLACE_INTRO_OFFERS, got %#v", attrs["offerEligibility"])
		}
		if attrs["duration"] != "ONE_MONTH" {
			t.Fatalf("expected normalized duration ONE_MONTH, got %#v", attrs["duration"])
		}
		if attrs["offerMode"] != "PAY_AS_YOU_GO" {
			t.Fatalf("expected normalized offerMode PAY_AS_YOU_GO, got %#v", attrs["offerMode"])
		}
		if attrs["numberOfPeriods"] != float64(2) {
			t.Fatalf("expected numberOfPeriods 2, got %#v", attrs["numberOfPeriods"])
		}
		if attrs["autoRenewEnabled"] != true {
			t.Fatalf("expected autoRenewEnabled true, got %#v", attrs["autoRenewEnabled"])
		}

		eligibilityItems := attrs["customerEligibilities"].([]any)
		gotEligibilities := make([]string, 0, len(eligibilityItems))
		for _, item := range eligibilityItems {
			gotEligibilities = append(gotEligibilities, item.(string))
		}
		wantEligibilities := []string{"NEW", "EXISTING"}
		if !slices.Equal(gotEligibilities, wantEligibilities) {
			t.Fatalf("expected customer eligibilities %v, got %v", wantEligibilities, gotEligibilities)
		}

		subscriptionRelationship := data["relationships"].(map[string]any)["subscription"].(map[string]any)["data"].(map[string]any)
		if subscriptionRelationship["id"] != "8000000001" {
			t.Fatalf("expected subscription id 8000000001, got %#v", subscriptionRelationship["id"])
		}

		included := payload["included"].([]any)
		if len(included) != 1 {
			t.Fatalf("expected 1 included price object, got %d", len(included))
		}
		territory := included[0].(map[string]any)["relationships"].(map[string]any)["territory"].(map[string]any)["data"].(map[string]any)
		if territory["id"] != "USA" {
			t.Fatalf("expected normalized territory id USA, got %#v", territory["id"])
		}

		body := `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-1","attributes":{"name":"Spring Promo","active":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "create",
			"--subscription-id", "8000000001",
			"--name", "Spring Promo",
			"--offer-eligibility", "replace_intro_offers",
			"--customer-eligibilities", "new,existing",
			"--offer-duration", "one_month",
			"--offer-mode", "pay_as_you_go",
			"--number-of-periods", "2",
			"--prices", "usa:pp-us",
			"--auto-renew-enabled", "true",
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

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "sub-offer-1" {
		t.Fatalf("expected created offer code id sub-offer-1, got %q", out.Data.ID)
	}
}

func TestSubscriptionsOfferCodesCreateFreeTrialIncludesTerritoryOnlyPrice(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data, ok := payload["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected payload.data to be an object, got %T", payload["data"])
		}
		attrs, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("expected payload.data.attributes to be an object, got %T", data["attributes"])
		}
		if attrs["offerMode"] != "FREE_TRIAL" {
			t.Fatalf("expected offerMode FREE_TRIAL, got %#v", attrs["offerMode"])
		}

		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected payload.data.relationships to be an object, got %T", data["relationships"])
		}
		pricesRelationship, ok := relationships["prices"].(map[string]any)
		if !ok {
			t.Fatalf("expected prices relationship for FREE_TRIAL, got %#v", relationships["prices"])
		}
		priceRefs, ok := pricesRelationship["data"].([]any)
		if !ok || len(priceRefs) != 1 {
			t.Fatalf("expected one price relationship, got %#v", pricesRelationship["data"])
		}
		included, ok := payload["included"].([]any)
		if !ok || len(included) != 1 {
			t.Fatalf("expected one included price, got %#v", payload["included"])
		}
		includedPrice, ok := included[0].(map[string]any)
		if !ok {
			t.Fatalf("expected included price object, got %T", included[0])
		}
		priceRelationships, ok := includedPrice["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected included price relationships, got %T", includedPrice["relationships"])
		}
		territory, ok := priceRelationships["territory"].(map[string]any)
		if !ok {
			t.Fatalf("expected territory relationship, got %#v", priceRelationships["territory"])
		}
		territoryData, ok := territory["data"].(map[string]any)
		if !ok || territoryData["id"] != "DEU" {
			t.Fatalf("expected normalized territory DEU, got %#v", territory["data"])
		}
		if _, ok := priceRelationships["subscriptionPricePoint"]; ok {
			t.Fatalf("expected subscriptionPricePoint to be omitted, got %#v", priceRelationships["subscriptionPricePoint"])
		}

		body := `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-ft-1","attributes":{"name":"One Year Free","active":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "create",
			"--subscription-id", "8000000001",
			"--name", "One Year Free",
			"--offer-eligibility", "stack_with_intro_offers",
			"--customer-eligibilities", "new",
			"--offer-duration", "one_year",
			"--offer-mode", "free_trial",
			"--number-of-periods", "1",
			"--prices", "de",
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

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "sub-offer-ft-1" {
		t.Fatalf("expected created offer code id sub-offer-ft-1, got %q", out.Data.ID)
	}
}

func TestSubscriptionsOfferCodesCreateRequiresPrices(t *testing.T) {
	tests := []struct {
		name      string
		offerMode string
	}{
		{"pay_as_you_go", "pay_as_you_go"},
		{"pay_up_front", "pay_up_front"},
		{"free_trial", "free_trial"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "offer-codes", "create",
					"--subscription-id", "8000000001",
					"--name", "Spring Promo",
					"--offer-eligibility", "stack_with_intro_offers",
					"--customer-eligibilities", "new",
					"--offer-duration", "one_month",
					"--offer-mode", test.offerMode,
					"--number-of-periods", "1",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if !strings.Contains(stderr, "--prices is required") {
				t.Fatalf("expected --prices is required in stderr, got %q", stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestSubscriptionsOfferCodesCreateReturnsCreateFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes, got %s", req.URL.Path)
		}
		body := `{"errors":[{"status":"422","title":"Unprocessable Entity","detail":"invalid offer settings"}]}`
		return &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "create",
			"--subscription-id", "8000000001",
			"--name", "Spring Promo",
			"--offer-eligibility", "replace_intro_offers",
			"--customer-eligibilities", "new",
			"--offer-duration", "one_month",
			"--offer-mode", "pay_as_you_go",
			"--number-of-periods", "1",
			"--prices", "usa:pp-us",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "subscriptions offers offer-codes create: failed to create") {
		t.Fatalf("expected create failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubscriptionsOfferCodesListPaginateReturnsSecondPageFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/offerCodes?cursor=AQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/offerCodes" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit 200 for first paginated request, got %q", req.URL.Query().Get("limit"))
			}
			body := `{
				"data":[{"type":"subscriptionOfferCodes","id":"code-1"}],
				"links":{"next":"` + nextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != nextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"errors":[{"status":"500","title":"Server Error","detail":"page 2 failed"}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "list",
			"--subscription-id", "8000000001",
			"--paginate",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "subscriptions offers offer-codes list: page 2:") {
		t.Fatalf("expected paginated page 2 error context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubscriptionsOfferCodesListRejectsInvalidNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid host",
			next:    "https://example.com/v1/subscriptions/8000000001/offerCodes?cursor=AQ",
			wantErr: "subscriptions offers offer-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "subscriptions offers offer-codes list: --next must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "offer-codes", "list",
					"--next", test.next,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestSubscriptionsOfferCodesListOutputErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/8000000001/offerCodes" {
			t.Fatalf("expected path /v1/subscriptions/8000000001/offerCodes, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"subscriptionOfferCodes","id":"code-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "8000000001", "--output", "yaml"},
			wantErr: "unsupported format: yaml",
		},
		{
			name:    "pretty with markdown",
			args:    []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "8000000001", "--output", "markdown", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected help error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsOfferCodesListPaginateFromNextWithoutSubscription(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/offerCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/offerCodes?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"subscriptionOfferCodes","id":"sub-code-next-1"}],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"subscriptionOfferCodes","id":"sub-code-next-2"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "offers", "offer-codes", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-code-next-1"`) || !strings.Contains(stdout, `"id":"sub-code-next-2"`) {
		t.Fatalf("expected both paginated subscription offer codes in output, got %q", stdout)
	}
}

func TestSubscriptionsOfferCodesListTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/8000000001/offerCodes" {
			t.Fatalf("expected path /v1/subscriptions/8000000001/offerCodes, got %s", req.URL.Path)
		}
		body := `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-table-1","attributes":{"name":"Spring","offerEligibility":"REPLACE_INTRO_OFFERS","customerEligibilities":["NEW","EXISTING"],"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1,"active":true,"totalNumberOfCodes":500,"productionCodeCount":300,"sandboxCodeCount":200}}],
			"links":{"next":""}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "list",
			"--subscription-id", "8000000001",
			"--output", "table",
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
	if strings.Contains(stdout, `"data"`) {
		t.Fatalf("expected table output, got JSON: %q", stdout)
	}
	for _, want := range []string{"ID", "Name", "Customer Eligibilities", "sub-code-table-1", "Spring", "NEW, EXISTING", "500"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
	for _, want := range []string{
		"Follow-up commands",
		`asc subscriptions offers offer-codes one-time-codes list --offer-code-id "sub-code-table-1"`,
		`asc subscriptions offers offer-codes values --batch-id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.csv" --format csv`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain follow-up hint %q, got %q", want, stdout)
		}
	}
}

func TestSubscriptionsOfferCodesListMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/8000000001/offerCodes" {
			t.Fatalf("expected path /v1/subscriptions/8000000001/offerCodes, got %s", req.URL.Path)
		}
		body := `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-md-1","attributes":{"name":"Spring","active":true}}],
			"links":{"next":""}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "offer-codes", "list",
			"--subscription-id", "8000000001",
			"--output", "markdown",
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
	if !strings.Contains(stdout, "sub-code-md-1") {
		t.Fatalf("expected markdown output to contain offer code id, got %q", stdout)
	}
	if strings.Contains(stdout, "Follow-up commands") {
		t.Fatalf("expected markdown output without follow-up hints, got %q", stdout)
	}
}
