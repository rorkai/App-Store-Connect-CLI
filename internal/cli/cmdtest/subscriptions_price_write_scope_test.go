package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSubscriptionPriceWriteProofReadsStayInScope(t *testing.T) {
	for _, test := range []struct {
		name, path, next                   string
		tiers, force, missingData, allowed bool
	}{
		{name: "price existence redirect", path: "/v1/subscriptions/8000000003/relationships/prices"},
		{name: "prices redirect", path: "/v1/subscriptions/8000000003/prices"},
		{name: "prices continuation", path: "/v1/subscriptions/8000000003/prices", next: "/v1/subscriptions/other/prices?cursor=next"},
		{name: "tier readback redirect", path: "/v1/subscriptionPrices/current/additionalPriceTiers", tiers: true},
		{name: "tier readback continuation", path: "/v1/subscriptionPrices/current/additionalPriceTiers", next: "/v1/subscriptionPrices/other/additionalPriceTiers?cursor=next", tiers: true},
		{name: "tier readback missing data", path: "/v1/subscriptionPrices/current/additionalPriceTiers", tiers: true, missingData: true},
		{name: "global tier guard redirect", path: "/v1/subscriptionPriceTiers", force: true},
		{name: "global tier guard continuation", path: "/v1/subscriptionPriceTiers", next: "/v1/subscriptionPriceTiers?filter%5Bsubscription%5D=other", force: true},
		{name: "global tier guard missing data", path: "/v1/subscriptionPriceTiers", force: true, missingData: true},
		{name: "repair equalizations redirect", path: "/v1/subscriptionPricePoints/new-base/equalizations", force: true},
		{name: "repair equalizations continuation", path: "/v1/subscriptionPricePoints/new-base/equalizations", next: "/v1/subscriptionPricePoints/other/equalizations?cursor=next", force: true},
		{name: "opaque price cursor", path: "/v1/subscriptions/8000000003/prices", next: "/v1/subscriptions/8000000003/prices?cursor=next", allowed: true},
		{name: "opaque tier cursor", path: "/v1/subscriptionPrices/current/additionalPriceTiers", next: "/v1/subscriptionPrices/current/additionalPriceTiers?cursor=next", tiers: true, allowed: true},
		{name: "opaque force tier cursor", path: "/v1/subscriptionPriceTiers", next: "/v1/subscriptionPriceTiers?cursor=next", force: true, allowed: true},
		{name: "opaque repair equalization cursor", path: "/v1/subscriptionPricePoints/new-base/equalizations", next: "/v1/subscriptionPricePoints/new-base/equalizations?cursor=next", force: true, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			writes, escaped := 0, 0
			captured.response = func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method != http.MethodGet {
					writes++
					_, _ = io.WriteString(w, `{"data":{"type":"subscriptions","id":"saved"}}`)
					return
				}
				if req.URL.Path == test.path && req.URL.Query().Get("cursor") == "" {
					switch {
					case test.missingData:
						_, _ = io.WriteString(w, `{}`)
					case test.next != "":
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "links": map[string]string{"next": test.next}})
					default:
						http.Redirect(w, req, "/fixture/scope-sentinel", http.StatusTemporaryRedirect)
					}
					return
				}
				var body string
				switch req.URL.Path {
				case "/v1/subscriptions/8000000003/relationships/prices":
					body = `{"data":[{"type":"subscriptionPrices","id":"current"}]}`
				case "/v1/subscriptions/8000000003/prices":
					body = `{"data":[{"type":"subscriptionPrices","id":"current","relationships":{"territory":{"data":{"type":"territories","id":"NOR"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"new-base"}},"additionalPriceTiers":{"data":[]}}}]} `
				case "/v1/subscriptionPrices/current/additionalPriceTiers", "/v1/subscriptionPriceTiers", "/v1/subscriptionPricePoints/new-base/equalizations":
					body = `{"data":[],"links":{}}`
				default:
					escaped++
					body = `{"data":[],"links":{}}`
				}
				_, _ = io.WriteString(w, body)
			}
			args := []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--price-point", "new-base", "--territory", "NOR"}
			if test.force {
				args = append(args, "--force")
			}
			if test.tiers {
				path := filepath.Join(t.TempDir(), "tiers.json")
				if err := os.WriteFile(path, []byte(`[]`), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--additional-price-tiers", path)
			}
			stdout, stderr, err := runSubscriptionPricesListQuerySurface(t, args...)
			wantWrites := 0
			if test.allowed && test.force {
				wantWrites = 1
			}
			if (err == nil) != test.allowed || escaped != 0 || writes != wantWrites {
				t.Fatalf("scope refusal: escaped=%d writes=%d stdout=%s stderr=%s err=%v", escaped, writes, stdout, stderr, err)
			}
		})
	}
}
