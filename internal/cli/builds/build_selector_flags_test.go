package builds

import (
	"errors"
	"flag"
	"testing"
)

func TestBuildSelectorFlagsResolveExcludeExpired(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "exclude-expired",
			args: []string{"--app", "123456789", "--latest", "--exclude-expired"},
		},
		{
			name: "not-expired alias",
			args: []string{"--app", "123456789", "--latest", "--not-expired"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			d, _, _ := newBuildsTestDeps(nil)
			selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{})
			if err := fs.Parse(test.args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			opts := selectors.resolveOptions()
			if !opts.ExcludeExpired {
				t.Fatal("expected ExcludeExpired to be true")
			}
		})
	}
}

func TestBuildSelectorFlagsRejectExcludeExpiredWithoutLatest(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	d, _, _ := newBuildsTestDeps(nil)
	selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{})
	if err := fs.Parse([]string{"--app", "123456789", "--exclude-expired"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	err := selectors.validate()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
	}
}
