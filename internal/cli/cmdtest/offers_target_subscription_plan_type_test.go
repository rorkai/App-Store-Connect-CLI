package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const targetPlanTypeFlag = "--target-subscription-plan-type"

func offerCreateCommandsForPlanType() []struct {
	name string
	args []string
	path string
} {
	return []struct {
		name string
		args []string
		path string
	}{
		{
			name: "introductory",
			args: []string{"subscriptions", "offers", "introductory", "create", "--subscription-id", "8000000001", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--territory", "USA"},
			path: "/v1/subscriptionIntroductoryOffers",
		},
		{
			name: "promotional",
			args: []string{"subscriptions", "offers", "promotional", "create", "--subscription-id", "8000000001", "--offer-code", "SPRING", "--name", "Spring", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--prices", "USA"},
			path: "/v1/subscriptionPromotionalOffers",
		},
		{
			name: "subscription offer code",
			args: []string{"subscriptions", "offers", "offer-codes", "create", "--subscription-id", "8000000001", "--name", "SPRING", "--offer-eligibility", "STACK_WITH_INTRO_OFFERS", "--customer-eligibilities", "NEW", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--prices", "USA"},
			path: "/v1/subscriptionOfferCodes",
		},
		{
			name: "win-back",
			args: []string{"subscriptions", "offers", "win-back", "create", "--subscription-id", "8000000001", "--reference-name", "spring-2026", "--offer-id", "OFFER-1", "--duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL", "--period-count", "1", "--eligibility-paid-months", "6", "--eligibility-last-subscribed-min", "3", "--start-date", "2026-02-01", "--priority", "HIGH", "--territory", "USA"},
			path: "/v1/winBackOffers",
		},
	}
}

// Catches a create command that drops the flag, and one that starts sending
// the attribute when the flag is omitted.
func TestOfferCreateCommandsSendTargetSubscriptionPlanTypeOnlyWhenSet(t *testing.T) {
	for _, command := range offerCreateCommandsForPlanType() {
		for _, test := range []struct {
			name      string
			extraArgs []string
			want      any
		}{
			{name: "omitted", want: nil},
			{name: "monthly", extraArgs: []string{targetPlanTypeFlag, "monthly"}, want: "MONTHLY"},
			{name: "upfront", extraArgs: []string{targetPlanTypeFlag, "UPFRONT"}, want: "UPFRONT"},
		} {
			t.Run(command.name+"/"+test.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

				reg := asctest.NewRegistry(t)
				reg.Install()
				reg.Register(asctest.POST(command.path), asctest.JSON(http.StatusCreated, `{"data":{"type":"offers","id":"offer-1"}}`))

				if _, _, err := runCommand(t, append(append([]string{}, command.args...), test.extraArgs...)); err != nil {
					t.Fatalf("run error: %v", err)
				}
				requests := reg.Requests()
				if posts := len(requests); posts != 1 {
					t.Fatalf("expected one create request, got %d", posts)
				}
				var payload struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal(requests[0].Body, &payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				got, present := payload.Data.Attributes["targetSubscriptionPlanType"]
				if present != (test.want != nil) || got != test.want {
					t.Fatalf("targetSubscriptionPlanType = %#v (present %t), want %#v", got, present, test.want)
				}
			})
		}
	}
}

// Catches a typo reaching Apple, a blank value silently meaning "omitted", and
// validation placed after a path that returns early, such as a dry run.
func TestOfferCreateCommandsRejectInvalidTargetSubscriptionPlanTypeBeforeHTTP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

	type command struct {
		name string
		args []string
	}
	commands := make([]command, 0, 6)
	for _, create := range offerCreateCommandsForPlanType() {
		commands = append(commands, command{name: create.name, args: create.args})
	}
	commands = append(commands,
		command{
			name: "introductory dry run",
			args: []string{"subscriptions", "offers", "introductory", "create", "--subscription-id", "8000000001", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--territory", "USA", "--dry-run"},
		},
		command{
			name: "introductory import",
			args: []string{"subscriptions", "offers", "introductory", "import", "--subscription-id", "8000000001", "--input", writeTempIntroOffersCSV(t, "territory\nUSA\n"), "--dry-run"},
		},
	)

	for _, test := range commands {
		for _, value := range []string{"WEEKLY", "", "  "} {
			t.Run(test.name+"/"+value, func(t *testing.T) {
				wantErr := targetPlanTypeFlag + " must be one of: MONTHLY, UPFRONT"
				if strings.TrimSpace(value) == "" {
					wantErr = "invalid value for " + targetPlanTypeFlag + ": cannot be empty"
				}
				assertUsageExit(t, append(append([]string{}, test.args...), targetPlanTypeFlag, value), wantErr)
			})
		}
	}
}

// Catches typed response attributes dropping Apple's value from JSON, a table
// that hides it, and a win-back --fields allow-list that refuses it.
func TestOfferListCommandsShowTargetSubscriptionPlanType(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		path       string
		resource   string
		wantFields string
	}{
		{name: "introductory", args: []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "8000000001"}, path: "/v1/subscriptions/8000000001/introductoryOffers", resource: "subscriptionIntroductoryOffers"},
		{name: "promotional", args: []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "8000000001"}, path: "/v1/subscriptions/8000000001/promotionalOffers", resource: "subscriptionPromotionalOffers"},
		{name: "offer code", args: []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "8000000001"}, path: "/v1/subscriptions/8000000001/offerCodes", resource: "subscriptionOfferCodes"},
		{name: "win-back", args: []string{"subscriptions", "offers", "win-back", "list", "--subscription-id", "8000000001", "--fields", "targetSubscriptionPlanType"}, path: "/v1/subscriptions/8000000001/winBackOffers", resource: "winBackOffers", wantFields: "targetSubscriptionPlanType"},
	}

	for _, test := range tests {
		for _, format := range []string{"json", "table"} {
			t.Run(test.name+"/"+format, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

				var query []string
				if test.wantFields != "" {
					query = append(query, "fields[winBackOffers]="+test.wantFields)
				}
				reg := asctest.NewRegistry(t)
				reg.Install()
				reg.Register(asctest.GET(test.path, query...),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"`+test.resource+`","id":"offer-1","attributes":{"targetSubscriptionPlanType":"MONTHLY"}}],"links":{}}`))

				stdout, _, err := runCommand(t, append(append([]string{}, test.args...), "--output", format))
				if err != nil {
					t.Fatalf("run error: %v", err)
				}

				if format == "json" {
					var envelope struct {
						Data []struct {
							Attributes struct {
								TargetSubscriptionPlanType string `json:"targetSubscriptionPlanType"`
							} `json:"attributes"`
						} `json:"data"`
					}
					if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
						t.Fatalf("parse JSON: %v", err)
					}
					if len(envelope.Data) != 1 || envelope.Data[0].Attributes.TargetSubscriptionPlanType != "MONTHLY" {
						t.Fatalf("expected MONTHLY in the envelope, got %s", stdout)
					}
					return
				}
				if !strings.Contains(stdout, "Target Plan Type") || !strings.Contains(stdout, "MONTHLY") {
					t.Fatalf("expected the plan type column, got %q", stdout)
				}
			})
		}
	}
}

// Catches --all-territories reporting "already exists" and creating nothing
// when the only offer in a territory targets the other billing plan.
func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesScopesExistingOffersToTargetPlanType(t *testing.T) {
	tests := []struct {
		name        string
		extraArgs   []string
		existing    string
		wantCreated int
		wantSkipped int
	}{
		{name: "other plan does not block the target", extraArgs: []string{targetPlanTypeFlag, "MONTHLY"}, existing: `"attributes":{"targetSubscriptionPlanType":"UPFRONT"},`, wantCreated: 1},
		{name: "same plan is skipped", extraArgs: []string{targetPlanTypeFlag, "MONTHLY"}, existing: `"attributes":{"targetSubscriptionPlanType":"MONTHLY"},`, wantSkipped: 1},
		{name: "offer without a plan is skipped", extraArgs: []string{targetPlanTypeFlag, "MONTHLY"}, wantSkipped: 1},
		{name: "no flag keeps skipping any offer", existing: `"attributes":{"targetSubscriptionPlanType":"MONTHLY"},`, wantSkipped: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/subscriptions/8000000001/subscriptionAvailability"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`))
			reg.Register(asctest.GET("/v1/subscriptionAvailabilities/avail-1/availableTerritories", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{}}`))
			reg.Register(asctest.GET("/v1/subscriptions/8000000001/introductoryOffers", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"intro-existing",`+test.existing+`"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`))
			if test.wantCreated == 1 {
				reg.Register(asctest.POST("/v1/subscriptionIntroductoryOffers").JSONBody(`{"data":{"type":"subscriptionIntroductoryOffers","attributes":{"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"MONTHLY"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"territory":{"data":{"type":"territories","id":"USA"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`))
			}

			args := []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--all-territories",
				"--output", "json",
			}
			stdout, _, err := runCommand(t, append(args, test.extraArgs...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var summary struct {
				Created int `json:"created"`
				Skipped int `json:"skipped"`
			}
			if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
				t.Fatalf("parse JSON summary: %v", err)
			}
			posts := 0
			for _, req := range reg.Requests() {
				if req.Method == http.MethodPost {
					posts++
				}
			}
			if summary.Created != test.wantCreated || summary.Skipped != test.wantSkipped || posts != test.wantCreated {
				t.Fatalf("created=%d skipped=%d posts=%d, want created=%d skipped=%d", summary.Created, summary.Skipped, posts, test.wantCreated, test.wantSkipped)
			}
		})
	}
}

// Catches the import hard-coding UPFRONT over a requested plan, a blank cell
// ignoring the flag default, and a file without the column changing behavior.
func TestSubscriptionsIntroductoryOffersImportTargetSubscriptionPlanType(t *testing.T) {
	tests := []struct {
		name      string
		csv       string
		extraArgs []string
		want      []string
	}{
		{name: "no column and no flag stays upfront", csv: "territory\nUSA\n", want: []string{"UPFRONT"}},
		{name: "column", csv: "territory,target_subscription_plan_type\nUSA,monthly\nFRA,UPFRONT\n", want: []string{"MONTHLY", "UPFRONT"}},
		{name: "flag fills blanks and rows override it", csv: "territory,target_subscription_plan_type\nUSA,\nFRA,UPFRONT\n", extraArgs: []string{targetPlanTypeFlag, "MONTHLY"}, want: []string{"MONTHLY", "UPFRONT"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/subscriptions/8000000003/introductoryOffers", "fields[subscriptionIntroductoryOffers]=startDate,endDate,duration,offerMode,numberOfPeriods,targetSubscriptionPlanType,territory,subscriptionPricePoint", "include=territory,subscriptionPricePoint", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
			for index, planType := range test.want {
				territory := []string{"USA", "FRA"}[index]
				reg.Register(asctest.POST("/v1/subscriptionIntroductoryOffers").JSONBody(`{"data":{"type":"subscriptionIntroductoryOffers","attributes":{"duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"`+planType+`"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000003"}},"territory":{"data":{"type":"territories","id":"`+territory+`"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`))
			}

			args := []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "8000000003",
				"--input", writeTempIntroOffersCSV(t, test.csv),
				"--offer-duration", "ONE_WEEK",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--confirm",
			}
			stdout, _, err := runCommand(t, append(args, test.extraArgs...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var summary struct {
				Results []struct {
					TargetSubscriptionPlanType string `json:"targetSubscriptionPlanType"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
				t.Fatalf("parse JSON summary: %v", err)
			}
			if len(summary.Results) != len(test.want) {
				t.Fatalf("results = %+v, want %d", summary.Results, len(test.want))
			}
			for index, result := range summary.Results {
				if result.TargetSubscriptionPlanType != test.want[index] {
					t.Fatalf("result %d plan type = %q, want %q", index, result.TargetSubscriptionPlanType, test.want[index])
				}
			}
		})
	}
}

// Catches a bad cell being sent to Apple row by row after earlier rows landed.
func TestSubscriptionsIntroductoryOffersImportRejectsInvalidRowPlanTypeBeforeHTTP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

	assertUsageExit(t, []string{
		"subscriptions", "offers", "introductory", "import",
		"--subscription-id", "8000000003",
		"--input", writeTempIntroOffersCSV(t, "territory,target_subscription_plan_type\nUSA,MONTHLY\nFRA,WEEKLY\n"),
		"--offer-duration", "ONE_WEEK",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
		"--confirm",
	}, "row 2: target_subscription_plan_type must be one of: MONTHLY, UPFRONT")
}
