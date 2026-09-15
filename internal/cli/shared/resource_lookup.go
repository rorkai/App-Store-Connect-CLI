package shared

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var (
	errSelectorNotFound  = errors.New("selector not found")
	errSelectorAmbiguous = errors.New("selector ambiguous")
)

type selectorNotFoundError struct {
	resourceName string
	fieldName    string
	selector     string
}

func (e selectorNotFoundError) Error() string {
	return fmt.Sprintf("%s %s %q not found", e.resourceName, e.fieldName, e.selector)
}

func (e selectorNotFoundError) Is(target error) bool {
	return target == errSelectorNotFound
}

type selectorAmbiguousError struct {
	resourceName string
	fieldName    string
	selector     string
	matches      []ExactSelectorCandidate
	// flag overrides the default stable-selector flag named in the message.
	flag string
}

func (e selectorAmbiguousError) Error() string {
	return e.ambiguous().Error()
}

// Unwrap exposes the rendered ambiguity error so callers can detect stable
// selector ambiguity with IsAmbiguousSelection and read the candidates.
func (e selectorAmbiguousError) Unwrap() error { return e.ambiguous() }

func (e selectorAmbiguousError) ambiguous() *AmbiguousSelectionError {
	flag := strings.TrimSpace(e.flag)
	if flag == "" {
		flag = SelectorFlagForResource(e.resourceName)
	}
	return &AmbiguousSelectionError{
		Kind:        e.resourceName,
		Description: fmt.Sprintf("%q by %s", e.selector, e.fieldName),
		Flag:        flag,
		Candidates:  ExactSelectorAmbiguousCandidates(e.matches),
	}
}

// SelectorFlagForResource names the stable-selector flag that accepts an
// explicit ASC ID for a resource kind resolved by ResolveIAPID,
// ResolveSubscriptionID, or ResolveExactSelectorCandidate. Commands that bind
// a different flag wrap the resolver error with WithSelectorFlag.
func SelectorFlagForResource(resourceName string) string {
	switch strings.TrimSpace(resourceName) {
	case "in-app purchase":
		return "--iap-id"
	case "subscription":
		return "--subscription-id"
	case "subscription group":
		return "--group-id"
	default:
		return ""
	}
}

// WithSelectorFlag names the flag that accepts an explicit ASC ID in an
// ambiguous stable-selector error, for commands whose selector flag differs
// from the default for the resource kind. Other errors pass through
// unchanged.
func WithSelectorFlag(err error, flag string) error {
	var ambiguous selectorAmbiguousError
	if !errors.As(err, &ambiguous) {
		return err
	}
	ambiguous.flag = strings.TrimSpace(flag)
	return ambiguous
}

// ExactSelectorAmbiguousCandidates converts selector matches into ambiguity
// candidates (ID, product ID, name).
func ExactSelectorAmbiguousCandidates(matches []ExactSelectorCandidate) []AmbiguousCandidate {
	candidates := make([]AmbiguousCandidate, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, AmbiguousCandidate{
			ID:    strings.TrimSpace(match.ID),
			Label: strings.TrimSpace(match.ProductID),
			Extra: strings.TrimSpace(match.Name),
		})
	}
	return candidates
}

func (e selectorAmbiguousError) Is(target error) bool {
	return target == errSelectorAmbiguous
}

// ExactSelectorCandidate is a resource that can be matched by ASC ID, product ID, or current name.
type ExactSelectorCandidate struct {
	ID        string
	ProductID string
	Name      string
}

type iapSelectorClient interface {
	GetInAppPurchasesV2(ctx context.Context, appID string, opts ...asc.IAPOption) (*asc.InAppPurchasesV2Response, error)
}

type subscriptionSelectorClient interface {
	GetSubscriptionGroups(ctx context.Context, appID string, opts ...asc.SubscriptionGroupsOption) (*asc.SubscriptionGroupsResponse, error)
	GetSubscriptions(ctx context.Context, groupID string, opts ...asc.SubscriptionsOption) (*asc.SubscriptionsResponse, error)
}

// SelectorNeedsLookup reports whether a selector needs app-scoped lookup instead of direct ASC ID passthrough.
func SelectorNeedsLookup(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return !isNumericSelectorID(value)
}

// RequireAppForStableSelector returns a usage error when a non-numeric selector needs app context.
func RequireAppForStableSelector(appID, selector, flagName string) error {
	if !SelectorNeedsLookup(selector) {
		return nil
	}
	if strings.TrimSpace(appID) != "" {
		return nil
	}
	return UsageErrorf("--app is required (or set ASC_APP_ID) when %s is a product ID or name", strings.TrimSpace(flagName))
}

// ResolveExactSelectorCandidate resolves a unique candidate by exact product ID first, then exact current name.
func ResolveExactSelectorCandidate(selector, resourceName string, candidates []ExactSelectorCandidate) (ExactSelectorCandidate, error) {
	candidate, err := resolveExactProductIDCandidate(selector, resourceName, candidates)
	if err == nil || !errors.Is(err, errSelectorNotFound) {
		return candidate, err
	}
	return resolveExactNameCandidate(selector, resourceName, candidates)
}

// ResolveIAPID resolves an in-app purchase selector to its canonical ASC ID.
func ResolveIAPID(ctx context.Context, client iapSelectorClient, appID, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	resolvedAppID := strings.TrimSpace(appID)
	needsLookup := SelectorNeedsLookup(selector)
	if !needsLookup && resolvedAppID == "" {
		return selector, nil
	}
	if resolvedAppID == "" {
		return "", fmt.Errorf("app context is required to resolve in-app purchase selector %q", selector)
	}
	if client == nil {
		return "", fmt.Errorf("in-app purchase lookup client is required")
	}

	candidate, err := resolveIAPCandidate(ctx, client, resolvedAppID, selector)
	if err != nil {
		// Preserve legacy raw-ID behavior for numeric selectors when app-scoped lookup
		// misses or fails, but keep ambiguity errors so callers still get the
		// disambiguation guidance for selector-shaped values like "2024".
		if shouldFallbackToRawNumericSelector(needsLookup, err) {
			return selector, nil
		}
		return "", err
	}
	return candidate.ID, nil
}

// ResolveSubscriptionID resolves a subscription selector to its canonical ASC ID.
func ResolveSubscriptionID(ctx context.Context, client subscriptionSelectorClient, appID, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	resolvedAppID := strings.TrimSpace(appID)
	needsLookup := SelectorNeedsLookup(selector)
	if !needsLookup && resolvedAppID == "" {
		return selector, nil
	}
	if resolvedAppID == "" {
		return "", fmt.Errorf("app context is required to resolve subscription selector %q", selector)
	}
	if client == nil {
		return "", fmt.Errorf("subscription lookup client is required")
	}

	candidate, err := resolveSubscriptionCandidate(ctx, client, resolvedAppID, selector)
	if err != nil {
		// Preserve legacy raw-ID behavior for numeric selectors when app-scoped lookup
		// misses or fails, but keep ambiguity errors so callers still get the
		// disambiguation guidance for selector-shaped values like "2024".
		if shouldFallbackToRawNumericSelector(needsLookup, err) {
			return selector, nil
		}
		return "", err
	}
	return candidate.ID, nil
}

func resolveIAPCandidate(ctx context.Context, client iapSelectorClient, appID, selector string) (ExactSelectorCandidate, error) {
	candidates, err := listIAPCandidates(ctx, client, appID, asc.WithIAPProductIDs([]string{selector}))
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve in-app purchase by product ID: %w", err)
	}

	candidate, err := resolveExactProductIDCandidate(selector, "in-app purchase", candidates)
	if err == nil || !errors.Is(err, errSelectorNotFound) {
		return candidate, err
	}

	candidates, err = listIAPCandidates(ctx, client, appID, asc.WithIAPNames([]string{selector}))
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve in-app purchase by name: %w", err)
	}

	candidate, err = resolveExactNameCandidate(selector, "in-app purchase", candidates)
	if err == nil || !errors.Is(err, errSelectorNotFound) {
		return candidate, err
	}

	candidates, err = listIAPCandidates(ctx, client, appID)
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve in-app purchase by name: %w", err)
	}

	return resolveExactNameCandidate(selector, "in-app purchase", candidates)
}

func resolveSubscriptionCandidate(ctx context.Context, client subscriptionSelectorClient, appID, selector string) (ExactSelectorCandidate, error) {
	candidates, err := listSubscriptionCandidates(ctx, client, appID, asc.WithSubscriptionsProductIDs([]string{selector}))
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve subscription by product ID: %w", err)
	}

	candidate, err := resolveExactProductIDCandidate(selector, "subscription", candidates)
	if err == nil || !errors.Is(err, errSelectorNotFound) {
		return candidate, err
	}

	candidates, err = listSubscriptionCandidates(ctx, client, appID, asc.WithSubscriptionsNames([]string{selector}))
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve subscription by name: %w", err)
	}

	candidate, err = resolveExactNameCandidate(selector, "subscription", candidates)
	if err == nil || !errors.Is(err, errSelectorNotFound) {
		return candidate, err
	}

	candidates, err = listSubscriptionCandidates(ctx, client, appID)
	if err != nil {
		return ExactSelectorCandidate{}, fmt.Errorf("resolve subscription by name: %w", err)
	}

	return resolveExactNameCandidate(selector, "subscription", candidates)
}

func listIAPCandidates(ctx context.Context, client iapSelectorClient, appID string, opts ...asc.IAPOption) ([]ExactSelectorCandidate, error) {
	opts = append([]asc.IAPOption{asc.WithIAPLimit(200)}, opts...)
	firstPage, err := client.GetInAppPurchasesV2(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, nil
	}

	candidates := make([]ExactSelectorCandidate, 0, len(firstPage.Data))
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetInAppPurchasesV2(ctx, appID, asc.WithIAPNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.InAppPurchasesV2Response)
			if !ok {
				return fmt.Errorf("unexpected in-app purchases pagination type %T", page)
			}
			appendUniqueSelectorCandidates(&candidates, mapIAPCandidates(resp.Data))
			return nil
		},
	); err != nil {
		return nil, err
	}

	return candidates, nil
}

func listSubscriptionCandidates(ctx context.Context, client subscriptionSelectorClient, appID string, opts ...asc.SubscriptionsOption) ([]ExactSelectorCandidate, error) {
	groupIDs, err := listSubscriptionGroupIDs(ctx, client, appID)
	if err != nil {
		return nil, err
	}

	candidates := make([]ExactSelectorCandidate, 0)
	for _, groupID := range groupIDs {
		groupCandidates, err := listSubscriptionsForGroup(ctx, client, groupID, opts...)
		if err != nil {
			return nil, err
		}
		appendUniqueSelectorCandidates(&candidates, groupCandidates)
	}

	return candidates, nil
}

func listSubscriptionGroupIDs(ctx context.Context, client subscriptionSelectorClient, appID string) ([]string, error) {
	firstPage, err := client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, nil
	}

	groupIDs := make([]string, 0, len(firstPage.Data))
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionGroupsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription groups pagination type %T", page)
			}
			for _, group := range resp.Data {
				groupID := strings.TrimSpace(group.ID)
				if groupID == "" || slices.Contains(groupIDs, groupID) {
					continue
				}
				groupIDs = append(groupIDs, groupID)
			}
			return nil
		},
	); err != nil {
		return nil, err
	}

	return groupIDs, nil
}

func listSubscriptionsForGroup(ctx context.Context, client subscriptionSelectorClient, groupID string, opts ...asc.SubscriptionsOption) ([]ExactSelectorCandidate, error) {
	opts = append([]asc.SubscriptionsOption{asc.WithSubscriptionsLimit(200)}, opts...)
	firstPage, err := client.GetSubscriptions(ctx, groupID, opts...)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, nil
	}

	candidates := make([]ExactSelectorCandidate, 0, len(firstPage.Data))
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptions(ctx, groupID, asc.WithSubscriptionsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscriptions pagination type %T", page)
			}
			appendUniqueSelectorCandidates(&candidates, mapSubscriptionCandidates(resp.Data))
			return nil
		},
	); err != nil {
		return nil, err
	}

	return candidates, nil
}

func mapIAPCandidates(resources []asc.Resource[asc.InAppPurchaseV2Attributes]) []ExactSelectorCandidate {
	candidates := make([]ExactSelectorCandidate, 0, len(resources))
	for _, resource := range resources {
		candidates = append(candidates, ExactSelectorCandidate{
			ID:        strings.TrimSpace(resource.ID),
			ProductID: strings.TrimSpace(resource.Attributes.ProductID),
			Name:      strings.TrimSpace(resource.Attributes.Name),
		})
	}
	return candidates
}

func mapSubscriptionCandidates(resources []asc.Resource[asc.SubscriptionAttributes]) []ExactSelectorCandidate {
	candidates := make([]ExactSelectorCandidate, 0, len(resources))
	for _, resource := range resources {
		candidates = append(candidates, ExactSelectorCandidate{
			ID:        strings.TrimSpace(resource.ID),
			ProductID: strings.TrimSpace(resource.Attributes.ProductID),
			Name:      strings.TrimSpace(resource.Attributes.Name),
		})
	}
	return candidates
}

func appendUniqueSelectorCandidates(dst *[]ExactSelectorCandidate, candidates []ExactSelectorCandidate) {
	if len(candidates) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(*dst))
	for _, candidate := range *dst {
		if candidate.ID == "" {
			continue
		}
		seen[candidate.ID] = struct{}{}
	}
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		if _, ok := seen[candidate.ID]; ok {
			continue
		}
		seen[candidate.ID] = struct{}{}
		*dst = append(*dst, candidate)
	}
}

func resolveExactProductIDCandidate(selector, resourceName string, candidates []ExactSelectorCandidate) (ExactSelectorCandidate, error) {
	return resolveUniqueSelectorCandidate(
		selector,
		resourceName,
		"product ID",
		candidates,
		func(candidate ExactSelectorCandidate, selector string) bool {
			return strings.TrimSpace(candidate.ProductID) == selector
		},
	)
}

func resolveExactNameCandidate(selector, resourceName string, candidates []ExactSelectorCandidate) (ExactSelectorCandidate, error) {
	return resolveUniqueSelectorCandidate(
		selector,
		resourceName,
		"name",
		candidates,
		func(candidate ExactSelectorCandidate, selector string) bool {
			return strings.EqualFold(strings.TrimSpace(candidate.Name), selector)
		},
	)
}

func resolveUniqueSelectorCandidate(
	selector string,
	resourceName string,
	fieldName string,
	candidates []ExactSelectorCandidate,
	match func(candidate ExactSelectorCandidate, selector string) bool,
) (ExactSelectorCandidate, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return ExactSelectorCandidate{}, selectorNotFoundError{resourceName: resourceName, fieldName: fieldName, selector: selector}
	}

	matches := make([]ExactSelectorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !match(candidate, selector) {
			continue
		}
		matches = append(matches, candidate)
	}

	switch len(matches) {
	case 0:
		return ExactSelectorCandidate{}, selectorNotFoundError{resourceName: resourceName, fieldName: fieldName, selector: selector}
	case 1:
		return matches[0], nil
	default:
		return ExactSelectorCandidate{}, selectorAmbiguousError{
			resourceName: resourceName,
			fieldName:    fieldName,
			selector:     selector,
			matches:      matches,
		}
	}
}

func shouldFallbackToRawNumericSelector(needsLookup bool, err error) bool {
	return !needsLookup && !errors.Is(err, errSelectorAmbiguous)
}

func isNumericSelectorID(value string) bool {
	for _, ch := range strings.TrimSpace(value) {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return strings.TrimSpace(value) != ""
}
