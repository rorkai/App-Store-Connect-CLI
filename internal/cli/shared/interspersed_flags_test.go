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

// TestParseInterspersedFlagsResolvesAliases keeps a flag alias after a
// positional argument the same flag as its canonical spelling: the canonical
// flag is the one set and visited, and a second spelling with another value is
// an error instead of a silent last-one-wins.
func TestParseInterspersedFlagsResolvesAliases(t *testing.T) {
	tests := []struct {
		name        string
		before      []string
		args        []string
		wantVisited string
		wantValue   string
		wantErr     string
	}{
		{name: "alias sets the canonical flag", args: []string{"POS", "--data", "x"}, wantVisited: "body", wantValue: "x"},
		{name: "inline alias", args: []string{"POS", "--data=x"}, wantVisited: "body", wantValue: "x"},
		{name: "same value twice", args: []string{"POS", "--body", "x", "--data", "x"}, wantVisited: "body", wantValue: "x"},
		{name: "different values", args: []string{"POS", "--body", "x", "--data", "y"}, wantErr: "`--body` and `--data` are the same flag; pass one value"},
		{name: "canonical set before the positional", before: []string{"--body", "x"}, args: []string{"POS", "--data", "y"}, wantErr: "`--body` and `--data` are the same flag; pass one value"},
		{name: "value alias", args: []string{"POS", "--json"}, wantVisited: "output", wantValue: "json"},
		{name: "value alias against another value", before: []string{"--output", "table"}, args: []string{"POS", "--json"}, wantErr: "`--output` and `--json` are the same flag; pass one value"},
		// The root rewrites an alias before the positional to its canonical
		// spelling, so `--data x POS --body y` arrives as below.
		{name: "canonical before and after the positional", before: []string{"--body", "x"}, args: []string{"POS", "--body", "y"}, wantErr: "`--body` is given before and after the positional arguments with different values (`--data` is the same flag); pass one value"},
		{name: "value alias before, canonical after", before: []string{"--output", "json"}, args: []string{"POS", "--output", "table"}, wantErr: "`--output` is given before and after the positional arguments with different values (`--json` means `--output json`); pass one value"},
		{name: "canonical before and after with one value", before: []string{"--body", "x"}, args: []string{"POS", "--body", "x"}, wantVisited: "body", wantValue: "x"},
		{name: "repeatable flag before and after", before: []string{"--query", "a=1"}, args: []string{"POS", "--query", "b=2"}, wantVisited: "query", wantValue: "a=1,b=2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			body := fs.String("body", "", "Body")
			output := fs.String("output", "", "Output format: json, table")
			var query MultiStringFlag
			fs.Var(&query, "query", "Query parameter (repeatable)")
			BindFlagAlias(fs, "data", "body")
			BindFlagValueAlias(fs, "json", "output", "json")
			if err := fs.Parse(test.before); err != nil {
				t.Fatal(err)
			}
			_, err := ParseInterspersedFlags(fs, test.args)
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
			if !visited[test.wantVisited] {
				t.Fatalf("visited = %v, want --%s", visited, test.wantVisited)
			}
			got := map[string]string{"body": *body, "output": *output, "query": query.String()}[test.wantVisited]
			if got != test.wantValue {
				t.Fatalf("--%s = %q, want %q", test.wantVisited, got, test.wantValue)
			}
		})
	}
}
