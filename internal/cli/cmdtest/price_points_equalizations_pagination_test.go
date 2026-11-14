package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

type pricePointEqualizationsCommandCase struct {
	name          string
	commandPath   []string
	argsPrefix    []string
	parentFlag    string
	parentValue   string
	limitValue    string
	limitMax      int
	requestPath   string
	nextURL       string
	wantErrPrefix string
}

func pricePointEqualizationsCommandCases() []pricePointEqualizationsCommandCase {
	return []pricePointEqualizationsCommandCase{
		{
			name:          "app pricing",
			commandPath:   []string{"pricing", "price-points", "equalizations"},
			argsPrefix:    []string{"pricing", "price-points", "equalizations"},
			parentFlag:    "price-point",
			parentValue:   "pp-1",
			limitValue:    "175",
			limitMax:      200,
			requestPath:   "/v3/appPricePoints/pp-1/equalizations",
			nextURL:       "https://api.appstoreconnect.apple.com/v3/appPricePoints/pp-1/equalizations?cursor=AQ&limit=200",
			wantErrPrefix: "pricing price-points equalizations: --next",
		},
		{
			name:          "iap pricing",
			commandPath:   []string{"iap", "pricing", "price-points", "equalizations"},
			argsPrefix:    []string{"iap", "pricing", "price-points", "equalizations"},
			parentFlag:    "id",
			parentValue:   "pp-1",
			limitValue:    "500",
			limitMax:      8000,
			requestPath:   "/v1/inAppPurchasePricePoints/pp-1/equalizations",
			nextURL:       "https://api.appstoreconnect.apple.com/v1/inAppPurchasePricePoints/pp-1/equalizations?cursor=AQ&limit=8000",
			wantErrPrefix: "iap pricing price-points equalizations: --next",
		},
		{
			name:          "subscription pricing",
			commandPath:   []string{"subscriptions", "pricing", "price-points", "equalizations"},
			argsPrefix:    []string{"subscriptions", "pricing", "price-points", "equalizations"},
			parentFlag:    "price-point-id",
			parentValue:   "pp-1",
			limitValue:    "500",
			limitMax:      8000,
			requestPath:   "/v1/subscriptionPricePoints/pp-1/equalizations",
			nextURL:       "https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/pp-1/equalizations?cursor=AQ&limit=8000",
			wantErrPrefix: "subscriptions pricing price-points equalizations: --next",
		},
	}
}

func TestPricePointEqualizationsHelpShowsPaginationFlags(t *testing.T) {
	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			usage := usageForCommand(t, tt.commandPath...)
			for _, want := range []string{"--limit", "--next", "--paginate"} {
				if !strings.Contains(usage, want) {
					t.Fatalf("expected usage to contain %q, got %q", want, usage)
				}
			}
		})
	}
}

func TestPricePointEqualizationsLimit(t *testing.T) {
	setupAuth(t)

	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.requestPath, "limit="+tt.limitValue),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-limit"}],"links":{"next":""}}`))

			args := append(append([]string{}, tt.argsPrefix...), "--"+tt.parentFlag, tt.parentValue, "--limit", tt.limitValue, "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"id":"eq-limit"`) {
				t.Fatalf("expected equalization output, got %q", stdout)
			}
		})
	}
}

func TestPricePointEqualizationsRejectOutOfRangeLimit(t *testing.T) {
	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			args := append(append([]string{}, tt.argsPrefix...), "--"+tt.parentFlag, tt.parentValue, "--limit", strconv.Itoa(tt.limitMax+1))
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			wantErr := "Error: " + tt.wantErrPrefix[:strings.LastIndex(tt.wantErrPrefix, ":")] + ": --limit must be between 1 and " + strconv.Itoa(tt.limitMax)
			if !strings.Contains(stderr, wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestPricePointEqualizationsPaginate(t *testing.T) {
	setupAuth(t)

	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			secondURL := strings.Replace(tt.nextURL, "cursor=AQ", "cursor=BQ", 1)

			maxLimit := "limit=" + strconv.Itoa(tt.limitMax)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.requestPath, maxLimit),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-1"}],"links":{"next":"`+tt.nextURL+`"}}`))
			reg.Register(asctest.GET(tt.requestPath, "cursor=AQ", maxLimit),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-2"}],"links":{"next":"`+secondURL+`"}}`))
			reg.Register(asctest.GET(tt.requestPath, "cursor=BQ", maxLimit),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-3"}],"links":{"next":""}}`))

			args := append(append([]string{}, tt.argsPrefix...), "--"+tt.parentFlag, tt.parentValue, "--paginate", "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requestCount := len(reg.Requests()); requestCount != 3 {
				t.Fatalf("expected 3 paginated requests, got %d", requestCount)
			}
			for _, want := range []string{`"id":"eq-1"`, `"id":"eq-2"`, `"id":"eq-3"`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected output to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}

func TestPricePointEqualizationsPaginateRespectsRequestedLimit(t *testing.T) {
	setupAuth(t)

	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.requestPath, "limit="+tt.limitValue),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-custom-limit-1"}],"links":{"next":"`+tt.nextURL+`"}}`))
			reg.Register(asctest.GET(tt.requestPath, "cursor=AQ", "limit="+strconv.Itoa(tt.limitMax)),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-custom-limit-2"}],"links":{"next":""}}`))

			args := append(append([]string{}, tt.argsPrefix...), "--"+tt.parentFlag, tt.parentValue, "--paginate", "--limit", tt.limitValue, "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requestCount := len(reg.Requests()); requestCount != 2 {
				t.Fatalf("expected 2 paginated requests, got %d", requestCount)
			}
			for _, want := range []string{`"id":"eq-custom-limit-1"`, `"id":"eq-custom-limit-2"`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected output to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}

func TestPricePointEqualizationsWithoutPaginateUsesSinglePage(t *testing.T) {
	setupAuth(t)

	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.requestPath),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-1"}],"links":{"next":"`+tt.nextURL+`"}}`))

			args := append(append([]string{}, tt.argsPrefix...), "--"+tt.parentFlag, tt.parentValue, "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			wantWarning := "Warning: showing 1 results; more pages exist (use --paginate or --next where supported)\n"
			if stderr != wantWarning {
				t.Fatalf("expected truncation warning on stderr, got %q", stderr)
			}
			if requestCount := len(reg.Requests()); requestCount != 1 {
				t.Fatalf("expected exactly one request without --paginate, got %d", requestCount)
			}
			if !strings.Contains(stdout, `"id":"eq-1"`) {
				t.Fatalf("expected first page output, got %q", stdout)
			}
		})
	}
}

func TestPricePointEqualizationsRejectInvalidNextURL(t *testing.T) {
	tests := []struct {
		name string
		next string
	}{
		{
			name: "invalid scheme",
			next: "http://api.appstoreconnect.apple.com/v1/appPricePoints/pp-1/equalizations?cursor=AQ",
		},
		{
			name: "malformed URL",
			next: "https://api.appstoreconnect.apple.com/%zz",
		},
	}

	for _, tt := range pricePointEqualizationsCommandCases() {
		for _, bad := range tests {
			t.Run(tt.name+" "+bad.name, func(t *testing.T) {
				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)

				var runErr error
				args := append(append([]string{}, tt.argsPrefix...), "--next", bad.next)
				stdout, stderr := captureOutput(t, func() {
					if err := root.Parse(args); err != nil {
						t.Fatalf("parse error: %v", err)
					}
					runErr = root.Run(context.Background())
				})

				if runErr == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", runErr)
				}
				wantErr := "Error: " + tt.wantErrPrefix
				if !strings.Contains(stderr, wantErr) {
					t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
				}
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
			})
		}
	}
}

func TestPricePointEqualizationsPaginateFromNextWithoutParentID(t *testing.T) {
	setupAuth(t)

	for _, tt := range pricePointEqualizationsCommandCases() {
		t.Run(tt.name, func(t *testing.T) {
			secondURL := strings.Replace(tt.nextURL, "cursor=AQ", "cursor=BQ", 1)

			maxLimit := "limit=" + strconv.Itoa(tt.limitMax)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(tt.requestPath, "cursor=AQ", maxLimit),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-next-1"}],"links":{"next":"`+secondURL+`"}}`))
			reg.Register(asctest.GET(tt.requestPath, "cursor=BQ", maxLimit),
				asctest.JSON(http.StatusOK, `{"data":[{"id":"eq-next-2"}],"links":{"next":""}}`))

			args := append(append([]string{}, tt.argsPrefix...), "--paginate", "--next", tt.nextURL, "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requestCount := len(reg.Requests()); requestCount != 2 {
				t.Fatalf("expected 2 paginated requests from --next, got %d", requestCount)
			}
			for _, want := range []string{`"id":"eq-next-1"`, `"id":"eq-next-2"`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected output to contain %q, got %q", want, stdout)
				}
			}
		})
	}
}
