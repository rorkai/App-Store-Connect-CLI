package asc

import (
	"fmt"
	"strings"
)

// WebAppBundleAppsResult is the receipt for `asc web app-bundles apps set`.
type WebAppBundleAppsResult struct {
	BundleID     string   `json:"bundleId"`
	PreviousApps []string `json:"previousApps"`
	Apps         []string `json:"apps"`
	Changed      bool     `json:"changed"`
}

// WebAppBundleReviewResult is the receipt for submitting a bundle or removing it from review.
type WebAppBundleReviewResult struct {
	BundleID              string `json:"bundleId"`
	PreviousState         string `json:"previousState"`
	State                 string `json:"state"`
	PreviousInFlightState string `json:"previousInFlightState,omitempty"`
	InFlightState         string `json:"inFlightState,omitempty"`
}

func webAppBundleReviewRows(r *WebAppBundleReviewResult) ([]string, [][]string) {
	return []string{"Bundle ID", "Previous State", "State", "Previous In-Flight State", "In-Flight State"}, [][]string{{r.BundleID, r.PreviousState, r.State, r.PreviousInFlightState, r.InFlightState}}
}

func webAppBundleAppsRows(r *WebAppBundleAppsResult) ([]string, [][]string) {
	return []string{"Bundle ID", "Previous Apps", "Apps", "Changed"}, [][]string{{r.BundleID, strings.Join(r.PreviousApps, ","), strings.Join(r.Apps, ","), fmt.Sprint(r.Changed)}}
}
