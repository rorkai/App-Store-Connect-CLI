package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
)

// PaginateFunc is a function that fetches a page of results
type PaginateFunc func(ctx context.Context, nextURL string) (PaginatedResponse, error)

// RequestContextFunc creates a fresh context for one outbound request while
// retaining the caller's parent context and cancellation.
type RequestContextFunc func(context.Context) (context.Context, context.CancelFunc)

func requestContextFor(parent context.Context, factory RequestContextFunc) (context.Context, context.CancelFunc) {
	if factory == nil {
		return parent, func() {}
	}
	return factory(parent)
}

// PageConsumer handles one pagination page.
type PageConsumer func(page PaginatedResponse) error

// PaginateAll fetches all pages and aggregates results.
// It uses reflection to create an empty result container of the same type as
// firstPage, eliminating the need for a type switch per response type.
func PaginateAll(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc) (PaginatedResponse, error) {
	if firstPage == nil {
		return nil, nil
	}

	// Check for typed nil (non-nil interface containing a nil value).
	// Return an empty result of the same type rather than panicking.
	if isNilPaginatedResponse(firstPage) {
		return newEmptyPaginatedResponse(firstPage)
	}

	// Create an empty result of the same concrete type using reflection.
	result, err := newEmptyPaginatedResponse(firstPage)
	if err != nil {
		return nil, err
	}
	if err := initializeAggregatedResponse(result, firstPage); err != nil {
		return nil, err
	}

	page := 1
	seenNext := make(map[string]struct{})
	included := &rawJSONArrayAccumulator{}
	// pageErr records a pagination failure that still yields a partial result,
	// so the accumulated included array is written before returning.
	var pageErr error
	for {
		// Aggregate data from current page using reflection over the Data field.
		if err := aggregatePageData(result, firstPage, included); err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		if page > 1 {
			if err := clearPageLocalContext(result); err != nil {
				return nil, fmt.Errorf("page %d: %w", page, err)
			}
		}

		links := firstPage.GetLinks()
		if links == nil || links.Next == "" {
			break
		}

		nextURL := links.Next
		nextIdentity := PaginationURLIdentity(nextURL)
		if _, ok := seenNext[nextIdentity]; ok {
			pageErr = fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
			break
		}
		seenNext[nextIdentity] = struct{}{}
		page++

		if fetchNext == nil {
			pageErr = fmt.Errorf("page %d: %w", page, ErrMissingPaginationFetcher)
			break
		}

		// Fetch next page
		nextPage, err := fetchNext(ctx, nextURL)
		if err != nil {
			pageErr = fmt.Errorf("page %d: %w", page, err)
			break
		}
		if isNilPaginatedResponse(nextPage) {
			pageErr = fmt.Errorf("page %d: %w", page, ErrNilPaginationPage)
			break
		}

		// Validate that the response type matches
		if reflect.TypeOf(nextPage) != reflect.TypeOf(firstPage) {
			pageErr = fmt.Errorf("page %d: unexpected response type (expected %T, got %T)", page, firstPage, nextPage)
			break
		}

		firstPage = nextPage
	}

	// Write the merged included array once, after every page has been collected.
	if err := setJSONRawArrayField(result, includedFieldName, included); err != nil {
		if pageErr != nil {
			return result, pageErr
		}
		return nil, err
	}
	if pageErr != nil {
		return result, pageErr
	}
	if links := result.GetLinks(); links != nil {
		links.Next = ""
	}

	return result, nil
}

// PaginateEach iterates pages and invokes consume for each page without
// aggregating all page data in memory.
func PaginateEach(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc, consume PageConsumer) error {
	return paginateEach(ctx, firstPage, fetchNext, consume, 0)
}

// PaginateEachWithMaxPages iterates pages and invokes consume for each page,
// stopping before it would fetch a page beyond maxPages. A positive limit is
// required; use PaginateEach when the caller intentionally has no page cap.
func PaginateEachWithMaxPages(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc, consume PageConsumer, maxPages int) error {
	if maxPages <= 0 {
		return fmt.Errorf("max pages must be greater than zero")
	}
	return paginateEach(ctx, firstPage, fetchNext, consume, maxPages)
}

func paginateEach(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc, consume PageConsumer, maxPages int) error {
	if firstPage == nil {
		return nil
	}
	if consume == nil {
		return fmt.Errorf("page consumer is required")
	}

	// Handle typed nil (non-nil interface containing a nil value).
	if isNilPaginatedResponse(firstPage) {
		return nil
	}

	page := 1
	current := firstPage
	seenNext := make(map[string]struct{})

	for {
		// Reject a missing fetcher before invoking the consumer when this page
		// already advertises another page. Consumers may perform side effects.
		preflightLinks := current.GetLinks()
		if preflightLinks != nil && preflightLinks.Next != "" && fetchNext == nil {
			return fmt.Errorf("page %d: %w", page+1, ErrMissingPaginationFetcher)
		}

		if err := consume(current); err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}

		links := current.GetLinks()
		if links == nil || links.Next == "" {
			return nil
		}
		if maxPages > 0 && page >= maxPages {
			return fmt.Errorf("page %d: exceeded the %d-page safety limit", page+1, maxPages)
		}
		nextURL := links.Next
		nextIdentity := PaginationURLIdentity(nextURL)
		if _, ok := seenNext[nextIdentity]; ok {
			return fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
		}
		seenNext[nextIdentity] = struct{}{}

		if fetchNext == nil {
			return fmt.Errorf("page %d: %w", page+1, ErrMissingPaginationFetcher)
		}

		nextPage, err := fetchNext(ctx, nextURL)
		if err != nil {
			return fmt.Errorf("page %d: %w", page+1, err)
		}
		if isNilPaginatedResponse(nextPage) {
			return fmt.Errorf("page %d: %w", page+1, ErrNilPaginationPage)
		}
		if reflect.TypeOf(nextPage) != reflect.TypeOf(current) {
			return fmt.Errorf("page %d: unexpected response type (expected %T, got %T)", page+1, current, nextPage)
		}

		current = nextPage
		page++
	}
}

func isNilPaginatedResponse(page PaginatedResponse) bool {
	if page == nil {
		return true
	}

	value := reflect.ValueOf(page)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// PaginationURLIdentity returns the request identity used for cycle
// detection. It intentionally leaves the URL passed to fetchNext untouched:
// callers may rely on the provider's exact next-link spelling. Only a
// same-host HTTPS absolute URL is collapsed to its request URI so it compares
// equal to the equivalent relative link. Query parameters are decoded and
// re-encoded to make their order irrelevant. Invalid, insecure, and untrusted
// absolute URLs retain their trimmed spelling for the caller's validation and
// error handling.
func PaginationURLIdentity(nextURL string) string {
	nextURL = strings.TrimSpace(nextURL)
	if nextURL == "" {
		return nextURL
	}

	parsed, err := url.Parse(nextURL)
	if err != nil {
		return nextURL
	}

	// Match validateNextURL's absolute-URL recognition so this helper cannot
	// turn a URL that the request path treats as relative into a trusted one.
	if !strings.HasPrefix(nextURL, "https://") {
		if !strings.HasPrefix(nextURL, "http://") {
			return canonicalPaginationRequestURI(parsed, nextURL)
		}
		return nextURL
	}

	baseURL, err := url.Parse(BaseURL)
	if err != nil || parsed.Scheme != baseURL.Scheme || parsed.Host != baseURL.Host || parsed.User != nil {
		return nextURL
	}
	return canonicalPaginationRequestURI(parsed, nextURL)
}

func canonicalPaginationRequestURI(parsed *url.URL, fallback string) string {
	if parsed == nil {
		return fallback
	}
	if parsed.RawQuery != "" {
		values, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return fallback
		}
		parsed.RawQuery = values.Encode()
	}
	// A trailing '?' does not change the request's empty query. Do not let
	// URL.Parse's ForceQuery marker split equivalent continuation identities.
	if parsed.RawQuery == "" {
		parsed.ForceQuery = false
	}
	requestURI := parsed.RequestURI()
	if requestURI == "" && fallback != "" {
		return fallback
	}
	return requestURI
}

// newEmptyPaginatedResponse creates a new zero-valued instance of the same
// concrete type as src. The returned value is a pointer to a new struct that
// satisfies PaginatedResponse.
func newEmptyPaginatedResponse(src PaginatedResponse) (PaginatedResponse, error) {
	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("unsupported response type for pagination: %T (expected pointer)", src)
	}
	if srcValue.Type().Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported response type for pagination: %T (expected pointer to struct)", src)
	}

	// Create a new zero-valued struct of the same type.
	// Use srcValue.Type().Elem() instead of srcValue.Elem().Type() to handle
	// typed nil pointers (e.g., var resp *Type = nil passed as interface).
	newPtr := reflect.New(srcValue.Type().Elem())
	initializeEmptyDataSlice(newPtr.Elem())
	result, ok := newPtr.Interface().(PaginatedResponse)
	if !ok {
		return nil, fmt.Errorf("unsupported response type for pagination: %T does not implement PaginatedResponse", src)
	}
	return result, nil
}

// initializeAggregatedResponse preserves the first page's document context
// while preparing a non-nil data slice for the aggregated collection.
func initializeAggregatedResponse(result, firstPage PaginatedResponse) error {
	resultValue := reflect.ValueOf(result)
	pageValue := reflect.ValueOf(firstPage)
	if resultValue.Kind() != reflect.Pointer || pageValue.Kind() != reflect.Pointer ||
		resultValue.IsNil() || pageValue.IsNil() {
		return fmt.Errorf("pagination initialization expects non-nil pointers (got %T and %T)", result, firstPage)
	}
	if resultValue.Type() != pageValue.Type() {
		return fmt.Errorf("pagination initialization type mismatch: page is %T but result is %T", firstPage, result)
	}

	resultElem := resultValue.Elem()
	pageElem := pageValue.Elem()
	initializeEmptyDataSlice(resultElem)
	for _, fieldName := range []string{"Links", "Meta"} {
		resultField := resultElem.FieldByName(fieldName)
		pageField := pageElem.FieldByName(fieldName)
		if resultField.IsValid() && pageField.IsValid() && resultField.CanSet() && resultField.Type() == pageField.Type() {
			resultField.Set(pageField)
		}
	}
	return nil
}

// clearPageLocalContext removes links and metadata that describe an individual
// API page once the result contains data aggregated from multiple pages.
func clearPageLocalContext(response PaginatedResponse) error {
	responseValue := reflect.ValueOf(response)
	if responseValue.Kind() != reflect.Pointer || responseValue.IsNil() {
		return fmt.Errorf("pagination context clearing expects a non-nil pointer (got %T)", response)
	}

	responseElem := responseValue.Elem()
	for _, fieldName := range []string{"Links", "Meta"} {
		field := responseElem.FieldByName(fieldName)
		if field.IsValid() && field.CanSet() {
			field.Set(reflect.Zero(field.Type()))
		}
	}
	return nil
}

func initializeEmptyDataSlice(responseValue reflect.Value) {
	data := responseValue.FieldByName("Data")
	if data.IsValid() && data.CanSet() && data.Kind() == reflect.Slice && data.IsNil() {
		data.Set(reflect.MakeSlice(data.Type(), 0, 0))
	}
}

// PageDataLen reports how many items a page's Data collection holds.
// It returns ok=false when the page is nil (including a typed nil pointer) or
// when GetData does not expose a countable item slice — byte slices such as
// json.RawMessage are payloads, not item lists, so they are not counted.
func PageDataLen(page PaginatedResponse) (int, bool) {
	if page == nil {
		return 0, false
	}

	// Handle typed nil (non-nil interface containing nil pointer) before
	// invoking interface methods, mirroring the PaginateAll guard.
	pageValue := reflect.ValueOf(page)
	if pageValue.Kind() == reflect.Pointer && pageValue.IsNil() {
		return 0, false
	}

	data := reflect.ValueOf(page.GetData())
	if !data.IsValid() || data.Kind() != reflect.Slice || data.Type().Elem().Kind() == reflect.Uint8 {
		return 0, false
	}
	return data.Len(), true
}

// aggregatePageData appends page data to result by reflecting on the shared Data field.
// This keeps pagination aggregation generic while still validating type compatibility.
// The page's included resources are collected into included rather than merged
// into result, so the aggregated array is marshaled only once.
func aggregatePageData(result, page PaginatedResponse, included *rawJSONArrayAccumulator) error {
	if result == nil || page == nil {
		return fmt.Errorf("page aggregation received nil result or page")
	}

	resultValue := reflect.ValueOf(result)
	pageValue := reflect.ValueOf(page)
	if resultValue.Kind() != reflect.Pointer || pageValue.Kind() != reflect.Pointer {
		return fmt.Errorf("page aggregation expects pointer types (got %T and %T)", result, page)
	}

	if resultValue.Type() != pageValue.Type() {
		return fmt.Errorf("type mismatch: page is %T but result is %T", page, result)
	}

	// Handle typed nil pointers (non-nil interface containing nil pointer).
	// A typed nil page has no data to aggregate, so skip it.
	if pageValue.IsNil() {
		return nil
	}
	if resultValue.IsNil() {
		return fmt.Errorf("page aggregation received nil result pointer")
	}

	resultElem := resultValue.Elem()
	pageElem := pageValue.Elem()
	resultData := resultElem.FieldByName("Data")
	pageData := pageElem.FieldByName("Data")
	if !resultData.IsValid() || !pageData.IsValid() {
		return fmt.Errorf("missing Data field for %T", page)
	}
	if resultData.Kind() != reflect.Slice || pageData.Kind() != reflect.Slice {
		return fmt.Errorf("data field is not a slice for %T", page)
	}
	if resultData.Type() != pageData.Type() {
		return fmt.Errorf("data field type mismatch: %s vs %s", resultData.Type(), pageData.Type())
	}

	resultData.Set(reflect.AppendSlice(resultData, pageData))
	if err := collectJSONRawArrayField(resultElem, pageElem, includedFieldName, included); err != nil {
		return err
	}
	return nil
}

// includedFieldName is the JSON:API sideloaded-resources field aggregated across pages.
const includedFieldName = "Included"

var rawJSONMessageType = reflect.TypeOf(json.RawMessage{})

// collectJSONRawArrayField records one page's raw JSON array field in acc. The
// field is only collected when both the aggregated response and the page expose
// it as a json.RawMessage.
func collectJSONRawArrayField(resultElem, pageElem reflect.Value, fieldName string, acc *rawJSONArrayAccumulator) error {
	resultField := resultElem.FieldByName(fieldName)
	pageField := pageElem.FieldByName(fieldName)
	if !resultField.IsValid() || !pageField.IsValid() {
		return nil
	}
	if resultField.Type() != rawJSONMessageType || pageField.Type() != rawJSONMessageType {
		return nil
	}

	if err := acc.add(pageField.Interface().(json.RawMessage)); err != nil {
		return fmt.Errorf("merge %s: %w", fieldName, err)
	}
	return nil
}

// setJSONRawArrayField writes the array accumulated across pages to the
// aggregated response, marshaling it a single time.
func setJSONRawArrayField(result PaginatedResponse, fieldName string, acc *rawJSONArrayAccumulator) error {
	resultValue := reflect.ValueOf(result)
	if resultValue.Kind() != reflect.Pointer || resultValue.IsNil() {
		return nil
	}
	field := resultValue.Elem().FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() || field.Type() != rawJSONMessageType {
		return nil
	}

	merged, err := acc.merged()
	if err != nil {
		return fmt.Errorf("merge %s: %w", fieldName, err)
	}
	field.Set(reflect.ValueOf(merged))
	return nil
}

// rawJSONArrayAccumulator merges JSON:API arrays such as `included` across
// pages in linear time. It keeps the raw items collected so far alongside the
// identity set used for deduplication, instead of reparsing and remarshaling
// the accumulated array once per page.
//
// A lone payload is retained verbatim and never parsed, so a single-page
// response keeps Apple's array exactly as it arrived.
type rawJSONArrayAccumulator struct {
	sole     json.RawMessage
	items    []json.RawMessage
	seen     map[string]struct{}
	expanded bool
}

// add records one page's array. The first non-empty payload is only parsed once
// a second payload arrives and the arrays actually have to be merged.
func (a *rawJSONArrayAccumulator) add(payload json.RawMessage) error {
	if len(payload) == 0 {
		return nil
	}
	if !a.expanded && a.sole == nil {
		a.sole = append(json.RawMessage(nil), payload...)
		return nil
	}
	if !a.expanded {
		if err := a.appendUnique(a.sole, "parse existing array"); err != nil {
			return err
		}
		a.sole = nil
		a.expanded = true
	}
	return a.appendUnique(payload, "parse incoming array")
}

// merged reports the aggregated array, marshaling the collected items once.
func (a *rawJSONArrayAccumulator) merged() (json.RawMessage, error) {
	if !a.expanded {
		return a.sole, nil
	}

	merged, err := json.Marshal(a.items)
	if err != nil {
		return nil, fmt.Errorf("marshal merged array: %w", err)
	}
	return merged, nil
}

func (a *rawJSONArrayAccumulator) appendUnique(payload json.RawMessage, stage string) error {
	var items []json.RawMessage
	if err := json.Unmarshal(payload, &items); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}

	if a.items == nil {
		a.items = make([]json.RawMessage, 0, len(items))
	}
	if a.seen == nil {
		a.seen = make(map[string]struct{}, len(items))
	}
	for _, item := range items {
		key := rawJSONArrayItemKey(item)
		if _, ok := a.seen[key]; ok {
			continue
		}
		a.seen[key] = struct{}{}
		a.items = append(a.items, item)
	}
	return nil
}

// rawJSONArrayItemKey follows JSON:API's resource identity rule when an item
// has both type and id. Items without a usable resource identity retain the
// previous raw-JSON equality behavior instead of being collapsed together.
func rawJSONArrayItemKey(item json.RawMessage) string {
	var identity struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(item, &identity); err == nil && identity.Type != "" && identity.ID != "" {
		return "resource\x00" + identity.Type + "\x00" + identity.ID
	}
	return "raw\x00" + string(item)
}
