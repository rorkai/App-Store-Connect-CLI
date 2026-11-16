package shared

import (
	"testing"
	"time"
)

func TestRequiresExplicitBaseTerritory(t *testing.T) {
	tests := []struct {
		name          string
		config        PricingSetCommandConfig
		baseTerritory string
		tier          int
		price         string
		free          bool
		want          bool
	}{
		{
			name: "schedule create free without base territory still requires explicit territory",
			config: PricingSetCommandConfig{
				Deps:                 DefaultDeps(),
				RequireBaseTerritory: true,
			},
			free: true,
			want: true,
		},
		{
			name: "app setup free without base territory reuses existing schedule territory",
			config: PricingSetCommandConfig{
				Deps:                 DefaultDeps(),
				ResolveBaseTerritory: true,
			},
			free: true,
			want: false,
		},
		{
			name: "app setup explicit base territory never requires another one",
			config: PricingSetCommandConfig{
				Deps:                 DefaultDeps(),
				ResolveBaseTerritory: true,
			},
			baseTerritory: "USA",
			free:          true,
			want:          false,
		},
		{
			name: "app setup direct price point can omit base territory",
			config: PricingSetCommandConfig{
				Deps:                 DefaultDeps(),
				ResolveBaseTerritory: true,
			},
			want: false,
		},
		{
			name: "tier-based pricing still requires base territory for resolution",
			config: PricingSetCommandConfig{
				Deps:                 DefaultDeps(),
				ResolveBaseTerritory: true,
			},
			tier: 1,
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := requiresExplicitBaseTerritory(tc.config, tc.baseTerritory, tc.tier, tc.price, tc.free)
			if got != tc.want {
				t.Fatalf("requiresExplicitBaseTerritory() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestPricingDefaultStartDateUsesUSPacificCalendar(t *testing.T) {
	tests := []struct {
		now  string
		want string
	}{
		// Live evidence (2026-09-27): App Store Connect rejected the UTC date
		// and accepted the Pacific date in the same minute.
		{now: "2026-09-27T00:03:00Z", want: "2026-09-26"},
		{now: "2026-09-27T06:59:59Z", want: "2026-09-26"},
		{now: "2026-09-27T07:00:00Z", want: "2026-09-27"},
		{now: "2026-09-27T08:30:00Z", want: "2026-09-27"},
		// Spring forward: 2026-03-08 02:00 PST becomes 03:00 PDT (10:00 UTC).
		{now: "2026-03-08T07:59:59Z", want: "2026-03-07"},
		{now: "2026-03-08T08:00:00Z", want: "2026-03-08"},
		{now: "2026-03-09T06:59:59Z", want: "2026-03-08"},
		{now: "2026-03-09T07:00:00Z", want: "2026-03-09"},
		// Fall back: 2026-11-01 02:00 PDT becomes 01:00 PST (09:00 UTC).
		{now: "2026-11-01T06:59:59Z", want: "2026-10-31"},
		{now: "2026-11-01T07:00:00Z", want: "2026-11-01"},
		{now: "2026-11-02T07:59:59Z", want: "2026-11-01"},
		{now: "2026-11-02T08:00:00Z", want: "2026-11-02"},
		// The input's own location must not matter.
		{now: "2026-09-27T09:03:00+09:00", want: "2026-09-26"},
	}

	for _, test := range tests {
		t.Run(test.now, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.now)
			if err != nil {
				t.Fatalf("parse now: %v", err)
			}
			if got := pricingDefaultStartDate(now); got != test.want {
				t.Fatalf("pricingDefaultStartDate(%s) = %q, want %q", test.now, got, test.want)
			}
		})
	}
}
