package asc

// GameCenterGroupDeleteResult represents CLI output for group deletions.
type GameCenterGroupDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
