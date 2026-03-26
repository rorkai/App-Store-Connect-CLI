package subscriptions

import (
	"slices"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPlanSubscriptionMoveToPeerSlot(t *testing.T) {
	tests := []struct {
		name           string
		subscriptions  []asc.Resource[asc.SubscriptionAttributes]
		sourceID       string
		peerID         string
		wantOrder      []string
		wantFromLevel  int
		wantToLevel    int
		wantChangedIDs []string
		wantErr        string
	}{
		{
			name: "move source into earlier slot",
			subscriptions: []asc.Resource[asc.SubscriptionAttributes]{
				{ID: "sub-weekly", Attributes: asc.SubscriptionAttributes{GroupLevel: 1}},
				{ID: "sub-monthly", Attributes: asc.SubscriptionAttributes{GroupLevel: 2}},
				{ID: "sub-yearly", Attributes: asc.SubscriptionAttributes{GroupLevel: 3}},
			},
			sourceID:       "sub-yearly",
			peerID:         "sub-monthly",
			wantOrder:      []string{"sub-weekly", "sub-yearly", "sub-monthly"},
			wantFromLevel:  3,
			wantToLevel:    2,
			wantChangedIDs: []string{"sub-monthly", "sub-yearly"},
		},
		{
			name: "move source into later slot",
			subscriptions: []asc.Resource[asc.SubscriptionAttributes]{
				{ID: "sub-weekly", Attributes: asc.SubscriptionAttributes{GroupLevel: 1}},
				{ID: "sub-monthly", Attributes: asc.SubscriptionAttributes{GroupLevel: 2}},
				{ID: "sub-yearly", Attributes: asc.SubscriptionAttributes{GroupLevel: 3}},
			},
			sourceID:       "sub-monthly",
			peerID:         "sub-yearly",
			wantOrder:      []string{"sub-weekly", "sub-yearly", "sub-monthly"},
			wantFromLevel:  2,
			wantToLevel:    3,
			wantChangedIDs: []string{"sub-monthly", "sub-yearly"},
		},
		{
			name: "missing peer fails",
			subscriptions: []asc.Resource[asc.SubscriptionAttributes]{
				{ID: "sub-weekly", Attributes: asc.SubscriptionAttributes{GroupLevel: 1}},
				{ID: "sub-monthly", Attributes: asc.SubscriptionAttributes{GroupLevel: 2}},
			},
			sourceID: "sub-monthly",
			peerID:   "sub-missing",
			wantErr:  `subscription "sub-missing" not found in the subscription group`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := planSubscriptionMoveToPeerSlot(test.subscriptions, test.sourceID, test.peerID)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", test.wantErr)
				}
				if err.Error() != test.wantErr {
					t.Fatalf("expected error %q, got %q", test.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(plan.DesiredOrder, test.wantOrder) {
				t.Fatalf("expected desired order %v, got %v", test.wantOrder, plan.DesiredOrder)
			}
			if plan.SourceFromLevel != test.wantFromLevel {
				t.Fatalf("expected source from level %d, got %d", test.wantFromLevel, plan.SourceFromLevel)
			}
			if plan.SourceToLevel != test.wantToLevel {
				t.Fatalf("expected source to level %d, got %d", test.wantToLevel, plan.SourceToLevel)
			}
			if !slices.Equal(plan.ChangedLevelIDs, test.wantChangedIDs) {
				t.Fatalf("expected changed ids %v, got %v", test.wantChangedIDs, plan.ChangedLevelIDs)
			}

			for idx, subID := range test.wantOrder {
				wantLevel := idx + 1
				if got := plan.TargetLevels[subID]; got != wantLevel {
					t.Fatalf("expected target level %d for %s, got %d", wantLevel, subID, got)
				}
			}
		})
	}
}
