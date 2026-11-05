package asc

import "strconv"

// RetentionMessageAssignResult records a subscription's retention message assignment.
type RetentionMessageAssignResult struct {
	SubscriptionID     string `json:"subscriptionId"`
	RetentionMessageID string `json:"retentionMessageId"`
	Assigned           bool   `json:"assigned"`
}

// RetentionMessagePlacementCreateResult records an image placement created for a retention message version.
type RetentionMessagePlacementCreateResult struct {
	ID                        string `json:"id"`
	Created                   bool   `json:"created"`
	RetentionMessageVersionID string `json:"retentionMessageVersionId"`
	ImageID                   string `json:"imageId"`
	PlacementType             string `json:"placementType"`
	PlacementGroup            string `json:"placementGroup"`
	State                     string `json:"state,omitempty"`
}

func retentionMessagesRows(resp *RetentionMessagesResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, compactWhitespace(item.Attributes.ReferenceName)})
	}
	return []string{"ID", "Reference Name"}, rows
}

func retentionMessageVersionsRows(resp *RetentionMessageVersionsResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.State})
	}
	return []string{"ID", "State"}, rows
}

func retentionMessageLocalizationsRows(resp *RetentionMessageLocalizationsResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{item.ID, attrs.Locale, compactWhitespace(attrs.Header), compactWhitespace(attrs.Body)})
	}
	return []string{"ID", "Locale", "Header", "Body"}, rows
}

func retentionMessageAssignResultRows(result *RetentionMessageAssignResult) ([]string, [][]string) {
	return []string{"Subscription ID", "Retention Message ID", "Assigned"},
		[][]string{{result.SubscriptionID, result.RetentionMessageID, strconv.FormatBool(result.Assigned)}}
}

func retentionMessagePlacementCreateResultRows(r *RetentionMessagePlacementCreateResult) ([]string, [][]string) {
	return []string{"ID", "Created", "Version ID", "Image ID", "Placement Type", "Placement Group", "State"},
		[][]string{{r.ID, strconv.FormatBool(r.Created), r.RetentionMessageVersionID, r.ImageID, r.PlacementType, r.PlacementGroup, r.State}}
}
