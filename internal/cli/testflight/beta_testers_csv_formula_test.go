package testflight

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeutralizeSpreadsheetFormulaCoversEveryMarkerAndLeadingVariant(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "equals", input: `=1+1`, want: `'=1+1`},
		{name: "plus", input: `+1`, want: `'+1`},
		{name: "minus", input: `-1+1`, want: `'-1+1`},
		{name: "at", input: `@SUM(A1)`, want: `'@SUM(A1)`},
		{name: "dde", input: `=cmd|' /C calc'!A0`, want: `'=cmd|' /C calc'!A0`},
		{name: "leading space", input: ` =1+1`, want: `' =1+1`},
		{name: "leading tab", input: "\t=1+1", want: "'\t=1+1"},
		{name: "leading carriage return", input: "\r=1+1", want: "'\r=1+1"},
		{name: "leading newline", input: "\n@SUM(A1)", want: "'\n@SUM(A1)"},
		{name: "leading non-breaking space", input: "\u00a0-1", want: "'\u00a0-1"},
		{name: "leading control character", input: "\x01+1", want: "'\x01+1"},
		{name: "empty", input: ``, want: ``},
		{name: "whitespace only", input: `  `, want: `  `},
		{name: "plain email", input: `tester@example.com`, want: `tester@example.com`},
		{name: "plain name", input: `Ada Lovelace`, want: `Ada Lovelace`},
		{name: "group list", input: `Beta;Internal`, want: `Beta;Internal`},
		{name: "internal marker", input: `Team=Beta`, want: `Team=Beta`},
		{name: "literal apostrophe before formula is escaped", input: `'=1+1`, want: `''=1+1`},
		{name: "double apostrophe before formula is escaped", input: `''=1+1`, want: `'''=1+1`},
		{name: "apostrophe before plain text", input: `'quoted`, want: `'quoted`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := neutralizeSpreadsheetFormula(tc.input); got != tc.want {
				t.Fatalf("neutralizeSpreadsheetFormula(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWriteCSVFileNeutralizesFormulaLeadingCells(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "testers.csv")
	header := []string{"email", "first_name", "last_name", "groups"}
	rows := [][]string{
		{"tester@example.com", "Ada", "Lovelace", "Beta;Internal"},
		{"-tester@example.com", "=1+1", " @SUM(A1)", "+Internal;-Beta"},
		{"formula@example.com", "\t-2+3", "Byron", `=cmd|' /C calc'!A0`},
	}

	if err := writeCSVFileAtomicNoSymlink(outputPath, header, rows); err != nil {
		t.Fatalf("writeCSVFileAtomicNoSymlink() error: %v", err)
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != len(rows)+1 {
		t.Fatalf("expected %d records, got %d", len(rows)+1, len(records))
	}

	wantHeader := append(append([]string(nil), header...), "_asc_formula_escaping")
	if strings.Join(records[0], ",") != strings.Join(wantHeader, ",") {
		t.Fatalf("header = %q, want %q", records[0], wantHeader)
	}
	wantSafeRow := append(append([]string(nil), rows[0]...), "apostrophe-v1")
	if strings.Join(records[1], ",") != strings.Join(wantSafeRow, ",") {
		t.Fatalf("safe row = %q, want values unchanged plus provenance %q", records[1], wantSafeRow)
	}

	for _, record := range records[1:] {
		for _, cell := range record[:len(header)] {
			if isSpreadsheetFormulaCell(cell) {
				t.Fatalf("cell %q would be interpreted as a spreadsheet formula", cell)
			}
		}
	}

	want := []string{"'-tester@example.com", "'=1+1", "' @SUM(A1)", "'+Internal;-Beta", "apostrophe-v1"}
	if strings.Join(records[2], ",") != strings.Join(want, ",") {
		t.Fatalf("neutralized row = %q, want %q", records[2], want)
	}
}

func TestWriteCSVFileKeepsCanonicalSchemaWhenNoCellNeedsEscaping(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "testers.csv")
	header := []string{"email", "first_name", "last_name", "groups"}
	rows := [][]string{{"tester@example.com", "Ada", "Lovelace", "Beta"}}

	if err := writeCSVFileAtomicNoSymlink(outputPath, header, rows); err != nil {
		t.Fatalf("writeCSVFileAtomicNoSymlink() error: %v", err)
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if strings.Join(records[0], ",") != strings.Join(header, ",") {
		t.Fatalf("header = %q, want unchanged canonical header %q", records[0], header)
	}
	if strings.Join(records[1], ",") != strings.Join(rows[0], ",") {
		t.Fatalf("row = %q, want unchanged canonical row %q", records[1], rows[0])
	}
}

func TestBetaTesterCSVRoundTripsNeutralizedCells(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "testers.csv")
	header := []string{"email", "first_name", "last_name", "groups"}
	rows := [][]string{
		{"-tester@example.com", "Ada", "Lovelace", "+Internal;-Beta"},
		{"quoted@example.com", "'=Ada", "''@Lovelace", "'=Internal"},
	}

	if err := writeCSVFileAtomicNoSymlink(outputPath, header, rows); err != nil {
		t.Fatalf("writeCSVFileAtomicNoSymlink() error: %v", err)
	}

	parsed, err := readBetaTestersCSV(csvTestDeps(), outputPath)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(parsed))
	}
	if parsed[0].email != "-tester@example.com" {
		t.Fatalf("email = %q, want the original %q", parsed[0].email, "-tester@example.com")
	}
	wantGroups := []string{"+Internal", "-Beta"}
	if strings.Join(parsed[0].groups, ";") != strings.Join(wantGroups, ";") {
		t.Fatalf("groups = %q, want %q", parsed[0].groups, wantGroups)
	}

	// Values that already begin with literal apostrophes before formula
	// characters must survive the export -> import round trip unchanged.
	if parsed[1].firstName != "'=Ada" {
		t.Fatalf("first name = %q, want the original %q", parsed[1].firstName, "'=Ada")
	}
	if parsed[1].lastName != "''@Lovelace" {
		t.Fatalf("last name = %q, want the original %q", parsed[1].lastName, "''@Lovelace")
	}
	if strings.Join(parsed[1].groups, ";") != "'=Internal" {
		t.Fatalf("groups = %q, want %q", parsed[1].groups, []string{"'=Internal"})
	}
}

func TestBetaTesterCSVImportPreservesUnmarkedLeadingApostrophes(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "third-party.csv")
	content := "email,first_name,last_name,groups\n" +
		"tester@example.com,'=Ada,''@Lovelace,'=Internal\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	parsed, err := readBetaTestersCSV(csvTestDeps(), inputPath)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 row, got %d", len(parsed))
	}
	if parsed[0].firstName != "'=Ada" {
		t.Fatalf("first name = %q, want literal user-authored apostrophe preserved", parsed[0].firstName)
	}
	if parsed[0].lastName != "''@Lovelace" {
		t.Fatalf("last name = %q, want literal user-authored apostrophes preserved", parsed[0].lastName)
	}
	if strings.Join(parsed[0].groups, ";") != "'=Internal" {
		t.Fatalf("groups = %q, want literal user-authored apostrophe preserved", parsed[0].groups)
	}
}
