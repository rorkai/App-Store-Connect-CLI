package shared

import (
	"flag"
	"testing"
)

func TestParseInterspersedFlagsMarksExplicitFlagsVisited(t *testing.T) {
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.String("body", "", "")

	positionals, err := ParseInterspersedFlags(fs, []string{"POST", "/v1/apps", "--body", ""})
	if err != nil {
		t.Fatalf("ParseInterspersedFlags() error: %v", err)
	}
	if want := []string{"POST", "/v1/apps"}; len(positionals) != len(want) || positionals[0] != want[0] || positionals[1] != want[1] {
		t.Fatalf("positionals = %#v, want %#v", positionals, want)
	}

	visited := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "body" {
			visited = true
		}
	})
	if !visited {
		t.Fatal("FlagSet.Visit() did not report --body explicitly set after positionals")
	}
}
