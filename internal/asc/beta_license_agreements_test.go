package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetBetaLicenseAgreements(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements" {
			t.Fatalf("expected path /v1/betaLicenseAgreements, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if got := values.Get("filter[app]"); got != "app-1,app-2" {
			t.Fatalf("expected filter[app]=app-1,app-2, got %q", got)
		}
		if got := values.Get("fields[betaLicenseAgreements]"); got != "agreementText" {
			t.Fatalf("expected fields[betaLicenseAgreements]=agreementText, got %q", got)
		}
		if got := values.Get("fields[apps]"); got != "name" {
			t.Fatalf("expected fields[apps]=name, got %q", got)
		}
		if got := values.Get("include"); got != "app" {
			t.Fatalf("expected include=app, got %q", got)
		}
		if got := values.Get("limit"); got != "50" {
			t.Fatalf("expected limit=50, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetBetaLicenseAgreements(
		context.Background(),
		WithBetaLicenseAgreementsAppIDs([]string{"app-1", "app-2"}),
		WithBetaLicenseAgreementsFields([]string{"agreementText"}),
		WithBetaLicenseAgreementsAppFields([]string{"name"}),
		WithBetaLicenseAgreementsInclude([]string{"app"}),
		WithBetaLicenseAgreementsLimit(50),
	)
	if err != nil {
		t.Fatalf("GetBetaLicenseAgreements() error: %v", err)
	}
}

func TestGetBetaLicenseAgreements_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/betaLicenseAgreements?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaLicenseAgreements(context.Background(), WithBetaLicenseAgreementsNextURL(next)); err != nil {
		t.Fatalf("GetBetaLicenseAgreements() error: %v", err)
	}
}

func TestGetBetaLicenseAgreement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaLicenseAgreements","id":"bla-1","attributes":{"agreementText":"Terms"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements/bla-1" {
			t.Fatalf("expected path /v1/betaLicenseAgreements/bla-1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if got := values.Get("fields[betaLicenseAgreements]"); got != "agreementText" {
			t.Fatalf("expected fields[betaLicenseAgreements]=agreementText, got %q", got)
		}
		if got := values.Get("include"); got != "app" {
			t.Fatalf("expected include=app, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaLicenseAgreement(
		context.Background(), "bla-1",
		WithBetaLicenseAgreementFields([]string{"agreementText"}),
		WithBetaLicenseAgreementInclude([]string{"app"}),
	); err != nil {
		t.Fatalf("GetBetaLicenseAgreement() error: %v", err)
	}
}

func TestGetBetaLicenseAgreementForApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaLicenseAgreements","id":"bla-1","attributes":{"agreementText":"Terms"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/betaLicenseAgreement" {
			t.Fatalf("expected path /v1/apps/app-1/betaLicenseAgreement, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("fields[betaLicenseAgreements]"); got != "agreementText" {
			t.Fatalf("expected fields[betaLicenseAgreements]=agreementText, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaLicenseAgreementForApp(context.Background(), "app-1", []string{"agreementText"}); err != nil {
		t.Fatalf("GetBetaLicenseAgreementForApp() error: %v", err)
	}
}

func TestGetBetaLicenseAgreementApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements/bla-1/app" {
			t.Fatalf("expected path /v1/betaLicenseAgreements/bla-1/app, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("fields[apps]"); got != "name" {
			t.Fatalf("expected fields[apps]=name, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaLicenseAgreementApp(context.Background(), "bla-1", []string{"name"}); err != nil {
		t.Fatalf("GetBetaLicenseAgreementApp() error: %v", err)
	}
}

func TestUpdateBetaLicenseAgreement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaLicenseAgreements","id":"bla-1","attributes":{"agreementText":"Updated terms"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaLicenseAgreements/bla-1" {
			t.Fatalf("expected path /v1/betaLicenseAgreements/bla-1, got %s", req.URL.Path)
		}
		var payload BetaLicenseAgreementUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaLicenseAgreements {
			t.Fatalf("expected type betaLicenseAgreements, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.AgreementText == nil {
			t.Fatalf("expected agreementText in payload")
		}
		assertAuthorized(t, req)
	}, response)

	text := "Updated terms"
	if _, err := client.UpdateBetaLicenseAgreement(context.Background(), "bla-1", &text); err != nil {
		t.Fatalf("UpdateBetaLicenseAgreement() error: %v", err)
	}
}
