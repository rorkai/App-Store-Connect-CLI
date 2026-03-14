package asc

import "testing"

func TestBackgroundAssetsRows_IncludesUsedBytes(t *testing.T) {
	usedBytes := int64(2048)
	resp := &BackgroundAssetsResponse{
		Data: []Resource[BackgroundAssetAttributes]{
			{
				ID: "asset-1",
				Attributes: BackgroundAssetAttributes{
					AssetPackIdentifier: "com.example.assetpack",
					Archived:            false,
					UsedBytes:           &usedBytes,
					CreatedDate:         "2026-03-11T00:00:00Z",
				},
			},
		},
	}

	headers, rows := backgroundAssetsRows(resp)
	if len(headers) != 5 {
		t.Fatalf("expected 5 headers, got %d (%v)", len(headers), headers)
	}
	if headers[3] != "Used Bytes" {
		t.Fatalf("expected Used Bytes header, got %q", headers[3])
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 5 {
		t.Fatalf("expected 5 columns, got %d (%v)", len(rows[0]), rows[0])
	}
	if rows[0][3] != "2048" {
		t.Fatalf("expected used bytes column 2048, got %q", rows[0][3])
	}
}
