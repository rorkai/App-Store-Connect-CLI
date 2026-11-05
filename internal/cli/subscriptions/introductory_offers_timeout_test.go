package subscriptions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestContextWithSubscriptionIntroductoryOfferCreateTimeoutUsesOperationDefault(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithSubscriptionIntroductoryOfferCreateTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected create operation deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 4*time.Minute+59*time.Second || remaining > 5*time.Minute+time.Second {
		t.Fatalf("expected create operation timeout near 5m, got %v", remaining)
	}

	requestCtx, requestCancel := shared.ContextWithTimeout(ctx)
	defer requestCancel()
	requestDeadline, ok := requestCtx.Deadline()
	if !ok {
		t.Fatal("expected individual request deadline")
	}
	requestRemaining := time.Until(requestDeadline)
	if requestRemaining < 29*time.Second || requestRemaining > 31*time.Second {
		t.Fatalf("expected individual request timeout near 30s, got %v", requestRemaining)
	}
}

func TestContextWithSubscriptionIntroductoryOfferCreateTimeoutRespectsASCTimeout(t *testing.T) {
	tests := []struct {
		name string
		env  string
		unit string
	}{
		{name: "ASC_TIMEOUT", env: "ASC_TIMEOUT", unit: "45s"},
		{name: "ASC_TIMEOUT_SECONDS", env: "ASC_TIMEOUT_SECONDS", unit: "45"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "")
			t.Setenv("ASC_TIMEOUT_SECONDS", "")
			if err := os.Unsetenv("ASC_TIMEOUT"); err != nil {
				t.Fatalf("unset ASC_TIMEOUT: %v", err)
			}
			t.Setenv(test.env, test.unit)

			ctx, cancel := contextWithSubscriptionIntroductoryOfferCreateTimeout(context.Background())
			defer cancel()

			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected create operation deadline")
			}
			remaining := time.Until(deadline)
			if remaining < 44*time.Second || remaining > 46*time.Second {
				t.Fatalf("expected create operation timeout near 45s from %s, got %v", test.env, remaining)
			}
		})
	}
}
