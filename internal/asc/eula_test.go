package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newEULATestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if check != nil {
			check(req)
		}
		return response, nil
	})

	return &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func eulaJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetEndUserLicenseAgreement_ValidationErrors(t *testing.T) {
	client := newEULATestClient(t, nil, nil)

	_, err := client.GetEndUserLicenseAgreement(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing id, got nil")
	}
}

func TestGetEndUserLicenseAgreementForApp_ValidationErrors(t *testing.T) {
	client := newEULATestClient(t, nil, nil)

	_, err := client.GetEndUserLicenseAgreementForApp(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing appID, got nil")
	}
}

func TestCreateEndUserLicenseAgreement(t *testing.T) {
	response := eulaJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "endUserLicenseAgreements",
			"id": "eula-789",
			"attributes": {
				"agreementText": "New terms"
			}
		}
	}`)

	client := newEULATestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/endUserLicenseAgreements" {
			t.Fatalf("expected path /v1/endUserLicenseAgreements, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload EndUserLicenseAgreementCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeEndUserLicenseAgreements {
			t.Fatalf("expected type endUserLicenseAgreements, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.AgreementText != "New terms" {
			t.Fatalf("expected agreement text 'New terms', got %q", payload.Data.Attributes.AgreementText)
		}
		if payload.Data.Relationships.App.Data.ID != "app-456" {
			t.Fatalf("expected app ID app-456, got %s", payload.Data.Relationships.App.Data.ID)
		}
		if len(payload.Data.Relationships.Territories.Data) != 2 {
			t.Fatalf("expected 2 territories, got %d", len(payload.Data.Relationships.Territories.Data))
		}
	}, response)

	resp, err := client.CreateEndUserLicenseAgreement(context.Background(), "app-456", "New terms", []string{"USA", "CAN"})
	if err != nil {
		t.Fatalf("CreateEndUserLicenseAgreement() error: %v", err)
	}

	if resp.Data.ID != "eula-789" {
		t.Fatalf("expected ID eula-789, got %s", resp.Data.ID)
	}
}

func TestCreateEndUserLicenseAgreement_ValidationErrors(t *testing.T) {
	client := newEULATestClient(t, nil, nil)

	_, err := client.CreateEndUserLicenseAgreement(context.Background(), "", "text", []string{"USA"})
	if err == nil {
		t.Fatalf("expected error for missing appID, got nil")
	}
	_, err = client.CreateEndUserLicenseAgreement(context.Background(), "app-1", "", []string{"USA"})
	if err == nil {
		t.Fatalf("expected error for missing agreement text, got nil")
	}
	_, err = client.CreateEndUserLicenseAgreement(context.Background(), "app-1", "text", nil)
	if err == nil {
		t.Fatalf("expected error for missing territory IDs, got nil")
	}
}

func TestUpdateEndUserLicenseAgreement(t *testing.T) {
	response := eulaJSONResponse(http.StatusOK, `{
		"data": {
			"type": "endUserLicenseAgreements",
			"id": "eula-999",
			"attributes": {
				"agreementText": "Updated terms"
			}
		}
	}`)

	client := newEULATestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/endUserLicenseAgreements/eula-999" {
			t.Fatalf("expected path /v1/endUserLicenseAgreements/eula-999, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload EndUserLicenseAgreementUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.ID != "eula-999" {
			t.Fatalf("expected id eula-999, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.AgreementText == nil {
			t.Fatalf("expected agreementText to be set")
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Territories == nil {
			t.Fatalf("expected territories to be set")
		}
	}, response)

	newText := "Updated terms"
	resp, err := client.UpdateEndUserLicenseAgreement(context.Background(), "eula-999", &newText, []string{"USA"})
	if err != nil {
		t.Fatalf("UpdateEndUserLicenseAgreement() error: %v", err)
	}

	if resp.Data.ID != "eula-999" {
		t.Fatalf("expected ID eula-999, got %s", resp.Data.ID)
	}
}

func TestUpdateEndUserLicenseAgreement_ValidationErrors(t *testing.T) {
	client := newEULATestClient(t, nil, nil)

	_, err := client.UpdateEndUserLicenseAgreement(context.Background(), "", nil, nil)
	if err == nil {
		t.Fatalf("expected error for missing id, got nil")
	}

	_, err = client.UpdateEndUserLicenseAgreement(context.Background(), "eula-1", nil, nil)
	if err == nil {
		t.Fatalf("expected error for missing update fields, got nil")
	}
}

func TestDeleteEndUserLicenseAgreement_ValidationErrors(t *testing.T) {
	client := newEULATestClient(t, nil, nil)

	err := client.DeleteEndUserLicenseAgreement(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error for missing id, got nil")
	}
}
