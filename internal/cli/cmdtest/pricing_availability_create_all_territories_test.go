package cmdtest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's territory catalog split across two pages, as GET /v1/territories
// returns it when the catalog exceeds the page size.
const (
	territoryCatalogFirstPage  = `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/territories?cursor=next"}}`
	territoryCatalogSecondPage = `{"data":[{"type":"territories","id":"CAN"}],"links":{}}`
)

func TestPricingAvailabilityCreateAllTerritoriesIncludesEveryCatalogTerritory(t *testing.T) {
	for _, available := range []bool{true, false} {
		t.Run(strconv.FormatBool(available), func(t *testing.T) {
			var createBody string
			stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
				"pricing", "availability", "create",
				"--app", "app-1",
				"--all-territories",
				"--available", strconv.FormatBool(available),
				"--available-in-new-territories", "true",
				"--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.Path == "/v1/territories" && strings.Contains(req.Query, "cursor=next"):
					return jsonResponse(http.StatusOK, territoryCatalogSecondPage)
				case req.Method == http.MethodGet && req.Path == "/v1/territories":
					return jsonResponse(http.StatusOK, territoryCatalogFirstPage)
				case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
					createBody = req.Body
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
					return nil, nil
				}
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if len(seen) != 3 {
				t.Fatalf("requests = %+v, want two catalog pages and one create", seen)
			}

			var payload asc.AppAvailabilityV2CreateRequest
			if err := json.Unmarshal([]byte(createBody), &payload); err != nil {
				t.Fatalf("decode create body: %v; body=%q", err, createBody)
			}
			gotTerritories := make([]string, 0, len(payload.Included))
			for _, included := range payload.Included {
				if included.Attributes == nil || included.Relationships == nil {
					t.Fatalf("included territory availability is incomplete: %#v", included)
				}
				if included.Attributes.Available != available {
					t.Fatalf("territory %s available = %t, want %t", included.Relationships.Territory.Data.ID, included.Attributes.Available, available)
				}
				gotTerritories = append(gotTerritories, included.Relationships.Territory.Data.ID)
			}
			if got, want := strings.Join(gotTerritories, ","), "USA,FRA,CAN"; got != want {
				t.Fatalf("included territories = %s, want every catalog territory %s", got, want)
			}
			if !strings.Contains(stdout, `"id":"availability-1"`) {
				t.Fatalf("stdout = %q, want Apple's created availability", stdout)
			}
		})
	}
}

func TestPricingAvailabilityCreateTerritorySelectorValidation(t *testing.T) {
	tests := []struct {
		name     string
		selector []string
		wantErr  string
	}{
		{
			name:     "missing selector",
			selector: nil,
			wantErr:  "Error: --territory or --all-territories is required",
		},
		{
			name:     "territory and all territories",
			selector: []string{"--territory", "USA", "--all-territories"},
			wantErr:  "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name:     "blank territory and all territories",
			selector: []string{"--territory", "", "--all-territories"},
			wantErr:  "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name:     "territory list without values",
			selector: []string{"--territory", ","},
			wantErr:  "Error: --territory must include at least one value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"pricing", "availability", "create", "--app", "app-1"}, test.selector...)
			args = append(args, "--available", "true", "--available-in-new-territories", "true")
			stdout, stderr, seen, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s; selector validation must precede HTTP", req.Method, req.Path)
				return nil, nil
			})
			if runErr == nil {
				t.Fatal("run error = nil, want a usage error")
			}
			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, cmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
			if len(seen) != 0 {
				t.Fatalf("requests = %+v, want none", seen)
			}
		})
	}
}

func TestPricingAvailabilityCreateSendsPreOrderAttributesToSelectedTerritoriesOnly(t *testing.T) {
	preOrder := map[string]any{"available": true, "releaseDate": "2026-12-01", "preOrderEnabled": true}
	unselected := map[string]any{"available": false}

	tests := []struct {
		name  string
		extra []string
		want  map[string]map[string]any
	}{
		{
			name:  "omitted flags leave the request unchanged",
			extra: []string{"--territory", "USA"},
			want:  map[string]map[string]any{"USA": {"available": true}, "GBR": unselected},
		},
		{
			name:  "selected territory only",
			extra: []string{"--territory", "USA", "--release-date", "2026-12-01", "--pre-order-enabled", "true"},
			want:  map[string]map[string]any{"USA": preOrder, "GBR": unselected},
		},
		{
			name:  "every territory with all territories",
			extra: []string{"--all-territories", "--release-date", "2026-12-01", "--pre-order-enabled", "true"},
			want:  map[string]map[string]any{"USA": preOrder, "GBR": preOrder},
		},
		{
			name:  "release date alone",
			extra: []string{"--territory", "USA", "--release-date", "2026-12-01"},
			want:  map[string]map[string]any{"USA": {"available": true, "releaseDate": "2026-12-01"}, "GBR": unselected},
		},
		{
			name:  "explicit false is sent",
			extra: []string{"--territory", "USA", "--pre-order-enabled", "false"},
			want:  map[string]map[string]any{"USA": {"available": true, "preOrderEnabled": false}, "GBR": unselected},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"pricing", "availability", "create", "--app", "app-1", "--available", "true", "--available-in-new-territories", "true"}, test.extra...)
			var posted string
			_, _, _, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.Path == "/v1/territories":
					return jsonResponse(http.StatusOK, territoryCatalog)
				case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
					posted = req.Body
					return jsonResponse(http.StatusCreated, existingAvailability)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.Path)
					return nil, nil
				}
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}

			var payload struct {
				Included []struct {
					Attributes    map[string]any `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"included"`
			}
			if err := json.Unmarshal([]byte(posted), &payload); err != nil {
				t.Fatalf("decode POST body %q: %v", posted, err)
			}
			got := map[string]map[string]any{}
			for _, included := range payload.Included {
				got[included.Relationships.Territory.Data.ID] = included.Attributes
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("territory attributes = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPricingAvailabilityCreatePreOrderFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		extra   []string
		wantErr string
	}{
		{
			name:    "release date with unavailable territories",
			extra:   []string{"--available", "false", "--release-date", "2026-12-01"},
			wantErr: "--release-date and --pre-order-enabled require --available true",
		},
		{
			name:    "pre-order with unavailable territories",
			extra:   []string{"--available", "false", "--pre-order-enabled", "true"},
			wantErr: "--release-date and --pre-order-enabled require --available true",
		},
		{
			name:    "malformed release date",
			extra:   []string{"--release-date", "12/01/2026"},
			wantErr: "--release-date must be in YYYY-MM-DD format",
		},
		{
			name:    "blank release date",
			extra:   []string{"--release-date", " "},
			wantErr: "--release-date is required",
		},
		{
			name:    "release date while disabling pre-orders",
			extra:   []string{"--release-date", "2026-12-01", "--pre-order-enabled", "false"},
			wantErr: "--release-date cannot be set when disabling pre-orders",
		},
		{
			name:    "release date with if-exists update",
			extra:   []string{"--release-date", "2026-12-01", "--if-exists", "update"},
			wantErr: "--release-date and --pre-order-enabled cannot be used with --if-exists update",
		},
		{
			name:    "pre-order with if-exists update",
			extra:   []string{"--pre-order-enabled", "true", "--if-exists", "update"},
			wantErr: "--release-date and --pre-order-enabled cannot be used with --if-exists update",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, seen, runErr := runIfExistsCommand(t, availabilityCreateArgs(test.extra...), func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s; flag validation must precede HTTP", req.Method, req.Path)
				return nil, nil
			})
			if runErr == nil {
				t.Fatal("run error = nil, want a usage error")
			}
			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, cmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
			if len(seen) != 0 {
				t.Fatalf("requests = %+v, want none", seen)
			}
		})
	}
}

func TestPricingAvailabilityCreateAllTerritoriesIfExistsUpdatesEveryExistingTerritory(t *testing.T) {
	// The existing record has USA unavailable and GBR available, so
	// --all-territories --available true must PATCH only USA through the edit
	// path, without the caller naming either territory.
	territoriesBefore := `{"data":[{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"territoryAvailabilities","id":"ta-gbr","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}}],"links":{}}`
	patched := []string{}
	_, stderr, _, runErr := runIfExistsCommand(t, []string{
		"pricing", "availability", "create",
		"--app", "app-1",
		"--all-territories",
		"--available", "true",
		"--available-in-new-territories", "true",
		"--if-exists", "update",
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/territories":
			return jsonResponse(http.StatusOK, territoryCatalog)
		case req.Method == http.MethodPost && req.Path == "/v2/appAvailabilities":
			return jsonResponse(http.StatusConflict, availabilityExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonResponse(http.StatusOK, existingAvailability)
		case req.Method == http.MethodGet && req.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			if len(patched) > 0 {
				return jsonResponse(http.StatusOK, existingTerritoryAvailabilities)
			}
			return jsonResponse(http.StatusOK, territoriesBefore)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.Path, "/v1/territoryAvailabilities/"):
			patched = append(patched, strings.TrimPrefix(req.Path, "/v1/territoryAvailabilities/"))
			return jsonResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":true}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if got := strings.Join(patched, ","); got != "ta-usa" {
		t.Fatalf("patched = %q, want only ta-usa", got)
	}
	if !strings.Contains(stderr, "Updated 1 territories; 1 already matched") {
		t.Fatalf("stderr = %q, want the edit path's all-territories summary", stderr)
	}
	if !strings.Contains(stderr, "updated it in place (--if-exists update)") {
		t.Fatalf("stderr = %q, want the update outcome", stderr)
	}
}
