package web

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubmitAppBundle sends a bundle to App Review once Apple offers that action.
func (c *Client) SubmitAppBundle(ctx context.Context, bundleID string) (*asc.WebAppBundleReviewResult, error) {
	return c.changeAppBundleReview(ctx, bundleID, "/appbundles/submit/", appBundleInReview, func(detail *appBundleDetailData) error {
		if !detail.CanSubmit {
			return fmt.Errorf("app bundle is %s and cannot be submitted for review", detail.StateKey)
		}
		return nil
	})
}

// RemoveAppBundleFromReview withdraws a submitted bundle once Apple offers that action.
func (c *Client) RemoveAppBundleFromReview(ctx context.Context, bundleID string) (*asc.WebAppBundleReviewResult, error) {
	outOfReview := func(detail *appBundleDetailData) bool { return !appBundleInReview(detail) }
	return c.changeAppBundleReview(ctx, bundleID, "/appbundles/devreject/", outOfReview, func(detail *appBundleDetailData) error {
		if !detail.CanDevReject {
			return fmt.Errorf("app bundle is %s and cannot be removed from review", detail.StateKey)
		}
		return nil
	})
}

func (c *Client) changeAppBundleReview(ctx context.Context, bundleID, path string, confirmed func(*appBundleDetailData) bool, allowed func(*appBundleDetailData) error) (*asc.WebAppBundleReviewResult, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, errors.New("app bundle ID is required")
	}
	current, err := c.fetchAppBundleDetail(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	if err := allowed(current); err != nil {
		return nil, err
	}

	if err := c.postAppBundle(ctx, path+url.PathEscape(bundleID), nil); err != nil {
		return nil, err
	}

	updated, err := c.fetchAppBundleDetail(ctx, bundleID)
	if err != nil {
		return nil, fmt.Errorf("could not verify app bundle state: %w", err)
	}
	if !confirmed(updated) {
		return nil, fmt.Errorf("app bundle readback shows %s, not the requested review change", updated.StateKey)
	}
	return &asc.WebAppBundleReviewResult{BundleID: bundleID, PreviousState: current.StateKey, State: updated.StateKey}, nil
}
