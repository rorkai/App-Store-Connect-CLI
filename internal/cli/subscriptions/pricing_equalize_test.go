package subscriptions

import (
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestNormalizeEqualizeStartDateUsesUSPacificDay(t *testing.T) {
	// 17:30 PDT on 2026-09-30.
	beforePacificMidnight := time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC)
	pacificMidnight := time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		startDate string
		now       time.Time
		wantDate  time.Time
		wantErr   bool
	}{
		{
			name:     "omitted start date evaluates prices on the Pacific day",
			now:      beforePacificMidnight,
			wantDate: time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "the UTC date is still in the future before Pacific midnight",
			startDate: "2026-10-01",
			now:       beforePacificMidnight,
			wantDate:  time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "the Pacific date is not in the future",
			startDate: "2026-09-30",
			now:       beforePacificMidnight,
			wantErr:   true,
		},
		{
			name:      "the date is not in the future from Pacific midnight",
			startDate: "2026-10-01",
			now:       pacificMidnight,
			wantErr:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := shared.DefaultDeps()
			d.Now = func() time.Time { return test.now }

			normalized, date, err := normalizeEqualizeStartDate(d, test.startDate)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected --start-date %q to be rejected, got date %s", test.startDate, date)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeEqualizeStartDate(%q) error: %v", test.startDate, err)
			}
			if normalized != test.startDate {
				t.Fatalf("normalized start date = %q, want %q", normalized, test.startDate)
			}
			if !date.Equal(test.wantDate) {
				t.Fatalf("pricing date = %s, want %s", date, test.wantDate)
			}
		})
	}
}
