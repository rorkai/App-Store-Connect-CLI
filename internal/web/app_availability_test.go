package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAppAvailabilityCreateAttributes(t *testing.T) {
	t.Run("normalizes and deduplicates territory ids", func(t *testing.T) {
		attrs, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AppID:                " app-123 ",
			AvailableTerritories: []string{" usa ", "gbr", "USA", ""},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attrs.AppID != "app-123" {
			t.Fatalf("expected trimmed app id, got %q", attrs.AppID)
		}
		if got := strings.Join(attrs.AvailableTerritories, ","); got != "GBR,USA" {
			t.Fatalf("expected normalized territories GBR,USA, got %q", got)
		}
	})

	t.Run("requires app id", func(t *testing.T) {
		_, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AvailableTerritories: []string{"USA"},
		})
		if err == nil || !strings.Contains(err.Error(), "app id is required") {
			t.Fatalf("expected missing app id error, got %v", err)
		}
	})

	t.Run("requires at least one territory", func(t *testing.T) {
		_, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AppID:                "app-123",
			AvailableTerritories: []string{"", "   "},
		})
		if err == nil || !strings.Contains(err.Error(), "at least one available territory is required") {
			t.Fatalf("expected missing territory error, got %v", err)
		}
	})
}

func TestCreateAppAvailabilityBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appAvailabilities" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		data := body["data"].(map[string]any)
		attributes := data["attributes"].(map[string]any)
		if got := attributes["availableInNewTerritories"]; got != false {
			t.Fatalf("expected availableInNewTerritories=false, got %#v", got)
		}
		relationships := data["relationships"].(map[string]any)
		app := relationships["app"].(map[string]any)["data"].(map[string]any)
		if app["id"] != "app-123" || app["type"] != "apps" {
			t.Fatalf("unexpected app relationship: %#v", app)
		}
		territories := relationships["availableTerritories"].(map[string]any)["data"].([]any)
		if len(territories) != 2 {
			t.Fatalf("expected 2 territories, got %d", len(territories))
		}
		first := territories[0].(map[string]any)
		second := territories[1].(map[string]any)
		if first["id"] != "GBR" || second["id"] != "USA" {
			t.Fatalf("expected sorted territory ids GBR/USA, got %#v %#v", first, second)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "avail-123",
				"type": "appAvailabilities",
				"attributes": {"availableInNewTerritories": false},
				"relationships": {
					"availableTerritories": {
						"data": [
							{"type": "territories", "id": "GBR"},
							{"type": "territories", "id": "USA"}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	created, err := client.CreateAppAvailability(context.Background(), AppAvailabilityCreateAttributes{
		AppID:                     "app-123",
		AvailableInNewTerritories: false,
		AvailableTerritories:      []string{"usa", "gbr"},
	})
	if err != nil {
		t.Fatalf("CreateAppAvailability() error = %v", err)
	}
	if created == nil {
		t.Fatal("expected created app availability")
	}
	if created.ID != "avail-123" {
		t.Fatalf("expected id avail-123, got %q", created.ID)
	}
	if created.AvailableInNewTerritories {
		t.Fatal("expected availableInNewTerritories=false")
	}
	if got := strings.Join(created.AvailableTerritories, ","); got != "GBR,USA" {
		t.Fatalf("expected decoded territories GBR,USA, got %q", got)
	}
}

func TestGetAppAvailabilityBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "avail-123",
				"type": "appAvailabilities",
				"attributes": {"availableInNewTerritories": true},
				"relationships": {
					"availableTerritories": {
						"data": [
							{"type": "territories", "id": "USA"},
							{"type": "territories", "id": "GBR"}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.GetAppAvailability(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected app availability")
	}
	if got.ID != "avail-123" || !got.AvailableInNewTerritories {
		t.Fatalf("unexpected app availability payload: %#v", got)
	}
	if joined := strings.Join(got.AvailableTerritories, ","); joined != "GBR,USA" {
		t.Fatalf("expected sorted territories GBR,USA, got %q", joined)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(&APIError{Status: http.StatusNotFound}) {
		t.Fatal("expected 404 APIError to be treated as not found")
	}
	if IsNotFound(&APIError{Status: http.StatusConflict}) {
		t.Fatal("did not expect 409 APIError to be treated as not found")
	}
}
