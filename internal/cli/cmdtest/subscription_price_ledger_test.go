package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
)

// subscriptionPriceLedger is a fake App Store Connect price store: it records
// the prices a fake POST /v1/subscriptionPrices or initial-price PATCH applied
// and serves them from GET /v1/subscriptions/{id}/prices, so a command that
// reads prices back sees what it wrote.
type subscriptionPriceLedger struct {
	mu     sync.Mutex
	prices map[string]subscriptionPriceLedgerEntry
}

type subscriptionPriceLedgerEntry struct {
	pricePointID string
	startDate    string
}

func newSubscriptionPriceLedger() *subscriptionPriceLedger {
	return &subscriptionPriceLedger{prices: make(map[string]subscriptionPriceLedgerEntry)}
}

func (l *subscriptionPriceLedger) set(territory, pricePointID, startDate string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prices[territory] = subscriptionPriceLedgerEntry{pricePointID: pricePointID, startDate: startDate}
}

// record applies the price in a subscriptionPrices POST body.
func (l *subscriptionPriceLedger) record(t *testing.T, body []byte) {
	t.Helper()
	var payload struct {
		Data struct {
			Attributes struct {
				StartDate string `json:"startDate"`
			} `json:"attributes"`
			Relationships struct {
				Territory struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"territory"`
				SubscriptionPricePoint struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"subscriptionPricePoint"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse subscription price body: %v", err)
	}
	l.set(payload.Data.Relationships.Territory.Data.ID, payload.Data.Relationships.SubscriptionPricePoint.Data.ID, payload.Data.Attributes.StartDate)
}

func (l *subscriptionPriceLedger) response() *http.Response {
	l.mu.Lock()
	defer l.mu.Unlock()
	territories := make([]string, 0, len(l.prices))
	for territory := range l.prices {
		territories = append(territories, territory)
	}
	sort.Strings(territories)
	data := make([]string, 0, len(territories))
	included := make([]string, 0, 2*len(territories))
	for _, territory := range territories {
		entry := l.prices[territory]
		startDate := entry.startDate
		if startDate == "" {
			startDate = "2025-01-01"
		}
		data = append(data, fmt.Sprintf(`{"type":"subscriptionPrices","id":"price-%s","attributes":{"startDate":%q,"preserved":false},"relationships":{"territory":{"data":{"type":"territories","id":%q}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}}}`, territory, startDate, territory, entry.pricePointID))
		included = append(included,
			fmt.Sprintf(`{"type":"subscriptionPricePoints","id":%q,"attributes":{"customerPrice":"0.99","proceeds":"0.84","proceedsYear2":"0.84"}}`, entry.pricePointID),
			fmt.Sprintf(`{"type":"territories","id":%q,"attributes":{"currency":"USD"}}`, territory),
		)
	}
	return jsonHTTPResponse(http.StatusOK, `{"data":[`+strings.Join(data, ",")+`],"included":[`+strings.Join(included, ",")+`],"links":{"next":""}}`)
}
