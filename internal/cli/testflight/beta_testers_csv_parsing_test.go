package testflight

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// csvTestDeps returns deps whose streams are buffers, for the CSV readers
// that print notes and usage errors.
func csvTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
}

func TestReadBetaTestersCSV_CanonicalHeaderWithSemicolonGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "canonical.csv")
	body := strings.Join([]string{
		"email,first_name,last_name,groups",
		"ada@example.com,Ada,Lovelace,Alpha;Beta",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].email != "ada@example.com" {
		t.Fatalf("expected email ada@example.com, got %q", rows[0].email)
	}
	if got := strings.Join(rows[0].groups, ","); got != "Alpha,Beta" {
		t.Fatalf("expected groups Alpha,Beta, got %q", got)
	}
}

func TestReadBetaTestersCSV_SemicolonGroupsPreserveCommaInGroupName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comma-in-group.csv")
	body := strings.Join([]string{
		"email,first_name,last_name,groups",
		"ada@example.com,Ada,Lovelace,\"Core, Team;Beta\"",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got := strings.Join(rows[0].groups, "|"); got != "Core, Team|Beta" {
		t.Fatalf("expected groups Core, Team|Beta, got %q", got)
	}
}

func TestReadBetaTestersCSV_FastlaneHeaderAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fastlane-header.csv")
	body := strings.Join([]string{
		"First,Last,Email,Groups",
		"Grace,Hopper,grace@example.com,External Beta;Power Users",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].firstName != "Grace" || rows[0].lastName != "Hopper" {
		t.Fatalf("unexpected names: %+v", rows[0])
	}
	if rows[0].email != "grace@example.com" {
		t.Fatalf("expected grace@example.com, got %q", rows[0].email)
	}
	if got := strings.Join(rows[0].groups, ","); got != "External Beta,Power Users" {
		t.Fatalf("expected two groups, got %q", got)
	}
}

func TestReadBetaTestersCSV_FastlaneHeaderlessRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fastlane-no-header.csv")
	body := strings.Join([]string{
		"Linus,Torvalds,linus@example.com,Core Team;Linux",
		"Margaret,Hamilton,margaret@example.com,Core Team",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1].email != "margaret@example.com" {
		t.Fatalf("expected margaret@example.com, got %q", rows[1].email)
	}
	if got := strings.Join(rows[0].groups, ","); got != "Core Team,Linux" {
		t.Fatalf("expected groups Core Team,Linux, got %q", got)
	}
}

func TestReadBetaTestersCSV_HeaderlessRowWithHeaderWordsParsesAsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "header-words-data.csv")
	body := "First,Email,first.email@example.com,Alpha\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].firstName != "First" || rows[0].lastName != "Email" || rows[0].email != "first.email@example.com" {
		t.Fatalf("expected headerless row to be parsed as data, got %+v", rows[0])
	}
}

func TestReadBetaTestersCSV_HeaderlessTooFewColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-headerless.csv")
	body := "OnlyFirstName,OnlyLastName\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readBetaTestersCSV(csvTestDeps(), path)
	if err == nil {
		t.Fatal("expected error for invalid headerless format")
	}
}

func TestReadBetaTestersCSV_RandomFixtureParses(t *testing.T) {
	path := filepath.Join("testdata", "beta_testers_random.csv")
	rows, err := readBetaTestersCSV(csvTestDeps(), path)
	if err != nil {
		t.Fatalf("readBetaTestersCSV() error: %v", err)
	}
	if len(rows) < 25 {
		t.Fatalf("expected at least 25 rows, got %d", len(rows))
	}
	for i, row := range rows {
		if strings.TrimSpace(row.email) == "" {
			t.Fatalf("row %d has empty email", i+1)
		}
	}
}
