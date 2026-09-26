package shared

import (
	"flag"
	"testing"
)

func TestPrimaryIDFlagName(t *testing.T) {
	tests := []struct {
		name  string
		bind  func(*flag.FlagSet)
		want  string
		wantK bool
	}{
		{
			name: "single id flag",
			bind: func(fs *flag.FlagSet) {
				fs.String("id", "", "")
				fs.String("output", "", "")
			},
			want:  "id",
			wantK: true,
		},
		{
			name: "single app flag",
			bind: func(fs *flag.FlagSet) {
				fs.String("app", "", "")
				fs.Int("limit", 0, "")
			},
			want:  "app",
			wantK: true,
		},
		{
			name: "no identifier flag",
			bind: func(fs *flag.FlagSet) {
				fs.String("shell", "", "")
			},
		},
		{
			name: "ambiguous identifier flags",
			bind: func(fs *flag.FlagSet) {
				fs.String("app", "", "")
				fs.String("version-id", "", "")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			test.bind(fs)
			got, ok := PrimaryIDFlagName(fs)
			if got != test.want || ok != test.wantK {
				t.Fatalf("PrimaryIDFlagName() = %q, %v, want %q, %v", got, ok, test.want, test.wantK)
			}
		})
	}
}

func TestPrimaryIDFlagNameSkipsFlagsTheCallerAlreadyPassed(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("app", "", "")
	if err := fs.Parse([]string{"--app", "app-1", "stray"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, ok := PrimaryIDFlagName(fs); got != "" || ok {
		t.Fatalf("PrimaryIDFlagName() = %q, %v, want \"\", false", got, ok)
	}
}

func TestPrimaryIDFlagNameNilFlagSet(t *testing.T) {
	if got, ok := PrimaryIDFlagName(nil); got != "" || ok {
		t.Fatalf("PrimaryIDFlagName(nil) = %q, %v, want \"\", false", got, ok)
	}
}

func TestUsageDeclaresOperands(t *testing.T) {
	tests := []struct {
		usage string
		want  bool
	}{
		{usage: "asc apps view --id APP_ID", want: false},
		{usage: "asc apps list [flags]", want: false},
		{usage: "asc auth logout [--name NAME | --all] --confirm", want: false},
		{usage: "asc completion --shell <bash|zsh|fish>", want: false},
		{usage: "asc metadata approve [--review-dir \"DIR\"] (--all | --key \"KEY\")", want: false},
		{usage: "asc apps <subcommand> [flags]", want: false},
		{usage: "asc pricing price-points [subcommand] [flags]", want: false},
		{usage: "", want: false},
		{usage: "asc docs show <api-notes|reference>", want: true},
		{usage: "asc schema [flags] [query]", want: true},
		{usage: "asc search [flags] <query>", want: true},
		{usage: "asc workflow run [flags] <name> [KEY:VALUE ...]", want: true},
		{usage: "asc signing run --identity PATH --profile PATH [flags] -- <command> [args...]", want: true},
	}

	for _, test := range tests {
		t.Run(test.usage, func(t *testing.T) {
			if got := UsageDeclaresOperands(test.usage); got != test.want {
				t.Fatalf("UsageDeclaresOperands(%q) = %v, want %v", test.usage, got, test.want)
			}
		})
	}
}
