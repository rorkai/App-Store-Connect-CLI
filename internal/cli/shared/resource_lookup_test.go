package shared

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type iapLookupFixture struct {
	id        string
	productID string
	name      string
}

type subscriptionLookupFixture struct {
	id        string
	productID string
	name      string
}

func TestScopedPaginationURLBindsCollectionPath(t *testing.T) {
	expectedPath := "/v1/apps/123/inAppPurchasesV2"
	tests := []struct {
		name    string
		nextURL string
		wantURL string
		wantErr bool
	}{
		{name: "absolute same collection", nextURL: "https://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next"},
		{name: "uppercase scheme canonicalized", nextURL: "HTTPS://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantURL: "https://api.appstoreconnect.apple.com" + expectedPath + "?cursor=next"},
		{name: "relative same collection", nextURL: expectedPath + "?cursor=next"},
		{name: "different app", nextURL: "https://api.appstoreconnect.apple.com/v1/apps/999/inAppPurchasesV2?cursor=next", wantErr: true},
		{name: "different collection", nextURL: "https://api.appstoreconnect.apple.com/v1/apps/123/subscriptionGroups?cursor=next", wantErr: true},
		{name: "malformed", nextURL: "https://api.appstoreconnect.apple.com/%zz", wantErr: true},
		{name: "protocol relative", nextURL: "//api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "userinfo", nextURL: "https://user:pass@api.appstoreconnect.apple.com" + expectedPath + "?cursor=next", wantErr: true},
		{name: "opaque", nextURL: "https:" + expectedPath + "?cursor=next", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScopedPaginationURL(tt.nextURL, expectedPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected pagination URL rejection, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ScopedPaginationURL() error: %v", err)
			}
			wantURL := tt.wantURL
			if wantURL == "" {
				wantURL = tt.nextURL
			}
			if got != wantURL {
				t.Fatalf("ScopedPaginationURL() = %q, want %q", got, wantURL)
			}
		})
	}
}

type sequenceIAPLookupStub struct {
	responses []*asc.InAppPurchasesV2Response
	errs      []error
	calls     int
	appIDs    []string
}

func (s *sequenceIAPLookupStub) GetInAppPurchasesV2(_ context.Context, appID string, _ ...asc.IAPOption) (*asc.InAppPurchasesV2Response, error) {
	s.calls++
	s.appIDs = append(s.appIDs, appID)
	idx := s.calls - 1
	if len(s.errs) != 0 {
		errIdx := idx
		if errIdx >= len(s.errs) {
			errIdx = len(s.errs) - 1
		}
		if s.errs[errIdx] != nil {
			return nil, s.errs[errIdx]
		}
	}
	if len(s.responses) == 0 {
		return &asc.InAppPurchasesV2Response{}, nil
	}
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	if s.responses[idx] == nil {
		return &asc.InAppPurchasesV2Response{}, nil
	}
	return s.responses[idx], nil
}

type sequenceSubscriptionLookupStub struct {
	groupResponses []*asc.SubscriptionGroupsResponse
	groupErrors    []error
	groupCalls     int
	appIDs         []string

	subscriptionResponses map[string][]*asc.SubscriptionsResponse
	subscriptionCalls     map[string]int
}

func (s *sequenceSubscriptionLookupStub) GetSubscriptionGroups(_ context.Context, appID string, _ ...asc.SubscriptionGroupsOption) (*asc.SubscriptionGroupsResponse, error) {
	s.groupCalls++
	s.appIDs = append(s.appIDs, appID)
	idx := s.groupCalls - 1
	if len(s.groupErrors) != 0 {
		errIdx := idx
		if errIdx >= len(s.groupErrors) {
			errIdx = len(s.groupErrors) - 1
		}
		if s.groupErrors[errIdx] != nil {
			return nil, s.groupErrors[errIdx]
		}
	}
	if len(s.groupResponses) == 0 {
		return &asc.SubscriptionGroupsResponse{}, nil
	}
	if idx >= len(s.groupResponses) {
		idx = len(s.groupResponses) - 1
	}
	if s.groupResponses[idx] == nil {
		return &asc.SubscriptionGroupsResponse{}, nil
	}
	return s.groupResponses[idx], nil
}

func (s *sequenceSubscriptionLookupStub) GetSubscriptions(_ context.Context, groupID string, _ ...asc.SubscriptionsOption) (*asc.SubscriptionsResponse, error) {
	if s.subscriptionCalls == nil {
		s.subscriptionCalls = make(map[string]int)
	}
	s.subscriptionCalls[groupID]++

	responses := s.subscriptionResponses[groupID]
	if len(responses) == 0 {
		return &asc.SubscriptionsResponse{}, nil
	}
	idx := s.subscriptionCalls[groupID] - 1
	if idx >= len(responses) {
		idx = len(responses) - 1
	}
	if responses[idx] == nil {
		return &asc.SubscriptionsResponse{}, nil
	}
	return responses[idx], nil
}

func iapResponse(items ...iapLookupFixture) *asc.InAppPurchasesV2Response {
	resp := &asc.InAppPurchasesV2Response{
		Links: asc.Links{},
		Data:  make([]asc.Resource[asc.InAppPurchaseV2Attributes], 0, len(items)),
	}
	for _, item := range items {
		resp.Data = append(resp.Data, asc.Resource[asc.InAppPurchaseV2Attributes]{
			ID: item.id,
			Attributes: asc.InAppPurchaseV2Attributes{
				ProductID: item.productID,
				Name:      item.name,
			},
		})
	}
	return resp
}

func subscriptionGroupsResponse(groupIDs ...string) *asc.SubscriptionGroupsResponse {
	resp := &asc.SubscriptionGroupsResponse{
		Links: asc.Links{},
		Data:  make([]asc.Resource[asc.SubscriptionGroupAttributes], 0, len(groupIDs)),
	}
	for _, groupID := range groupIDs {
		resp.Data = append(resp.Data, asc.Resource[asc.SubscriptionGroupAttributes]{
			ID: groupID,
		})
	}
	return resp
}

func subscriptionsResponse(items ...subscriptionLookupFixture) *asc.SubscriptionsResponse {
	resp := &asc.SubscriptionsResponse{
		Links: asc.Links{},
		Data:  make([]asc.Resource[asc.SubscriptionAttributes], 0, len(items)),
	}
	for _, item := range items {
		resp.Data = append(resp.Data, asc.Resource[asc.SubscriptionAttributes]{
			ID: item.id,
			Attributes: asc.SubscriptionAttributes{
				ProductID: item.productID,
				Name:      item.name,
			},
		})
	}
	return resp
}

func TestRequireAppForStableSelector_MissingAppContext(t *testing.T) {
	err := DefaultDeps().RequireAppForStableSelector("", "com.example.pro", "--iap-id")
	if err == nil {
		t.Fatal("expected usage error")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp usage error, got %v", err)
	}
}

func TestResolveIAPID_NumericPassthroughSkipsLookup(t *testing.T) {
	stub := &sequenceIAPLookupStub{}

	got, err := ResolveIAPID(context.Background(), stub, "", "123456789")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "123456789" {
		t.Fatalf("expected numeric passthrough, got %q", got)
	}
	if stub.calls != 0 {
		t.Fatalf("expected no lookup calls, got %d", stub.calls)
	}
}

func TestResolveIAPID_ResolvesExactProductID(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(iapLookupFixture{id: "iap-1", productID: "com.example.pro", name: "Pro"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "com.example.pro")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "iap-1" {
		t.Fatalf("expected resolved IAP id iap-1, got %q", got)
	}
}

func TestResolveIAPID_WithAppContextResolvesNumericProductID(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(iapLookupFixture{id: "iap-2024", productID: "2024", name: "Spring Sale"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "2024")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "iap-2024" {
		t.Fatalf("expected resolved IAP id iap-2024, got %q", got)
	}
	if stub.calls == 0 {
		t.Fatal("expected lookup call for numeric selector with app context")
	}
}

func TestResolveIAPID_WithAppContextRejectsNumericResourceOutsideSelectedApp(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(),
		},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected numeric resource outside selected app to fail closed")
	}
	if !errors.Is(err, errSelectorNotFound) {
		t.Fatalf("expected selector-not-found error, got %v", err)
	}
	if !strings.Contains(err.Error(), `in-app purchase resource ID "2024" is not a member of selected app "app-1"`) {
		t.Fatalf("expected selected-app resource-ID diagnostic, got %v", err)
	}
	if stub.calls != 2 {
		t.Fatalf("expected product and full-scan lookups before rejection, got %d calls", stub.calls)
	}
}

func TestResolveIAPID_WithAppContextRejectsNumericResourceWhenLookupErrors(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		errs: []error{errors.New("lookup temporarily unavailable")},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected lookup error to reject numeric resource")
	}
	if stub.calls != 1 {
		t.Fatalf("expected single lookup attempt before rejection, got %d calls", stub.calls)
	}
}

func TestResolveIAPID_WithAppContextResolvesNumericResourceIDAfterPaginatedScan(t *testing.T) {
	firstPage := iapResponse()
	firstPage.Links.Next = "/v1/apps/app-1/inAppPurchasesV2?cursor=next"
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			firstPage,
			iapResponse(iapLookupFixture{id: "2024", productID: "com.example.pro", name: "Pro"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "2024")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "2024" {
		t.Fatalf("expected selected app resource ID, got %q", got)
	}
	if stub.calls != 3 {
		t.Fatalf("expected product and two full-scan pages, got %d calls", stub.calls)
	}
	for _, appID := range stub.appIDs {
		if appID != "app-1" {
			t.Fatalf("expected every IAP membership lookup to use selected app, got %q", appID)
		}
	}
}

func TestResolveIAPID_WithAppContextDoesNotSuppressNumericAmbiguity(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(
				iapLookupFixture{id: "iap-1", productID: "com.example.one", name: "2024"},
				iapLookupFixture{id: "iap-2", productID: "com.example.two", name: "2024"},
			),
		},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
	if !strings.Contains(err.Error(), "pass --iap-id with one of:") {
		t.Fatalf("expected disambiguation guidance, got %v", err)
	}
}

func TestResolveIAPID_FallsBackToFullScanForExactName(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(iapLookupFixture{id: "iap-fuzzy", productID: "com.example.fuzzy", name: "Premium Plus"}),
			iapResponse(iapLookupFixture{id: "iap-exact", productID: "com.example.exact", name: "Premium"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "Premium")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "iap-exact" {
		t.Fatalf("expected fallback exact-name IAP id iap-exact, got %q", got)
	}
	if stub.calls != 3 {
		t.Fatalf("expected product, name, and full-scan lookups, got %d calls", stub.calls)
	}
}

func TestResolveIAPID_AmbiguousExactNameFails(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(
				iapLookupFixture{id: "iap-1", productID: "com.example.one", name: "Pro"},
				iapLookupFixture{id: "iap-2", productID: "com.example.two", name: "Pro"},
			),
		},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", "Pro")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !strings.Contains(err.Error(), "pass --iap-id with one of:") {
		t.Fatalf("expected disambiguation guidance, got %v", err)
	}
}

func TestResolveIAPID_AmbiguousSelectorBoundsProviderText(t *testing.T) {
	recoveryID := "iap-recovery-id-" + strings.Repeat("9", AmbiguousDiagnosticTextLimit+32)
	providerName := strings.Repeat("名", AmbiguousDiagnosticTextLimit) + "-iap-name-tail"
	providerProductID := strings.Repeat("界", AmbiguousDiagnosticTextLimit) + "-iap-product-tail\x1b[31m"
	invalidUTF8 := string([]byte{'r', 'e', 'f', 0xff, 'e', 'r', 'e', 'n', 'c', 'e'})
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(
				iapLookupFixture{id: recoveryID, productID: providerProductID, name: providerName},
				iapLookupFixture{id: "iap-2", productID: invalidUTF8, name: providerName},
			),
		},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", providerName)
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	if strings.Contains(message, recoveryID) {
		t.Fatalf("displayed recovery ID should be bounded: %q", message)
	}
	if !strings.Contains(message, recoveryID[:len("iap-recovery-id-")]) {
		t.Fatalf("bounded recovery ID should retain useful context: %q", message)
	}
	for _, tail := range []string{"-iap-name-tail", "-iap-product-tail", "\x1b"} {
		if strings.Contains(message, tail) {
			t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
		}
	}
	if !strings.Contains(message, "ref�erence") {
		t.Fatalf("invalid provider UTF-8 should be replaced while retaining context: %q", message)
	}
	var structured *AmbiguousSelectionError
	if !errors.As(err, &structured) || len(structured.Candidates) != 2 {
		t.Fatalf("expected structured ambiguity candidates, got %#v", structured)
	}
	if structured.Candidates[0].ID != recoveryID || structured.Candidates[0].Label != providerProductID || structured.Candidates[0].Extra != providerName {
		t.Fatalf("structured ambiguity must retain exact IAP values: %#v", structured.Candidates[0])
	}
	if structured.Description != fmt.Sprintf("%q by name", providerName) || structured.Flag != "--iap-id" || structured.DisplayTextLimit != AmbiguousDiagnosticTextLimit {
		t.Fatalf("structured ambiguity lost IAP selector semantics: %#v", structured)
	}
}

func TestResolveSubscriptionID_DedupesAcrossGroups(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1", "group-2"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {subscriptionsResponse(subscriptionLookupFixture{id: "sub-1", productID: "com.example.monthly", name: "Monthly"})},
			"group-2": {subscriptionsResponse(subscriptionLookupFixture{id: "sub-1", productID: "com.example.monthly", name: "Monthly"})},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "com.example.monthly")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-1" {
		t.Fatalf("expected deduped subscription id sub-1, got %q", got)
	}
}

func TestResolveSubscriptionID_FallsBackToFullScanForExactName(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-fuzzy", productID: "com.example.plus", name: "Premium Plus"}),
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-exact", productID: "com.example.premium", name: "Premium"}),
			},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "Premium")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-exact" {
		t.Fatalf("expected fallback exact-name subscription id sub-exact, got %q", got)
	}
}

func TestResolveSubscriptionID_WithAppContextResolvesNumericName(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-2024", productID: "com.example.annual", name: "2024"}),
			},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-2024" {
		t.Fatalf("expected resolved subscription id sub-2024, got %q", got)
	}
	if stub.groupCalls == 0 {
		t.Fatal("expected group lookup for numeric selector with app context")
	}
}

func TestResolveSubscriptionID_WithAppContextResolvesNumericProductID(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-2024", productID: "2024", name: "Spring Sale"}),
			},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-2024" {
		t.Fatalf("expected resolved subscription ID sub-2024, got %q", got)
	}
}

func TestResolveSubscriptionID_WithAppContextRejectsNumericResourceOutsideSelectedApp(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(),
				subscriptionsResponse(),
			},
		},
	}

	_, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected numeric resource outside selected app to fail closed")
	}
	if !errors.Is(err, errSelectorNotFound) {
		t.Fatalf("expected selector-not-found error, got %v", err)
	}
	if !strings.Contains(err.Error(), `subscription resource ID "2024" is not a member of selected app "app-1"`) {
		t.Fatalf("expected selected-app resource-ID diagnostic, got %v", err)
	}
	if stub.groupCalls == 0 {
		t.Fatal("expected lookup attempt before rejection")
	}
}

func TestResolveSubscriptionID_WithAppContextRejectsNumericResourceWhenLookupErrors(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupErrors: []error{errors.New("lookup temporarily unavailable")},
	}

	_, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected lookup error to reject numeric resource")
	}
	if stub.groupCalls != 1 {
		t.Fatalf("expected single lookup attempt before rejection, got %d", stub.groupCalls)
	}
}

func TestResolveSubscriptionID_WithAppContextResolvesNumericResourceIDAfterPaginatedScan(t *testing.T) {
	firstGroupPage := subscriptionGroupsResponse("group-1")
	firstGroupPage.Links.Next = "/v1/apps/app-1/subscriptionGroups?cursor=next"
	firstPage := subscriptionsResponse()
	firstPage.Links.Next = "/v1/subscriptionGroups/group-1/subscriptions?cursor=next"
	secondGroupPage := subscriptionGroupsResponse("group-2")
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{firstGroupPage, secondGroupPage, firstGroupPage, secondGroupPage},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				firstPage,
				subscriptionsResponse(subscriptionLookupFixture{id: "2024", productID: "com.example.pro", name: "Pro"}),
			},
			"group-2": {subscriptionsResponse()},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "2024" {
		t.Fatalf("expected selected app resource ID, got %q", got)
	}
	if stub.groupCalls != 4 {
		t.Fatalf("expected two same-app subscription group pages per membership scan, got %d calls", stub.groupCalls)
	}
	for _, appID := range stub.appIDs {
		if appID != "app-1" {
			t.Fatalf("expected every subscription group lookup to use selected app, got %q", appID)
		}
	}
}

func TestResolveSubscriptionID_WithAppContextDoesNotSuppressNumericAmbiguity(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(
					subscriptionLookupFixture{id: "sub-1", productID: "com.example.one", name: "2024"},
					subscriptionLookupFixture{id: "sub-2", productID: "com.example.two", name: "2024"},
				),
			},
		},
	}

	_, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "2024")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
	if !strings.Contains(err.Error(), "pass --subscription-id with one of:") {
		t.Fatalf("expected disambiguation guidance, got %v", err)
	}
}

func TestResolveSubscriptionID_AmbiguousSelectorBoundsProviderText(t *testing.T) {
	recoveryID := "sub-recovery-id-" + strings.Repeat("9", AmbiguousDiagnosticTextLimit+32)
	providerName := strings.Repeat("名", AmbiguousDiagnosticTextLimit) + "-subscription-name-tail"
	providerProductID := strings.Repeat("界", AmbiguousDiagnosticTextLimit) + "-subscription-product-tail\x1b[31m"
	invalidUTF8 := string([]byte{'r', 'e', 'f', 0xff, 'e', 'r', 'e', 'n', 'c', 'e'})
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(
					subscriptionLookupFixture{id: recoveryID, productID: providerProductID, name: providerName},
					subscriptionLookupFixture{id: "sub-2", productID: invalidUTF8, name: providerName},
				),
			},
		},
	}

	_, err := ResolveSubscriptionID(context.Background(), stub, "app-1", providerName)
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	if strings.Contains(message, recoveryID) {
		t.Fatalf("displayed recovery ID should be bounded: %q", message)
	}
	if !strings.Contains(message, recoveryID[:len("sub-recovery-id-")]) {
		t.Fatalf("bounded recovery ID should retain useful context: %q", message)
	}
	for _, tail := range []string{"-subscription-name-tail", "-subscription-product-tail", "\x1b"} {
		if strings.Contains(message, tail) {
			t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
		}
	}
	if !strings.Contains(message, "ref�erence") {
		t.Fatalf("invalid provider UTF-8 should be replaced while retaining context: %q", message)
	}
	var structured *AmbiguousSelectionError
	if !errors.As(err, &structured) || len(structured.Candidates) != 2 {
		t.Fatalf("expected structured ambiguity candidates, got %#v", structured)
	}
	if structured.Candidates[0].ID != recoveryID || structured.Candidates[0].Label != providerProductID || structured.Candidates[0].Extra != providerName {
		t.Fatalf("structured ambiguity must retain exact subscription values: %#v", structured.Candidates[0])
	}
	if structured.Description != fmt.Sprintf("%q by name", providerName) || structured.Flag != "--subscription-id" || structured.DisplayTextLimit != AmbiguousDiagnosticTextLimit {
		t.Fatalf("structured ambiguity lost subscription selector semantics: %#v", structured)
	}
}

func TestWithSelectorFlagRenamesAmbiguousSelectorFlag(t *testing.T) {
	_, err := ResolveExactSelectorCandidate("Pro", "subscription", []ExactSelectorCandidate{
		{ID: "sub-1", ProductID: "pro.monthly", Name: "Pro"},
		{ID: "sub-2", ProductID: "pro.yearly", Name: "Pro"},
	})
	if !errors.Is(err, errSelectorAmbiguous) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
	if !strings.Contains(err.Error(), "pass --subscription-id with one of:") {
		t.Fatalf("expected default flag, got %v", err)
	}

	renamed := WithSelectorFlag(err, "--source-subscription-id")
	if !errors.Is(renamed, errSelectorAmbiguous) {
		t.Fatalf("renamed error must stay an ambiguous selector error, got %v", renamed)
	}
	if !strings.Contains(renamed.Error(), "pass --source-subscription-id with one of:") || !strings.Contains(renamed.Error(), "sub-1  pro.monthly  Pro") {
		t.Fatalf("expected renamed flag and candidates, got %v", renamed)
	}

	other := errors.New("boom")
	if got := WithSelectorFlag(other, "--x"); !errors.Is(got, other) {
		t.Fatalf("non-selector errors must pass through, got %v", got)
	}
}
