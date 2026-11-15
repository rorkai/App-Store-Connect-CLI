package subscriptions

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReadSubscriptionPricesImportCSV_SupportsHeaderAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"Countries or Regions,Currency Code,Price,start_date,preserved,ignored\n" +
		"USA,USD,19.99,2026-03-01,false,foo\n" +
		"Afghanistan,AFN,299.00,2026-03-01,true,bar\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readSubscriptionPricesImportCSV(shared.DefaultDeps(), path)
	if err != nil {
		t.Fatalf("readSubscriptionPricesImportCSV() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].territory != "USA" || rows[0].currencyCode != "USD" || rows[0].price != "19.99" {
		t.Fatalf("unexpected row[0]: %+v", rows[0])
	}
	if !rows[1].preserveSet || !rows[1].preserveCurrentPrice {
		t.Fatalf("expected row[1] preserved=true, got %+v", rows[1])
	}
}

func TestReadSubscriptionPricesImportCSV_DuplicateKnownColumnReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,Countries or Regions,price\n" +
		"USA,USA,19.99\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(shared.DefaultDeps(), path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReadSubscriptionPricesImportCSV_InvalidDateReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,price,start_date\n" +
		"USA,19.99,2026-13-01\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(shared.DefaultDeps(), path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReadSubscriptionPricesImportCSV_InvalidBooleanReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,price,preserve_current_price\n" +
		"USA,19.99,yes\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(shared.DefaultDeps(), path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_MapsCommonNames(t *testing.T) {
	got, err := resolveSubscriptionPriceImportTerritoryID("Afghanistan")
	if err != nil {
		t.Fatalf("resolveSubscriptionPriceImportTerritoryID() error: %v", err)
	}
	if got != "AFG" {
		t.Fatalf("expected AFG, got %q", got)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_RejectsUnknownThreeLetterCode(t *testing.T) {
	_, err := resolveSubscriptionPriceImportTerritoryID("ZZZ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_AcceptsSupportedThreeLetterCodeWithoutDisplayName(t *testing.T) {
	got, err := resolveSubscriptionPriceImportTerritoryID("ANT")
	if err != nil {
		t.Fatalf("resolveSubscriptionPriceImportTerritoryID() error: %v", err)
	}
	if got != "ANT" {
		t.Fatalf("expected ANT, got %q", got)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_RejectsTerritoriesOutsideASCSet(t *testing.T) {
	tests := []string{"ATA", "AQ", "Antarctica"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := resolveSubscriptionPriceImportTerritoryID(input)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", input)
			}
		})
	}
}

func TestWriteSubscriptionPriceImportFailureArtifact_ReturnsWriteError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := writeSubscriptionPriceImportFailureArtifact(shared.DefaultDeps(), &subscriptionPriceImportSummary{
		Failed:  1,
		Results: []subscriptionPriceImportResultItem{{Status: "failed"}},
	})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

func TestSubscriptionPriceImportStateMatchesIgnoresUnspecifiedPreservedValue(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		states: []subscriptionPriceImportState{{
			territoryID:          "USA",
			pricePointID:         "pp-usa",
			startDate:            "2026-07-01",
			preserveCurrentPrice: true,
			planType:             asc.SubscriptionPlanTypeUpfront,
		}},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID:  "USA",
		pricePointID: "pp-usa",
		startDate:    "2026-07-01",
		preserveSet:  false,
		planType:     asc.SubscriptionPlanTypeUpfront,
	}

	if !index.matches(target) {
		t.Fatal("expected omitted preserved value to match either remote state")
	}
}

func TestSubscriptionPriceImportStateMatchesCanonicalSameDayPrice(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		now: time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC),
		states: []subscriptionPriceImportState{
			{territoryID: "USA", pricePointID: "target", startDate: "2026-07-01", preserveCurrentPrice: true, planType: asc.SubscriptionPlanTypeUpfront},
			{territoryID: "USA", pricePointID: "canonical", startDate: "2026-07-01", preserveCurrentPrice: false, planType: asc.SubscriptionPlanTypeUpfront},
		},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID:  "USA",
		pricePointID: "target",
		planType:     asc.SubscriptionPlanTypeUpfront,
	}
	if index.matches(target) {
		t.Fatal("expected the same-day non-preserved canonical price to win")
	}

	target.pricePointID = "canonical"
	if !index.matches(target) {
		t.Fatal("expected the canonical non-preserved price to match")
	}
}

func TestSubscriptionPriceImportStateSelectsCanonicalWhenPreserveIsExplicit(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		now: time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC),
		states: []subscriptionPriceImportState{
			{territoryID: "USA", pricePointID: "target", startDate: "2026-07-01", preserveCurrentPrice: true, planType: asc.SubscriptionPlanTypeUpfront},
			{territoryID: "USA", pricePointID: "canonical", startDate: "2026-07-01", preserveCurrentPrice: false, planType: asc.SubscriptionPlanTypeUpfront},
		},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID:          "USA",
		pricePointID:         "target",
		preserveSet:          true,
		preserveCurrentPrice: true,
		planType:             asc.SubscriptionPlanTypeUpfront,
	}
	if index.matches(target) {
		t.Fatal("expected explicit preserved matching to compare against the canonical row")
	}

	target.pricePointID = "canonical"
	if !index.matches(target) {
		t.Fatal("expected preserveCurrentPrice not to be compared with the canonical row's historical preserved state")
	}
}

func TestSubscriptionPriceImportStateExplicitDateMatchesEitherPreservedValue(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		states: []subscriptionPriceImportState{
			{territoryID: "USA", pricePointID: "target", startDate: "2026-07-01", preserveCurrentPrice: true, planType: asc.SubscriptionPlanTypeUpfront},
			{territoryID: "USA", pricePointID: "other", startDate: "2026-07-01", preserveCurrentPrice: false, planType: asc.SubscriptionPlanTypeUpfront},
		},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID: "USA", pricePointID: "target", startDate: "2026-07-01", planType: asc.SubscriptionPlanTypeUpfront,
	}
	if !index.matches(target) {
		t.Fatal("expected an explicit-date target with omitted preserve to match either preserved value")
	}
}

func TestSubscriptionPriceImportStateExplicitDateIgnoresCreatePreserveOption(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		states: []subscriptionPriceImportState{
			{territoryID: "USA", pricePointID: "target", startDate: "2026-08-15", preserveCurrentPrice: false, planType: asc.SubscriptionPlanTypeUpfront},
		},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID:          "USA",
		pricePointID:         "target",
		startDate:            "2026-08-15",
		preserveSet:          true,
		preserveCurrentPrice: true,
		planType:             asc.SubscriptionPlanTypeUpfront,
	}
	if !index.matches(target) {
		t.Fatal("expected preserveCurrentPrice create option not to be compared with response preserved state")
	}
}

func TestSubscriptionPriceImportStateRejectsMonthlyPrice(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{
		now: time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC),
		states: []subscriptionPriceImportState{{
			territoryID: "USA", pricePointID: "pp-usa", startDate: "2026-07-01", planType: asc.SubscriptionPlanTypeMonthly,
		}},
	}
	target := subscriptionPriceImportResolvedRow{
		territoryID: "USA", pricePointID: "pp-usa", planType: asc.SubscriptionPlanTypeUpfront,
	}
	if index.matches(target) {
		t.Fatal("expected a MONTHLY price not to satisfy an UPFRONT import")
	}
}

func TestSubscriptionPriceImportStateUsesUSPacificDay(t *testing.T) {
	// The 2026-10-01 price ends the 2026-01-01 price on its start date.
	states := []subscriptionPriceImportState{
		{territoryID: "USA", pricePointID: "pp-old", startDate: "2026-01-01", planType: asc.SubscriptionPlanTypeUpfront},
		{territoryID: "USA", pricePointID: "pp-new", startDate: "2026-10-01", planType: asc.SubscriptionPlanTypeUpfront},
	}
	tests := []struct {
		name    string
		now     time.Time
		current string
	}{
		// 17:30 PDT on 2026-09-30: the 2026-10-01 price is still scheduled.
		{name: "00:30 UTC is the previous Pacific day", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), current: "pp-old"},
		{name: "Pacific midnight starts the next price", now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), current: "pp-new"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := &subscriptionPriceImportStateIndex{now: test.now, states: states}
			for _, pricePointID := range []string{"pp-old", "pp-new"} {
				target := subscriptionPriceImportResolvedRow{
					territoryID:  "USA",
					pricePointID: pricePointID,
					planType:     asc.SubscriptionPlanTypeUpfront,
				}
				if got, want := index.matches(target), pricePointID == test.current; got != want {
					t.Fatalf("matches(%s) = %t, want %t", pricePointID, got, want)
				}
			}
		})
	}
}

func TestSubscriptionPriceImportStateAddRecordsUSPacificDate(t *testing.T) {
	index := &subscriptionPriceImportStateIndex{now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC)}
	index.add(subscriptionPriceImportResolvedRow{
		territoryID:  "USA",
		pricePointID: "pp-new",
		planType:     asc.SubscriptionPlanTypeUpfront,
	})
	if len(index.states) != 1 || index.states[0].startDate != "2026-09-30" {
		t.Fatalf("expected the immediate price recorded on the Pacific date 2026-09-30, got %+v", index.states)
	}
}
