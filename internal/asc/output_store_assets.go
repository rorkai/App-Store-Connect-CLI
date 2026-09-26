package asc

// StoreAssetResult records a planned or completed store asset operation, including
// reservations that survive a failed transfer.
type StoreAssetResult struct {
	PreviousID      string `json:"previousId,omitempty"`
	PreviousDeleted bool   `json:"previousDeleted,omitempty"`
	Kind            string `json:"kind"`
	Locale          string `json:"locale,omitempty"`
	Path            string `json:"path,omitempty"`
	ID              string `json:"id,omitempty"`
	Action          string `json:"action"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}

// StoreAssetChange binds an asset target and content to a reviewed plan.
type StoreAssetChange struct {
	Kind   string `json:"kind"`
	Locale string `json:"locale,omitempty"`
	Path   string `json:"path"`
	Action string `json:"action"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}
