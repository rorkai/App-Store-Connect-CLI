package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type stubVersionLocalizationClient struct {
	getResp  *asc.AppStoreVersionLocalizationsResponse
	getResps []*asc.AppStoreVersionLocalizationsResponse
	getErr   error
	getErrs  []error
	getCalls int
	onGet    func(context.Context, int)

	createErrs        []error
	createCalls       []asc.AppStoreVersionLocalizationCreateAttributes
	updateErrs        []error
	updateCalls       []asc.AppStoreVersionLocalizationAttributes
	updateFieldsCalls []map[string]string
}

type expiringVersionLocalizationClient struct {
	getCalls    int
	updateCalls int
}

type stubAppInfoLocalizationClient struct {
	getResp     *asc.AppInfoLocalizationsResponse
	getResps    []*asc.AppInfoLocalizationsResponse
	getErrs     []error
	getCalls    int
	createErrs  []error
	createCalls []asc.AppInfoLocalizationCreateAttributes
	updateErrs  []error
	updateCalls []map[string]string
	onGet       func(context.Context, int)
}

func (s *stubAppInfoLocalizationClient) GetAppInfoLocalizations(ctx context.Context, _ string, _ ...asc.AppInfoLocalizationsOption) (*asc.AppInfoLocalizationsResponse, error) {
	s.getCalls++
	if s.onGet != nil {
		s.onGet(ctx, s.getCalls)
	}
	index := s.getCalls - 1
	if index >= 0 && index < len(s.getErrs) && s.getErrs[index] != nil {
		return nil, s.getErrs[index]
	}
	if index >= 0 && index < len(s.getResps) {
		return s.getResps[index], nil
	}
	return s.getResp, nil
}

func (s *stubAppInfoLocalizationClient) CreateAppInfoLocalization(_ context.Context, _ string, attrs asc.AppInfoLocalizationCreateAttributes) (*asc.AppInfoLocalizationResponse, error) {
	s.createCalls = append(s.createCalls, attrs)
	index := len(s.createCalls) - 1
	if index < len(s.createErrs) && s.createErrs[index] != nil {
		return nil, s.createErrs[index]
	}
	return &asc.AppInfoLocalizationResponse{Data: asc.Resource[asc.AppInfoLocalizationAttributes]{ID: "created-id", Attributes: asc.AppInfoLocalizationAttributes{
		Locale:            attrs.Locale,
		Name:              attrs.Name,
		Subtitle:          attrs.Subtitle,
		PrivacyPolicyURL:  attrs.PrivacyPolicyURL,
		PrivacyChoicesURL: attrs.PrivacyChoicesURL,
		PrivacyPolicyText: attrs.PrivacyPolicyText,
	}}}, nil
}

func (s *stubAppInfoLocalizationClient) UpdateAppInfoLocalizationFields(_ context.Context, id string, fields map[string]string) (*asc.AppInfoLocalizationResponse, error) {
	s.updateCalls = append(s.updateCalls, cloneLocalizationValues(fields))
	index := len(s.updateCalls) - 1
	if index < len(s.updateErrs) && s.updateErrs[index] != nil {
		return nil, s.updateErrs[index]
	}
	return &asc.AppInfoLocalizationResponse{Data: asc.Resource[asc.AppInfoLocalizationAttributes]{ID: id}}, nil
}

func (c *expiringVersionLocalizationClient) GetAppStoreVersionLocalizations(ctx context.Context, _ string, _ ...asc.AppStoreVersionLocalizationsOption) (*asc.AppStoreVersionLocalizationsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.getCalls++
	description := "Old description"
	if c.getCalls > 1 {
		description = "New description"
	}
	return &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: description}},
	}}, nil
}

func (c *expiringVersionLocalizationClient) CreateAppStoreVersionLocalization(context.Context, string, asc.AppStoreVersionLocalizationCreateAttributes) (*asc.AppStoreVersionLocalizationResponse, error) {
	return nil, errors.New("unexpected create")
}

func (c *expiringVersionLocalizationClient) UpdateAppStoreVersionLocalizationFields(ctx context.Context, _ string, _ map[string]string) (*asc.AppStoreVersionLocalizationResponse, error) {
	c.updateCalls++
	<-ctx.Done()
	return nil, &asc.RetryableError{Err: ctx.Err()}
}

func (s *stubVersionLocalizationClient) GetAppStoreVersionLocalizations(ctx context.Context, _ string, _ ...asc.AppStoreVersionLocalizationsOption) (*asc.AppStoreVersionLocalizationsResponse, error) {
	s.getCalls++
	if s.onGet != nil {
		s.onGet(ctx, s.getCalls)
	}
	if s.getErr != nil {
		return nil, s.getErr
	}
	index := s.getCalls - 1
	if index >= 0 && index < len(s.getErrs) && s.getErrs[index] != nil {
		return nil, s.getErrs[index]
	}
	if index >= 0 && index < len(s.getResps) {
		return s.getResps[index], nil
	}
	return s.getResp, nil
}

func (s *stubVersionLocalizationClient) CreateAppStoreVersionLocalization(_ context.Context, _ string, attrs asc.AppStoreVersionLocalizationCreateAttributes) (*asc.AppStoreVersionLocalizationResponse, error) {
	s.createCalls = append(s.createCalls, attrs)
	callIndex := len(s.createCalls) - 1
	if callIndex < len(s.createErrs) && s.createErrs[callIndex] != nil {
		return nil, s.createErrs[callIndex]
	}
	return &asc.AppStoreVersionLocalizationResponse{
		Data: asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			ID: "created-loc",
		},
	}, nil
}

func TestUploadVersionLocalizations_SkipsExactExistingValues(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			{
				ID: "existing-loc",
				Attributes: asc.AppStoreVersionLocalizationAttributes{
					Locale:      "en-US",
					Description: "Existing description",
					Keywords:    "one,two",
				},
			},
		}},
	}

	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"en-US": {
			"description": "Existing description",
			"keywords":    "one,two",
		},
	}, false)
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if len(client.updateCalls) != 0 || len(client.createCalls) != 0 {
		t.Fatalf("expected exact state to skip mutation, creates=%d updates=%d", len(client.createCalls), len(client.updateCalls))
	}
	if len(results) != 1 || results[0].Action != "skip" || results[0].LocalizationID != "existing-loc" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestUploadVersionLocalizations_SkipsExactExistingValuesOnSecondPage(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "150ms")
	client := &stubVersionLocalizationClient{
		getResps: []*asc.AppStoreVersionLocalizationsResponse{
			{
				Data:  []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{{ID: "first-page", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "de-DE"}}},
				Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-id/appStoreVersionLocalizations?cursor=page-2"},
			},
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{{
				ID: "existing-loc",
				Attributes: asc.AppStoreVersionLocalizationAttributes{
					Locale:      "en-US",
					Description: "Existing description",
				},
			}}},
		},
	}
	client.onGet = func(ctx context.Context, call int) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < 100*time.Millisecond {
			t.Errorf("page %d did not receive a fresh timeout: %s", call, time.Until(deadline))
		}
		if call == 1 {
			time.Sleep(75 * time.Millisecond)
		}
	}

	var results []asc.LocalizationUploadLocaleResult
	var err error
	synctest.Test(t, func(*testing.T) {
		results, err = DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
			"en-US": {"description": "Existing description"},
		}, false)
	})
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if client.getCalls != 2 || len(client.updateCalls) != 0 || len(client.createCalls) != 0 {
		t.Fatalf("expected two-page skip without mutation, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
	}
	if len(results) != 1 || results[0].Action != "skip" || results[0].LocalizationID != "existing-loc" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestUploadAppInfoLocalizations_SkipsExactExistingValuesOnSecondPage(t *testing.T) {
	client := &stubAppInfoLocalizationClient{getResps: []*asc.AppInfoLocalizationsResponse{
		{
			Data:  []asc.Resource[asc.AppInfoLocalizationAttributes]{{ID: "first-page", Attributes: asc.AppInfoLocalizationAttributes{Locale: "de-DE"}}},
			Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-id/appInfoLocalizations?cursor=page-2"},
		},
		{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{{
			ID:         "loc-id",
			Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "Existing", Subtitle: "Subtitle"},
		}}},
	}}
	results, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", map[string]map[string]string{
		"en-US": {"name": "Existing", "subtitle": "Subtitle"},
	}, false)
	if err != nil || client.getCalls != 2 || len(results) != 1 || results[0].Action != "skip" || len(client.updateCalls) != 0 || len(client.createCalls) != 0 {
		t.Fatalf("expected page-two app-info skip, results=%+v gets=%d creates=%d updates=%d err=%v", results, client.getCalls, len(client.createCalls), len(client.updateCalls), err)
	}
}

func TestUploadVersionLocalizations_StopsPaginationWhenParentCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &stubVersionLocalizationClient{getResps: []*asc.AppStoreVersionLocalizationsResponse{{
		Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-id/appStoreVersionLocalizations?cursor=page-2"},
	}}}
	client.onGet = func(_ context.Context, call int) {
		if call == 1 {
			cancel()
		}
	}

	_, err := DefaultDeps().UploadVersionLocalizations(ctx, client, "version-id", map[string]map[string]string{
		"en-US": {"description": "English"},
	}, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected parent cancellation, got %v", err)
	}
	if client.getCalls != 1 || len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
		t.Fatalf("expected pagination to stop before page two, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
	}
}

func TestUploadAppInfoLocalizations_ReturnsSecondPageError(t *testing.T) {
	pageErr := errors.New("page two failed")
	client := &stubAppInfoLocalizationClient{
		getResps: []*asc.AppInfoLocalizationsResponse{{
			Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-id/appInfoLocalizations?cursor=page-2"},
		}},
		getErrs: []error{nil, pageErr},
	}
	_, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", map[string]map[string]string{
		"en-US": {"name": "English"},
	}, false)
	if !errors.Is(err, pageErr) || !strings.Contains(err.Error(), "page 2") {
		t.Fatalf("expected page-two error, got %v", err)
	}
	if client.getCalls != 2 || len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
		t.Fatalf("expected pagination error before mutation, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
	}
}

func TestUploadVersionLocalizations_RejectsNilPaginationPage(t *testing.T) {
	tests := []struct {
		name      string
		responses []*asc.AppStoreVersionLocalizationsResponse
		wantReads int
	}{
		{name: "first page", responses: []*asc.AppStoreVersionLocalizationsResponse{nil}, wantReads: 1},
		{
			name: "later page",
			responses: []*asc.AppStoreVersionLocalizationsResponse{
				{Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-id/appStoreVersionLocalizations?cursor=page-2"}},
				nil,
			},
			wantReads: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &stubVersionLocalizationClient{getResps: test.responses}
			_, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
				"en-US": {"description": "English"},
			}, false)
			if err == nil || !strings.Contains(err.Error(), "empty version localization response") {
				t.Fatalf("expected empty page error, got %v", err)
			}
			if client.getCalls != test.wantReads || len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
				t.Fatalf("expected no mutation after empty page, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
			}
		})
	}
}

func TestUploadAppInfoLocalizations_RejectsNilPaginationPage(t *testing.T) {
	tests := []struct {
		name      string
		responses []*asc.AppInfoLocalizationsResponse
		wantReads int
	}{
		{name: "first page", responses: []*asc.AppInfoLocalizationsResponse{nil}, wantReads: 1},
		{
			name: "later page",
			responses: []*asc.AppInfoLocalizationsResponse{
				{Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-id/appInfoLocalizations?cursor=page-2"}},
				nil,
			},
			wantReads: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &stubAppInfoLocalizationClient{getResps: test.responses}
			_, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", map[string]map[string]string{
				"en-US": {"name": "English"},
			}, false)
			if err == nil || !strings.Contains(err.Error(), "empty app-info localization response") {
				t.Fatalf("expected empty page error, got %v", err)
			}
			if client.getCalls != test.wantReads || len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
				t.Fatalf("expected no mutation after empty page, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
			}
		})
	}
}

func TestUploadVersionLocalizations_ReconcilesAmbiguousCreateOnSecondReadbackPage(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	client := &stubVersionLocalizationClient{
		getResps: []*asc.AppStoreVersionLocalizationsResponse{
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{}},
			{
				Data:  []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{{ID: "first-page", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "de-DE"}}},
				Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-id/appStoreVersionLocalizations?cursor=page-2"},
			},
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{{
				ID:         "created-loc",
				Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "fr-FR", Description: "French"},
			}}},
		},
		createErrs: []error{&asc.RetryableError{Err: errors.New("ambiguous create")}},
	}
	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"fr-FR": {"description": "French"},
	}, false)
	if err != nil || len(results) != 1 || results[0].Action != "reconcile" || len(client.createCalls) != 1 || client.getCalls != 3 {
		t.Fatalf("unexpected page-two version recovery: results=%+v creates=%d reads=%d err=%v", results, len(client.createCalls), client.getCalls, err)
	}
}

func TestUploadAppInfoLocalizations_ReconcilesAmbiguousCreateOnSecondReadbackPage(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	client := &stubAppInfoLocalizationClient{
		getResps: []*asc.AppInfoLocalizationsResponse{
			{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{}},
			{
				Data:  []asc.Resource[asc.AppInfoLocalizationAttributes]{{ID: "first-page", Attributes: asc.AppInfoLocalizationAttributes{Locale: "de-DE"}}},
				Links: asc.Links{Next: "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-id/appInfoLocalizations?cursor=page-2"},
			},
			{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{{
				ID:         "created-id",
				Attributes: asc.AppInfoLocalizationAttributes{Locale: "fr-FR", Name: "French"},
			}}},
		},
		createErrs: []error{&asc.RetryableError{Err: errors.New("ambiguous create")}},
	}
	results, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", map[string]map[string]string{
		"fr-FR": {"name": "French"},
	}, false)
	if err != nil || len(results) != 1 || results[0].Action != "reconcile" || len(client.createCalls) != 1 || client.getCalls != 3 {
		t.Fatalf("unexpected page-two app-info recovery: results=%+v creates=%d reads=%d err=%v", results, len(client.createCalls), client.getCalls, err)
	}
}

func TestUploadAppInfoLocalizations_ReconcilesAmbiguousUpdate(t *testing.T) {
	client := &stubAppInfoLocalizationClient{
		getResps: []*asc.AppInfoLocalizationsResponse{
			{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{{ID: "loc-id", Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "Old"}}}},
			{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{{ID: "loc-id", Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "New"}}}},
		},
		updateErrs: []error{&asc.RetryableError{Err: errors.New("ambiguous update")}},
	}
	results, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", map[string]map[string]string{
		"en-US": {"name": "New"},
	}, false)
	if err != nil || len(results) != 1 || results[0].Action != "reconcile" || len(client.updateCalls) != 1 || client.getCalls != 2 {
		t.Fatalf("unexpected app-info update recovery: results=%+v updates=%d reads=%d err=%v", results, len(client.updateCalls), client.getCalls, err)
	}
}

func TestUploadVersionLocalizations_ReconcilesAmbiguousUpdateWithoutReplay(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResps: []*asc.AppStoreVersionLocalizationsResponse{
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old description"}},
			}},
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "New description"}},
			}},
		},
		updateErrs: []error{&asc.RetryableError{Err: errors.New("ambiguous timeout")}},
	}

	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"en-US": {"description": "New description"},
	}, false)
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if len(client.updateCalls) != 1 {
		t.Fatalf("expected one update before readback, got %d", len(client.updateCalls))
	}
	if client.getCalls != 2 {
		t.Fatalf("expected initial read and reconciliation read, got %d", client.getCalls)
	}
	if len(results) != 1 || results[0].Action != "reconcile" || results[0].LocalizationID != "existing-loc" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestUploadVersionLocalizations_UsesFreshContextForReadbackAfterRequestTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "20ms")
	client := &expiringVersionLocalizationClient{}
	var results []asc.LocalizationUploadLocaleResult
	var err error
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		results, err = DefaultDeps().UploadVersionLocalizations(ctx, client, "version-id", map[string]map[string]string{
			"en-US": {"description": "New description"},
		}, false)
	})
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if client.updateCalls != 1 || client.getCalls != 2 {
		t.Fatalf("expected one timed-out update and a fresh readback, updates=%d reads=%d", client.updateCalls, client.getCalls)
	}
	if len(results) != 1 || results[0].Action != "reconcile" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestUploadVersionLocalizations_RetriesAfterNegativeReadback(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	client := &stubVersionLocalizationClient{
		getResps: []*asc.AppStoreVersionLocalizationsResponse{
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old description"}},
			}},
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old description"}},
			}},
			{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old description"}},
			}},
		},
		updateErrs: []error{&asc.RetryableError{Err: errors.New("temporary failure")}},
	}

	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"en-US": {"description": "New description"},
	}, false)
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if len(client.updateCalls) != 2 || client.getCalls != 3 {
		t.Fatalf("expected negative readback before one replay, updates=%d reads=%d", len(client.updateCalls), client.getCalls)
	}
	if len(results) != 1 || results[0].Action != "update" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestUploadVersionLocalizations_ContinuesAfterLocaleFailure(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			{ID: "en-id", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old English"}},
			{ID: "fr-id", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "fr-FR", Description: "Old French"}},
		}},
		updateErrs: []error{errors.New("english update rejected")},
	}

	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"en-US": {"description": "New English"},
		"fr-FR": {"description": "New French"},
	}, false)
	if err == nil {
		t.Fatal("expected partial batch error")
	}
	if len(results) != 2 || len(client.updateCalls) != 2 {
		t.Fatalf("expected both locales to be attempted, results=%+v updates=%d", results, len(client.updateCalls))
	}
	if results[0].Locale != "en-US" || results[0].Status != "failed" || results[0].Error == "" || results[0].DesiredValues["description"] != "New English" {
		t.Fatalf("unexpected failed result: %+v", results[0])
	}
	if results[1].Locale != "fr-FR" || results[1].Status != "succeeded" || results[1].Error != "" {
		t.Fatalf("unexpected successful result: %+v", results[1])
	}
}

func TestUploadVersionLocalizations_PreflightsAllLocalesBeforeFetch(t *testing.T) {
	client := &stubVersionLocalizationClient{}
	_, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"en-US": {"description": "Valid"},
		"fr-FR": {},
	}, false)
	if err == nil || !strings.Contains(err.Error(), `no localization values for locale "fr-FR"`) {
		t.Fatalf("expected empty-locale validation error, got %v", err)
	}
	if client.getCalls != 0 || len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
		t.Fatalf("expected no requests before whole-batch validation, gets=%d creates=%d updates=%d", client.getCalls, len(client.createCalls), len(client.updateCalls))
	}
}

func TestUploadVersionLocalizations_PreflightsEmptyCreateAfterSnapshot(t *testing.T) {
	client := &stubVersionLocalizationClient{getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{}}}
	_, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", map[string]map[string]string{
		"fr-FR": {"promotionalText": ""},
	}, false)
	if err == nil || !strings.Contains(err.Error(), `cannot create version localization "fr-FR"`) {
		t.Fatalf("expected remote-aware create validation, got %v", err)
	}
	if client.getCalls != 1 || len(client.createCalls) != 0 {
		t.Fatalf("expected one snapshot and no mutation, gets=%d creates=%d", client.getCalls, len(client.createCalls))
	}
}

func TestReadLocalizationStringsCanonicalizesAndRejectsDuplicateLocales(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"en-US.strings", "en_us.strings"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("\"description\" = \"Value\";\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	_, err := ReadLocalizationStrings(dir, nil)
	if err == nil || !strings.Contains(err.Error(), `duplicate canonical locale "en-US"`) {
		t.Fatalf("expected canonical duplicate error, got %v", err)
	}
}

func TestReadLocalizationStringsRejectsMalformedLocale(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.strings"), []byte("\"description\" = \"Value\";\n"), 0o644); err != nil {
		t.Fatalf("write malformed locale: %v", err)
	}
	_, err := ReadLocalizationStrings(dir, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid locale") {
		t.Fatalf("expected malformed locale error, got %v", err)
	}
}

func TestUploadVersionLocalizations_PreservesExplicitEmptyUpdateField(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			{ID: "loc-id", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old", PromotionalText: "Old promo"}},
		}},
	}
	values := map[string]map[string]string{
		"en-US": {"promotionalText": ""},
	}
	results, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", values, false)
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if len(results) != 1 || len(client.updateFieldsCalls) != 1 {
		t.Fatalf("unexpected update result: results=%+v fields=%+v", results, client.updateFieldsCalls)
	}
	if value, ok := client.updateFieldsCalls[0]["promotionalText"]; !ok || value != "" {
		t.Fatalf("expected explicit empty promotionalText in update fields: %+v", client.updateFieldsCalls[0])
	}

	client = &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			{ID: "loc-id", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US", Description: "Old", PromotionalText: ""}},
		}},
	}
	results, err = DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", values, false)
	if err != nil || len(results) != 1 || results[0].Action != "skip" || len(client.updateFieldsCalls) != 0 {
		t.Fatalf("expected identical rerun skip, results=%+v fields=%+v err=%v", results, client.updateFieldsCalls, err)
	}
}

func TestUploadAppInfoLocalizations_ClearsOnlySuppliedFieldAndRerunSkips(t *testing.T) {
	values := map[string]map[string]string{"en-US": {"subtitle": ""}}
	client := &stubAppInfoLocalizationClient{getResp: &asc.AppInfoLocalizationsResponse{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{
		{ID: "loc-id", Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "Name", Subtitle: "Old subtitle"}},
	}}}
	results, err := UploadAppInfoLocalizations(context.Background(), client, "app-info-id", values, false)
	if err != nil || len(results) != 1 || len(client.updateCalls) != 1 {
		t.Fatalf("expected clear-only app-info update, results=%+v updates=%+v err=%v", results, client.updateCalls, err)
	}
	if value, ok := client.updateCalls[0]["subtitle"]; !ok || value != "" {
		t.Fatalf("expected explicit empty subtitle field: %+v", client.updateCalls[0])
	}

	client = &stubAppInfoLocalizationClient{getResp: &asc.AppInfoLocalizationsResponse{Data: []asc.Resource[asc.AppInfoLocalizationAttributes]{
		{ID: "loc-id", Attributes: asc.AppInfoLocalizationAttributes{Locale: "en-US", Name: "Name", Subtitle: ""}},
	}}}
	results, err = UploadAppInfoLocalizations(context.Background(), client, "app-info-id", values, false)
	if err != nil || len(results) != 1 || results[0].Action != "skip" || len(client.updateCalls) != 0 {
		t.Fatalf("expected clear-only app-info rerun skip, results=%+v updates=%+v err=%v", results, client.updateCalls, err)
	}
}

func TestFinalizeLocalizationUploadResultWritesResumableArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	result := &asc.LocalizationUploadResult{
		Type:      LocalizationTypeVersion,
		VersionID: "version-id",
		InputPath: "localizations",
		Results: []asc.LocalizationUploadLocaleResult{
			{Locale: "en-US", Action: "update", Status: "failed", LocalizationID: "loc-id", Error: "timeout", DesiredValues: map[string]string{"description": "Desired"}},
			{Locale: "fr-FR", Action: "skip", Status: "succeeded", LocalizationID: "fr-id"},
		},
	}

	FinalizeLocalizationUploadResult(result, "localizations upload")
	if result.Total != 2 || result.Succeeded != 1 || result.Failed != 1 || result.FailureArtifactPath == "" || result.FailureArtifactError != "" {
		t.Fatalf("unexpected finalized result: %+v", result)
	}
	payload, err := os.ReadFile(result.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var artifact localizationUploadFailureArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if artifact.SchemaVersion != 1 || artifact.InputPath != "localizations" || len(artifact.Results) != 1 || artifact.Results[0].DesiredValues["description"] != "Desired" {
		t.Fatalf("artifact is not resumable: %+v", artifact)
	}
}

func TestFinalizeLocalizationUploadResultRetainsArtifactWriteError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	result := &asc.LocalizationUploadResult{
		Results: []asc.LocalizationUploadLocaleResult{{Locale: "en-US", Action: "create", Status: "failed", Error: "timeout"}},
	}

	FinalizeLocalizationUploadResult(result, "localizations upload")
	if result.Total != 1 || result.Failed != 1 || result.FailureArtifactPath != "" || result.FailureArtifactError == "" {
		t.Fatalf("expected artifact write error in summary: %+v", result)
	}
}

func TestRunLocalizationMutationWithReadbackRetriesChildDeadline(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	mutations := 0
	readbacks := 0

	id, reconciled, err := runLocalizationMutationWithReadback(
		context.Background(),
		func(context.Context) (string, error) {
			mutations++
			if mutations == 1 {
				return "", context.DeadlineExceeded
			}
			return "existing-loc", nil
		},
		func(context.Context) (string, bool, error) {
			readbacks++
			return "", false, nil
		},
	)
	if err != nil {
		t.Fatalf("runLocalizationMutationWithReadback() error: %v", err)
	}
	if id != "existing-loc" || reconciled || mutations != 2 || readbacks != 2 {
		t.Fatalf("unexpected recovery: id=%q reconciled=%t mutations=%d readbacks=%d", id, reconciled, mutations, readbacks)
	}
}

func TestRunLocalizationMutationWithReadbackStopsOnParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mutations := 0
	readbacks := 0

	_, _, err := runLocalizationMutationWithReadback(
		ctx,
		func(context.Context) (string, error) {
			mutations++
			return "", context.DeadlineExceeded
		},
		func(context.Context) (string, bool, error) {
			readbacks++
			return "", false, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if mutations != 0 || readbacks != 0 {
		t.Fatalf("expected no requests after parent cancellation, mutations=%d readbacks=%d", mutations, readbacks)
	}
}

func (s *stubVersionLocalizationClient) UpdateAppStoreVersionLocalizationFields(_ context.Context, _ string, fields map[string]string) (*asc.AppStoreVersionLocalizationResponse, error) {
	s.updateFieldsCalls = append(s.updateFieldsCalls, cloneLocalizationValues(fields))
	s.updateCalls = append(s.updateCalls, buildVersionLocalizationAttributes("", fields, false))
	callIndex := len(s.updateCalls) - 1
	if callIndex < len(s.updateErrs) && s.updateErrs[callIndex] != nil {
		return nil, s.updateErrs[callIndex]
	}
	return &asc.AppStoreVersionLocalizationResponse{
		Data: asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			ID: "existing-loc",
		},
	}, nil
}

func TestUploadVersionLocalizationsWithWarnings_DoesNotWarnForFailedCreate(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp:    &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{}},
		createErrs: []error{errors.New("create rejected")},
	}
	results, warnings, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(
		context.Background(),
		client,
		"version-id",
		map[string]map[string]string{"fr-FR": {"description": "French"}},
		false,
		SubmitReadinessOptions{},
	)
	if err == nil || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("expected failed create result, results=%+v err=%v", results, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("failed create must not emit applied warning: %+v", warnings)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	defer func() {
		os.Stderr = oldStderr
	}()

	os.Stderr = writer
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		_ = reader.Close()
		done <- buf.String()
	}()

	fn()
	_ = writer.Close()

	return <-done
}

func TestUploadVersionLocalizations_RetriesWithoutWhatsNewOnInitialReleaseError(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{
					ID: "existing-loc",
					Attributes: asc.AppStoreVersionLocalizationAttributes{
						Locale: "en-US",
					},
				},
			},
		},
		updateErrs: []error{
			errors.New("An attribute value is not acceptable for the current resource state. The attribute 'whatsNew' is not editable."),
		},
	}

	valuesByLocale := map[string]map[string]string{
		"en-US": {
			"description": "A description",
			"whatsNew":    "Bug fixes and improvements",
		},
	}

	var (
		results []asc.LocalizationUploadLocaleResult
		err     error
	)
	stderr := captureStderr(t, func() {
		results, err = DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", valuesByLocale, false)
	})
	if err != nil {
		t.Fatalf("UploadVersionLocalizations() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(client.updateCalls) != 2 {
		t.Fatalf("expected 2 update attempts, got %d", len(client.updateCalls))
	}
	if got := client.updateCalls[0].WhatsNew; got != "Bug fixes and improvements" {
		t.Fatalf("expected first attempt to include whatsNew, got %q", got)
	}
	if got := client.updateCalls[1].WhatsNew; got != "" {
		t.Fatalf("expected retry attempt to clear whatsNew, got %q", got)
	}
	if !strings.Contains(stderr, "Retrying without it") {
		t.Fatalf("expected retry warning in stderr, got %q", stderr)
	}
}

func TestUploadVersionLocalizations_DoesNotRetryWhenErrorIsUnrelated(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{
					ID: "existing-loc",
					Attributes: asc.AppStoreVersionLocalizationAttributes{
						Locale: "en-US",
					},
				},
			},
		},
		updateErrs: []error{errors.New("network timeout")},
	}

	valuesByLocale := map[string]map[string]string{
		"en-US": {
			"description": "A description",
			"whatsNew":    "Bug fixes and improvements",
		},
	}

	_, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", valuesByLocale, false)
	if err == nil {
		t.Fatal("expected upload error")
	}
	if len(client.updateCalls) != 1 {
		t.Fatalf("expected a single update attempt, got %d", len(client.updateCalls))
	}
}

func TestUploadVersionLocalizations_DoesNotRetryWhenWhatsNewIsEmpty(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{
					ID: "existing-loc",
					Attributes: asc.AppStoreVersionLocalizationAttributes{
						Locale: "en-US",
					},
				},
			},
		},
		updateErrs: []error{
			errors.New("The attribute 'whatsNew' is not editable for this version"),
		},
	}

	valuesByLocale := map[string]map[string]string{
		"en-US": {
			"description": "A description",
		},
	}

	_, err := DefaultDeps().UploadVersionLocalizations(context.Background(), client, "version-id", valuesByLocale, false)
	if err == nil {
		t.Fatal("expected upload error")
	}
	if len(client.updateCalls) != 1 {
		t.Fatalf("expected a single update attempt, got %d", len(client.updateCalls))
	}
}

func TestUploadVersionLocalizationsWithWarnings_RejectsOverLimitKeywordCharactersBeforeFetch(t *testing.T) {
	client := &stubVersionLocalizationClient{}

	valuesByLocale := map[string]map[string]string{
		"ja": {
			"description": "日本語説明",
			"keywords":    strings.Repeat("語", 101),
		},
	}

	_, _, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(context.Background(), client, "version-id", valuesByLocale, true, SubmitReadinessOptions{})
	if err == nil {
		t.Fatal("expected upload validation error")
	}
	if !strings.Contains(err.Error(), "keywords exceed 100 characters") {
		t.Fatalf("expected keyword character-limit error, got %v", err)
	}
	if client.getCalls != 0 {
		t.Fatalf("expected no fetch calls, got %d", client.getCalls)
	}
	if len(client.createCalls) != 0 {
		t.Fatalf("expected no create calls, got %d", len(client.createCalls))
	}
	if len(client.updateCalls) != 0 {
		t.Fatalf("expected no update calls, got %d", len(client.updateCalls))
	}
}

func TestUploadVersionLocalizationsWithWarnings_DryRunWarnsOnlyForCreates(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{
					ID: "existing-loc",
					Attributes: asc.AppStoreVersionLocalizationAttributes{
						Locale: "en-US",
					},
				},
			},
		},
	}

	valuesByLocale := map[string]map[string]string{
		"en-US": {
			"description": "Updated description",
			"keywords":    "updated",
			"supportUrl":  "https://example.com/en",
		},
		"ja": {
			"description": "日本語説明",
		},
	}

	results, warnings, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(context.Background(), client, "version-id", valuesByLocale, true, SubmitReadinessOptions{})
	if err != nil {
		t.Fatalf("UploadVersionLocalizationsWithWarnings() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d (%+v)", len(warnings), warnings)
	}
	if warnings[0].Locale != "ja" {
		t.Fatalf("expected warning for ja locale, got %+v", warnings[0])
	}
	if warnings[0].Mode != SubmitReadinessCreateModePlanned {
		t.Fatalf("expected planned warning mode, got %+v", warnings[0])
	}
	if got := strings.Join(warnings[0].MissingFields, ","); got != "keywords,supportUrl" {
		t.Fatalf("unexpected missing fields %q", got)
	}
	if len(client.createCalls) != 0 {
		t.Fatalf("expected dry-run to avoid create calls, got %d", len(client.createCalls))
	}
	if len(client.updateCalls) != 0 {
		t.Fatalf("expected dry-run to avoid update calls, got %d", len(client.updateCalls))
	}
}

func TestUploadVersionLocalizationsWithWarnings_AppliedCompleteCreateDoesNotWarn(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{},
		},
	}

	valuesByLocale := map[string]map[string]string{
		"ja": {
			"description": "日本語説明",
			"keywords":    "一,二",
			"supportUrl":  "https://example.com/ja",
		},
	}

	results, warnings, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(context.Background(), client, "version-id", valuesByLocale, false, SubmitReadinessOptions{})
	if err != nil {
		t.Fatalf("UploadVersionLocalizationsWithWarnings() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if len(client.createCalls) != 1 {
		t.Fatalf("expected one create call, got %d", len(client.createCalls))
	}
}

func TestIsWhatsNewUnsupportedError(t *testing.T) {
	apiErr := &asc.APIError{
		Title:  "ENTITY_ERROR.ATTRIBUTE.INVALID",
		Detail: "The attribute 'whatsNew' cannot be set for this resource state.",
	}
	if !isWhatsNewUnsupportedError(apiErr) {
		t.Fatal("expected API error with whatsNew detail to be recognized")
	}
	if isWhatsNewUnsupportedError(errors.New("timeout")) {
		t.Fatal("did not expect unrelated error to match")
	}
}

func TestUploadVersionLocalizationsWithWarnings_RequiresWhatsNewWhenConfigured(t *testing.T) {
	client := &stubVersionLocalizationClient{
		getResp: &asc.AppStoreVersionLocalizationsResponse{
			Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{},
		},
	}

	valuesByLocale := map[string]map[string]string{
		"ja": {
			"description": "日本語説明",
			"keywords":    "一,二",
			"supportUrl":  "https://example.com/ja",
		},
	}

	results, warnings, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(
		context.Background(),
		client,
		"version-id",
		valuesByLocale,
		true,
		SubmitReadinessOptions{RequireWhatsNew: true},
	)
	if err != nil {
		t.Fatalf("UploadVersionLocalizationsWithWarnings() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %+v", warnings)
	}
	if warnings[0].Locale != "ja" {
		t.Fatalf("expected warning for ja locale, got %+v", warnings[0])
	}
	if len(warnings[0].MissingFields) != 1 || warnings[0].MissingFields[0] != "whatsNew" {
		t.Fatalf("expected missing fields [whatsNew], got %+v", warnings[0].MissingFields)
	}
}
