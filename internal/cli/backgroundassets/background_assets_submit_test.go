package backgroundassets

import (
	"context"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type fakeSubmitClient struct {
	assets             []ascBackgroundAssetItem
	versions           map[string][]ascBackgroundAssetVersionItem
	assetErr           error
	verErrs            map[string]error
	existingItems      map[string][]asc.ReviewSubmissionItemResource
	createSubmissionID string
	createErr          error
	createItemErrFor   map[string]error
	canceled           []string
	submitted          []string
	attached           []string
}

func (f *fakeSubmitClient) GetBackgroundAssets(_ context.Context, _ string, _ ...asc.BackgroundAssetsOption) (*asc.BackgroundAssetsResponse, error) {
	if f.assetErr != nil {
		return nil, f.assetErr
	}
	return &asc.BackgroundAssetsResponse{Data: f.assets}, nil
}

func (f *fakeSubmitClient) GetBackgroundAssetVersions(_ context.Context, backgroundAssetID string, _ ...asc.BackgroundAssetVersionsOption) (*asc.BackgroundAssetVersionsResponse, error) {
	if err, ok := f.verErrs[backgroundAssetID]; ok {
		return nil, err
	}
	return &asc.BackgroundAssetVersionsResponse{Data: f.versions[backgroundAssetID]}, nil
}

func (f *fakeSubmitClient) GetReviewSubmissionItems(_ context.Context, submissionID string, _ ...asc.ReviewSubmissionItemsOption) (*asc.ReviewSubmissionItemsResponse, error) {
	return &asc.ReviewSubmissionItemsResponse{Data: f.existingItems[submissionID]}, nil
}

func (f *fakeSubmitClient) CreateReviewSubmission(_ context.Context, _ string, platform asc.Platform) (*asc.ReviewSubmissionResponse, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	id := f.createSubmissionID
	if id == "" {
		id = "new-submission"
	}
	return &asc.ReviewSubmissionResponse{Data: asc.ReviewSubmissionResource{ID: id, Attributes: asc.ReviewSubmissionAttributes{Platform: platform}}}, nil
}

func (f *fakeSubmitClient) CreateReviewSubmissionItem(_ context.Context, submissionID string, _ asc.ReviewSubmissionItemType, itemID string) (*asc.ReviewSubmissionItemResponse, error) {
	if err, ok := f.createItemErrFor[itemID]; ok {
		return nil, err
	}
	f.attached = append(f.attached, submissionID+"|"+itemID)
	return &asc.ReviewSubmissionItemResponse{Data: asc.ReviewSubmissionItemResource{ID: submissionID + "|" + itemID}}, nil
}

func (f *fakeSubmitClient) SubmitReviewSubmission(_ context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error) {
	f.submitted = append(f.submitted, submissionID)
	return &asc.ReviewSubmissionResponse{Data: asc.ReviewSubmissionResource{ID: submissionID, Attributes: asc.ReviewSubmissionAttributes{SubmissionState: asc.ReviewSubmissionStateWaitingForReview, SubmittedDate: "2026-05-14T13:00:00Z"}}}, nil
}

func (f *fakeSubmitClient) CancelReviewSubmission(_ context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error) {
	f.canceled = append(f.canceled, submissionID)
	return &asc.ReviewSubmissionResponse{Data: asc.ReviewSubmissionResource{ID: submissionID}}, nil
}

func newFakeSubmitClient() *fakeSubmitClient {
	return &fakeSubmitClient{
		assets: []ascBackgroundAssetItem{
			{
				Type:       asc.ResourceTypeBackgroundAssets,
				ID:         "asset-1",
				Attributes: asc.BackgroundAssetAttributes{AssetPackIdentifier: "stamps.us"},
			},
			{
				Type:       asc.ResourceTypeBackgroundAssets,
				ID:         "asset-2",
				Attributes: asc.BackgroundAssetAttributes{AssetPackIdentifier: "stamps.de"},
			},
			{
				Type:       asc.ResourceTypeBackgroundAssets,
				ID:         "asset-archived",
				Attributes: asc.BackgroundAssetAttributes{AssetPackIdentifier: "stamps.archived", Archived: true},
			},
		},
		versions: map[string][]ascBackgroundAssetVersionItem{
			"asset-1": {
				{
					Type:       asc.ResourceTypeBackgroundAssetVersions,
					ID:         "ver-us-1",
					Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "1", CreatedDate: "2026-01-01T00:00:00Z"},
				},
				{
					Type:       asc.ResourceTypeBackgroundAssetVersions,
					ID:         "ver-us-2",
					Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "2", CreatedDate: "2026-02-01T00:00:00Z"},
				},
				{
					Type:       asc.ResourceTypeBackgroundAssetVersions,
					ID:         "ver-us-3-pending",
					Attributes: asc.BackgroundAssetVersionAttributes{State: "AWAITING_UPLOAD", Version: "3", CreatedDate: "2026-03-01T00:00:00Z"},
				},
			},
			"asset-2": {
				{
					Type:       asc.ResourceTypeBackgroundAssetVersions,
					ID:         "ver-de-1",
					Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "1", CreatedDate: "2026-01-15T00:00:00Z"},
				},
			},
		},
	}
}

func TestResolveBackgroundAssetSubmitItems_All(t *testing.T) {
	client := newFakeSubmitClient()
	items, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{appID: "APP", all: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 non-archived items, got %d (%v)", len(items), items)
	}
	for _, item := range items {
		switch item.BackgroundAssetID {
		case "asset-1":
			if item.BackgroundAssetVersionID != "ver-us-2" {
				t.Errorf("asset-1 should resolve to newest COMPLETE version ver-us-2, got %q", item.BackgroundAssetVersionID)
			}
		case "asset-2":
			if item.BackgroundAssetVersionID != "ver-de-1" {
				t.Errorf("asset-2 should resolve to ver-de-1, got %q", item.BackgroundAssetVersionID)
			}
		default:
			t.Errorf("unexpected asset id %q in result", item.BackgroundAssetID)
		}
	}
}

func TestResolveBackgroundAssetSubmitItems_ExplicitVersionIDs(t *testing.T) {
	client := newFakeSubmitClient()
	items, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:              "APP",
		explicitVersionIDs: []string{"vid-1", " vid-2 ", "", "vid-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items after trimming blanks and duplicates, got %d", len(items))
	}
	if items[0].BackgroundAssetVersionID != "vid-1" || items[1].BackgroundAssetVersionID != "vid-2" {
		t.Errorf("unexpected resolved version IDs: %+v", items)
	}
}

func TestResolveBackgroundAssetSubmitItems_AssetPackIdentifierMissing(t *testing.T) {
	client := newFakeSubmitClient()
	_, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:                "APP",
		assetPackIdentifiers: []string{"stamps.us", "stamps.fr"},
	})
	if err == nil || !strings.Contains(err.Error(), "stamps.fr") {
		t.Fatalf("expected error mentioning missing pack stamps.fr, got %v", err)
	}
}

func TestResolveBackgroundAssetSubmitItems_NoCompleteVersion(t *testing.T) {
	client := newFakeSubmitClient()
	client.versions["asset-2"] = []ascBackgroundAssetVersionItem{
		{
			Type:       asc.ResourceTypeBackgroundAssetVersions,
			ID:         "ver-pending",
			Attributes: asc.BackgroundAssetVersionAttributes{State: "AWAITING_UPLOAD", Version: "1"},
		},
	}
	_, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:                "APP",
		assetPackIdentifiers: []string{"stamps.de"},
	})
	if err == nil || !strings.Contains(err.Error(), "no version in state COMPLETE") {
		t.Fatalf("expected error about missing COMPLETE version, got %v", err)
	}
}

func TestResolveBackgroundAssetSubmitItems_FilterByID(t *testing.T) {
	client := newFakeSubmitClient()
	items, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:              "APP",
		backgroundAssetIDs: []string{"asset-2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].BackgroundAssetID != "asset-2" {
		t.Fatalf("expected only asset-2, got %+v", items)
	}
}

func TestResolveBackgroundAssetSubmitItems_PlatformFilter(t *testing.T) {
	client := newFakeSubmitClient()
	client.versions["asset-1"] = []ascBackgroundAssetVersionItem{
		{
			Type:       asc.ResourceTypeBackgroundAssetVersions,
			ID:         "ver-us-mac",
			Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "5", CreatedDate: "2026-04-01T00:00:00Z", Platforms: []asc.Platform{asc.PlatformMacOS}},
		},
		{
			Type:       asc.ResourceTypeBackgroundAssetVersions,
			ID:         "ver-us-ios",
			Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "2", CreatedDate: "2026-02-01T00:00:00Z", Platforms: []asc.Platform{asc.PlatformIOS}},
		},
	}
	items, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:              "APP",
		platform:           "IOS",
		backgroundAssetIDs: []string{"asset-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].BackgroundAssetVersionID != "ver-us-ios" {
		t.Fatalf("expected IOS version ver-us-ios, got %+v", items)
	}
}

func TestFetchAlreadyAttachedBackgroundAssetVersions(t *testing.T) {
	client := newFakeSubmitClient()
	client.existingItems = map[string][]asc.ReviewSubmissionItemResource{
		"sub-1": {
			{
				ID: "item-a",
				Relationships: &asc.ReviewSubmissionItemRelationships{
					BackgroundAssetVersion: &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeBackgroundAssetVersions, ID: "ver-a"}},
				},
			},
			{
				ID: "item-b",
				Relationships: &asc.ReviewSubmissionItemRelationships{
					BackgroundAssetVersion: &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeBackgroundAssetVersions, ID: "ver-b"}},
				},
			},
			{
				ID:         "item-removed",
				Attributes: asc.ReviewSubmissionItemAttributes{State: "REMOVED"},
				Relationships: &asc.ReviewSubmissionItemRelationships{
					BackgroundAssetVersion: &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeBackgroundAssetVersions, ID: "ver-removed"}},
				},
			},
			{
				ID: "item-app-version",
				Relationships: &asc.ReviewSubmissionItemRelationships{
					AppStoreVersion: &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeAppStoreVersions, ID: "asv-1"}},
				},
			},
		},
	}
	attached, err := fetchAlreadyAttachedBackgroundAssetVersions(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(attached) != 2 {
		t.Fatalf("expected 2 background-asset versions, got %d (%v)", len(attached), attached)
	}
	if _, ok := attached["ver-a"]; !ok {
		t.Errorf("missing ver-a")
	}
	if _, ok := attached["ver-b"]; !ok {
		t.Errorf("missing ver-b")
	}
	if _, ok := attached["ver-removed"]; ok {
		t.Errorf("expected REMOVED item ver-removed to be excluded")
	}
}

func TestResolveBackgroundAssetSubmitItems_PlatformMismatchProducesError(t *testing.T) {
	client := newFakeSubmitClient()
	client.versions["asset-1"] = []ascBackgroundAssetVersionItem{
		{
			Type:       asc.ResourceTypeBackgroundAssetVersions,
			ID:         "ver-mac-only",
			Attributes: asc.BackgroundAssetVersionAttributes{State: "COMPLETE", Version: "1", Platforms: []asc.Platform{asc.PlatformMacOS}},
		},
	}
	_, err := resolveBackgroundAssetSubmitItems(context.Background(), backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
		appID:              "APP",
		platform:           "IOS",
		backgroundAssetIDs: []string{"asset-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "for platform IOS") {
		t.Fatalf("expected error for platform mismatch, got %v", err)
	}
}
