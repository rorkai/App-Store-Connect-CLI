package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestProfileAttributesJSONPreservesSparseFields(t *testing.T) {
	var attributes ProfileAttributes
	if err := json.Unmarshal([]byte(`{"expirationDate":"2026-08-24T00:00:00Z"}`), &attributes); err != nil {
		t.Fatalf("unmarshal sparse profile attributes: %v", err)
	}

	encoded, err := json.Marshal(attributes)
	if err != nil {
		t.Fatalf("marshal sparse profile attributes: %v", err)
	}
	if got, want := string(encoded), `{"expirationDate":"2026-08-24T00:00:00Z"}`; got != want {
		t.Fatalf("sparse profile attributes JSON = %s, want %s", got, want)
	}
}

func TestProfileAttributesJSONKeepsFullResponseFields(t *testing.T) {
	attributes := ProfileAttributes{
		Name:           "Development",
		ProfileType:    "IOS_APP_DEVELOPMENT",
		ExpirationDate: "2026-08-24T00:00:00Z",
	}

	encoded, err := json.Marshal(attributes)
	if err != nil {
		t.Fatalf("marshal full profile attributes: %v", err)
	}
	if got, want := string(encoded), `{"name":"Development","profileType":"IOS_APP_DEVELOPMENT","expirationDate":"2026-08-24T00:00:00Z"}`; got != want {
		t.Fatalf("full profile attributes JSON = %s, want %s", got, want)
	}
}

func TestProfileAttributesJSONPreservesExplicitEmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty name", input: `{"name":""}`, want: `{"name":""}`},
		{name: "empty profile type", input: `{"profileType":""}`, want: `{"profileType":""}`},
		{name: "null platform", input: `{"platform":null}`, want: `{"platform":null}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var attributes ProfileAttributes
			if err := json.Unmarshal([]byte(test.input), &attributes); err != nil {
				t.Fatalf("unmarshal profile attributes: %v", err)
			}
			encoded, err := json.Marshal(attributes)
			if err != nil {
				t.Fatalf("marshal profile attributes: %v", err)
			}
			if got := string(encoded); got != test.want {
				t.Fatalf("profile attributes JSON = %s, want %s", got, test.want)
			}
		})
	}
}

func TestProfileAttributesJSONUsesModifiedValues(t *testing.T) {
	var attributes ProfileAttributes
	if err := json.Unmarshal([]byte(`{"name":"Old","platform":null}`), &attributes); err != nil {
		t.Fatalf("unmarshal profile attributes: %v", err)
	}
	attributes.Name = "New"

	encoded, err := json.Marshal(attributes)
	if err != nil {
		t.Fatalf("marshal profile attributes: %v", err)
	}
	if got, want := string(encoded), `{"name":"New","platform":null}`; got != want {
		t.Fatalf("profile attributes JSON = %s, want %s", got, want)
	}
}

func TestProfilesResponseJSONPreservesRelationshipOnlyAttributes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "attributes omitted",
			body: `{"data":[{"type":"profiles","id":"profile-1","relationships":{"devices":{"data":[]}}}],"links":{},"included":[]}`,
			want: `{"data":[{"type":"profiles","id":"profile-1","relationships":{"devices":{"data":[]}}}],"links":{},"included":[]}`,
		},
		{
			name: "attributes explicitly empty",
			body: `{"data":[{"type":"profiles","id":"profile-1","attributes":{},"relationships":{"devices":{"data":[]}}}],"links":{},"included":[]}`,
			want: `{"data":[{"type":"profiles","id":"profile-1","attributes":{},"relationships":{"devices":{"data":[]}}}],"links":{},"included":[]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response ProfilesResponse
			if err := json.Unmarshal([]byte(test.body), &response); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			if got := string(encoded); got != test.want {
				t.Fatalf("response JSON = %s, want %s", got, test.want)
			}
		})
	}
}

func TestGetProfiles_WithQuerySurface(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles" {
			t.Fatalf("expected path /v1/profiles, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		checks := map[string]string{
			"filter[name]":         "Development,Store",
			"filter[id]":           "profile-1,profile-2",
			"filter[profileType]":  "IOS_APP_DEVELOPMENT,IOS_APP_STORE",
			"filter[profileState]": "ACTIVE,INVALID",
			"sort":                 "name,-id",
			"fields[profiles]":     "name,expirationDate",
			"fields[bundleIds]":    "identifier",
			"fields[devices]":      "name,udid",
			"fields[certificates]": "displayName,serialNumber",
			"include":              "bundleId,devices,certificates",
			"limit[devices]":       "7",
			"limit[certificates]":  "9",
			"limit":                "5",
		}
		for key, want := range checks {
			if got := values.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfiles(
		context.Background(),
		WithProfilesFilterName(" Development, Store "),
		WithProfilesFilterIDs([]string{" profile-1 ", "profile-2"}),
		WithProfilesTypes([]string{"IOS_APP_DEVELOPMENT", "IOS_APP_STORE"}),
		WithProfilesStates([]string{"ACTIVE", "INVALID"}),
		WithProfilesSort("name,-id"),
		WithProfilesFields([]string{"name", "expirationDate"}),
		WithProfilesBundleIDFields([]string{"identifier"}),
		WithProfilesDeviceFields([]string{"name", "udid"}),
		WithProfilesCertificateFields([]string{"displayName", "serialNumber"}),
		WithProfilesInclude([]string{"bundleId", "devices", "certificates"}),
		WithProfilesDevicesLimit(7),
		WithProfilesCertificatesLimit(9),
		Limit[ProfilesOption](5),
	); err != nil {
		t.Fatalf("GetProfiles() error: %v", err)
	}
}

func TestGetProfiles_QueryOptionsAreIgnoredForNextURL(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/profiles?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if got := req.URL.String(); got != next {
			t.Fatalf("request URL = %q, want %q", got, next)
		}
	}, response)

	if _, err := client.GetProfiles(
		context.Background(),
		NextURL[ProfilesOption](next),
		WithProfilesFields([]string{"name"}),
		WithProfilesInclude([]string{"devices"}),
	); err != nil {
		t.Fatalf("GetProfiles() error: %v", err)
	}
}
