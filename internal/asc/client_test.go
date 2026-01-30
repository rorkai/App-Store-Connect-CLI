package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildReviewQuery(t *testing.T) {
	query := buildReviewQuery([]ReviewOption{
		WithRating(5),
		WithTerritory("us"),
		WithLimit(25),
		WithReviewSort("-createdDate"),
	})

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[rating]"); got != "5" {
		t.Fatalf("expected filter[rating]=5, got %q", got)
	}

	if got := values.Get("filter[territory]"); got != "US" {
		t.Fatalf("expected filter[territory]=US, got %q", got)
	}

	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}

	if got := values.Get("sort"); got != "-createdDate" {
		t.Fatalf("expected sort=-createdDate, got %q", got)
	}
}

func TestBuildReviewQuery_InvalidRating(t *testing.T) {
	query := buildReviewQuery([]ReviewOption{
		WithRating(9),
	})

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[rating]"); got != "" {
		t.Fatalf("expected empty filter[rating], got %q", got)
	}
}

func TestBuildFeedbackQuery(t *testing.T) {
	query := &feedbackQuery{}
	opts := []FeedbackOption{
		WithFeedbackDeviceModels([]string{"iPhone15,3", " iPhone15,2 "}),
		WithFeedbackOSVersions([]string{"17.2", ""}),
		WithFeedbackAppPlatforms([]string{"ios", "mac_os"}),
		WithFeedbackDevicePlatforms([]string{"tv_os"}),
		WithFeedbackBuildIDs([]string{"build-1"}),
		WithFeedbackBuildPreReleaseVersionIDs([]string{"pre-1"}),
		WithFeedbackTesterIDs([]string{"tester-1"}),
		WithFeedbackLimit(10),
		WithFeedbackSort("-createdDate"),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildFeedbackQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[deviceModel]"); got != "iPhone15,3,iPhone15,2" {
		t.Fatalf("expected filter[deviceModel] to be CSV, got %q", got)
	}
	if got := values.Get("filter[osVersion]"); got != "17.2" {
		t.Fatalf("expected filter[osVersion]=17.2, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
	if got := values.Get("filter[appPlatform]"); got != "IOS,MAC_OS" {
		t.Fatalf("expected filter[appPlatform]=IOS,MAC_OS, got %q", got)
	}
	if got := values.Get("filter[devicePlatform]"); got != "TV_OS" {
		t.Fatalf("expected filter[devicePlatform]=TV_OS, got %q", got)
	}
	if got := values.Get("filter[build]"); got != "build-1" {
		t.Fatalf("expected filter[build]=build-1, got %q", got)
	}
	if got := values.Get("filter[build.preReleaseVersion]"); got != "pre-1" {
		t.Fatalf("expected filter[build.preReleaseVersion]=pre-1, got %q", got)
	}
	if got := values.Get("filter[tester]"); got != "tester-1" {
		t.Fatalf("expected filter[tester]=tester-1, got %q", got)
	}
	if got := values.Get("sort"); got != "-createdDate" {
		t.Fatalf("expected sort=-createdDate, got %q", got)
	}
}

func TestBuildFeedbackQuery_IncludesScreenshots(t *testing.T) {
	query := &feedbackQuery{}
	WithFeedbackIncludeScreenshots()(query)

	values, err := url.ParseQuery(buildFeedbackQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	expected := "createdDate,comment,email,deviceModel,osVersion,appPlatform,devicePlatform,screenshots"
	if got := values.Get("fields[betaFeedbackScreenshotSubmissions]"); got != expected {
		t.Fatalf("expected fields to be %q, got %q", expected, got)
	}
}

func TestBuildCrashQuery(t *testing.T) {
	query := &crashQuery{}
	opts := []CrashOption{
		WithCrashDeviceModels([]string{"iPhone16,1"}),
		WithCrashOSVersions([]string{"18.0"}),
		WithCrashAppPlatforms([]string{"ios"}),
		WithCrashDevicePlatforms([]string{"mac_os"}),
		WithCrashBuildIDs([]string{"build-2"}),
		WithCrashBuildPreReleaseVersionIDs([]string{"pre-2"}),
		WithCrashTesterIDs([]string{"tester-2"}),
		WithCrashLimit(5),
		WithCrashSort("createdDate"),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildCrashQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[deviceModel]"); got != "iPhone16,1" {
		t.Fatalf("expected filter[deviceModel]=iPhone16,1, got %q", got)
	}
	if got := values.Get("filter[osVersion]"); got != "18.0" {
		t.Fatalf("expected filter[osVersion]=18.0, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
	if got := values.Get("filter[appPlatform]"); got != "IOS" {
		t.Fatalf("expected filter[appPlatform]=IOS, got %q", got)
	}
	if got := values.Get("filter[devicePlatform]"); got != "MAC_OS" {
		t.Fatalf("expected filter[devicePlatform]=MAC_OS, got %q", got)
	}
	if got := values.Get("filter[build]"); got != "build-2" {
		t.Fatalf("expected filter[build]=build-2, got %q", got)
	}
	if got := values.Get("filter[build.preReleaseVersion]"); got != "pre-2" {
		t.Fatalf("expected filter[build.preReleaseVersion]=pre-2, got %q", got)
	}
	if got := values.Get("filter[tester]"); got != "tester-2" {
		t.Fatalf("expected filter[tester]=tester-2, got %q", got)
	}
	if got := values.Get("sort"); got != "createdDate" {
		t.Fatalf("expected sort=createdDate, got %q", got)
	}
}

func TestBuildBetaGroupsQuery(t *testing.T) {
	query := &betaGroupsQuery{}
	WithBetaGroupsLimit(10)(query)

	values, err := url.ParseQuery(buildBetaGroupsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildAppTagsQuery(t *testing.T) {
	query := &appTagsQuery{}
	opts := []AppTagsOption{
		WithAppTagsLimit(25),
		WithAppTagsVisibleInAppStore([]string{"true", " false "}),
		WithAppTagsSort("-name"),
		WithAppTagsFields([]string{"name", "visibleInAppStore"}),
		WithAppTagsInclude([]string{"territories"}),
		WithAppTagsTerritoryFields([]string{"currency"}),
		WithAppTagsTerritoryLimit(50),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppTagsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[visibleInAppStore]"); got != "true,false" {
		t.Fatalf("expected filter[visibleInAppStore]=true,false, got %q", got)
	}
	if got := values.Get("sort"); got != "-name" {
		t.Fatalf("expected sort=-name, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
	if got := values.Get("fields[appTags]"); got != "name,visibleInAppStore" {
		t.Fatalf("expected fields[appTags]=name,visibleInAppStore, got %q", got)
	}
	if got := values.Get("include"); got != "territories" {
		t.Fatalf("expected include=territories, got %q", got)
	}
	if got := values.Get("fields[territories]"); got != "currency" {
		t.Fatalf("expected fields[territories]=currency, got %q", got)
	}
	if got := values.Get("limit[territories]"); got != "50" {
		t.Fatalf("expected limit[territories]=50, got %q", got)
	}
}

func TestBuildNominationsQuery(t *testing.T) {
	query := &nominationsQuery{}
	opts := []NominationsOption{
		WithNominationsLimit(50),
		WithNominationsTypes([]string{"app_launch", "NEW_CONTENT"}),
		WithNominationsStates([]string{"draft", "submitted"}),
		WithNominationsRelatedApps([]string{"app-1", " app-2 "}),
		WithNominationsSort("-publishEndDate"),
		WithNominationsFields([]string{"name", "type"}),
		WithNominationsInclude([]string{"relatedApps", "supportedTerritories"}),
		WithNominationsInAppEventsLimit(25),
		WithNominationsRelatedAppsLimit(10),
		WithNominationsSupportedTerritoriesLimit(200),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildNominationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[type]"); got != "APP_LAUNCH,NEW_CONTENT" {
		t.Fatalf("expected filter[type]=APP_LAUNCH,NEW_CONTENT, got %q", got)
	}
	if got := values.Get("filter[state]"); got != "DRAFT,SUBMITTED" {
		t.Fatalf("expected filter[state]=DRAFT,SUBMITTED, got %q", got)
	}
	if got := values.Get("filter[relatedApps]"); got != "app-1,app-2" {
		t.Fatalf("expected filter[relatedApps]=app-1,app-2, got %q", got)
	}
	if got := values.Get("sort"); got != "-publishEndDate" {
		t.Fatalf("expected sort=-publishEndDate, got %q", got)
	}
	if got := values.Get("fields[nominations]"); got != "name,type" {
		t.Fatalf("expected fields[nominations]=name,type, got %q", got)
	}
	if got := values.Get("include"); got != "relatedApps,supportedTerritories" {
		t.Fatalf("expected include=relatedApps,supportedTerritories, got %q", got)
	}
	if got := values.Get("limit"); got != "50" {
		t.Fatalf("expected limit=50, got %q", got)
	}
	if got := values.Get("limit[inAppEvents]"); got != "25" {
		t.Fatalf("expected limit[inAppEvents]=25, got %q", got)
	}
	if got := values.Get("limit[relatedApps]"); got != "10" {
		t.Fatalf("expected limit[relatedApps]=10, got %q", got)
	}
	if got := values.Get("limit[supportedTerritories]"); got != "200" {
		t.Fatalf("expected limit[supportedTerritories]=200, got %q", got)
	}
}

func TestNominationCreateRequest_JSON(t *testing.T) {
	publishEnd := "2026-02-15T08:00:00Z"
	hasInAppEvents := true
	launchInSelectMarketsFirst := false
	notes := "Launch notes"
	preOrderEnabled := true

	req := NominationCreateRequest{
		Data: NominationCreateData{
			Type: ResourceTypeNominations,
			Attributes: NominationCreateAttributes{
				Name:                       "Spring Launch",
				Type:                       NominationTypeAppLaunch,
				Description:                "Major launch",
				Submitted:                  true,
				PublishStartDate:           "2026-02-01T08:00:00Z",
				PublishEndDate:             &publishEnd,
				DeviceFamilies:             []DeviceFamily{DeviceFamilyIPhone, DeviceFamilyIPad},
				Locales:                    []string{"en-US", "fr-FR"},
				SupplementalMaterialsURIs:  []string{"https://example.com/presskit"},
				HasInAppEvents:             &hasInAppEvents,
				LaunchInSelectMarketsFirst: &launchInSelectMarketsFirst,
				Notes:                      &notes,
				PreOrderEnabled:            &preOrderEnabled,
			},
			Relationships: NominationRelationships{
				RelatedApps: &RelationshipList{Data: []ResourceData{
					{Type: ResourceTypeApps, ID: "APP_ID_1"},
				}},
				InAppEvents: &RelationshipList{Data: []ResourceData{
					{Type: ResourceTypeAppEvents, ID: "EVENT_ID_1"},
				}},
				SupportedTerritories: &RelationshipList{Data: []ResourceData{
					{Type: ResourceTypeTerritories, ID: "US"},
				}},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				Name                       string   `json:"name"`
				Type                       string   `json:"type"`
				Description                string   `json:"description"`
				Submitted                  bool     `json:"submitted"`
				PublishStartDate           string   `json:"publishStartDate"`
				PublishEndDate             *string  `json:"publishEndDate"`
				DeviceFamilies             []string `json:"deviceFamilies"`
				Locales                    []string `json:"locales"`
				SupplementalMaterialsURIs  []string `json:"supplementalMaterialsUris"`
				HasInAppEvents             *bool    `json:"hasInAppEvents"`
				LaunchInSelectMarketsFirst *bool    `json:"launchInSelectMarketsFirst"`
				Notes                      *string  `json:"notes"`
				PreOrderEnabled            *bool    `json:"preOrderEnabled"`
			} `json:"attributes"`
			Relationships struct {
				RelatedApps struct {
					Data []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"relatedApps"`
				InAppEvents struct {
					Data []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"inAppEvents"`
				SupportedTerritories struct {
					Data []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"supportedTerritories"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "nominations" {
		t.Fatalf("expected type=nominations, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.Name != "Spring Launch" {
		t.Fatalf("expected name=Spring Launch, got %q", parsed.Data.Attributes.Name)
	}
	if parsed.Data.Attributes.Type != "APP_LAUNCH" {
		t.Fatalf("expected type=APP_LAUNCH, got %q", parsed.Data.Attributes.Type)
	}
	if parsed.Data.Attributes.Description != "Major launch" {
		t.Fatalf("expected description=Major launch, got %q", parsed.Data.Attributes.Description)
	}
	if !parsed.Data.Attributes.Submitted {
		t.Fatalf("expected submitted=true, got false")
	}
	if parsed.Data.Attributes.PublishStartDate != "2026-02-01T08:00:00Z" {
		t.Fatalf("expected publishStartDate, got %q", parsed.Data.Attributes.PublishStartDate)
	}
	if parsed.Data.Attributes.PublishEndDate == nil || *parsed.Data.Attributes.PublishEndDate != publishEnd {
		t.Fatalf("expected publishEndDate=%q, got %v", publishEnd, parsed.Data.Attributes.PublishEndDate)
	}
	if len(parsed.Data.Attributes.DeviceFamilies) != 2 {
		t.Fatalf("expected 2 device families, got %v", parsed.Data.Attributes.DeviceFamilies)
	}
	if len(parsed.Data.Attributes.Locales) != 2 {
		t.Fatalf("expected 2 locales, got %v", parsed.Data.Attributes.Locales)
	}
	if len(parsed.Data.Attributes.SupplementalMaterialsURIs) != 1 || parsed.Data.Attributes.SupplementalMaterialsURIs[0] != "https://example.com/presskit" {
		t.Fatalf("expected supplementalMaterialsUris to include presskit, got %v", parsed.Data.Attributes.SupplementalMaterialsURIs)
	}
	if parsed.Data.Attributes.HasInAppEvents == nil || !*parsed.Data.Attributes.HasInAppEvents {
		t.Fatalf("expected hasInAppEvents=true, got %v", parsed.Data.Attributes.HasInAppEvents)
	}
	if parsed.Data.Attributes.LaunchInSelectMarketsFirst == nil || *parsed.Data.Attributes.LaunchInSelectMarketsFirst {
		t.Fatalf("expected launchInSelectMarketsFirst=false, got %v", parsed.Data.Attributes.LaunchInSelectMarketsFirst)
	}
	if parsed.Data.Attributes.Notes == nil || *parsed.Data.Attributes.Notes != notes {
		t.Fatalf("expected notes=%q, got %v", notes, parsed.Data.Attributes.Notes)
	}
	if parsed.Data.Attributes.PreOrderEnabled == nil || !*parsed.Data.Attributes.PreOrderEnabled {
		t.Fatalf("expected preOrderEnabled=true, got %v", parsed.Data.Attributes.PreOrderEnabled)
	}
	if len(parsed.Data.Relationships.RelatedApps.Data) != 1 || parsed.Data.Relationships.RelatedApps.Data[0].Type != "apps" {
		t.Fatalf("expected relatedApps relationship, got %v", parsed.Data.Relationships.RelatedApps.Data)
	}
	if parsed.Data.Relationships.RelatedApps.Data[0].ID != "APP_ID_1" {
		t.Fatalf("expected relatedApps id=APP_ID_1, got %q", parsed.Data.Relationships.RelatedApps.Data[0].ID)
	}
	if len(parsed.Data.Relationships.InAppEvents.Data) != 1 || parsed.Data.Relationships.InAppEvents.Data[0].Type != "appEvents" {
		t.Fatalf("expected inAppEvents relationship, got %v", parsed.Data.Relationships.InAppEvents.Data)
	}
	if parsed.Data.Relationships.InAppEvents.Data[0].ID != "EVENT_ID_1" {
		t.Fatalf("expected inAppEvents id=EVENT_ID_1, got %q", parsed.Data.Relationships.InAppEvents.Data[0].ID)
	}
	if len(parsed.Data.Relationships.SupportedTerritories.Data) != 1 || parsed.Data.Relationships.SupportedTerritories.Data[0].Type != "territories" {
		t.Fatalf("expected supportedTerritories relationship, got %v", parsed.Data.Relationships.SupportedTerritories.Data)
	}
	if parsed.Data.Relationships.SupportedTerritories.Data[0].ID != "US" {
		t.Fatalf("expected supportedTerritories id=US, got %q", parsed.Data.Relationships.SupportedTerritories.Data[0].ID)
	}
}

func TestNominationUpdateRequest_JSON(t *testing.T) {
	submitted := true
	archived := false
	hasInAppEvents := false
	publishStart := "2026-02-05T08:00:00Z"
	name := "Updated Launch"
	description := "Updated description"
	notes := "Updated notes"
	nomType := NominationTypeAppEnhancements

	attrs := NominationUpdateAttributes{
		Name:                      &name,
		Type:                      &nomType,
		Description:               &description,
		Submitted:                 &submitted,
		Archived:                  &archived,
		PublishStartDate:          &publishStart,
		DeviceFamilies:            []DeviceFamily{DeviceFamilyMac},
		Locales:                   []string{"en-US"},
		SupplementalMaterialsURIs: []string{"https://example.com/update"},
		HasInAppEvents:            &hasInAppEvents,
		Notes:                     &notes,
	}

	req := NominationUpdateRequest{
		Data: NominationUpdateData{
			Type:          ResourceTypeNominations,
			ID:            "NOMINATION_ID",
			Attributes:    &attrs,
			Relationships: &NominationRelationships{},
		},
	}
	req.Data.Relationships.RelatedApps = &RelationshipList{Data: []ResourceData{
		{Type: ResourceTypeApps, ID: "APP_ID_2"},
	}}
	req.Data.Relationships.SupportedTerritories = &RelationshipList{Data: []ResourceData{
		{Type: ResourceTypeTerritories, ID: "CA"},
	}}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Name                      *string  `json:"name"`
				Type                      *string  `json:"type"`
				Description               *string  `json:"description"`
				Submitted                 *bool    `json:"submitted"`
				Archived                  *bool    `json:"archived"`
				PublishStartDate          *string  `json:"publishStartDate"`
				DeviceFamilies            []string `json:"deviceFamilies"`
				Locales                   []string `json:"locales"`
				SupplementalMaterialsURIs []string `json:"supplementalMaterialsUris"`
				HasInAppEvents            *bool    `json:"hasInAppEvents"`
				Notes                     *string  `json:"notes"`
			} `json:"attributes"`
			Relationships struct {
				RelatedApps struct {
					Data []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"relatedApps"`
				SupportedTerritories struct {
					Data []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"supportedTerritories"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "nominations" {
		t.Fatalf("expected type=nominations, got %q", parsed.Data.Type)
	}
	if parsed.Data.ID != "NOMINATION_ID" {
		t.Fatalf("expected id=NOMINATION_ID, got %q", parsed.Data.ID)
	}
	if parsed.Data.Attributes.Name == nil || *parsed.Data.Attributes.Name != name {
		t.Fatalf("expected name=%q, got %v", name, parsed.Data.Attributes.Name)
	}
	if parsed.Data.Attributes.Type == nil || *parsed.Data.Attributes.Type != "APP_ENHANCEMENTS" {
		t.Fatalf("expected type=APP_ENHANCEMENTS, got %v", parsed.Data.Attributes.Type)
	}
	if parsed.Data.Attributes.Description == nil || *parsed.Data.Attributes.Description != description {
		t.Fatalf("expected description=%q, got %v", description, parsed.Data.Attributes.Description)
	}
	if parsed.Data.Attributes.Submitted == nil || !*parsed.Data.Attributes.Submitted {
		t.Fatalf("expected submitted=true, got %v", parsed.Data.Attributes.Submitted)
	}
	if parsed.Data.Attributes.Archived == nil || *parsed.Data.Attributes.Archived {
		t.Fatalf("expected archived=false, got %v", parsed.Data.Attributes.Archived)
	}
	if parsed.Data.Attributes.PublishStartDate == nil || *parsed.Data.Attributes.PublishStartDate != publishStart {
		t.Fatalf("expected publishStartDate=%q, got %v", publishStart, parsed.Data.Attributes.PublishStartDate)
	}
	if len(parsed.Data.Attributes.DeviceFamilies) != 1 || parsed.Data.Attributes.DeviceFamilies[0] != "MAC" {
		t.Fatalf("expected deviceFamilies=[MAC], got %v", parsed.Data.Attributes.DeviceFamilies)
	}
	if len(parsed.Data.Attributes.Locales) != 1 || parsed.Data.Attributes.Locales[0] != "en-US" {
		t.Fatalf("expected locales=[en-US], got %v", parsed.Data.Attributes.Locales)
	}
	if len(parsed.Data.Attributes.SupplementalMaterialsURIs) != 1 || parsed.Data.Attributes.SupplementalMaterialsURIs[0] != "https://example.com/update" {
		t.Fatalf("expected supplementalMaterialsUris to include update, got %v", parsed.Data.Attributes.SupplementalMaterialsURIs)
	}
	if parsed.Data.Attributes.HasInAppEvents == nil || *parsed.Data.Attributes.HasInAppEvents {
		t.Fatalf("expected hasInAppEvents=false, got %v", parsed.Data.Attributes.HasInAppEvents)
	}
	if parsed.Data.Attributes.Notes == nil || *parsed.Data.Attributes.Notes != notes {
		t.Fatalf("expected notes=%q, got %v", notes, parsed.Data.Attributes.Notes)
	}
	if len(parsed.Data.Relationships.RelatedApps.Data) != 1 || parsed.Data.Relationships.RelatedApps.Data[0].Type != "apps" {
		t.Fatalf("expected relatedApps relationship, got %v", parsed.Data.Relationships.RelatedApps.Data)
	}
	if parsed.Data.Relationships.RelatedApps.Data[0].ID != "APP_ID_2" {
		t.Fatalf("expected relatedApps id=APP_ID_2, got %q", parsed.Data.Relationships.RelatedApps.Data[0].ID)
	}
	if len(parsed.Data.Relationships.SupportedTerritories.Data) != 1 || parsed.Data.Relationships.SupportedTerritories.Data[0].Type != "territories" {
		t.Fatalf("expected supportedTerritories relationship, got %v", parsed.Data.Relationships.SupportedTerritories.Data)
	}
	if parsed.Data.Relationships.SupportedTerritories.Data[0].ID != "CA" {
		t.Fatalf("expected supportedTerritories id=CA, got %q", parsed.Data.Relationships.SupportedTerritories.Data[0].ID)
	}
}

func TestBuildAccessibilityDeclarationsQuery(t *testing.T) {
	query := &accessibilityDeclarationsQuery{}
	opts := []AccessibilityDeclarationsOption{
		WithAccessibilityDeclarationsDeviceFamilies([]string{"iphone", " ipad "}),
		WithAccessibilityDeclarationsStates([]string{"draft", "published"}),
		WithAccessibilityDeclarationsFields([]string{"deviceFamily", "state"}),
		WithAccessibilityDeclarationsLimit(5),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAccessibilityDeclarationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[deviceFamily]"); got != "IPHONE,IPAD" {
		t.Fatalf("expected filter[deviceFamily]=IPHONE,IPAD, got %q", got)
	}
	if got := values.Get("filter[state]"); got != "DRAFT,PUBLISHED" {
		t.Fatalf("expected filter[state]=DRAFT,PUBLISHED, got %q", got)
	}
	if got := values.Get("fields[accessibilityDeclarations]"); got != "deviceFamily,state" {
		t.Fatalf("expected fields[accessibilityDeclarations]=deviceFamily,state, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
}

func TestBuildAppStoreReviewAttachmentsQuery(t *testing.T) {
	query := &appStoreReviewAttachmentsQuery{}
	opts := []AppStoreReviewAttachmentsOption{
		WithAppStoreReviewAttachmentsFields([]string{"fileName", "fileSize"}),
		WithAppStoreReviewAttachmentReviewDetailFields([]string{"contactEmail", "notes"}),
		WithAppStoreReviewAttachmentsInclude([]string{"appStoreReviewDetail"}),
		WithAppStoreReviewAttachmentsLimit(10),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppStoreReviewAttachmentsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("fields[appStoreReviewAttachments]"); got != "fileName,fileSize" {
		t.Fatalf("expected fields[appStoreReviewAttachments]=fileName,fileSize, got %q", got)
	}
	if got := values.Get("fields[appStoreReviewDetails]"); got != "contactEmail,notes" {
		t.Fatalf("expected fields[appStoreReviewDetails]=contactEmail,notes, got %q", got)
	}
	if got := values.Get("include"); got != "appStoreReviewDetail" {
		t.Fatalf("expected include=appStoreReviewDetail, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildAppEncryptionDeclarationsQuery(t *testing.T) {
	query := &appEncryptionDeclarationsQuery{}
	opts := []AppEncryptionDeclarationsOption{
		WithAppEncryptionDeclarationsBuildIDs([]string{"build-1", " build-2 "}),
		WithAppEncryptionDeclarationsFields([]string{"appDescription", "exempt"}),
		WithAppEncryptionDeclarationsDocumentFields([]string{"fileName", "fileSize"}),
		WithAppEncryptionDeclarationsInclude([]string{"app", "builds"}),
		WithAppEncryptionDeclarationsLimit(5),
		WithAppEncryptionDeclarationsBuildLimit(10),
	}
	for _, opt := range opts {
		opt(query)
	}
	query.appID = "app-1"

	values, err := url.ParseQuery(buildAppEncryptionDeclarationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[app]"); got != "app-1" {
		t.Fatalf("expected filter[app]=app-1, got %q", got)
	}
	if got := values.Get("filter[builds]"); got != "build-1,build-2" {
		t.Fatalf("expected filter[builds]=build-1,build-2, got %q", got)
	}
	if got := values.Get("fields[appEncryptionDeclarations]"); got != "appDescription,exempt" {
		t.Fatalf("expected fields[appEncryptionDeclarations]=appDescription,exempt, got %q", got)
	}
	if got := values.Get("fields[appEncryptionDeclarationDocuments]"); got != "fileName,fileSize" {
		t.Fatalf("expected fields[appEncryptionDeclarationDocuments]=fileName,fileSize, got %q", got)
	}
	if got := values.Get("include"); got != "app,builds" {
		t.Fatalf("expected include=app,builds, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
	if got := values.Get("limit[builds]"); got != "10" {
		t.Fatalf("expected limit[builds]=10, got %q", got)
	}
}

func TestBuildBetaTestersQuery(t *testing.T) {
	query := &betaTestersQuery{}
	opts := []BetaTestersOption{
		WithBetaTestersLimit(25),
		WithBetaTestersEmail("tester@example.com"),
		WithBetaTestersGroupIDs([]string{"group-1", " group-2 "}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBetaTestersQuery("APP_ID", query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[apps]"); got != "APP_ID" {
		t.Fatalf("expected filter[apps]=APP_ID, got %q", got)
	}
	if got := values.Get("filter[email]"); got != "tester@example.com" {
		t.Fatalf("expected filter[email]=tester@example.com, got %q", got)
	}
	if got := values.Get("filter[betaGroups]"); got != "group-1,group-2" {
		t.Fatalf("expected filter[betaGroups]=group-1,group-2, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}

func TestBuildAppStoreVersionsQuery(t *testing.T) {
	query := &appStoreVersionsQuery{}
	opts := []AppStoreVersionsOption{
		WithAppStoreVersionsLimit(20),
		WithAppStoreVersionsPlatforms([]string{"ios", "MAC_OS"}),
		WithAppStoreVersionsVersionStrings([]string{"1.0.0", "1.1.0"}),
		WithAppStoreVersionsStates([]string{"ready_for_review"}),
		WithAppStoreVersionsInclude([]string{"appStoreReviewDetail"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppStoreVersionsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[platform]"); got != "IOS,MAC_OS" {
		t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", got)
	}
	if got := values.Get("filter[versionString]"); got != "1.0.0,1.1.0" {
		t.Fatalf("expected filter[versionString]=1.0.0,1.1.0, got %q", got)
	}
	if got := values.Get("filter[appStoreState]"); got != "READY_FOR_REVIEW" {
		t.Fatalf("expected filter[appStoreState]=READY_FOR_REVIEW, got %q", got)
	}
	if got := values.Get("include"); got != "appStoreReviewDetail" {
		t.Fatalf("expected include=appStoreReviewDetail, got %q", got)
	}
	if got := values.Get("limit"); got != "20" {
		t.Fatalf("expected limit=20, got %q", got)
	}
}

func TestBuildAppSearchKeywordsQuery(t *testing.T) {
	query := &appSearchKeywordsQuery{}
	opts := []AppSearchKeywordsOption{
		WithAppSearchKeywordsLimit(15),
		WithAppSearchKeywordsPlatforms([]string{"ios", "MAC_OS"}),
		WithAppSearchKeywordsLocales([]string{"en-US", "ja"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppSearchKeywordsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[platform]"); got != "IOS,MAC_OS" {
		t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", got)
	}
	if got := values.Get("filter[locale]"); got != "en-US,ja" {
		t.Fatalf("expected filter[locale]=en-US,ja, got %q", got)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected limit=15, got %q", got)
	}
}

func TestBuildAppStoreVersionQuery(t *testing.T) {
	query := &appStoreVersionQuery{}
	WithAppStoreVersionInclude([]string{"appStoreReviewDetail", "ageRatingDeclaration"})(query)

	values, err := url.ParseQuery(buildAppStoreVersionQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("include"); got != "appStoreReviewDetail,ageRatingDeclaration" {
		t.Fatalf("expected include=appStoreReviewDetail,ageRatingDeclaration, got %q", got)
	}
}

func TestBuildAppInfoQuery(t *testing.T) {
	query := &appInfoQuery{}
	WithAppInfoInclude([]string{"ageRatingDeclaration", "territoryAgeRatings"})(query)

	values, err := url.ParseQuery(buildAppInfoQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("include"); got != "ageRatingDeclaration,territoryAgeRatings" {
		t.Fatalf("expected include=ageRatingDeclaration,territoryAgeRatings, got %q", got)
	}
}

func TestBuildPreReleaseVersionsQuery(t *testing.T) {
	query := &preReleaseVersionsQuery{}
	opts := []PreReleaseVersionsOption{
		WithPreReleaseVersionsLimit(15),
		WithPreReleaseVersionsPlatform(" ios, MAC_OS "),
		WithPreReleaseVersionsVersion("1.0.0, 1.1.0"),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildPreReleaseVersionsQuery("APP_ID", query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[app]"); got != "APP_ID" {
		t.Fatalf("expected filter[app]=APP_ID, got %q", got)
	}
	if got := values.Get("filter[platform]"); got != "IOS,MAC_OS" {
		t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", got)
	}
	if got := values.Get("filter[version]"); got != "1.0.0,1.1.0" {
		t.Fatalf("expected filter[version]=1.0.0,1.1.0, got %q", got)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected limit=15, got %q", got)
	}
}

func TestBuildAppStoreVersionLocalizationsQuery(t *testing.T) {
	query := &appStoreVersionLocalizationsQuery{}
	opts := []AppStoreVersionLocalizationsOption{
		WithAppStoreVersionLocalizationsLimit(10),
		WithAppStoreVersionLocalizationLocales([]string{"en-US", "ja"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppStoreVersionLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[locale]"); got != "en-US,ja" {
		t.Fatalf("expected filter[locale]=en-US,ja, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildBetaAppLocalizationsQuery(t *testing.T) {
	query := &betaAppLocalizationsQuery{}
	opts := []BetaAppLocalizationsOption{
		WithBetaAppLocalizationsLimit(12),
		WithBetaAppLocalizationLocales([]string{"en-US", "fr-FR"}),
		WithBetaAppLocalizationAppIDs([]string{"app-1", "app-2"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBetaAppLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[locale]"); got != "en-US,fr-FR" {
		t.Fatalf("expected filter[locale]=en-US,fr-FR, got %q", got)
	}
	if got := values.Get("filter[app]"); got != "app-1,app-2" {
		t.Fatalf("expected filter[app]=app-1,app-2, got %q", got)
	}
	if got := values.Get("limit"); got != "12" {
		t.Fatalf("expected limit=12, got %q", got)
	}
}

func TestBuildBetaBuildLocalizationsQuery(t *testing.T) {
	query := &betaBuildLocalizationsQuery{}
	opts := []BetaBuildLocalizationsOption{
		WithBetaBuildLocalizationsLimit(25),
		WithBetaBuildLocalizationLocales([]string{"en-US", "fr-FR"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBetaBuildLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[locale]"); got != "en-US,fr-FR" {
		t.Fatalf("expected filter[locale]=en-US,fr-FR, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}

func TestBuildBuildUploadsQuery(t *testing.T) {
	query := &buildUploadsQuery{}
	opts := []BuildUploadsOption{
		WithBuildUploadsCFBundleShortVersionStrings([]string{"1.0", "1.1"}),
		WithBuildUploadsCFBundleVersions([]string{"100", "200"}),
		WithBuildUploadsPlatforms([]string{"ios", "MAC_OS"}),
		WithBuildUploadsStates([]string{"processing"}),
		WithBuildUploadsSort("-uploadedDate"),
		WithBuildUploadsLimit(15),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBuildUploadsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[cfBundleShortVersionString]"); got != "1.0,1.1" {
		t.Fatalf("expected filter[cfBundleShortVersionString]=1.0,1.1, got %q", got)
	}
	if got := values.Get("filter[cfBundleVersion]"); got != "100,200" {
		t.Fatalf("expected filter[cfBundleVersion]=100,200, got %q", got)
	}
	if got := values.Get("filter[platform]"); got != "IOS,MAC_OS" {
		t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", got)
	}
	if got := values.Get("filter[state]"); got != "PROCESSING" {
		t.Fatalf("expected filter[state]=PROCESSING, got %q", got)
	}
	if got := values.Get("sort"); got != "-uploadedDate" {
		t.Fatalf("expected sort=-uploadedDate, got %q", got)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected limit=15, got %q", got)
	}
}

func TestBuildBuildUploadFilesQuery(t *testing.T) {
	query := &buildUploadFilesQuery{}
	opts := []BuildUploadFilesOption{
		WithBuildUploadFilesLimit(20),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBuildUploadFilesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "20" {
		t.Fatalf("expected limit=20, got %q", got)
	}
}

func TestBuildBuildIndividualTestersQuery(t *testing.T) {
	query := &buildIndividualTestersQuery{}
	opts := []BuildIndividualTestersOption{
		WithBuildIndividualTestersLimit(30),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBuildIndividualTestersQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "30" {
		t.Fatalf("expected limit=30, got %q", got)
	}
}

func TestBuildBetaBuildUsagesQuery(t *testing.T) {
	query := &betaBuildUsagesQuery{}
	opts := []BetaBuildUsagesOption{
		WithBetaBuildUsagesLimit(40),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBetaBuildUsagesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "40" {
		t.Fatalf("expected limit=40, got %q", got)
	}
}

func TestBuildBetaTesterUsagesQuery(t *testing.T) {
	query := &betaTesterUsagesQuery{}
	opts := []BetaTesterUsagesOption{
		WithBetaTesterUsagesPeriod("P7D"),
		WithBetaTesterUsagesAppID("app-1"),
		WithBetaTesterUsagesLimit(10),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildBetaTesterUsagesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("period"); got != "P7D" {
		t.Fatalf("expected period=P7D, got %q", got)
	}
	if got := values.Get("filter[apps]"); got != "app-1" {
		t.Fatalf("expected filter[apps]=app-1, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildAppInfoLocalizationsQuery(t *testing.T) {
	query := &appInfoLocalizationsQuery{}
	opts := []AppInfoLocalizationsOption{
		WithAppInfoLocalizationsLimit(5),
		WithAppInfoLocalizationLocales([]string{"en-US"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppInfoLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[locale]"); got != "en-US" {
		t.Fatalf("expected filter[locale]=en-US, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
}

func TestBuildRequestBody(t *testing.T) {
	body, err := BuildRequestBody(map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if parsed["hello"] != "world" {
		t.Fatalf("expected hello=world, got %q", parsed["hello"])
	}
}

func TestParseError(t *testing.T) {
	payload := []byte(`{"errors":[{"code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	err := ParseError(payload)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !errors.Is(apiErr, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestBuildAppsQuery(t *testing.T) {
	query := &appsQuery{}
	opts := []AppsOption{
		WithAppsLimit(10),
		WithAppsNextURL("https://api.appstoreconnect.apple.com/v1/apps?cursor=abc123"),
		WithAppsSort("-name"),
		WithAppsBundleIDs([]string{"com.example.app", " ", "com.example.other"}),
		WithAppsNames([]string{"Demo", " "}),
		WithAppsSKUs([]string{"SKU1", "SKU2"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	if query.limit != 10 {
		t.Fatalf("expected limit=10, got %d", query.limit)
	}
	if query.nextURL != "https://api.appstoreconnect.apple.com/v1/apps?cursor=abc123" {
		t.Fatalf("expected nextURL, got %q", query.nextURL)
	}
	if query.sort != "-name" {
		t.Fatalf("expected sort=-name, got %q", query.sort)
	}
	if len(query.bundleIDs) != 2 || query.bundleIDs[0] != "com.example.app" || query.bundleIDs[1] != "com.example.other" {
		t.Fatalf("expected bundleIDs to be normalized, got %v", query.bundleIDs)
	}
	if len(query.names) != 1 || query.names[0] != "Demo" {
		t.Fatalf("expected names to be normalized, got %v", query.names)
	}
	if len(query.skus) != 2 || query.skus[0] != "SKU1" || query.skus[1] != "SKU2" {
		t.Fatalf("expected skus to be set, got %v", query.skus)
	}
}

func TestBuildBuildsQuery(t *testing.T) {
	query := &buildsQuery{}
	opts := []BuildsOption{
		WithBuildsLimit(25),
		WithBuildsNextURL("https://api.appstoreconnect.apple.com/v1/apps/123/builds?cursor=abc"),
		WithBuildsSort("-uploadedDate"),
	}
	for _, opt := range opts {
		opt(query)
	}

	if query.limit != 25 {
		t.Fatalf("expected limit=25, got %d", query.limit)
	}
	if query.nextURL != "https://api.appstoreconnect.apple.com/v1/apps/123/builds?cursor=abc" {
		t.Fatalf("expected nextURL to be set, got %q", query.nextURL)
	}
	if query.sort != "-uploadedDate" {
		t.Fatalf("expected sort=-uploadedDate, got %q", query.sort)
	}
}

func TestBuildSubscriptionOfferCodeOneTimeUseCodesQuery(t *testing.T) {
	query := &subscriptionOfferCodeOneTimeUseCodesQuery{}
	opts := []SubscriptionOfferCodeOneTimeUseCodesOption{
		WithSubscriptionOfferCodeOneTimeUseCodesLimit(10),
		WithSubscriptionOfferCodeOneTimeUseCodesNextURL("https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/123/oneTimeUseCodes?cursor=abc"),
	}
	for _, opt := range opts {
		opt(query)
	}

	if query.limit != 10 {
		t.Fatalf("expected limit=10, got %d", query.limit)
	}
	if query.nextURL != "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/123/oneTimeUseCodes?cursor=abc" {
		t.Fatalf("expected nextURL to be set, got %q", query.nextURL)
	}

	values, err := url.ParseQuery(buildSubscriptionOfferCodeOneTimeUseCodesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildWinBackOffersQuery(t *testing.T) {
	query := &winBackOffersQuery{}
	opts := []WinBackOffersOption{
		WithWinBackOffersLimit(25),
		WithWinBackOffersFields([]string{"referenceName", "offerMode"}),
		WithWinBackOffersPriceFields([]string{"territory"}),
		WithWinBackOffersInclude([]string{"prices"}),
		WithWinBackOffersPricesLimit(10),
	}
	for _, opt := range opts {
		opt(query)
	}

	if query.limit != 25 {
		t.Fatalf("expected limit=25, got %d", query.limit)
	}

	values, err := url.ParseQuery(buildWinBackOffersQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
	if got := values.Get("fields[winBackOffers]"); got != "referenceName,offerMode" {
		t.Fatalf("expected winBackOffers fields, got %q", got)
	}
	if got := values.Get("fields[winBackOfferPrices]"); got != "territory" {
		t.Fatalf("expected winBackOfferPrices fields, got %q", got)
	}
	if got := values.Get("include"); got != "prices" {
		t.Fatalf("expected include=prices, got %q", got)
	}
	if got := values.Get("limit[prices]"); got != "10" {
		t.Fatalf("expected limit[prices]=10, got %q", got)
	}
}

func TestBuildWinBackOfferPricesQuery(t *testing.T) {
	query := &winBackOfferPricesQuery{}
	opts := []WinBackOfferPricesOption{
		WithWinBackOfferPricesLimit(15),
		WithWinBackOfferPricesTerritoryFilter([]string{"USA", "CAN"}),
		WithWinBackOfferPricesFields([]string{"territory"}),
		WithWinBackOfferPricesTerritoryFields([]string{"currency"}),
		WithWinBackOfferPricesSubscriptionPricePointFields([]string{"customerPrice", "proceeds"}),
		WithWinBackOfferPricesInclude([]string{"territory", "subscriptionPricePoint"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	if query.limit != 15 {
		t.Fatalf("expected limit=15, got %d", query.limit)
	}

	values, err := url.ParseQuery(buildWinBackOfferPricesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "15" {
		t.Fatalf("expected limit=15, got %q", got)
	}
	if got := values.Get("filter[territory]"); got != "USA,CAN" {
		t.Fatalf("expected territory filter, got %q", got)
	}
	if got := values.Get("fields[winBackOfferPrices]"); got != "territory" {
		t.Fatalf("expected winBackOfferPrices fields, got %q", got)
	}
	if got := values.Get("fields[territories]"); got != "currency" {
		t.Fatalf("expected territories fields, got %q", got)
	}
	if got := values.Get("fields[subscriptionPricePoints]"); got != "customerPrice,proceeds" {
		t.Fatalf("expected subscriptionPricePoints fields, got %q", got)
	}
	if got := values.Get("include"); got != "territory,subscriptionPricePoint" {
		t.Fatalf("expected include list, got %q", got)
	}
}

func TestBuildMerchantIDsQuery(t *testing.T) {
	query := &merchantIDsQuery{}
	opts := []MerchantIDsOption{
		WithMerchantIDsFilterName("Example"),
		WithMerchantIDsFilterIdentifier("merchant.com.example"),
		WithMerchantIDsSort("-identifier"),
		WithMerchantIDsFields([]string{"name", "identifier"}),
		WithMerchantIDsCertificateFields([]string{"certificateType"}),
		WithMerchantIDsInclude([]string{"certificates"}),
		WithMerchantIDsCertificatesLimit(10),
		WithMerchantIDsLimit(5),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildMerchantIDsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[name]"); got != "Example" {
		t.Fatalf("expected filter[name]=Example, got %q", got)
	}
	if got := values.Get("filter[identifier]"); got != "merchant.com.example" {
		t.Fatalf("expected filter[identifier], got %q", got)
	}
	if got := values.Get("sort"); got != "-identifier" {
		t.Fatalf("expected sort=-identifier, got %q", got)
	}
	if got := values.Get("fields[merchantIds]"); got != "name,identifier" {
		t.Fatalf("expected fields[merchantIds], got %q", got)
	}
	if got := values.Get("fields[certificates]"); got != "certificateType" {
		t.Fatalf("expected fields[certificates], got %q", got)
	}
	if got := values.Get("include"); got != "certificates" {
		t.Fatalf("expected include=certificates, got %q", got)
	}
	if got := values.Get("limit[certificates]"); got != "10" {
		t.Fatalf("expected limit[certificates]=10, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
}

func TestBuildMerchantIDCertificatesQuery(t *testing.T) {
	query := &merchantIDCertificatesQuery{}
	opts := []MerchantIDCertificatesOption{
		WithMerchantIDCertificatesFilterDisplayName("Cert Name"),
		WithMerchantIDCertificatesFilterCertificateTypes("PASS_TYPE_ID"),
		WithMerchantIDCertificatesFilterSerialNumbers("SN123"),
		WithMerchantIDCertificatesFilterIDs("c1"),
		WithMerchantIDCertificatesSort("-serialNumber"),
		WithMerchantIDCertificatesFields([]string{"serialNumber"}),
		WithMerchantIDCertificatesPassTypeFields([]string{"identifier"}),
		WithMerchantIDCertificatesInclude([]string{"passTypeId"}),
		WithMerchantIDCertificatesLimit(5),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildMerchantIDCertificatesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[displayName]"); got != "Cert Name" {
		t.Fatalf("expected filter[displayName], got %q", got)
	}
	if got := values.Get("filter[certificateType]"); got != "PASS_TYPE_ID" {
		t.Fatalf("expected filter[certificateType], got %q", got)
	}
	if got := values.Get("filter[serialNumber]"); got != "SN123" {
		t.Fatalf("expected filter[serialNumber], got %q", got)
	}
	if got := values.Get("filter[id]"); got != "c1" {
		t.Fatalf("expected filter[id]=c1, got %q", got)
	}
	if got := values.Get("sort"); got != "-serialNumber" {
		t.Fatalf("expected sort=-serialNumber, got %q", got)
	}
	if got := values.Get("fields[certificates]"); got != "serialNumber" {
		t.Fatalf("expected fields[certificates], got %q", got)
	}
	if got := values.Get("fields[passTypeIds]"); got != "identifier" {
		t.Fatalf("expected fields[passTypeIds], got %q", got)
	}
	if got := values.Get("include"); got != "passTypeId" {
		t.Fatalf("expected include=passTypeId, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
}

func TestBuildPassTypeIDsQuery(t *testing.T) {
	query := &passTypeIDsQuery{}
	opts := []PassTypeIDsOption{
		WithPassTypeIDsFilterName("Example"),
		WithPassTypeIDsFilterIdentifier("pass.com.example"),
		WithPassTypeIDsFilterIDs([]string{"p1"}),
		WithPassTypeIDsSort("id"),
		WithPassTypeIDsFields([]string{"identifier"}),
		WithPassTypeIDsCertificateFields([]string{"certificateType"}),
		WithPassTypeIDsInclude([]string{"certificates"}),
		WithPassTypeIDsCertificatesLimit(5),
		WithPassTypeIDsLimit(10),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildPassTypeIDsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[name]"); got != "Example" {
		t.Fatalf("expected filter[name]=Example, got %q", got)
	}
	if got := values.Get("filter[identifier]"); got != "pass.com.example" {
		t.Fatalf("expected filter[identifier], got %q", got)
	}
	if got := values.Get("filter[id]"); got != "p1" {
		t.Fatalf("expected filter[id]=p1, got %q", got)
	}
	if got := values.Get("sort"); got != "id" {
		t.Fatalf("expected sort=id, got %q", got)
	}
	if got := values.Get("fields[passTypeIds]"); got != "identifier" {
		t.Fatalf("expected fields[passTypeIds], got %q", got)
	}
	if got := values.Get("fields[certificates]"); got != "certificateType" {
		t.Fatalf("expected fields[certificates], got %q", got)
	}
	if got := values.Get("include"); got != "certificates" {
		t.Fatalf("expected include=certificates, got %q", got)
	}
	if got := values.Get("limit[certificates]"); got != "5" {
		t.Fatalf("expected limit[certificates]=5, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

func TestBuildPassTypeIDCertificatesQuery(t *testing.T) {
	query := &passTypeIDCertificatesQuery{}
	opts := []PassTypeIDCertificatesOption{
		WithPassTypeIDCertificatesFilterDisplayNames([]string{"Cert Name"}),
		WithPassTypeIDCertificatesFilterCertificateTypes([]string{"PASS_TYPE_ID"}),
		WithPassTypeIDCertificatesFilterSerialNumbers([]string{"SN123"}),
		WithPassTypeIDCertificatesFilterIDs([]string{"c1"}),
		WithPassTypeIDCertificatesSort("serialNumber"),
		WithPassTypeIDCertificatesFields([]string{"serialNumber"}),
		WithPassTypeIDCertificatesPassTypeIDFields([]string{"identifier"}),
		WithPassTypeIDCertificatesInclude([]string{"passTypeId"}),
		WithPassTypeIDCertificatesLimit(5),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildPassTypeIDCertificatesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("filter[displayName]"); got != "Cert Name" {
		t.Fatalf("expected filter[displayName], got %q", got)
	}
	if got := values.Get("filter[certificateType]"); got != "PASS_TYPE_ID" {
		t.Fatalf("expected filter[certificateType], got %q", got)
	}
	if got := values.Get("filter[serialNumber]"); got != "SN123" {
		t.Fatalf("expected filter[serialNumber], got %q", got)
	}
	if got := values.Get("filter[id]"); got != "c1" {
		t.Fatalf("expected filter[id]=c1, got %q", got)
	}
	if got := values.Get("sort"); got != "serialNumber" {
		t.Fatalf("expected sort=serialNumber, got %q", got)
	}
	if got := values.Get("fields[certificates]"); got != "serialNumber" {
		t.Fatalf("expected fields[certificates], got %q", got)
	}
	if got := values.Get("fields[passTypeIds]"); got != "identifier" {
		t.Fatalf("expected fields[passTypeIds], got %q", got)
	}
	if got := values.Get("include"); got != "passTypeId" {
		t.Fatalf("expected include=passTypeId, got %q", got)
	}
	if got := values.Get("limit"); got != "5" {
		t.Fatalf("expected limit=5, got %q", got)
	}
}

func TestBuildPerfPowerMetricsQuery(t *testing.T) {
	query := &perfPowerMetricsQuery{
		platforms:   []string{"IOS"},
		metricTypes: []string{"DISK", "HANG"},
		deviceTypes: []string{"iPhone15,2"},
	}
	values, err := url.ParseQuery(buildPerfPowerMetricsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("filter[platform]") != "IOS" {
		t.Fatalf("expected platform filter, got %q", values.Get("filter[platform]"))
	}
	if values.Get("filter[metricType]") != "DISK,HANG" {
		t.Fatalf("expected metricType filter, got %q", values.Get("filter[metricType]"))
	}
	if values.Get("filter[deviceType]") != "iPhone15,2" {
		t.Fatalf("expected deviceType filter, got %q", values.Get("filter[deviceType]"))
	}
}

func TestBuildDiagnosticSignaturesQuery(t *testing.T) {
	query := &diagnosticSignaturesQuery{
		listQuery:       listQuery{limit: 25},
		diagnosticTypes: []string{"HANGS"},
		fields:          []string{"diagnosticType", "signature"},
	}
	values, err := url.ParseQuery(buildDiagnosticSignaturesQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("filter[diagnosticType]") != "HANGS" {
		t.Fatalf("expected diagnosticType filter, got %q", values.Get("filter[diagnosticType]"))
	}
	if values.Get("fields[diagnosticSignatures]") != "diagnosticType,signature" {
		t.Fatalf("expected fields, got %q", values.Get("fields[diagnosticSignatures]"))
	}
	if values.Get("limit") != "25" {
		t.Fatalf("expected limit=25, got %q", values.Get("limit"))
	}
}

func TestBuildDiagnosticLogsQuery(t *testing.T) {
	query := &diagnosticLogsQuery{listQuery: listQuery{limit: 50}}
	values, err := url.ParseQuery(buildDiagnosticLogsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "50" {
		t.Fatalf("expected limit=50, got %q", values.Get("limit"))
	}
}

func TestBuildAndroidToIosAppMappingDetailsQuery(t *testing.T) {
	query := &androidToIosAppMappingDetailsQuery{
		listQuery: listQuery{limit: 10},
		fields:    []string{"packageName", "appSigningKeyPublicCertificateSha256Fingerprints"},
	}
	values, err := url.ParseQuery(buildAndroidToIosAppMappingDetailsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
	if values.Get("fields[androidToIosAppMappingDetails]") != "packageName,appSigningKeyPublicCertificateSha256Fingerprints" {
		t.Fatalf("unexpected fields, got %q", values.Get("fields[androidToIosAppMappingDetails]"))
	}
}

func TestBuildAlternativeDistributionDomainsQuery(t *testing.T) {
	query := &alternativeDistributionDomainsQuery{
		listQuery: listQuery{limit: 20},
		fields:    []string{"domain", "referenceName"},
	}
	values, err := url.ParseQuery(buildAlternativeDistributionDomainsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "20" {
		t.Fatalf("expected limit=20, got %q", values.Get("limit"))
	}
	if values.Get("fields[alternativeDistributionDomains]") != "domain,referenceName" {
		t.Fatalf("unexpected fields, got %q", values.Get("fields[alternativeDistributionDomains]"))
	}
}

func TestBuildAlternativeDistributionKeysQuery(t *testing.T) {
	existsApp := true
	query := &alternativeDistributionKeysQuery{
		listQuery: listQuery{limit: 15},
		fields:    []string{"publicKey"},
		existsApp: &existsApp,
	}
	values, err := url.ParseQuery(buildAlternativeDistributionKeysQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "15" {
		t.Fatalf("expected limit=15, got %q", values.Get("limit"))
	}
	if values.Get("fields[alternativeDistributionKeys]") != "publicKey" {
		t.Fatalf("unexpected fields, got %q", values.Get("fields[alternativeDistributionKeys]"))
	}
	if values.Get("exists[app]") != "true" {
		t.Fatalf("expected exists[app]=true, got %q", values.Get("exists[app]"))
	}
}

func TestBuildAlternativeDistributionPackageVersionsQuery(t *testing.T) {
	query := &alternativeDistributionPackageVersionsQuery{
		listQuery: listQuery{limit: 7},
	}
	values, err := url.ParseQuery(buildAlternativeDistributionPackageVersionsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "7" {
		t.Fatalf("expected limit=7, got %q", values.Get("limit"))
	}
}

func TestBuildAlternativeDistributionPackageVariantsQuery(t *testing.T) {
	query := &alternativeDistributionPackageVariantsQuery{
		listQuery: listQuery{limit: 9},
		fields:    []string{"url", "fileChecksum"},
	}
	values, err := url.ParseQuery(buildAlternativeDistributionPackageVariantsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "9" {
		t.Fatalf("expected limit=9, got %q", values.Get("limit"))
	}
	if values.Get("fields[alternativeDistributionPackageVariants]") != "url,fileChecksum" {
		t.Fatalf("unexpected fields, got %q", values.Get("fields[alternativeDistributionPackageVariants]"))
	}
}

func TestBuildAlternativeDistributionPackageDeltasQuery(t *testing.T) {
	query := &alternativeDistributionPackageDeltasQuery{
		listQuery: listQuery{limit: 11},
		fields:    []string{"url", "fileChecksum"},
	}
	values, err := url.ParseQuery(buildAlternativeDistributionPackageDeltasQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "11" {
		t.Fatalf("expected limit=11, got %q", values.Get("limit"))
	}
	if values.Get("fields[alternativeDistributionPackageDeltas]") != "url,fileChecksum" {
		t.Fatalf("unexpected fields, got %q", values.Get("fields[alternativeDistributionPackageDeltas]"))
	}
}

func TestBuildBackgroundAssetsQuery(t *testing.T) {
	query := &backgroundAssetsQuery{
		listQuery:            listQuery{limit: 10},
		archived:             []string{"true"},
		assetPackIdentifiers: []string{"pack-1", "pack-2"},
	}
	values, err := url.ParseQuery(buildBackgroundAssetsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
	if values.Get("filter[archived]") != "true" {
		t.Fatalf("expected filter[archived]=true, got %q", values.Get("filter[archived]"))
	}
	if values.Get("filter[assetPackIdentifier]") != "pack-1,pack-2" {
		t.Fatalf("expected filter[assetPackIdentifier]=pack-1,pack-2, got %q", values.Get("filter[assetPackIdentifier]"))
	}
}

func TestBuildBackgroundAssetVersionsQuery(t *testing.T) {
	query := &backgroundAssetVersionsQuery{listQuery: listQuery{limit: 25}}
	values, err := url.ParseQuery(buildBackgroundAssetVersionsQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "25" {
		t.Fatalf("expected limit=25, got %q", values.Get("limit"))
	}
}

func TestBuildBackgroundAssetUploadFilesQuery(t *testing.T) {
	query := &backgroundAssetUploadFilesQuery{listQuery: listQuery{limit: 15}}
	values, err := url.ParseQuery(buildBackgroundAssetUploadFilesQuery(query))
	if err != nil {
		t.Fatalf("ParseQuery() error: %v", err)
	}
	if values.Get("limit") != "15" {
		t.Fatalf("expected limit=15, got %q", values.Get("limit"))
	}
}

func TestBuildUploadCreateRequest_JSON(t *testing.T) {
	req := BuildUploadCreateRequest{
		Data: BuildUploadCreateData{
			Type: ResourceTypeBuildUploads,
			Attributes: BuildUploadAttributes{
				CFBundleShortVersionString: "1.0.0",
				CFBundleVersion:            "123",
				Platform:                   PlatformIOS,
			},
			Relationships: &BuildUploadRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   "APP_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	// Unmarshal and verify structure
	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				CFBundleShortVersionString string `json:"cfBundleShortVersionString"`
				CFBundleVersion            string `json:"cfBundleVersion"`
				Platform                   string `json:"platform"`
			} `json:"attributes"`
			Relationships struct {
				App struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"app"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "buildUploads" {
		t.Fatalf("expected type=buildUploads, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.CFBundleShortVersionString != "1.0.0" {
		t.Fatalf("expected cfBundleShortVersionString=1.0.0, got %q", parsed.Data.Attributes.CFBundleShortVersionString)
	}
	if parsed.Data.Attributes.CFBundleVersion != "123" {
		t.Fatalf("expected cfBundleVersion=123, got %q", parsed.Data.Attributes.CFBundleVersion)
	}
	if parsed.Data.Attributes.Platform != "IOS" {
		t.Fatalf("expected platform=IOS, got %q", parsed.Data.Attributes.Platform)
	}
	if parsed.Data.Relationships.App.Data.Type != "apps" {
		t.Fatalf("expected app type=apps, got %q", parsed.Data.Relationships.App.Data.Type)
	}
	if parsed.Data.Relationships.App.Data.ID != "APP_ID_123" {
		t.Fatalf("expected app id=APP_ID_123, got %q", parsed.Data.Relationships.App.Data.ID)
	}
}

func TestSubscriptionOfferCodeOneTimeUseCodeCreateRequest_JSON(t *testing.T) {
	req := SubscriptionOfferCodeOneTimeUseCodeCreateRequest{
		Data: SubscriptionOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeSubscriptionOfferCodeOneTimeUseCodes,
			Attributes: SubscriptionOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  3,
				ExpirationDate: "2026-02-01",
			},
			Relationships: SubscriptionOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptionOfferCodes,
						ID:   "OFFER_CODE_ID",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				NumberOfCodes  int    `json:"numberOfCodes"`
				ExpirationDate string `json:"expirationDate"`
			} `json:"attributes"`
			Relationships struct {
				OfferCode struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"offerCode"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "subscriptionOfferCodeOneTimeUseCodes" {
		t.Fatalf("expected type=subscriptionOfferCodeOneTimeUseCodes, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.NumberOfCodes != 3 {
		t.Fatalf("expected numberOfCodes=3, got %d", parsed.Data.Attributes.NumberOfCodes)
	}
	if parsed.Data.Attributes.ExpirationDate != "2026-02-01" {
		t.Fatalf("expected expirationDate=2026-02-01, got %q", parsed.Data.Attributes.ExpirationDate)
	}
	if parsed.Data.Relationships.OfferCode.Data.Type != "subscriptionOfferCodes" {
		t.Fatalf("expected offerCode type=subscriptionOfferCodes, got %q", parsed.Data.Relationships.OfferCode.Data.Type)
	}
	if parsed.Data.Relationships.OfferCode.Data.ID != "OFFER_CODE_ID" {
		t.Fatalf("expected offerCode id=OFFER_CODE_ID, got %q", parsed.Data.Relationships.OfferCode.Data.ID)
	}
}

func TestBuildUploadFileCreateRequest_JSON(t *testing.T) {
	req := BuildUploadFileCreateRequest{
		Data: BuildUploadFileCreateData{
			Type: ResourceTypeBuildUploadFiles,
			Attributes: BuildUploadFileAttributes{
				FileName:  "app.ipa",
				FileSize:  1024000,
				UTI:       UTIIPA,
				AssetType: AssetTypeAsset,
			},
			Relationships: &BuildUploadFileRelationships{
				BuildUpload: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBuildUploads,
						ID:   "UPLOAD_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	// Unmarshal and verify structure
	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id,omitempty"`
			Attributes struct {
				FileName  string `json:"fileName"`
				FileSize  int64  `json:"fileSize"`
				UTI       string `json:"uti"`
				AssetType string `json:"assetType"`
			} `json:"attributes"`
			Relationships struct {
				BuildUpload struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"buildUpload"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "buildUploadFiles" {
		t.Fatalf("expected type=buildUploadFiles, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.FileName != "app.ipa" {
		t.Fatalf("expected fileName=app.ipa, got %q", parsed.Data.Attributes.FileName)
	}
	if parsed.Data.Attributes.FileSize != 1024000 {
		t.Fatalf("expected fileSize=1024000, got %d", parsed.Data.Attributes.FileSize)
	}
	if parsed.Data.Attributes.UTI != "com.apple.ipa" {
		t.Fatalf("expected uti=com.apple.ipa, got %q", parsed.Data.Attributes.UTI)
	}
	if parsed.Data.Attributes.AssetType != "ASSET" {
		t.Fatalf("expected assetType=ASSET, got %q", parsed.Data.Attributes.AssetType)
	}
	if parsed.Data.Relationships.BuildUpload.Data.Type != "buildUploads" {
		t.Fatalf("expected buildUpload type=buildUploads, got %q", parsed.Data.Relationships.BuildUpload.Data.Type)
	}
	if parsed.Data.Relationships.BuildUpload.Data.ID != "UPLOAD_ID_123" {
		t.Fatalf("expected buildUpload id=UPLOAD_ID_123, got %q", parsed.Data.Relationships.BuildUpload.Data.ID)
	}
}

func TestBuildUploadFileUpdateRequest_JSON(t *testing.T) {
	uploaded := true
	req := BuildUploadFileUpdateRequest{
		Data: BuildUploadFileUpdateData{
			Type: ResourceTypeBuildUploadFiles,
			ID:   "FILE_ID_123",
			Attributes: &BuildUploadFileUpdateAttributes{
				SourceFileChecksums: &Checksums{
					File: &Checksum{
						Hash:      "abc123def456",
						Algorithm: ChecksumAlgorithmSHA256,
					},
				},
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	// Unmarshal and verify structure
	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				SourceFileChecksums struct {
					File struct {
						Hash      string `json:"hash"`
						Algorithm string `json:"algorithm"`
					} `json:"file"`
				} `json:"sourceFileChecksums"`
				Uploaded bool `json:"uploaded"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.ID != "FILE_ID_123" {
		t.Fatalf("expected id=FILE_ID_123, got %q", parsed.Data.ID)
	}
	if !parsed.Data.Attributes.Uploaded {
		t.Fatalf("expected uploaded=true, got false")
	}
	if parsed.Data.Attributes.SourceFileChecksums.File.Hash != "abc123def456" {
		t.Fatalf("expected checksum hash=abc123def456, got %q", parsed.Data.Attributes.SourceFileChecksums.File.Hash)
	}
	if parsed.Data.Attributes.SourceFileChecksums.File.Algorithm != "SHA_256" {
		t.Fatalf("expected algorithm=SHA_256, got %q", parsed.Data.Attributes.SourceFileChecksums.File.Algorithm)
	}
}

func TestAppScreenshotSetCreateRequest_JSON(t *testing.T) {
	req := AppScreenshotSetCreateRequest{
		Data: AppScreenshotSetCreateData{
			Type:       ResourceTypeAppScreenshotSets,
			Attributes: AppScreenshotSetAttributes{ScreenshotDisplayType: "APP_IPHONE_65"},
			Relationships: &AppScreenshotSetRelationships{
				AppStoreVersionLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersionLocalizations,
						ID:   "LOC_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				ScreenshotDisplayType string `json:"screenshotDisplayType"`
			} `json:"attributes"`
			Relationships struct {
				AppStoreVersionLocalization struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"appStoreVersionLocalization"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appScreenshotSets" {
		t.Fatalf("expected type=appScreenshotSets, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.ScreenshotDisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected screenshotDisplayType=APP_IPHONE_65, got %q", parsed.Data.Attributes.ScreenshotDisplayType)
	}
	if parsed.Data.Relationships.AppStoreVersionLocalization.Data.Type != "appStoreVersionLocalizations" {
		t.Fatalf("expected relationship type=appStoreVersionLocalizations, got %q", parsed.Data.Relationships.AppStoreVersionLocalization.Data.Type)
	}
	if parsed.Data.Relationships.AppStoreVersionLocalization.Data.ID != "LOC_ID_123" {
		t.Fatalf("expected relationship id=LOC_ID_123, got %q", parsed.Data.Relationships.AppStoreVersionLocalization.Data.ID)
	}
}

func TestAppScreenshotCreateRequest_JSON(t *testing.T) {
	req := AppScreenshotCreateRequest{
		Data: AppScreenshotCreateData{
			Type: ResourceTypeAppScreenshots,
			Attributes: AppScreenshotAttributes{
				FileName: "shot.png",
				FileSize: 1024,
			},
			Relationships: &AppScreenshotRelationships{
				AppScreenshotSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppScreenshotSets,
						ID:   "SET_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				FileName string `json:"fileName"`
				FileSize int64  `json:"fileSize"`
			} `json:"attributes"`
			Relationships struct {
				AppScreenshotSet struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"appScreenshotSet"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appScreenshots" {
		t.Fatalf("expected type=appScreenshots, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.FileName != "shot.png" {
		t.Fatalf("expected fileName=shot.png, got %q", parsed.Data.Attributes.FileName)
	}
	if parsed.Data.Attributes.FileSize != 1024 {
		t.Fatalf("expected fileSize=1024, got %d", parsed.Data.Attributes.FileSize)
	}
	if parsed.Data.Relationships.AppScreenshotSet.Data.Type != "appScreenshotSets" {
		t.Fatalf("expected appScreenshotSet type=appScreenshotSets, got %q", parsed.Data.Relationships.AppScreenshotSet.Data.Type)
	}
	if parsed.Data.Relationships.AppScreenshotSet.Data.ID != "SET_ID_123" {
		t.Fatalf("expected appScreenshotSet id=SET_ID_123, got %q", parsed.Data.Relationships.AppScreenshotSet.Data.ID)
	}
}

func TestAppScreenshotUpdateRequest_JSON(t *testing.T) {
	uploaded := true
	checksum := "abc123"
	req := AppScreenshotUpdateRequest{
		Data: AppScreenshotUpdateData{
			Type: ResourceTypeAppScreenshots,
			ID:   "SCREENSHOT_ID_123",
			Attributes: &AppScreenshotUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum,
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Uploaded           bool   `json:"uploaded"`
				SourceFileChecksum string `json:"sourceFileChecksum"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appScreenshots" {
		t.Fatalf("expected type=appScreenshots, got %q", parsed.Data.Type)
	}
	if parsed.Data.ID != "SCREENSHOT_ID_123" {
		t.Fatalf("expected id=SCREENSHOT_ID_123, got %q", parsed.Data.ID)
	}
	if !parsed.Data.Attributes.Uploaded {
		t.Fatalf("expected uploaded=true")
	}
	if parsed.Data.Attributes.SourceFileChecksum != "abc123" {
		t.Fatalf("expected checksum=abc123, got %q", parsed.Data.Attributes.SourceFileChecksum)
	}
}

func TestAppPreviewSetCreateRequest_JSON(t *testing.T) {
	req := AppPreviewSetCreateRequest{
		Data: AppPreviewSetCreateData{
			Type:       ResourceTypeAppPreviewSets,
			Attributes: AppPreviewSetAttributes{PreviewType: "IPHONE_65"},
			Relationships: &AppPreviewSetRelationships{
				AppStoreVersionLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersionLocalizations,
						ID:   "LOC_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				PreviewType string `json:"previewType"`
			} `json:"attributes"`
			Relationships struct {
				AppStoreVersionLocalization struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"appStoreVersionLocalization"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appPreviewSets" {
		t.Fatalf("expected type=appPreviewSets, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.PreviewType != "IPHONE_65" {
		t.Fatalf("expected previewType=IPHONE_65, got %q", parsed.Data.Attributes.PreviewType)
	}
	if parsed.Data.Relationships.AppStoreVersionLocalization.Data.Type != "appStoreVersionLocalizations" {
		t.Fatalf("expected relationship type=appStoreVersionLocalizations, got %q", parsed.Data.Relationships.AppStoreVersionLocalization.Data.Type)
	}
	if parsed.Data.Relationships.AppStoreVersionLocalization.Data.ID != "LOC_ID_123" {
		t.Fatalf("expected relationship id=LOC_ID_123, got %q", parsed.Data.Relationships.AppStoreVersionLocalization.Data.ID)
	}
}

func TestAppPreviewCreateRequest_JSON(t *testing.T) {
	req := AppPreviewCreateRequest{
		Data: AppPreviewCreateData{
			Type: ResourceTypeAppPreviews,
			Attributes: AppPreviewAttributes{
				FileName: "preview.mov",
				FileSize: 2048,
				MimeType: "video/quicktime",
			},
			Relationships: &AppPreviewRelationships{
				AppPreviewSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppPreviewSets,
						ID:   "SET_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				FileName string `json:"fileName"`
				FileSize int64  `json:"fileSize"`
				MimeType string `json:"mimeType"`
			} `json:"attributes"`
			Relationships struct {
				AppPreviewSet struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"appPreviewSet"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appPreviews" {
		t.Fatalf("expected type=appPreviews, got %q", parsed.Data.Type)
	}
	if parsed.Data.Attributes.FileName != "preview.mov" {
		t.Fatalf("expected fileName=preview.mov, got %q", parsed.Data.Attributes.FileName)
	}
	if parsed.Data.Attributes.FileSize != 2048 {
		t.Fatalf("expected fileSize=2048, got %d", parsed.Data.Attributes.FileSize)
	}
	if parsed.Data.Attributes.MimeType != "video/quicktime" {
		t.Fatalf("expected mimeType=video/quicktime, got %q", parsed.Data.Attributes.MimeType)
	}
	if parsed.Data.Relationships.AppPreviewSet.Data.Type != "appPreviewSets" {
		t.Fatalf("expected appPreviewSet type=appPreviewSets, got %q", parsed.Data.Relationships.AppPreviewSet.Data.Type)
	}
	if parsed.Data.Relationships.AppPreviewSet.Data.ID != "SET_ID_123" {
		t.Fatalf("expected appPreviewSet id=SET_ID_123, got %q", parsed.Data.Relationships.AppPreviewSet.Data.ID)
	}
}

func TestAppPreviewUpdateRequest_JSON(t *testing.T) {
	uploaded := true
	checksum := "def456"
	req := AppPreviewUpdateRequest{
		Data: AppPreviewUpdateData{
			Type: ResourceTypeAppPreviews,
			ID:   "PREVIEW_ID_123",
			Attributes: &AppPreviewUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksum,
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	var parsed struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Uploaded           bool   `json:"uploaded"`
				SourceFileChecksum string `json:"sourceFileChecksum"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appPreviews" {
		t.Fatalf("expected type=appPreviews, got %q", parsed.Data.Type)
	}
	if parsed.Data.ID != "PREVIEW_ID_123" {
		t.Fatalf("expected id=PREVIEW_ID_123, got %q", parsed.Data.ID)
	}
	if !parsed.Data.Attributes.Uploaded {
		t.Fatalf("expected uploaded=true")
	}
	if parsed.Data.Attributes.SourceFileChecksum != "def456" {
		t.Fatalf("expected checksum=def456, got %q", parsed.Data.Attributes.SourceFileChecksum)
	}
}

func TestAppStoreVersionSubmissionCreateRequest_JSON(t *testing.T) {
	req := AppStoreVersionSubmissionCreateRequest{
		Data: AppStoreVersionSubmissionCreateData{
			Type: ResourceTypeAppStoreVersionSubmissions,
			Relationships: &AppStoreVersionSubmissionRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   "VERSION_ID_123",
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(req)
	if err != nil {
		t.Fatalf("BuildRequestBody() error: %v", err)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		t.Fatalf("read body error: %v", err)
	}

	// Unmarshal and verify structure
	var parsed struct {
		Data struct {
			Type          string `json:"type"`
			Relationships struct {
				AppStoreVersion struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"appStoreVersion"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if parsed.Data.Type != "appStoreVersionSubmissions" {
		t.Fatalf("expected type=appStoreVersionSubmissions, got %q", parsed.Data.Type)
	}
	if parsed.Data.Relationships.AppStoreVersion.Data.Type != "appStoreVersions" {
		t.Fatalf("expected version type=appStoreVersions, got %q", parsed.Data.Relationships.AppStoreVersion.Data.Type)
	}
	if parsed.Data.Relationships.AppStoreVersion.Data.ID != "VERSION_ID_123" {
		t.Fatalf("expected version id=VERSION_ID_123, got %q", parsed.Data.Relationships.AppStoreVersion.Data.ID)
	}
}

func TestWithRetry_ZeroRetries(t *testing.T) {
	callCount := 0
	wantErr := fmt.Errorf("transient error")

	_, err := WithRetry(context.Background(), func() (string, error) {
		callCount++
		return "", wantErr
	}, RetryOptions{MaxRetries: 0})

	// Should fail immediately without retries
	if callCount != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", callCount)
	}
	if err != wantErr {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestWithRetry_NegativeRetriesUsesDefault(t *testing.T) {
	callCount := 0

	WithRetry(context.Background(), func() (string, error) {
		callCount++
		if callCount < DefaultMaxRetries+1 {
			return "", &RetryableError{RetryAfter: time.Millisecond}
		}
		return "success", nil
	}, RetryOptions{MaxRetries: -1})

	// Should use default retries (3)
	if callCount != DefaultMaxRetries+1 {
		t.Fatalf("expected %d calls (default retries), got %d", DefaultMaxRetries+1, callCount)
	}
}

func TestWithRetry_AttemptCountInErrorMessage(t *testing.T) {
	const maxRetries = 2
	callCount := 0

	_, err := WithRetry(context.Background(), func() (string, error) {
		callCount++
		return "", &RetryableError{RetryAfter: time.Millisecond}
	}, RetryOptions{MaxRetries: maxRetries})

	// Should have exhausted all retries
	if callCount != maxRetries+1 {
		t.Fatalf("expected %d calls, got %d", maxRetries+1, callCount)
	}

	// Error message should report the correct number of retries
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	// With MaxRetries=2, we try 3 times total (initial + 2 retries)
	// Error message should say "after 3 retries"
	if !strings.Contains(errMsg, "after 3 retries") {
		t.Fatalf("error message should mention 'after 3 retries', got: %s", errMsg)
	}
}

func TestWithRetry_NonRetryableErrorFailsFast(t *testing.T) {
	callCount := 0
	wantErr := fmt.Errorf("non-retryable error")

	_, err := WithRetry(context.Background(), func() (string, error) {
		callCount++
		return "", wantErr
	}, RetryOptions{MaxRetries: 3, BaseDelay: time.Millisecond})

	// Should fail immediately without retries
	if callCount != 1 {
		t.Fatalf("expected 1 call (no retries for non-retryable error), got %d", callCount)
	}
	if err != wantErr {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}

func TestWithRetry_SuccessOnFirstTry(t *testing.T) {
	callCount := 0

	result, err := WithRetry(context.Background(), func() (string, error) {
		callCount++
		return "success", nil
	}, RetryOptions{MaxRetries: 3})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %q", result)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

func TestPaginateAll_CiBuildRuns_ManyPages(t *testing.T) {
	const totalPages = 20
	const perPage = 50

	makePage := func(page int) *CiBuildRunsResponse {
		data := make([]CiBuildRunResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiBuildRunResource{
				Type: ResourceTypeCiBuildRuns,
				ID:   fmt.Sprintf("run-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiBuildRunsResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	buildRuns, ok := response.(*CiBuildRunsResponse)
	if !ok {
		t.Fatalf("expected CiBuildRunsResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(buildRuns.Data) != expected {
		t.Fatalf("expected %d build runs, got %d", expected, len(buildRuns.Data))
	}
	if buildRuns.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", buildRuns.Links.Next)
	}
}

func TestPaginateAll_DetectsRepeatedNextURL(t *testing.T) {
	firstPage := &AppsResponse{
		Data: []Resource[AppAttributes]{
			{Type: ResourceTypeApps, ID: "app-1"},
		},
		Links: Links{Next: "page=1"},
	}

	calls := 0
	_, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		calls++
		return &AppsResponse{
			Data: []Resource[AppAttributes]{
				{Type: ResourceTypeApps, ID: fmt.Sprintf("app-%d", calls+1)},
			},
			Links: Links{Next: "page=1"},
		}, nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 fetch, got %d", calls)
	}
}

func TestPaginateAll_CiArtifacts_ManyPages(t *testing.T) {
	const totalPages = 4
	const perPage = 3

	makePage := func(page int) *CiArtifactsResponse {
		data := make([]CiArtifactResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiArtifactResource{
				Type: ResourceTypeCiArtifacts,
				ID:   fmt.Sprintf("artifact-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiArtifactsResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	artifacts, ok := response.(*CiArtifactsResponse)
	if !ok {
		t.Fatalf("expected CiArtifactsResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(artifacts.Data) != expected {
		t.Fatalf("expected %d artifacts, got %d", expected, len(artifacts.Data))
	}
	if artifacts.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", artifacts.Links.Next)
	}
}

func TestPaginateAll_CiTestResults_ManyPages(t *testing.T) {
	const totalPages = 3
	const perPage = 4

	makePage := func(page int) *CiTestResultsResponse {
		data := make([]CiTestResultResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiTestResultResource{
				Type: ResourceTypeCiTestResults,
				ID:   fmt.Sprintf("test-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiTestResultsResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	results, ok := response.(*CiTestResultsResponse)
	if !ok {
		t.Fatalf("expected CiTestResultsResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(results.Data) != expected {
		t.Fatalf("expected %d test results, got %d", expected, len(results.Data))
	}
	if results.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", results.Links.Next)
	}
}

func TestPaginateAll_CiIssues_ManyPages(t *testing.T) {
	const totalPages = 5
	const perPage = 2

	makePage := func(page int) *CiIssuesResponse {
		data := make([]CiIssueResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiIssueResource{
				Type: ResourceTypeCiIssues,
				ID:   fmt.Sprintf("issue-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiIssuesResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	issues, ok := response.(*CiIssuesResponse)
	if !ok {
		t.Fatalf("expected CiIssuesResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(issues.Data) != expected {
		t.Fatalf("expected %d issues, got %d", expected, len(issues.Data))
	}
	if issues.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", issues.Links.Next)
	}
}

func TestPaginateAll_ScmRepositories_ManyPages(t *testing.T) {
	const totalPages = 3
	const perPage = 2

	makePage := func(page int) *ScmRepositoriesResponse {
		data := make([]ScmRepositoryResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, ScmRepositoryResource{
				Type: ResourceTypeScmRepositories,
				ID:   fmt.Sprintf("repo-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &ScmRepositoriesResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	repos, ok := response.(*ScmRepositoriesResponse)
	if !ok {
		t.Fatalf("expected ScmRepositoriesResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(repos.Data) != expected {
		t.Fatalf("expected %d repositories, got %d", expected, len(repos.Data))
	}
	if repos.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", repos.Links.Next)
	}
}

func TestPaginateAll_CiMacOsVersions_ManyPages(t *testing.T) {
	const totalPages = 4
	const perPage = 2

	makePage := func(page int) *CiMacOsVersionsResponse {
		data := make([]CiMacOsVersionResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiMacOsVersionResource{
				Type: ResourceTypeCiMacOsVersions,
				ID:   fmt.Sprintf("macos-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiMacOsVersionsResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	versions, ok := response.(*CiMacOsVersionsResponse)
	if !ok {
		t.Fatalf("expected CiMacOsVersionsResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(versions.Data) != expected {
		t.Fatalf("expected %d macOS versions, got %d", expected, len(versions.Data))
	}
	if versions.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", versions.Links.Next)
	}
}

func TestPaginateAll_CiXcodeVersions_ManyPages(t *testing.T) {
	const totalPages = 3
	const perPage = 3

	makePage := func(page int) *CiXcodeVersionsResponse {
		data := make([]CiXcodeVersionResource, 0, perPage)
		for i := 0; i < perPage; i++ {
			data = append(data, CiXcodeVersionResource{
				Type: ResourceTypeCiXcodeVersions,
				ID:   fmt.Sprintf("xcode-%d-%d", page, i),
			})
		}
		links := Links{}
		if page < totalPages {
			links.Next = fmt.Sprintf("page=%d", page+1)
		}
		return &CiXcodeVersionsResponse{
			Data:  data,
			Links: links,
		}
	}

	firstPage := makePage(1)
	response, err := PaginateAll(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		pageStr := strings.TrimPrefix(nextURL, "page=")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid next URL %q", nextURL)
		}
		return makePage(page), nil
	})
	if err != nil {
		t.Fatalf("PaginateAll() error: %v", err)
	}

	versions, ok := response.(*CiXcodeVersionsResponse)
	if !ok {
		t.Fatalf("expected CiXcodeVersionsResponse, got %T", response)
	}
	expected := totalPages * perPage
	if len(versions.Data) != expected {
		t.Fatalf("expected %d Xcode versions, got %d", expected, len(versions.Data))
	}
	if versions.Links.Next != "" {
		t.Fatalf("expected next link to be cleared, got %q", versions.Links.Next)
	}
}
