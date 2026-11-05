package appleads

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPlatformMapsRepresentativeRequests(t *testing.T) {
	var seen []string
	client, err := NewClient(Credentials{AccessToken: "ACCESS", AdAccountID: "ACCOUNT"}, WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		if req.Body != nil {
			var readErr error
			body, readErr = io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read request: %v", readErr)
			}
		}
		seen = append(seen, req.Method+" "+req.URL.Path+" "+req.Header.Get("X-AP-Context")+" "+string(body))
		return jsonResponse(200, `{"data":{}}`), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}

	brandQuery, _ := PlatformEndpointByCommandPath("brands", "find")
	if _, err := client.Do(context.Background(), brandQuery, nil, url.Values{}, []byte(`{"pagination":{"offset":0}}`)); err != nil {
		t.Fatalf("brand query: %v", err)
	}
	creativeUpdate, _ := PlatformEndpointByCommandPath("creatives", "update")
	if _, err := client.Do(context.Background(), creativeUpdate, map[string]string{"id": "creative-123"}, nil, []byte(`{"name":"Updated"}`)); err != nil {
		t.Fatalf("creative update: %v", err)
	}
	locationView, _ := PlatformEndpointByCommandPath("locations", "view")
	if _, err := client.Do(context.Background(), locationView, map[string]string{"id": "location-123"}, nil, nil); err != nil {
		t.Fatalf("location view: %v", err)
	}

	want := []string{
		`POST /v1/business-brands/query adAccountId=ACCOUNT; {"pagination":{"offset":0}}`,
		`PUT /v1/creatives/creative-123 adAccountId=ACCOUNT; {"name":"Updated"}`,
		`GET /v1/locations/location-123 adAccountId=ACCOUNT; `,
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
}

func TestPlatformMapsEndpointIdentifierTypesAndUploadInventory(t *testing.T) {
	for _, path := range [][]string{
		{"brands", "view"},
		{"business-categories", "view"},
		{"locations", "view"},
		{"location-groups", "view"},
		{"location-groups", "update"},
		{"location-groups", "delete"},
		{"creatives", "view"},
		{"creatives", "update"},
		{"creatives", "delete"},
		{"assets", "view"},
		{"assets", "delete"},
	} {
		spec, ok := PlatformEndpointByCommandPath(path...)
		if !ok || len(spec.PathParams) != 1 || spec.PathParams[0].Type != ParamString {
			t.Fatalf("%s identifier metadata = %+v", strings.Join(path, " "), spec.PathParams)
		}
	}
	upload, ok := PlatformEndpointByCommandPath("assets", "upload")
	if !ok || upload.Method != "POST" || upload.Path != "v1/assets/upload" || upload.BodyKind != BodyMultipart {
		t.Fatalf("assets upload inventory = %+v", upload)
	}
	assets, ok := PlatformEndpointByCommandPath("assets", "find")
	if !ok || !assets.BodyOptional || assets.BodyFileExample != "query.json" {
		t.Fatalf("assets find query metadata = %+v", assets)
	}
	for _, want := range []string{"default page", "non-deleted assets", "selected ad account", "promotedObjectId", "providerAssetId", "assetType (IMAGE)"} {
		if !strings.Contains(assets.BodyHint, want) {
			t.Fatalf("assets find body hint = %q, want %q", assets.BodyHint, want)
		}
	}
}
