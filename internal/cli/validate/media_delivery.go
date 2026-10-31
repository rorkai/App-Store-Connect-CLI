package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	mediaLabelPreviews   = "app previews"
	mediaLabelPlacements = "asset library placements"

	// versionMediaPlacementTypes limits placement reads to the screenshots and
	// previews that appear on the version's product page.
	versionMediaPlacementTypes = "APP_SCREENSHOT,IMESSAGE_APP_SCREENSHOT,APP_PREVIEW"
	mediaCollectionPageLimit   = 200
)

type mediaDeliveryData struct {
	previewSets []validation.PreviewSet
	placements  []validation.MediaPlacement
	gaps        []validation.MediaReadGap
}

// fetchMediaDelivery reads the media that fetchScreenshotSets does not cover.
// A read failure becomes a per-locale gap so one unreadable source reports as
// unverified instead of aborting the report or reading as delivered.
func fetchMediaDelivery(ctx context.Context, client *asc.Client, localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) (mediaDeliveryData, error) {
	ctx = withReadinessRequestGate(ctx)
	previewSets := make([][]validation.PreviewSet, len(localizations))
	placements := make([][]validation.MediaPlacement, len(localizations))
	previewGaps := make([]*validation.MediaReadGap, len(localizations))
	placementGaps := make([]*validation.MediaReadGap, len(localizations))

	tasks := make([]readinessTask, 0, len(localizations)*2)
	for index, localization := range localizations {
		tasks = append(tasks,
			func(taskCtx context.Context) error {
				sets, err := fetchLocalizationPreviewSets(taskCtx, client, localization)
				if err != nil {
					gap, gapErr := mediaReadGap(taskCtx, err, localization, mediaLabelPreviews)
					previewGaps[index] = gap
					return gapErr
				}
				previewSets[index] = sets
				return nil
			},
			func(taskCtx context.Context) error {
				read, err := fetchLocalizationPlacements(taskCtx, client, localization)
				if err != nil {
					gap, gapErr := mediaReadGap(taskCtx, err, localization, mediaLabelPlacements)
					placementGaps[index] = gap
					return gapErr
				}
				placements[index] = read
				return nil
			},
		)
	}
	if err := runReadinessTasks(ctx, tasks...); err != nil {
		return mediaDeliveryData{}, err
	}

	var data mediaDeliveryData
	for index := range localizations {
		data.previewSets = append(data.previewSets, previewSets[index]...)
		data.placements = append(data.placements, placements[index]...)
		for _, gap := range []*validation.MediaReadGap{previewGaps[index], placementGaps[index]} {
			if gap != nil {
				data.gaps = append(data.gaps, *gap)
			}
		}
	}
	return data, nil
}

func mediaReadGap(ctx context.Context, err error, localization asc.Resource[asc.AppStoreVersionLocalizationAttributes], media string) (*validation.MediaReadGap, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return nil, err
	}
	label := media + " delivery"
	reason, ok := metadataCheckSkipReason(err, label)
	if !ok {
		reason = fmt.Sprintf("Validation skipped %s because its App Store Connect data could not be read: %v", label, err)
	}
	return &validation.MediaReadGap{
		Media:          media,
		Locale:         localization.Attributes.Locale,
		LocalizationID: localization.ID,
		Reason:         reason,
	}, nil
}

func fetchLocalizationPreviewSets(ctx context.Context, client *asc.Client, localization asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.PreviewSet, error) {
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppPreviewSetsResponse, error) {
		return client.GetAppStoreVersionLocalizationPreviewSets(
			requestCtx,
			localization.ID,
			asc.WithAppStoreVersionLocalizationPreviewSetsLimit(mediaCollectionPageLimit),
			asc.WithAppStoreVersionLocalizationPreviewSetsIncludePreviews(),
		)
	})
	if err != nil {
		return nil, err
	}
	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetAppStoreVersionLocalizationPreviewSets(requestCtx, "", asc.WithAppStoreVersionLocalizationPreviewSetsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, err
	}
	response, ok := paginated.(*asc.AppPreviewSetsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected preview sets pagination response type %T", paginated)
	}

	included := asc.IncludedAppPreviews(response)
	sets := make([]validation.PreviewSet, 0, len(response.Data))
	for _, set := range response.Data {
		resources, inlined := included[set.ID]
		if !inlined {
			resources, err = fetchSetPreviews(ctx, client, set.ID)
			if err != nil {
				return nil, err
			}
		}
		previews := make([]validation.Preview, 0, len(resources))
		for _, preview := range resources {
			state, details := previewDeliveryState(preview.Attributes)
			previews = append(previews, validation.Preview{
				ID:             preview.ID,
				FileName:       preview.Attributes.FileName,
				DeliveryState:  state,
				DeliveryErrors: details,
			})
		}
		sets = append(sets, validation.PreviewSet{
			ID:             set.ID,
			PreviewType:    set.Attributes.PreviewType,
			Locale:         localization.Attributes.Locale,
			LocalizationID: localization.ID,
			Previews:       previews,
		})
	}
	return sets, nil
}

func fetchSetPreviews(ctx context.Context, client *asc.Client, setID string) ([]asc.Resource[asc.AppPreviewAttributes], error) {
	firstPage, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppPreviewsResponse, error) {
		return client.GetAppPreviews(requestCtx, setID)
	})
	if err != nil {
		return nil, err
	}
	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(ctx, func(requestCtx context.Context) (asc.PaginatedResponse, error) {
			return client.GetAppPreviews(requestCtx, "", asc.WithAppPreviewsNextURL(nextURL))
		})
	})
	if err != nil {
		return nil, err
	}
	response, ok := paginated.(*asc.AppPreviewsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected previews pagination response type %T", paginated)
	}
	return response.Data, nil
}

func fetchLocalizationPlacements(ctx context.Context, client *asc.Client, localization asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.MediaPlacement, error) {
	query := url.Values{}
	query.Set("filter[placementType]", versionMediaPlacementTypes)
	query.Set("limit", fmt.Sprint(mediaCollectionPageLimit))
	target := "/v1/appStoreVersionLocalizations/" + url.PathEscape(localization.ID) + "/placements?" + query.Encode()

	// The whole page walk holds one request slot, because RawPaginatedGET
	// issues its pages internally.
	body, err := doReadinessRequest(ctx, func(requestCtx context.Context) ([]byte, error) {
		return client.RawPaginatedGET(requestCtx, target, nil)
	})
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				PlacementType  string            `json:"placementType"`
				PlacementGroup string            `json:"placementGroup"`
				State          string            `json:"state"`
				StateDetails   []asc.StateDetail `json:"stateDetails"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse placements: %w", err)
	}

	placements := make([]validation.MediaPlacement, 0, len(response.Data))
	for _, placement := range response.Data {
		placements = append(placements, validation.MediaPlacement{
			ID:             placement.ID,
			Locale:         localization.Attributes.Locale,
			PlacementType:  placement.Attributes.PlacementType,
			PlacementGroup: placement.Attributes.PlacementGroup,
			State:          placement.Attributes.State,
			StateDetails:   stateDetailTexts(placement.Attributes.StateDetails),
		})
	}
	return placements, nil
}

func screenshotDeliveryState(state *asc.AssetDeliveryState) (string, []string) {
	if state == nil {
		return "", nil
	}
	details := make([]string, 0, len(state.Errors))
	for _, detail := range state.Errors {
		text := detail.Description
		if strings.TrimSpace(text) == "" {
			text = detail.Message
		}
		details = appendDeliveryDetail(details, detail.Code, text)
	}
	return state.State, details
}

// previewDeliveryState prefers videoDeliveryState, because the deprecated
// assetDeliveryState covers the upload but not video processing.
func previewDeliveryState(attributes asc.AppPreviewAttributes) (string, []string) {
	if video := attributes.VideoDeliveryState; video != nil && video.State != nil {
		return *video.State, stateDetailTexts(video.Errors)
	}
	return screenshotDeliveryState(attributes.AssetDeliveryState)
}

func stateDetailTexts(details []asc.StateDetail) []string {
	texts := make([]string, 0, len(details))
	for _, detail := range details {
		texts = appendDeliveryDetail(texts, detail.Code, detail.Description)
	}
	return texts
}

func appendDeliveryDetail(details []string, code, text string) []string {
	code = strings.TrimSpace(code)
	text = strings.TrimSpace(text)
	switch {
	case code != "" && text != "":
		return append(details, code+": "+text)
	case code != "":
		return append(details, code)
	case text != "":
		return append(details, text)
	}
	return details
}
