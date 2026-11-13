package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestGetPassTypeIDs_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds" {
			t.Fatalf("expected path /v1/passTypeIds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[identifier]") != "pass.com.example" {
			t.Fatalf("expected filter[identifier]=pass.com.example, got %q", values.Get("filter[identifier]"))
		}
		if values.Get("filter[name]") != "Wallet" {
			t.Fatalf("expected filter[name]=Wallet, got %q", values.Get("filter[name]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeIDs(
		context.Background(),
		WithPassTypeIDsFilterIdentifier("pass.com.example"),
		WithPassTypeIDsFilterName("Wallet"),
	); err != nil {
		t.Fatalf("GetPassTypeIDs() error: %v", err)
	}
}

func TestGetPassTypeIDs_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds" {
			t.Fatalf("expected path /v1/passTypeIds, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeIDs(context.Background(), WithPassTypeIDsLimit(10)); err != nil {
		t.Fatalf("GetPassTypeIDs() error: %v", err)
	}
}

func TestGetPassTypeIDs_WithFieldsIncludeSort(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/passTypeIds" {
			t.Fatalf("expected path /v1/passTypeIds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[id]") != "p1,p2" {
			t.Fatalf("expected filter[id]=p1,p2, got %q", values.Get("filter[id]"))
		}
		if values.Get("sort") != "-name" {
			t.Fatalf("expected sort -name, got %q", values.Get("sort"))
		}
		if values.Get("fields[passTypeIds]") != "name,identifier" {
			t.Fatalf("expected fields[passTypeIds], got %q", values.Get("fields[passTypeIds]"))
		}
		if values.Get("fields[certificates]") != "serialNumber" {
			t.Fatalf("expected fields[certificates], got %q", values.Get("fields[certificates]"))
		}
		if values.Get("include") != "certificates" {
			t.Fatalf("expected include certificates, got %q", values.Get("include"))
		}
		if values.Get("limit[certificates]") != "10" {
			t.Fatalf("expected limit[certificates]=10, got %q", values.Get("limit[certificates]"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetPassTypeIDs(
		context.Background(),
		WithPassTypeIDsFilterIDs([]string{"p1", "p2"}),
		WithPassTypeIDsSort("-name"),
		WithPassTypeIDsFields([]string{"name", "identifier"}),
		WithPassTypeIDsCertificateFields([]string{"serialNumber"}),
		WithPassTypeIDsInclude([]string{"certificates"}),
		WithPassTypeIDsCertificatesLimit(10),
	)
	if err != nil {
		t.Fatalf("GetPassTypeIDs() error: %v", err)
	}
}

func TestGetPassTypeIDs_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/passTypeIds?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeIDs(context.Background(), WithPassTypeIDsNextURL(next)); err != nil {
		t.Fatalf("GetPassTypeIDs() error: %v", err)
	}
}

func TestGetPassTypeID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds/p1" {
			t.Fatalf("expected path /v1/passTypeIds/p1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeID(context.Background(), "p1"); err != nil {
		t.Fatalf("GetPassTypeID() error: %v", err)
	}
}

func TestGetPassTypeID_WithFieldsIncludeLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/passTypeIds/p1" {
			t.Fatalf("expected path /v1/passTypeIds/p1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[passTypeIds]") != "name,identifier" {
			t.Fatalf("expected fields[passTypeIds], got %q", values.Get("fields[passTypeIds]"))
		}
		if values.Get("fields[certificates]") != "serialNumber" {
			t.Fatalf("expected fields[certificates], got %q", values.Get("fields[certificates]"))
		}
		if values.Get("include") != "certificates" {
			t.Fatalf("expected include certificates, got %q", values.Get("include"))
		}
		if values.Get("limit[certificates]") != "5" {
			t.Fatalf("expected limit[certificates]=5, got %q", values.Get("limit[certificates]"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetPassTypeID(
		context.Background(), "p1",
		WithPassTypeIDFields([]string{"name", "identifier"}),
		WithPassTypeIDCertificateFields([]string{"serialNumber"}),
		WithPassTypeIDInclude([]string{"certificates"}),
		WithPassTypeIDCertificatesIncludeLimit(5),
	)
	if err != nil {
		t.Fatalf("GetPassTypeID() error: %v", err)
	}
}

func TestCreatePassTypeID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds" {
			t.Fatalf("expected path /v1/passTypeIds, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload PassTypeIDCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypePassTypeIds {
			t.Fatalf("expected type passTypeIds, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Identifier != "pass.com.example" {
			t.Fatalf("expected identifier pass.com.example, got %q", payload.Data.Attributes.Identifier)
		}
		if payload.Data.Attributes.Name != "Wallet" {
			t.Fatalf("expected name Wallet, got %q", payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := PassTypeIDCreateAttributes{
		Name:       "Wallet",
		Identifier: "pass.com.example",
	}
	if _, err := client.CreatePassTypeID(context.Background(), attrs); err != nil {
		t.Fatalf("CreatePassTypeID() error: %v", err)
	}
}

func TestUpdatePassTypeID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"passTypeIds","id":"p1","attributes":{"name":"Updated","identifier":"pass.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds/p1" {
			t.Fatalf("expected path /v1/passTypeIds/p1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload PassTypeIDUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypePassTypeIds {
			t.Fatalf("expected type passTypeIds, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "p1" {
			t.Fatalf("expected id p1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || payload.Data.Attributes.Name.Value == nil {
			t.Fatalf("expected name Updated, got %v", payload.Data.Attributes)
		}
		if *payload.Data.Attributes.Name.Value != "Updated" {
			t.Fatalf("expected name Updated, got %v", payload.Data.Attributes.Name.Value)
		}
		assertAuthorized(t, req)
	}, response)

	updated := "Updated"
	attrs := PassTypeIDUpdateAttributes{Name: &NullableString{Value: &updated}}
	if _, err := client.UpdatePassTypeID(context.Background(), "p1", attrs); err != nil {
		t.Fatalf("UpdatePassTypeID() error: %v", err)
	}
}

func TestDeletePassTypeID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds/p1" {
			t.Fatalf("expected path /v1/passTypeIds/p1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeletePassTypeID(context.Background(), "p1"); err != nil {
		t.Fatalf("DeletePassTypeID() error: %v", err)
	}
}

func TestGetPassTypeIDCertificates_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"APPLE_PAY"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds/p1/certificates" {
			t.Fatalf("expected path /v1/passTypeIds/p1/certificates, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeIDCertificates(context.Background(), "p1", WithPassTypeIDCertificatesLimit(10)); err != nil {
		t.Fatalf("GetPassTypeIDCertificates() error: %v", err)
	}
}

func TestGetPassTypeIDCertificates_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/passTypeIds/p1/certificates" {
			t.Fatalf("expected path /v1/passTypeIds/p1/certificates, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[displayName]") != "Display" {
			t.Fatalf("expected filter[displayName]=Display, got %q", values.Get("filter[displayName]"))
		}
		if values.Get("filter[certificateType]") != "PASS_TYPE_ID" {
			t.Fatalf("expected filter[certificateType]=PASS_TYPE_ID, got %q", values.Get("filter[certificateType]"))
		}
		if values.Get("filter[serialNumber]") != "1234" {
			t.Fatalf("expected filter[serialNumber]=1234, got %q", values.Get("filter[serialNumber]"))
		}
		if values.Get("filter[id]") != "c1,c2" {
			t.Fatalf("expected filter[id]=c1,c2, got %q", values.Get("filter[id]"))
		}
		if values.Get("sort") != "-displayName" {
			t.Fatalf("expected sort -displayName, got %q", values.Get("sort"))
		}
		if values.Get("fields[certificates]") != "serialNumber,certificateType" {
			t.Fatalf("expected fields[certificates], got %q", values.Get("fields[certificates]"))
		}
		if values.Get("fields[passTypeIds]") != "identifier" {
			t.Fatalf("expected fields[passTypeIds], got %q", values.Get("fields[passTypeIds]"))
		}
		if values.Get("include") != "passTypeId" {
			t.Fatalf("expected include passTypeId, got %q", values.Get("include"))
		}
		if values.Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetPassTypeIDCertificates(
		context.Background(),
		"p1",
		WithPassTypeIDCertificatesFilterDisplayNames([]string{"Display"}),
		WithPassTypeIDCertificatesFilterCertificateTypes([]string{"PASS_TYPE_ID"}),
		WithPassTypeIDCertificatesFilterSerialNumbers([]string{"1234"}),
		WithPassTypeIDCertificatesFilterIDs([]string{"c1", "c2"}),
		WithPassTypeIDCertificatesSort("-displayName"),
		WithPassTypeIDCertificatesFields([]string{"serialNumber", "certificateType"}),
		WithPassTypeIDCertificatesPassTypeIDFields([]string{"identifier"}),
		WithPassTypeIDCertificatesInclude([]string{"passTypeId"}),
		WithPassTypeIDCertificatesLimit(25),
	)
	if err != nil {
		t.Fatalf("GetPassTypeIDCertificates() error: %v", err)
	}
}

func TestGetPassTypeIDCertificatesRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/passTypeIds/p1/relationships/certificates" {
			t.Fatalf("expected path /v1/passTypeIds/p1/relationships/certificates, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPassTypeIDCertificatesRelationships(context.Background(), "p1", Limit[LinkagesOption](5)); err != nil {
		t.Fatalf("GetPassTypeIDCertificatesRelationships() error: %v", err)
	}
}

func TestGetCertificatePassTypeID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"passTypeIds","id":"p1","attributes":{"name":"Wallet","identifier":"pass.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates/c1/passTypeId" {
			t.Fatalf("expected path /v1/certificates/c1/passTypeId, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificatePassTypeID(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCertificatePassTypeID() error: %v", err)
	}
}

func TestGetCertificatePassTypeIDRelationship_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"passTypeIds","id":"p1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates/c1/relationships/passTypeId" {
			t.Fatalf("expected path /v1/certificates/c1/relationships/passTypeId, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificatePassTypeIDRelationship(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCertificatePassTypeIDRelationship() error: %v", err)
	}
}
