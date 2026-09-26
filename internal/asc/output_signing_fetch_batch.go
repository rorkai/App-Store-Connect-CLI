package asc

// SigningFetchBatchResult is the receipt for signing fetch --match-extensions
// when more than one registered bundle ID matched.
type SigningFetchBatchResult struct {
	MatchedBundleIDs []string                   `json:"matchedBundleIds"`
	Results          []SigningFetchResult       `json:"results"`
	Failures         []SigningFetchBatchFailure `json:"failures,omitempty"`
}

// SigningFetchBatchFailure is one matched bundle ID whose fetch failed.
// StaleProfiles is the --delete-stale-profiles receipt for that bundle ID,
// kept because deletions cannot be undone even when the fetch failed.
type SigningFetchBatchFailure struct {
	BundleID      string                     `json:"bundleId"`
	Error         string                     `json:"error"`
	StaleProfiles *SigningFetchStaleProfiles `json:"staleProfiles,omitempty"`
	// ProfileID and ProfileCreationState report a profile this run created
	// ("created") or may have created ("unknown") before the target failed.
	ProfileID            string `json:"profileId,omitempty"`
	ProfileCreationState string `json:"profileCreationState,omitempty"`
}

func signingFetchBatchResultRender(result *SigningFetchBatchResult, render func([]string, [][]string)) error {
	var headers []string
	rows := make([][]string, 0, len(result.Results))
	for i := range result.Results {
		resultHeaders, resultRows := signingFetchResultRows(&result.Results[i])
		if len(resultHeaders) > len(headers) {
			headers = resultHeaders
		}
		rows = append(rows, resultRows...)
	}
	if headers == nil {
		headers, _ = signingFetchResultRows(&SigningFetchResult{})
	}
	for i := range rows {
		for len(rows[i]) < len(headers) {
			rows[i] = append(rows[i], "")
		}
	}
	render(headers, rows)
	if len(result.Failures) > 0 {
		failureRows := make([][]string, 0, len(result.Failures))
		for _, failure := range result.Failures {
			deleted, failed := "", ""
			if stale := failure.StaleProfiles; stale != nil {
				deleted = joinSigningList(signingStaleProfileIDs(stale.Deleted))
				failedIDs := make([]string, 0, len(stale.Failed))
				for _, item := range stale.Failed {
					failedIDs = append(failedIDs, item.ID+": "+item.Error)
				}
				failed = joinSigningList(failedIDs)
			}
			failureRows = append(failureRows, []string{failure.BundleID, failure.Error, failure.ProfileID, failure.ProfileCreationState, deleted, failed})
		}
		render([]string{"Failed Bundle ID", "Error", "Profile ID", "Profile Creation", "Stale Deleted", "Stale Failed"}, failureRows)
	}
	return nil
}
