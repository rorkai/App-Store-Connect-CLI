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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsOfferCodesCreateNormalizesValuesAndBuildsPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/subscriptionOfferCodes").JSONBody(`{"data":{"type":"subscriptionOfferCodes","attributes":{"name":"Spring Promo","offerEligibility":"REPLACE_INTRO_OFFERS","customerEligibilities":["NEW","EXISTING"],"duration":"ONE_MONTH","offerMode":"PAY_AS_YOU_GO","numberOfPeriods":2,"autoRenewEnabled":true},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"prices":{"data":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-us"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-1","attributes":{"name":"Spring Promo","active":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
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
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/subscriptionOfferCodes").JSONBody(`{"data":{"type":"subscriptionOfferCodes","attributes":{"name":"One Year Free","offerEligibility":"STACK_WITH_INTRO_OFFERS","customerEligibilities":["NEW"],"duration":"ONE_YEAR","offerMode":"FREE_TRIAL","numberOfPeriods":1},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"prices":{"data":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"DEU"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-ft-1","attributes":{"name":"One Year Free","active":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--subscription-id", "8000000001",
		"--name", "One Year Free",
		"--offer-eligibility", "stack_with_intro_offers",
		"--customer-eligibilities", "new",
		"--offer-duration", "one_year",
		"--offer-mode", "free_trial",
		"--number-of-periods", "1",
		"--prices", "de",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/subscriptionOfferCodes").JSONBody(`{"data":{"type":"subscriptionOfferCodes","attributes":{"name":"Spring Promo","offerEligibility":"REPLACE_INTRO_OFFERS","customerEligibilities":["NEW"],"duration":"ONE_MONTH","offerMode":"PAY_AS_YOU_GO","numberOfPeriods":1},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"prices":{"data":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-us"}}}}]}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","title":"Unprocessable Entity","detail":"invalid offer settings"}]}`))

	stdout, _, runErr := runCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--subscription-id", "8000000001",
		"--name", "Spring Promo",
		"--offer-eligibility", "replace_intro_offers",
		"--customer-eligibilities", "new",
		"--offer-duration", "one_month",
		"--offer-mode", "pay_as_you_go",
		"--number-of-periods", "1",
		"--prices", "usa:pp-us",
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"subscriptionOfferCodes","id":"code-1"}],
			"links":{"next":"`+nextURL+`"}
		}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes", "cursor=AQ", "limit=200"),
		asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","title":"Server Error","detail":"page 2 failed"}]}`))

	stdout, _, runErr := runCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "list",
		"--subscription-id", "8000000001",
		"--paginate",
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

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: "+test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", "Error: "+test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsOfferCodesListOutputErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "8000000001", "--output", "yaml"},
			wantErr: `(got "yaml")`,
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

			if !isUsageClassError(runErr) {
				t.Fatalf("expected usage-class error, got %v", runErr)
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes", "cursor=AQ", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-next-1"}],
			"links":{"next":"`+secondURL+`"}
		}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes", "cursor=BQ", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-next-2"}],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{"subscriptions", "offers", "offer-codes", "list", "--paginate", "--next", firstURL})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-table-1","attributes":{"name":"Spring","offerEligibility":"REPLACE_INTRO_OFFERS","customerEligibilities":["NEW","EXISTING"],"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1,"active":true,"totalNumberOfCodes":500,"productionCodeCount":300,"sandboxCodeCount":200}}],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "list",
		"--subscription-id", "8000000001",
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/offerCodes"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"subscriptionOfferCodes","id":"sub-code-md-1","attributes":{"name":"Spring","active":true}}],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "list",
		"--subscription-id", "8000000001",
		"--output", "markdown",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
