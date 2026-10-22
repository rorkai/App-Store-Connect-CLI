package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type versionReadinessData struct {
	response      *asc.AppStoreVersionResponse
	localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes]
	reviewDetails *validation.ReviewDetails
	build         *validation.Build
}

type appInfoReadinessData struct {
	appInfoID            string
	app                  asc.Resource[asc.AppAttributes]
	localizations        []asc.Resource[asc.AppInfoLocalizationAttributes]
	primaryCategoryID    string
	ageRatingDeclaration *validation.AgeRatingDeclaration
	response             *asc.AppInfosResponse
	included             includedResourceIndex
}

func fetchVersionReadinessData(ctx context.Context, client *asc.Client, versionID string, suppliedBuild *validation.Build) (versionReadinessData, error) {
	response, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppStoreVersionResponse, error) {
		return client.GetAppStoreVersion(
			requestCtx,
			versionID,
			asc.WithAppStoreVersionInclude([]string{"appStoreVersionLocalizations", "build", "appStoreReviewDetail"}),
			asc.WithAppStoreVersionLocalizationsIncludeLimit(compoundLocalizationLimit),
		)
	})
	if err != nil {
		return versionReadinessData{}, fmt.Errorf("failed to fetch app store version: %w", err)
	}

	result := versionReadinessData{response: response, build: copyBuild(suppliedBuild)}
	included := newIncludedResourceIndex(response.Included)

	localizations, localizationsResolved := resolveCompoundToMany[asc.AppStoreVersionLocalizationAttributes](
		response.Data.Relationships,
		included,
		"appStoreVersionLocalizations",
		asc.ResourceTypeAppStoreVersionLocalizations,
	)
	if localizationsResolved {
		result.localizations = localizations
	}

	reviewResource, reviewState := resolveCompoundToOne[asc.AppStoreReviewDetailAttributes](
		response.Data.Relationships,
		included,
		"appStoreReviewDetail",
		asc.ResourceTypeAppStoreReviewDetails,
	)
	if reviewState == compoundLinkageResolved {
		result.reviewDetails = mapReviewDetails(reviewResource)
	}

	buildState := compoundLinkageResolved
	if suppliedBuild == nil {
		buildResource, state := resolveCompoundToOne[asc.BuildAttributes](
			response.Data.Relationships,
			included,
			"build",
			asc.ResourceTypeBuilds,
		)
		buildState = state
		if state == compoundLinkageResolved {
			result.build = mapBuild(buildResource)
		}
	}

	tasks := make([]readinessTask, 0, 3)
	if !localizationsResolved {
		tasks = append(tasks, func(taskCtx context.Context) error {
			localizationsResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppStoreVersionLocalizationsResponse, error) {
				return client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
			})
			if err != nil {
				return fmt.Errorf("failed to fetch version localizations: %w", err)
			}
			result.localizations = localizationsResponse.Data
			return nil
		})
	}
	if reviewState == compoundLinkageFallback {
		tasks = append(tasks, func(taskCtx context.Context) error {
			reviewResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppStoreReviewDetailResponse, error) {
				return client.GetAppStoreReviewDetailForVersion(requestCtx, versionID)
			})
			if err != nil {
				if asc.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch review details: %w", err)
			}
			result.reviewDetails = mapReviewDetails(reviewResponse.Data)
			return nil
		})
	}
	if suppliedBuild == nil && buildState == compoundLinkageFallback {
		tasks = append(tasks, func(taskCtx context.Context) error {
			buildResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.BuildResponse, error) {
				return client.GetAppStoreVersionBuild(requestCtx, versionID)
			})
			if err != nil {
				if asc.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch attached build: %w", err)
			}
			if strings.TrimSpace(buildResponse.Data.ID) != "" {
				result.build = mapBuild(buildResponse.Data)
			}
			return nil
		})
	}

	if err := runReadinessTasks(ctx, tasks...); err != nil {
		return versionReadinessData{}, err
	}
	return result, nil
}

func fetchAppInfoReadinessData(ctx context.Context, client *asc.Client, appID string) (appInfoReadinessData, error) {
	response, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppInfosResponse, error) {
		return client.GetAppInfos(
			requestCtx,
			appID,
			asc.WithAppInfoInclude([]string{"app", "ageRatingDeclaration", "appInfoLocalizations", "primaryCategory"}),
			asc.WithAppInfoLocalizationsIncludeLimit(compoundLocalizationLimit),
		)
	})
	if err != nil {
		return appInfoReadinessData{}, fmt.Errorf("failed to fetch app info: %w", err)
	}
	if response == nil {
		return appInfoReadinessData{}, fmt.Errorf("failed to fetch app info: empty response")
	}
	if strings.TrimSpace(response.Links.Next) != "" {
		return appInfoReadinessData{}, fmt.Errorf("failed to select app info: %w", shared.MarkAmbiguousSelectionSample(shared.AmbiguousAppInfoError(appID, "", asc.AppInfoCandidates(response.Data))))
	}

	appInfoID := shared.SelectBestAppInfoID(response)
	if strings.TrimSpace(appInfoID) == "" {
		return appInfoReadinessData{}, fmt.Errorf("failed to select app info for app")
	}

	result := appInfoReadinessData{
		appInfoID: appInfoID,
		response:  response,
		included:  newIncludedResourceIndex(response.Included),
	}
	selected, selectedOK := selectAppInfoResource(response.Data, appInfoID)

	appState := compoundLinkageFallback
	ageRatingState := compoundLinkageFallback
	resolveAgeRatingNow := len(response.Data) == 1
	localizationsResolved := false
	primaryCategoryState := compoundLinkageFallback
	if selectedOK {
		appResource, state := resolveCompoundToOne[asc.AppAttributes](selected.Relationships, result.included, "app", asc.ResourceTypeApps)
		appState = state
		if state == compoundLinkageResolved && strings.TrimSpace(appResource.ID) == strings.TrimSpace(appID) {
			result.app = appResource
		} else if state == compoundLinkageResolved {
			appState = compoundLinkageFallback
		}

		if resolveAgeRatingNow {
			ageRatingResource, state := resolveCompoundToOne[asc.AgeRatingDeclarationAttributes](
				selected.Relationships,
				result.included,
				"ageRatingDeclaration",
				asc.ResourceTypeAgeRatingDeclarations,
			)
			ageRatingState = state
			if state == compoundLinkageResolved {
				result.ageRatingDeclaration = mapAgeRatingDeclaration(ageRatingResource.Attributes)
			}
		}

		result.localizations, localizationsResolved = resolveCompoundToMany[asc.AppInfoLocalizationAttributes](
			selected.Relationships,
			result.included,
			"appInfoLocalizations",
			asc.ResourceTypeAppInfoLocalizations,
		)

		categoryResource, state := resolveCompoundToOne[asc.AppCategoryAttributes](
			selected.Relationships,
			result.included,
			"primaryCategory",
			asc.ResourceTypeAppCategories,
		)
		primaryCategoryState = state
		if state == compoundLinkageResolved {
			result.primaryCategoryID = strings.TrimSpace(categoryResource.ID)
		}
	}

	tasks := make([]readinessTask, 0, 4)
	if appState != compoundLinkageResolved {
		tasks = append(tasks, func(taskCtx context.Context) error {
			appResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppResponse, error) {
				return client.GetApp(requestCtx, appID)
			})
			if err != nil {
				return fmt.Errorf("failed to fetch app: %w", err)
			}
			result.app = appResponse.Data
			return nil
		})
	}
	if !localizationsResolved {
		tasks = append(tasks, func(taskCtx context.Context) error {
			localizationsResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppInfoLocalizationsResponse, error) {
				return client.GetAppInfoLocalizations(requestCtx, appInfoID, asc.WithAppInfoLocalizationsLimit(200))
			})
			if err != nil {
				return fmt.Errorf("failed to fetch app info localizations: %w", err)
			}
			result.localizations = localizationsResponse.Data
			return nil
		})
	}
	if primaryCategoryState == compoundLinkageFallback {
		tasks = append(tasks, func(taskCtx context.Context) error {
			categoryResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AppInfoPrimaryCategoryLinkageResponse, error) {
				return client.GetAppInfoPrimaryCategoryRelationship(requestCtx, appInfoID)
			})
			if err != nil {
				if asc.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch app primary category: %w", err)
			}
			result.primaryCategoryID = strings.TrimSpace(categoryResponse.Data.ID)
			return nil
		})
	}
	if resolveAgeRatingNow && ageRatingState == compoundLinkageFallback {
		tasks = append(tasks, func(taskCtx context.Context) error {
			ageRatingResponse, err := doReadinessRequest(taskCtx, func(requestCtx context.Context) (*asc.AgeRatingDeclarationResponse, error) {
				return client.GetAgeRatingDeclarationForAppInfo(requestCtx, appInfoID)
			})
			if err != nil {
				if asc.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch age rating declaration: %w", err)
			}
			result.ageRatingDeclaration = mapAgeRatingDeclaration(ageRatingResponse.Data.Attributes)
			return nil
		})
	}

	if err := runReadinessTasks(ctx, tasks...); err != nil {
		return appInfoReadinessData{}, err
	}
	return result, nil
}

func resolveMultipleAppInfoAgeRating(ctx context.Context, client *asc.Client, result *appInfoReadinessData, appID, versionState string) error {
	if result == nil || result.response == nil || len(result.response.Data) <= 1 {
		return nil
	}

	candidates := asc.AppInfoCandidates(result.response.Data)
	ageRatingAppInfoID, ok := asc.AutoResolveAppInfoIDByVersionState(candidates, versionState)
	if !ok && versionUsesNextAppInfo(versionState) {
		// A version that has not shipped uses the app's one non-live app
		// info, whatever that info's state is: REJECTED after another
		// platform's rejection, or IN_REVIEW while another platform is in
		// review.
		ageRatingAppInfoID, ok = soleNonLiveAppInfoID(candidates)
	}
	if !ok {
		return fmt.Errorf("failed to fetch age rating declaration: %w", &shared.AmbiguousSelectionError{
			Kind:        "app info",
			Description: fmt.Sprintf("app %q", strings.TrimSpace(appID)),
			Candidates:  shared.AppInfoAmbiguousCandidates(candidates),
			Hint:        fmt.Sprintf("Inspect them with `asc apps info list --app %q`, then read the declaration with `asc age-rating view --app-info-id <ID>`.", strings.TrimSpace(appID)),
		})
	}

	resource, unique := selectAppInfoResource(result.response.Data, ageRatingAppInfoID)
	if unique {
		ageRatingResource, state := resolveCompoundToOne[asc.AgeRatingDeclarationAttributes](
			resource.Relationships,
			result.included,
			"ageRatingDeclaration",
			asc.ResourceTypeAgeRatingDeclarations,
		)
		switch state {
		case compoundLinkageEmpty:
			result.ageRatingDeclaration = nil
			return nil
		case compoundLinkageResolved:
			result.ageRatingDeclaration = mapAgeRatingDeclaration(ageRatingResource.Attributes)
			return nil
		}
	}

	ageRatingResponse, err := doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AgeRatingDeclarationResponse, error) {
		return client.GetAgeRatingDeclarationForAppInfo(requestCtx, ageRatingAppInfoID)
	})
	if err != nil {
		if asc.IsNotFound(err) {
			result.ageRatingDeclaration = nil
			return nil
		}
		return fmt.Errorf("failed to fetch age rating declaration: %w", err)
	}
	result.ageRatingDeclaration = mapAgeRatingDeclaration(ageRatingResponse.Data.Attributes)
	return nil
}

// versionUsesNextAppInfo reports whether a version is still on its way to
// release, so its metadata lives on the app's non-live app info.
func versionUsesNextAppInfo(versionState string) bool {
	if shared.IsLiveAppStoreVersionState(versionState) {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(versionState)) {
	case "", "PREORDER_READY_FOR_SALE", "REPLACED_WITH_NEW_VERSION", "REMOVED_FROM_SALE":
		return false
	default:
		return true
	}
}

// soleNonLiveAppInfoID returns the only current app info that is not live.
// App infos use the same live state spellings as versions.
func soleNonLiveAppInfoID(candidates []asc.AppInfoCandidate) (string, bool) {
	selected := ""
	for _, candidate := range asc.CurrentAppInfoCandidates(candidates) {
		if shared.IsLiveAppStoreVersionState(candidate.State) {
			continue
		}
		if candidate.ID == "" || selected != "" {
			return "", false
		}
		selected = candidate.ID
	}
	return selected, selected != ""
}

func selectAppInfoResource(resources []asc.Resource[asc.AppInfoAttributes], appInfoID string) (asc.Resource[asc.AppInfoAttributes], bool) {
	var selected asc.Resource[asc.AppInfoAttributes]
	matches := 0
	for _, resource := range resources {
		if strings.TrimSpace(resource.ID) != strings.TrimSpace(appInfoID) {
			continue
		}
		selected = resource
		matches++
	}
	return selected, matches == 1
}
