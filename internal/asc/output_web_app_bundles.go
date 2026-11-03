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

func webAppBundleAppsRows(r *WebAppBundleAppsResult) ([]string, [][]string) {
	return []string{"Bundle ID", "Previous Apps", "Apps", "Changed"}, [][]string{{r.BundleID, strings.Join(r.PreviousApps, ","), strings.Join(r.Apps, ","), fmt.Sprint(r.Changed)}}
}
