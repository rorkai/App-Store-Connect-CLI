package asc

import "encoding/json"

// BetaBuildUsagesResponse wraps raw beta build usage metrics JSON.
type BetaBuildUsagesResponse struct {
	Data json.RawMessage `json:"-"`
}

// MarshalJSON preserves raw API JSON for beta build usage metrics.
func (r BetaBuildUsagesResponse) MarshalJSON() ([]byte, error) {
	if len(r.Data) == 0 {
		return []byte("null"), nil
	}
	return r.Data, nil
}

// BetaTesterUsagesResponse wraps raw beta tester usage metrics JSON.
type BetaTesterUsagesResponse struct {
	Data json.RawMessage `json:"-"`
}

// MarshalJSON preserves raw API JSON for beta tester usage metrics.
func (r BetaTesterUsagesResponse) MarshalJSON() ([]byte, error) {
	if len(r.Data) == 0 {
		return []byte("null"), nil
	}
	return r.Data, nil
}
