package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type postedSubscriptionPrice struct {
	territory  string
	attributes string
}

func decodePostedSubscriptionPrice(t *testing.T, req *http.Request) postedSubscriptionPrice {
	t.Helper()
	var payload struct {
		Data struct {
			Attributes    json.RawMessage `json:"attributes"`
			Relationships struct {
				Territory struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"territory"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode price create: %v", err)
	}
	return postedSubscriptionPrice{
		territory:  payload.Data.Relationships.Territory.Data.ID,
		attributes: string(payload.Data.Attributes),
	}
}

func runSubscriptionPriceCommand(t *testing.T, args []string) (string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, runErr
}

func TestSubscriptionPriceAttributeFlagsRejectInvalidValuesBeforeRequests(t *testing.T) {
	past := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	badCSV := filepath.Join(t.TempDir(), "prices.csv")
	if err := os.WriteFile(badCSV, []byte("territory,price,plan_type\nNOR,9.99,annual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	monthlyEnable := []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--subscription-id", "sub-1", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway", "--confirm"}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "prices set invalid plan type",
			args:    []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "sub-1", "--price-point", "pp-1", "--plan-type", "annual"},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "prices set rejects MONTHLY in USA",
			args:    []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "sub-1", "--price-point", "pp-1", "--territory", "US", "--plan-type", "MONTHLY"},
			wantErr: "--territory cannot be USA with plan type MONTHLY",
		},
		{
			name:    "prices set rejects MONTHLY force repair",
			args:    []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "sub-1", "--price-point", "pp-1", "--territory", "NOR", "--plan-type", "MONTHLY", "--force"},
			wantErr: "--force only repairs UPFRONT prices",
		},
		{
			name:    "prices import invalid plan type flag",
			args:    []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "sub-1", "--input", badCSV, "--plan-type", "annual", "--dry-run"},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "prices import invalid plan_type column",
			args:    []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "sub-1", "--input", badCSV, "--dry-run"},
			wantErr: "row 1: plan_type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "monthly commitment rejects a past start date",
			args:    append(slices.Clone(monthlyEnable), "--start-date", past),
			wantErr: "--start-date must be a future date",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestSubscriptionsPricesSet_PlanTypeReachesMatchAndWrite(t *testing.T) {
	tests := []struct {
		name          string
		flags         []string
		hasPrices     bool
		wantFilter    string
		wantAttribute string
	}{
		{name: "omitted flag sends no plan type", hasPrices: true},
		{name: "MONTHLY change", flags: []string{"--plan-type", "monthly"}, hasPrices: true, wantFilter: "MONTHLY", wantAttribute: `"planType":"MONTHLY"`},
		{name: "explicit UPFRONT initial price", flags: []string{"--plan-type", "UPFRONT"}, wantAttribute: `"planType":"UPFRONT"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			var written string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/relationships/prices":
					if test.hasPrices {
						return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"existing"}],"links":{}}`)
					}
					return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
					if got := req.URL.Query().Get("filter[planType]"); got != test.wantFilter {
						t.Fatalf("filter[planType] = %q, want %q", got, test.wantFilter)
					}
					return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
					body, _ := io.ReadAll(req.Body)
					written = string(body)
					return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"created"}}`)
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/8000000001":
					body, _ := io.ReadAll(req.Body)
					written = string(body)
					return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001"}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := append([]string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000001", "--price-point", "pp-1", "--territory", "NOR"}, test.flags...)
			if _, err := runSubscriptionPriceCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			if test.wantAttribute == "" && strings.Contains(written, "planType") {
				t.Fatalf("expected no planType in %s", written)
			}
			if !strings.Contains(written, test.wantAttribute) {
				t.Fatalf("expected %s in %s", test.wantAttribute, written)
			}
		})
	}
}

func TestSubscriptionsPricesImport_PlanTypeColumnAndDefault(t *testing.T) {
	existingUpfront := `{"data":[{"type":"subscriptionPrices","id":"price-upfront","attributes":{"startDate":"2026-08-01","planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"NOR"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-nor"}}}}],"links":{}}`
	tests := []struct {
		name       string
		csv        string
		flags      []string
		wantFilter string
		wantPosts  map[string]string
		wantFailed int
	}{
		{
			name:       "column mixes plans and rejects MONTHLY USA",
			csv:        "territory,price,start_date,plan_type,price_point_id\nNOR,9.99,2026-08-01,monthly,pp-nor\nDEU,9.99,2026-08-01,,pp-deu\nUSA,9.99,2026-08-01,MONTHLY,pp-usa\n",
			wantFilter: "MONTHLY,UPFRONT",
			wantPosts:  map[string]string{"NOR": "MONTHLY", "DEU": "UPFRONT"},
			wantFailed: 1,
		},
		{
			name:       "flag defaults rows without a value and the column wins",
			csv:        "territory,price,start_date,Plan Type,price_point_id\nNOR,9.99,2026-08-01,,pp-nor\nDEU,9.99,2026-08-01,UPFRONT,pp-deu\n",
			flags:      []string{"--plan-type", "MONTHLY"},
			wantFilter: "MONTHLY,UPFRONT",
			wantPosts:  map[string]string{"NOR": "MONTHLY", "DEU": "UPFRONT"},
		},
		{
			name:       "flag applies to a CSV without the column",
			csv:        "territory,price,start_date,price_point_id\nNOR,9.99,2026-08-01,pp-nor\n",
			flags:      []string{"--plan-type", "MONTHLY"},
			wantFilter: "MONTHLY",
			wantPosts:  map[string]string{"NOR": "MONTHLY"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			posts := map[string]string{}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
					if got := req.URL.Query().Get("filter[planType]"); got != test.wantFilter {
						t.Fatalf("filter[planType] = %q, want %q", got, test.wantFilter)
					}
					return jsonResponse(http.StatusOK, existingUpfront)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
					post := decodePostedSubscriptionPrice(t, req)
					var attrs struct {
						PlanType string `json:"planType"`
					}
					if err := json.Unmarshal([]byte(post.attributes), &attrs); err != nil {
						t.Fatalf("decode attributes: %v", err)
					}
					posts[post.territory] = attrs.PlanType
					return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"created"}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			csvPath := filepath.Join(t.TempDir(), "input.csv")
			if err := os.WriteFile(csvPath, []byte(test.csv), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}
			args := append([]string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csvPath, "--output", "json", "--confirm"}, test.flags...)
			stdout, runErr := runSubscriptionPriceCommand(t, args)
			if _, ok := errors.AsType[ReportedError](runErr); test.wantFailed > 0 && !ok {
				t.Fatalf("expected reported error, got %v", runErr)
			}
			if test.wantFailed == 0 && runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if len(posts) != len(test.wantPosts) {
				t.Fatalf("posts = %v, want %v", posts, test.wantPosts)
			}
			for territory, plan := range test.wantPosts {
				if posts[territory] != plan {
					t.Fatalf("posts = %v, want %v", posts, test.wantPosts)
				}
			}
			var summary struct {
				Failed int `json:"failed"`
			}
			if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
				t.Fatalf("parse JSON summary: %v; stdout=%q", err, stdout)
			}
			if summary.Failed != test.wantFailed {
				t.Fatalf("failed = %d, want %d; stdout=%s", summary.Failed, test.wantFailed, stdout)
			}
		})
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnable_StartDateAndPreserved(t *testing.T) {
	const scheduled = "2099-01-01"
	scheduledMonthly := `{
		"data":[{
			"type":"subscriptionPrices","id":"price-monthly-future",
			"attributes":{"startDate":"` + scheduled + `"},
			"relationships":{
				"territory":{"data":{"type":"territories","id":"NOR"}},
				"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-future"}}
			}
		}],
		"included":[{"type":"subscriptionPricePoints","id":"pp-future","attributes":{"customerPrice":"10.00"}}],
		"links":{"next":""}
	}`
	noMonthly := `{"data":[],"links":{"next":""}}`

	tests := []struct {
		name           string
		flags          []string
		monthlyPrices  string
		wantAttributes string
	}{
		{name: "omitted flags keep the MONTHLY-only body", monthlyPrices: noMonthly, wantAttributes: `{"planType":"MONTHLY"}`},
		{
			name:           "scheduled preserved price",
			flags:          []string{"--start-date", scheduled, "--preserved"},
			monthlyPrices:  noMonthly,
			wantAttributes: `{"startDate":"` + scheduled + `","preserveCurrentPrice":true,"planType":"MONTHLY"}`,
		},
		{name: "same price already scheduled on that date is not created again", flags: []string{"--start-date", scheduled}, monthlyPrices: scheduledMonthly},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			var posted []postedSubscriptionPrice
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
					return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"subscriptionPeriod":"ONE_YEAR"}}}`)
				case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
					if req.URL.Query().Get("filter[planType]") == "MONTHLY" {
						return jsonResponse(http.StatusOK, test.monthlyPrices)
					}
					return jsonResponse(http.StatusOK, `{
						"data":[{
							"type":"subscriptionPrices","id":"price-upfront",
							"attributes":{"startDate":"2024-01-01"},
							"relationships":{
								"territory":{"data":{"type":"territories","id":"NOR"}},
								"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-upfront"}}
							}
						}],
						"included":[
							{"type":"subscriptionPricePoints","id":"pp-upfront","attributes":{"customerPrice":"120.00"}},
							{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
						],
						"links":{"next":""}
					}`)
				case req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.Method == http.MethodGet:
					return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-monthly","attributes":{"customerPrice":"10.00"}}],"links":{"next":""}}`)
				case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
					return jsonResponse(http.StatusOK, `{"data":[]}`)
				case req.URL.Path == "/v1/subscriptionPlanAvailabilities" && req.Method == http.MethodPost:
					return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY"}}}`)
				case req.URL.Path == "/v1/subscriptionPrices" && req.Method == http.MethodPost:
					posted = append(posted, decodePostedSubscriptionPrice(t, req))
					return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"created"}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := append([]string{
				"subscriptions", "pricing", "monthly-commitment", "enable", "--confirm",
				"--subscription-id", "8000000001", "--price", "10.00",
				"--price-territory", "Norway", "--territories", "Norway",
			}, test.flags...)
			if _, err := runSubscriptionPriceCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}
			if test.wantAttributes == "" {
				if len(posted) != 0 {
					t.Fatalf("expected no create, got %+v", posted)
				}
				return
			}
			if len(posted) != 1 || posted[0].attributes != test.wantAttributes {
				t.Fatalf("posted = %+v, want attributes %s", posted, test.wantAttributes)
			}
		})
	}
}
