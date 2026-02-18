package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestGetAlternativeDistributionDomains_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionDomains" {
			t.Fatalf("expected path /v1/alternativeDistributionDomains, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionDomains(context.Background(), WithAlternativeDistributionDomainsLimit(5)); err != nil {
		t.Fatalf("GetAlternativeDistributionDomains() error: %v", err)
	}
}

func TestGetAlternativeDistributionDomain_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionDomains","id":"domain-1","attributes":{"domain":"example.com","referenceName":"Example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionDomains/domain-1" {
			t.Fatalf("expected path /v1/alternativeDistributionDomains/domain-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionDomain(context.Background(), "domain-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionDomain() error: %v", err)
	}
}

func TestCreateAlternativeDistributionDomain_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionDomains","id":"domain-1","attributes":{"domain":"example.com","referenceName":"Example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionDomains" {
			t.Fatalf("expected path /v1/alternativeDistributionDomains, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionDomainCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionDomains {
			t.Fatalf("expected type alternativeDistributionDomains, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Domain != "example.com" {
			t.Fatalf("expected domain example.com, got %q", payload.Data.Attributes.Domain)
		}
		if payload.Data.Attributes.ReferenceName != "Example" {
			t.Fatalf("expected reference name Example, got %q", payload.Data.Attributes.ReferenceName)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionDomain(context.Background(), "example.com", "Example"); err != nil {
		t.Fatalf("CreateAlternativeDistributionDomain() error: %v", err)
	}
}

func TestDeleteAlternativeDistributionDomain_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionDomains/domain-1" {
			t.Fatalf("expected path /v1/alternativeDistributionDomains/domain-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAlternativeDistributionDomain(context.Background(), "domain-1"); err != nil {
		t.Fatalf("DeleteAlternativeDistributionDomain() error: %v", err)
	}
}

func TestGetAlternativeDistributionKeys_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionKeys" {
			t.Fatalf("expected path /v1/alternativeDistributionKeys, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionKeys(context.Background(), WithAlternativeDistributionKeysLimit(10)); err != nil {
		t.Fatalf("GetAlternativeDistributionKeys() error: %v", err)
	}
}

func TestGetAlternativeDistributionKey_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionKeys","id":"key-1","attributes":{"publicKey":"KEY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionKeys/key-1" {
			t.Fatalf("expected path /v1/alternativeDistributionKeys/key-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionKey(context.Background(), "key-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionKey() error: %v", err)
	}
}

func TestCreateAlternativeDistributionKey_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionKeys","id":"key-1","attributes":{"publicKey":"KEY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionKeys" {
			t.Fatalf("expected path /v1/alternativeDistributionKeys, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionKeyCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionKeys {
			t.Fatalf("expected type alternativeDistributionKeys, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.PublicKey != "KEY" {
			t.Fatalf("expected public key KEY, got %q", payload.Data.Attributes.PublicKey)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionKey(context.Background(), "app-1", "KEY"); err != nil {
		t.Fatalf("CreateAlternativeDistributionKey() error: %v", err)
	}
}

func TestDeleteAlternativeDistributionKey_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionKeys/key-1" {
			t.Fatalf("expected path /v1/alternativeDistributionKeys/key-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAlternativeDistributionKey(context.Background(), "key-1"); err != nil {
		t.Fatalf("DeleteAlternativeDistributionKey() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackage_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackages/pkg-1" {
			t.Fatalf("expected path /v1/alternativeDistributionPackages/pkg-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackage(context.Background(), "pkg-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionPackage() error: %v", err)
	}
}

func TestCreateAlternativeDistributionPackage_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackages" {
			t.Fatalf("expected path /v1/alternativeDistributionPackages, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionPackageCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionPackages {
			t.Fatalf("expected type alternativeDistributionPackages, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected app store version id version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionPackage(context.Background(), "version-1"); err != nil {
		t.Fatalf("CreateAlternativeDistributionPackage() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersions_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackages/pkg-1/versions" {
			t.Fatalf("expected path /v1/alternativeDistributionPackages/pkg-1/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "2" {
			t.Fatalf("expected limit=2, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersions(context.Background(), "pkg-1", WithAlternativeDistributionPackageVersionsLimit(2)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersions() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersionsRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackages/pkg-1/relationships/versions" {
			t.Fatalf("expected path /v1/alternativeDistributionPackages/pkg-1/relationships/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "3" {
			t.Fatalf("expected limit=3, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersionsRelationships(context.Background(), "pkg-1", WithLinkagesLimit(3)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersionsRelationships() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersion_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackageVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVersions/ver-1" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersion() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersionDeltas_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVersions/ver-1/deltas" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVersions/ver-1/deltas, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "4" {
			t.Fatalf("expected limit=4, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersionDeltas(context.Background(), "ver-1", WithAlternativeDistributionPackageDeltasLimit(4)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersionDeltas() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersionDeltasRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVersions/ver-1/relationships/deltas" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVersions/ver-1/relationships/deltas, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "4" {
			t.Fatalf("expected limit=4, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersionDeltasRelationships(context.Background(), "ver-1", WithLinkagesLimit(4)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersionDeltasRelationships() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersionVariants_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVersions/ver-1/variants" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVersions/ver-1/variants, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "6" {
			t.Fatalf("expected limit=6, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersionVariants(context.Background(), "ver-1", WithAlternativeDistributionPackageVariantsLimit(6)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersionVariants() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVersionVariantsRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVersions/ver-1/relationships/variants" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVersions/ver-1/relationships/variants, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "6" {
			t.Fatalf("expected limit=6, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVersionVariantsRelationships(context.Background(), "ver-1", WithLinkagesLimit(6)); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVersionVariantsRelationships() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageVariant_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackageVariants","id":"var-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageVariants/var-1" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageVariants/var-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageVariant(context.Background(), "var-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageVariant() error: %v", err)
	}
}

func TestGetAlternativeDistributionPackageDelta_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackageDeltas","id":"delta-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackageDeltas/delta-1" {
			t.Fatalf("expected path /v1/alternativeDistributionPackageDeltas/delta-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAlternativeDistributionPackageDelta(context.Background(), "delta-1"); err != nil {
		t.Fatalf("GetAlternativeDistributionPackageDelta() error: %v", err)
	}
}

func TestGetAppAlternativeDistributionKey_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionKeys","id":"key-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/alternativeDistributionKey" {
			t.Fatalf("expected path /v1/apps/app-1/alternativeDistributionKey, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppAlternativeDistributionKey(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetAppAlternativeDistributionKey() error: %v", err)
	}
}

func TestGetAppStoreVersionAlternativeDistributionPackage_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/ver-1/alternativeDistributionPackage" {
			t.Fatalf("expected path /v1/appStoreVersions/ver-1/alternativeDistributionPackage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionAlternativeDistributionPackage(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetAppStoreVersionAlternativeDistributionPackage() error: %v", err)
	}
}

func TestGetAppAlternativeDistributionKeyRelationship_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionKeys","id":"key-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/alternativeDistributionKey" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/alternativeDistributionKey, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppAlternativeDistributionKeyRelationship(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetAppAlternativeDistributionKeyRelationship() error: %v", err)
	}
}

func TestGetAppStoreVersionAlternativeDistributionPackageRelationship_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/ver-1/relationships/alternativeDistributionPackage" {
			t.Fatalf("expected path /v1/appStoreVersions/ver-1/relationships/alternativeDistributionPackage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionAlternativeDistributionPackageRelationship(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetAppStoreVersionAlternativeDistributionPackageRelationship() error: %v", err)
	}
}
