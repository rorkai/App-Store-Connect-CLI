package shared

import (
	"bytes"
	"context"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestQueryFlagName(t *testing.T) {
	tests := []struct {
		param, resource, want string
	}{
		{"filter[email]", "userInvitations", "email"},
		{"filter[roles]", "userInvitations", "role"},
		{"filter[visibleApps]", "userInvitations", "visible-app"},
		{"filter[territories]", "x", "territory"},
		{"filter[status]", "x", "status"},
		{"filter[placeStatus]", "x", "place-status"},
		{"filter[bundleId]", "x", "bundle-id"},
		{"filter[appStoreVersions.appStoreState]", "x", "app-store-versions-app-store-state"},
		{"filter[preReleaseVersion.platforms]", "x", "pre-release-version-platform"},
		{"filter[cfBundleShortVersionString]", "x", "cf-bundle-short-version-string"},
		{"fields[userInvitations]", "userInvitations", "fields"},
		{"fields[apps]", "userInvitations", "app-fields"},
		{"fields[betaTesters]", "builds", "beta-tester-fields"},
		{"fields[territories]", "x", "territory-fields"},
		{"fields[subscriptionPricePoints]", "x", "subscription-price-point-fields"},
		{"include", "x", "include"},
		{"sort", "x", "sort"},
		{"limit[visibleApps]", "userInvitations", "visible-apps-limit"},
		{"limit[territories]", "x", "territories-limit"},
		{"exists[publishedResponse]", "x", "exists-published-response"},
		{"exists[app]", "x", "exists-app"},
		{"groupBy", "x", "group-by"},
		{"granularity", "x", "granularity"},
	}
	for _, tt := range tests {
		if got := QueryFlagName(tt.param, tt.resource); got != tt.want {
			t.Errorf("QueryFlagName(%q, %q) = %q, want %q", tt.param, tt.resource, got, tt.want)
		}
	}
}

type fakeQuerySource struct {
	schema asc.QuerySchema
	values map[string]string
}

func (q *fakeQuerySource) querySchema() asc.QuerySchema { return q.schema }

func (q *fakeQuerySource) setQueryValues(values map[string]string) { q.values = values }

var fakeQuerySchema = asc.QuerySchema{
	Resource: "widgets",
	Params: []asc.QueryParam{
		{Name: "filter[colors]", Type: "array", Values: []string{"RED", "BLUE"}},
		{Name: "filter[name]", Type: "array"},
		{Name: "include", Type: "array", Values: []string{"parts"}},
		{Name: "limit[parts]", Type: "integer", Maximum: 50},
		{Name: "exists[owner]", Type: "boolean"},
		{Name: "filter[size]", Type: "string", Values: []string{"BIG", "SMALL"}},
	},
}

func TestBuildPaginatedListCommandQueryFlags(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/widgets?cursor=2"
	tests := []struct {
		name       string
		args       []string
		names      map[string]string
		wantValues map[string]string
		wantErr    string
	}{
		{
			name:       "no query flags send no values",
			wantValues: map[string]string{},
		},
		{
			name: "each flag sends its parameter",
			args: []string{"--color", "red , BLUE", "--name", "a,b", "--include", "parts", "--parts-limit", "5", "--exists-owner", "--size", "BIG"},
			wantValues: map[string]string{
				"filter[colors]": "RED,BLUE", "filter[name]": "a,b", "include": "parts",
				"limit[parts]": "5", "exists[owner]": "true", "filter[size]": "BIG",
			},
		},
		{
			name:       "a false boolean is sent",
			args:       []string{"--exists-owner=false"},
			wantValues: map[string]string{"exists[owner]": "false"},
		},
		{
			name:       "an override renames a flag",
			names:      map[string]string{"filter[name]": "widget-name"},
			args:       []string{"--widget-name", "a"},
			wantValues: map[string]string{"filter[name]": "a"},
		},
		{name: "a value outside the enum", args: []string{"--color", "GREEN"}, wantErr: "widgets list: --color must be one of: RED, BLUE"},
		{name: "an empty list", args: []string{"--name", " , "}, wantErr: "widgets list: --name must not be empty"},
		{name: "a string outside the enum", args: []string{"--size", "HUGE"}, wantErr: "widgets list: --size must be one of: BIG, SMALL"},
		{name: "an integer above the maximum", args: []string{"--include", "parts", "--parts-limit", "51"}, wantErr: "widgets list: --parts-limit must be between 1 and 50"},
		{name: "a zero integer", args: []string{"--include", "parts", "--parts-limit", "0"}, wantErr: "widgets list: --parts-limit must be between 1 and 50"},
		{name: "a relationship limit without its include", args: []string{"--parts-limit", "5"}, wantErr: "widgets list: --parts-limit requires --include parts"},
		{name: "a query flag with --next", args: []string{"--next", nextURL, "--color", "RED"}, wantErr: "widgets list: --next cannot be combined with --color"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			query := &fakeQuerySource{schema: fakeQuerySchema}
			fetched := false
			cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
				ShortUsage:     "asc widgets list [flags]",
				TopLevel:       true,
				Query:          query,
				QueryFlagNames: tt.names,
				Deps:           newBuilderTestDeps(&stdout, &stderr),
				FetchPage: func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error) {
					fetched = true
					return &testPaginatedResponse{}, nil
				},
			})
			err := cmd.FlagSet.Parse(append(tt.args, "--output", "json"))
			if err == nil {
				err = cmd.Exec(context.Background(), nil)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error()+stderr.String(), tt.wantErr) {
					t.Fatalf("error = %v, stderr = %q; want %q", err, stderr.String(), tt.wantErr)
				}
				if fetched {
					t.Fatal("fetched despite a usage error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Exec() error: %v; stderr = %q", err, stderr.String())
			}
			if len(query.values) != len(tt.wantValues) {
				t.Fatalf("values = %v, want %v", query.values, tt.wantValues)
			}
			for name, want := range tt.wantValues {
				if query.values[name] != want {
					t.Fatalf("values = %v, want %v", query.values, tt.wantValues)
				}
			}
		})
	}
}

func TestQueryFlagsRejectAmbiguousNames(t *testing.T) {
	tests := []struct {
		name  string
		flags func(*flag.FlagSet) func() error
		names map[string]string
		want  string
	}{
		{
			name:  "a generated name taken by a command flag",
			flags: func(fs *flag.FlagSet) func() error { fs.String("name", "", "Name"); return nil },
			want:  "flag --name for query parameter filter[name] is already defined",
		},
		{
			name:  "an override for a parameter the read does not define",
			names: map[string]string{"filter[shape]": "shape"},
			want:  "QueryFlagNames names filter[shape], which the read does not define",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got == nil || !strings.Contains(got.(string), tt.want) {
					t.Fatalf("panic = %v, want %q", got, tt.want)
				}
			}()
			BuildIDGetCommand(IDGetCommandConfig{
				ShortUsage:     "asc widgets view --id ID",
				Flags:          tt.flags,
				Query:          &fakeQuerySource{schema: fakeQuerySchema},
				QueryFlagNames: tt.names,
				Fetch:          func(context.Context, *asc.Client, string) (any, error) { return nil, nil },
			})
		})
	}
}

func TestQueryFlagNamesLeaveAParameterToACommandFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	query := &fakeQuerySource{schema: fakeQuerySchema}
	cmd := BuildIDGetCommand(IDGetCommandConfig{
		ShortUsage:     "asc widgets view --id ID",
		Flags:          func(fs *flag.FlagSet) func() error { fs.String("name", "", "Widget name"); return nil },
		Query:          query,
		QueryFlagNames: map[string]string{"filter[name]": ""},
		Deps:           newBuilderTestDeps(&stdout, &stderr),
		Fetch:          func(context.Context, *asc.Client, string) (any, error) { return map[string]string{}, nil },
	})
	if got := cmd.FlagSet.Lookup("name").Usage; got != "Widget name" {
		t.Fatalf("--name usage = %q, want the command's own flag", got)
	}
	if err := cmd.FlagSet.Parse([]string{"--id", "1", "--name", "a", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error: %v; stderr = %q", err, stderr.String())
	}
	if _, sent := query.values["filter[name]"]; sent {
		t.Fatalf("values = %v, want filter[name] left to the command's flag", query.values)
	}
}

func TestQueryFlagsHaveTheirOwnHelpSection(t *testing.T) {
	cmd := BuildIDGetCommand(IDGetCommandConfig{
		ShortUsage: "asc widgets view --id ID",
		Query:      &fakeQuerySource{schema: fakeQuerySchema},
		Fetch:      func(context.Context, *asc.Client, string) (any, error) { return nil, nil },
	})
	help := DefaultUsageFunc(cmd)
	own, query, ok := strings.Cut(help, "QUERY FLAGS\n")
	if !ok {
		t.Fatalf("help has no QUERY FLAGS section:\n%s", help)
	}
	if strings.Contains(own, "--color") || !strings.Contains(own, "--id") {
		t.Fatalf("FLAGS section:\n%s", own)
	}
	if !strings.Contains(query, "--color") || strings.Contains(query, "--id") {
		t.Fatalf("QUERY FLAGS section:\n%s", query)
	}
}
