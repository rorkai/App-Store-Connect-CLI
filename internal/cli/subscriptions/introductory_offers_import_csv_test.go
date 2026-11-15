package subscriptions

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestParseSubscriptionIntroductoryOffersImportCSVHeader_StripsUTF8BOM(t *testing.T) {
	got, err := parseSubscriptionIntroductoryOffersImportCSVHeader(shared.DefaultDeps(), []string{"\ufeffterritory", "offer_mode"})
	if err != nil {
		t.Fatalf("parseSubscriptionIntroductoryOffersImportCSVHeader() error: %v", err)
	}
	if got["territory"] != 0 {
		t.Fatalf("expected territory column at index 0, got %d", got["territory"])
	}
	if got["offer_mode"] != 1 {
		t.Fatalf("expected offer_mode column at index 1, got %d", got["offer_mode"])
	}
}

func TestWriteSubscriptionIntroductoryOfferImportFailureArtifact_ReturnsWriteError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := writeSubscriptionIntroductoryOfferImportFailureArtifact(shared.DefaultDeps(), &subscriptionIntroductoryOfferImportSummary{
		Failed:  1,
		Results: []subscriptionIntroductoryOfferImportResultItem{{Status: "failed"}},
	})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

func TestSubscriptionIntroductoryOfferImportStateMatchesImmediateUpfrontOffer(t *testing.T) {
	index := &subscriptionIntroductoryOfferImportStateIndex{
		now: time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC),
		offers: []subscriptionIntroductoryOfferImportResolvedRow{{
			territory:       "USA",
			offerMode:       "FREE_TRIAL",
			offerDuration:   "ONE_WEEK",
			numberOfPeriods: 1,
			startDate:       "2026-06-23",
			planType:        asc.SubscriptionPlanTypeUpfront,
		}},
	}
	target := subscriptionIntroductoryOfferImportResolvedRow{
		territory:       "USA",
		offerMode:       "FREE_TRIAL",
		offerDuration:   "ONE_WEEK",
		numberOfPeriods: 1,
		planType:        asc.SubscriptionPlanTypeUpfront,
	}
	if !index.matches(target) {
		t.Fatal("expected an already-active UPFRONT offer to match an immediate target")
	}

	index.offers[0].startDate = "2026-06-25"
	if index.matches(target) {
		t.Fatal("expected a future offer not to match an immediate target")
	}

	index.offers[0].startDate = "2026-06-23"
	index.offers[0].planType = asc.SubscriptionPlanTypeMonthly
	if index.matches(target) {
		t.Fatal("expected a MONTHLY offer not to match an UPFRONT target")
	}
}

func TestSubscriptionIntroductoryOfferImportStateUsesUSPacificDay(t *testing.T) {
	target := subscriptionIntroductoryOfferImportResolvedRow{
		territory:       "USA",
		offerMode:       "FREE_TRIAL",
		offerDuration:   "ONE_WEEK",
		numberOfPeriods: 1,
		planType:        asc.SubscriptionPlanTypeUpfront,
	}
	scheduled := target
	scheduled.startDate = "2026-06-25"

	// 17:30 PDT on 2026-06-24: an offer starting 2026-06-25 is still
	// scheduled, so it does not satisfy an immediate target.
	beforePacificMidnight := time.Date(2026, time.June, 25, 0, 30, 0, 0, time.UTC)
	index := &subscriptionIntroductoryOfferImportStateIndex{
		now:    beforePacificMidnight,
		offers: []subscriptionIntroductoryOfferImportResolvedRow{scheduled},
	}
	if index.matches(target) {
		t.Fatal("expected an offer starting on the next Pacific day not to match an immediate target")
	}

	index.now = time.Date(2026, time.June, 25, 7, 0, 0, 0, time.UTC)
	if !index.matches(target) {
		t.Fatal("expected the offer to match an immediate target from Pacific midnight")
	}

	recorded := &subscriptionIntroductoryOfferImportStateIndex{now: beforePacificMidnight}
	recorded.add(target)
	if len(recorded.offers) != 1 || recorded.offers[0].startDate != "2026-06-24" {
		t.Fatalf("expected the immediate offer recorded on the Pacific date 2026-06-24, got %+v", recorded.offers)
	}
}

func TestSubscriptionIntroductoryOfferImportReconcilesAcrossPacificMidnight(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	// 23:59 PDT on 2026-06-24.
	current := time.Date(2026, time.June, 25, 6, 59, 0, 0, time.UTC)
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time {
		return current
	}))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = introImportRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/introductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		// The read finishes after Pacific midnight, when Apple reports the
		// immediate offer on the new Pacific date.
		current = time.Date(2026, time.June, 25, 7, 1, 0, 0, time.UTC)
		body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"startDate":"2026-06-25","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	target := subscriptionIntroductoryOfferImportResolvedRow{
		territory: "USA", offerMode: "FREE_TRIAL", offerDuration: "ONE_WEEK", numberOfPeriods: 1, planType: asc.SubscriptionPlanTypeUpfront,
	}
	mutations := 0
	status, err := runReconciledMutation(
		context.Background(),
		shared.DefaultDeps(),
		func(readbackCtx context.Context) (bool, error) {
			index, err := fetchSubscriptionIntroductoryOfferImportState(readbackCtx, shared.DefaultDeps(), client, "sub-1")
			if err != nil {
				return false, err
			}
			return index.matches(target), nil
		},
		func(context.Context) error {
			mutations++
			return &asc.RetryableError{Err: errors.New("ambiguous timeout")}
		},
	)
	if err != nil {
		t.Fatalf("runReconciledMutation() error: %v", err)
	}
	if status != reconciledMutationReconciled || mutations != 1 {
		t.Fatalf("unexpected midnight recovery: status=%q mutations=%d", status, mutations)
	}
}

type introImportRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn introImportRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func introImportTestPrivateKeyPEM(t testing.TB) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}
