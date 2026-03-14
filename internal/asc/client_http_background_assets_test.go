package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestGetBackgroundAssets_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/backgroundAssets" {
			t.Fatalf("expected path /v1/apps/app-1/backgroundAssets, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("filter[archived]") != "true" {
			t.Fatalf("expected filter[archived]=true, got %q", values.Get("filter[archived]"))
		}
		if values.Get("filter[assetPackIdentifier]") != "pack-1,pack-2" {
			t.Fatalf("expected filter[assetPackIdentifier]=pack-1,pack-2, got %q", values.Get("filter[assetPackIdentifier]"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetBackgroundAssets(
		context.Background(),
		"app-1",
		WithBackgroundAssetsLimit(5),
		WithBackgroundAssetsFilterArchived([]string{"true"}),
		WithBackgroundAssetsFilterAssetPackIdentifier([]string{"pack-1", "pack-2"}),
	)
	if err != nil {
		t.Fatalf("GetBackgroundAssets() error: %v", err)
	}
}

func TestGetBackgroundAsset_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"backgroundAssets","id":"asset-1","attributes":{"assetPackIdentifier":"pack","usedBytes":1234}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssets/asset-1" {
			t.Fatalf("expected path /v1/backgroundAssets/asset-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetBackgroundAsset(context.Background(), "asset-1")
	if err != nil {
		t.Fatalf("GetBackgroundAsset() error: %v", err)
	}
	if resp.Data.Attributes.UsedBytes == nil || *resp.Data.Attributes.UsedBytes != 1234 {
		t.Fatalf("expected usedBytes=1234, got %+v", resp.Data.Attributes.UsedBytes)
	}
}

func TestCreateBackgroundAsset_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"backgroundAssets","id":"asset-1","attributes":{"assetPackIdentifier":"pack"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssets" {
			t.Fatalf("expected path /v1/backgroundAssets, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BackgroundAssetCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBackgroundAssets {
			t.Fatalf("expected type backgroundAssets, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.AssetPackIdentifier != "com.example.assetpack" {
			t.Fatalf("expected assetPackIdentifier com.example.assetpack, got %q", payload.Data.Attributes.AssetPackIdentifier)
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBackgroundAsset(context.Background(), "app-1", "com.example.assetpack"); err != nil {
		t.Fatalf("CreateBackgroundAsset() error: %v", err)
	}
}

func TestUpdateBackgroundAsset_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"backgroundAssets","id":"asset-1","attributes":{"archived":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssets/asset-1" {
			t.Fatalf("expected path /v1/backgroundAssets/asset-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BackgroundAssetUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBackgroundAssets {
			t.Fatalf("expected type backgroundAssets, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "asset-1" {
			t.Fatalf("expected id asset-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Archived == nil || !*payload.Data.Attributes.Archived {
			t.Fatalf("expected archived true")
		}
		assertAuthorized(t, req)
	}, response)

	archived := true
	if _, err := client.UpdateBackgroundAsset(context.Background(), "asset-1", BackgroundAssetUpdateAttributes{Archived: &archived}); err != nil {
		t.Fatalf("UpdateBackgroundAsset() error: %v", err)
	}
}

func TestGetBackgroundAssetVersions_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssets/asset-1/versions" {
			t.Fatalf("expected path /v1/backgroundAssets/asset-1/versions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBackgroundAssetVersions(context.Background(), "asset-1", WithBackgroundAssetVersionsLimit(10)); err != nil {
		t.Fatalf("GetBackgroundAssetVersions() error: %v", err)
	}
}

func TestGetBackgroundAssetVersion_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"backgroundAssetVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetVersions/ver-1" {
			t.Fatalf("expected path /v1/backgroundAssetVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBackgroundAssetVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetBackgroundAssetVersion() error: %v", err)
	}
}

func TestCreateBackgroundAssetVersion_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"backgroundAssetVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetVersions" {
			t.Fatalf("expected path /v1/backgroundAssetVersions, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BackgroundAssetVersionCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBackgroundAssetVersions {
			t.Fatalf("expected type backgroundAssetVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.BackgroundAsset.Data.ID != "asset-1" {
			t.Fatalf("expected background asset id asset-1, got %q", payload.Data.Relationships.BackgroundAsset.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBackgroundAssetVersion(context.Background(), "asset-1"); err != nil {
		t.Fatalf("CreateBackgroundAssetVersion() error: %v", err)
	}
}

func TestGetBackgroundAssetUploadFiles_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetVersions/ver-1/backgroundAssetUploadFiles" {
			t.Fatalf("expected path /v1/backgroundAssetVersions/ver-1/backgroundAssetUploadFiles, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "3" {
			t.Fatalf("expected limit=3, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBackgroundAssetUploadFiles(context.Background(), "ver-1", WithBackgroundAssetUploadFilesLimit(3)); err != nil {
		t.Fatalf("GetBackgroundAssetUploadFiles() error: %v", err)
	}
}

func TestGetBackgroundAssetUploadFile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"backgroundAssetUploadFiles","id":"file-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetUploadFiles/file-1" {
			t.Fatalf("expected path /v1/backgroundAssetUploadFiles/file-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBackgroundAssetUploadFile(context.Background(), "file-1"); err != nil {
		t.Fatalf("GetBackgroundAssetUploadFile() error: %v", err)
	}
}

func TestCreateBackgroundAssetUploadFile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"backgroundAssetUploadFiles","id":"file-1","attributes":{"fileName":"asset.zip","fileSize":12,"assetType":"ASSET"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetUploadFiles" {
			t.Fatalf("expected path /v1/backgroundAssetUploadFiles, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BackgroundAssetUploadFileCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBackgroundAssetUploadFiles {
			t.Fatalf("expected type backgroundAssetUploadFiles, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.AssetType != BackgroundAssetUploadFileAssetTypeAsset {
			t.Fatalf("expected asset type ASSET, got %q", payload.Data.Attributes.AssetType)
		}
		if payload.Data.Attributes.FileName != "asset.zip" {
			t.Fatalf("expected file name asset.zip, got %q", payload.Data.Attributes.FileName)
		}
		if payload.Data.Relationships.BackgroundAssetVersion.Data.ID != "ver-1" {
			t.Fatalf("expected version id ver-1, got %q", payload.Data.Relationships.BackgroundAssetVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBackgroundAssetUploadFile(context.Background(), "ver-1", "asset.zip", 12, BackgroundAssetUploadFileAssetTypeAsset); err != nil {
		t.Fatalf("CreateBackgroundAssetUploadFile() error: %v", err)
	}
}

func TestUpdateBackgroundAssetUploadFile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"backgroundAssetUploadFiles","id":"file-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/backgroundAssetUploadFiles/file-1" {
			t.Fatalf("expected path /v1/backgroundAssetUploadFiles/file-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BackgroundAssetUploadFileUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBackgroundAssetUploadFiles {
			t.Fatalf("expected type backgroundAssetUploadFiles, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "file-1" {
			t.Fatalf("expected id file-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true")
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	if _, err := client.UpdateBackgroundAssetUploadFile(context.Background(), "file-1", BackgroundAssetUploadFileUpdateAttributes{Uploaded: &uploaded}); err != nil {
		t.Fatalf("UpdateBackgroundAssetUploadFile() error: %v", err)
	}
}
