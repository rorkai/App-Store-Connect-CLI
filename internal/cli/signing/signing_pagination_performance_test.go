package signing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSigningInventoryPageSizeReducesRequests(t *testing.T) {
	withSigningFetchNow(t, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	tests := []struct {
		name, path, resourceType string
		fetch                    func(context.Context, *asc.Client) ([]string, error)
	}{
		{"certificates", "/v1/certificates", "certificates", func(ctx context.Context, client *asc.Client) ([]string, error) {
			response, err := findCertificates(ctx, client, "IOS_APP_STORE", "IOS_DISTRIBUTION")
			if err != nil {
				return nil, err
			}
			return signingInventoryIDs(response.Data), nil
		}},
		{"active profiles", "/v1/bundleIds/bundle-main/profiles", "profiles", func(ctx context.Context, client *asc.Client) ([]string, error) {
			profiles, err := findActiveProfiles(ctx, client, "bundle-main", "IOS_APP_STORE", signingFetchNowFn)
			return signingInventoryIDs(profiles), err
		}},
		{"profile certificates", "/v1/profiles/profile-main/certificates", "certificates", func(ctx context.Context, client *asc.Client) ([]string, error) {
			response, err := findProfileCertificates(ctx, client, "profile-main", "IOS_DISTRIBUTION", signingFetchNowFn)
			if err != nil {
				return nil, err
			}
			return signingInventoryIDs(response.Data), nil
		}},
		{"stale profiles", "/v1/bundleIds/bundle-main/profiles", "profiles", func(ctx context.Context, client *asc.Client) ([]string, error) {
			profiles, err := findStaleSigningProfiles(ctx, client, "bundle-main", "IOS_APP_STORE", signingFetchNowFn)
			ids := make([]string, 0, len(profiles))
			for _, profile := range profiles {
				ids = append(ids, profile.ID)
			}
			return ids, err
		}},
		{"latest expired profile", "/v1/bundleIds/bundle-main/profiles", "profiles", func(ctx context.Context, client *asc.Client) ([]string, error) {
			profile, err := findLatestExpiredProfile(ctx, client, "bundle-main", "IOS_APP_STORE", signingFetchNowFn)
			if err != nil || profile == nil {
				return nil, err
			}
			return []string{profile.ID}, nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := make([]map[string]any, 201)
			var want []string
			for index := range data {
				id := fmt.Sprintf("resource-%03d", index)
				attributes := map[string]any{"certificateType": "IOS_DISTRIBUTION", "profileType": "IOS_APP_STORE", "profileState": "ACTIVE", "expirationDate": "2100-01-01T00:00:00Z"}
				if index%3 == 0 {
					attributes["expirationDate"] = "2020-01-01T00:00:00Z"
				}
				if index%3 == 1 {
					attributes["certificateType"] = "DEVELOPMENT"
					attributes["profileType"] = "IOS_APP_DEVELOPMENT"
				}
				if index == 200 {
					attributes["expirationDate"] = "2025-01-01T00:00:00Z"
				}
				data[index] = map[string]any{"type": test.resourceType, "id": id, "attributes": attributes}
				switch test.name {
				case "certificates":
					want = append(want, id)
				case "active profiles":
					if index%3 == 2 && index != 200 {
						want = append(want, id)
					}
				case "profile certificates":
					if index%3 == 2 && index != 200 {
						want = append(want, id)
					}
				case "stale profiles":
					if index%3 == 0 || index == 200 {
						want = append(want, id)
					}
				case "latest expired profile":
					if index == 200 {
						want = []string{id}
					}
				}
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Errorf("unexpected request %s %s", req.Method, req.URL)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				offset, size := 0, 50
				if cursor := req.URL.Query().Get("cursor"); cursor != "" {
					if _, err := fmt.Sscanf(cursor, "opaque-%d-%d", &offset, &size); err != nil {
						t.Errorf("invalid cursor: %q", cursor)
						http.Error(w, "invalid cursor", http.StatusBadRequest)
						return
					}
					if req.URL.RawQuery != "cursor="+cursor {
						t.Errorf("continuation changed: %s", req.URL.RawQuery)
					}
				} else if limit := req.URL.Query().Get("limit"); limit != "" {
					parsed, err := strconv.Atoi(limit)
					if err != nil || parsed < 1 || parsed > 200 {
						t.Errorf("invalid page size %q", limit)
						http.Error(w, "invalid limit", http.StatusBadRequest)
						return
					}
					size = parsed
				}
				end := min(offset+size, len(data))
				links := map[string]string{}
				if end < len(data) {
					links["next"] = fmt.Sprintf("https://api.appstoreconnect.apple.com%s?cursor=opaque-%d-%d", test.path, end, size)
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{"data": data[offset:end], "links": links}); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}))
			defer server.Close()
			got, err := test.fetch(context.Background(), newSigningFetchServerTestClient(t, server))
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("inventory = %v, want %v", got, want)
			}
			if calls != 2 {
				t.Fatalf("requests = %d, want 2 for 201 resources", calls)
			}
		})
	}
}

func signingInventoryIDs[T any](resources []asc.Resource[T]) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ID)
	}
	return ids
}
