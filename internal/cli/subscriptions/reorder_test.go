package subscriptions

import (
	"slices"
	"strings"
	"testing"
)

func TestSubscriptionsReorderCommand_FlagDefinitions(t *testing.T) {
	cmd := SubscriptionsReorderCommand()

	expectedFlags := []string{"id", "before", "after", "top", "bottom", "output", "pretty"}
	for _, name := range expectedFlags {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected --%s flag to be defined", name)
		}
	}
}

func TestSubscriptionsReorderCommand_HelpContainsExamples(t *testing.T) {
	cmd := SubscriptionsReorderCommand()

	for _, snippet := range []string{
		`--before "OTHER_SUB_ID"`,
		`--after "OTHER_SUB_ID"`,
		`--top`,
		`--bottom`,
	} {
		if !strings.Contains(cmd.LongHelp, snippet) {
			t.Fatalf("expected help text to contain %q, got %q", snippet, cmd.LongHelp)
		}
	}
}

func TestPlanSubscriptionReorder(t *testing.T) {
	tests := []struct {
		name         string
		currentOrder []string
		sourceID     string
		placement    subscriptionReorderPlacement
		anchorID     string
		wantOrder    []string
		wantFrom     int
		wantTo       int
		wantChanged  bool
		wantErr      string
	}{
		{
			name:         "move before anchor",
			currentOrder: []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			sourceID:     "sub-yearly",
			placement:    subscriptionReorderPlacementBefore,
			anchorID:     "sub-monthly",
			wantOrder:    []string{"sub-weekly", "sub-yearly", "sub-monthly"},
			wantFrom:     3,
			wantTo:       2,
			wantChanged:  true,
		},
		{
			name:         "move after anchor",
			currentOrder: []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			sourceID:     "sub-weekly",
			placement:    subscriptionReorderPlacementAfter,
			anchorID:     "sub-monthly",
			wantOrder:    []string{"sub-monthly", "sub-weekly", "sub-yearly"},
			wantFrom:     1,
			wantTo:       2,
			wantChanged:  true,
		},
		{
			name:         "move to top",
			currentOrder: []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			sourceID:     "sub-yearly",
			placement:    subscriptionReorderPlacementTop,
			wantOrder:    []string{"sub-yearly", "sub-weekly", "sub-monthly"},
			wantFrom:     3,
			wantTo:       1,
			wantChanged:  true,
		},
		{
			name:         "already bottom is no-op",
			currentOrder: []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			sourceID:     "sub-yearly",
			placement:    subscriptionReorderPlacementBottom,
			wantOrder:    []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			wantFrom:     3,
			wantTo:       3,
			wantChanged:  false,
		},
		{
			name:         "target missing",
			currentOrder: []string{"sub-weekly", "sub-monthly", "sub-yearly"},
			sourceID:     "sub-yearly",
			placement:    subscriptionReorderPlacementBefore,
			anchorID:     "sub-missing",
			wantErr:      "not found in the subscription group",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := planSubscriptionReorder(test.currentOrder, test.sourceID, test.placement, test.anchorID)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.FromGroupLevel != test.wantFrom {
				t.Fatalf("expected from group level %d, got %d", test.wantFrom, plan.FromGroupLevel)
			}
			if plan.ToGroupLevel != test.wantTo {
				t.Fatalf("expected to group level %d, got %d", test.wantTo, plan.ToGroupLevel)
			}
			if plan.Changed != test.wantChanged {
				t.Fatalf("expected changed=%t, got %t", test.wantChanged, plan.Changed)
			}
			if !slices.Equal(plan.DesiredOrder, test.wantOrder) {
				t.Fatalf("expected desired order %v, got %v", test.wantOrder, plan.DesiredOrder)
			}
		})
	}
}
