package shared

import "testing"

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
				RequireBaseTerritory: true,
			},
			free: true,
			want: true,
		},
		{
			name: "app setup free without base territory reuses existing schedule territory",
			config: PricingSetCommandConfig{
				ResolveBaseTerritory: true,
			},
			free: true,
			want: false,
		},
		{
			name: "app setup explicit base territory never requires another one",
			config: PricingSetCommandConfig{
				ResolveBaseTerritory: true,
			},
			baseTerritory: "USA",
			free:          true,
			want:          false,
		},
		{
			name: "app setup direct price point can omit base territory",
			config: PricingSetCommandConfig{
				ResolveBaseTerritory: true,
			},
			want: false,
		},
		{
			name: "tier-based pricing still requires base territory for resolution",
			config: PricingSetCommandConfig{
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
