package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func newWebAppsAgeRatingsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "age-ratings",
		ShortUsage: "asc web apps age-ratings <subcommand> [flags]",
		ShortHelp:  "Read app age ratings via a web session.",
		FlagSet:    d.NewFlagSet("web apps age-ratings", flag.ExitOnError),
		UsageFunc:  d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAgeRatingsReadCommand(d, "list", "app", "APP_ID", "List app info records and their age rating declarations.", (*webcore.Client).ListAppAgeRatings),
			newWebAgeRatingsReadCommand(d, "territories", "app-info", "APP_INFO_ID", "List territory age ratings for an app info record.", (*webcore.Client).ListAppTerritoryAgeRatings),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newWebAgeRatingsReadCommand(
	d *shared.Deps, name, idFlag, idLabel, help string,
	fetch func(*webcore.Client, context.Context, string) (json.RawMessage, error),
) *ffcli.Command {
	command := "web apps age-ratings " + name
	fs := d.NewFlagSet(command, flag.ExitOnError)
	idHelp := "App info record ID from asc web apps age-ratings list"
	if idFlag == "app" {
		idHelp = "App Store Connect app ID (or ASC_APP_ID)"
	}
	id := fs.String(idFlag, "", idHelp)
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: fmt.Sprintf("asc %s --%s %s [flags]", command, idFlag, idLabel),
		ShortHelp:  help,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			value := strings.TrimSpace(*id)
			if idFlag == "app" {
				value = strings.TrimSpace(d.ResolveAppID(value))
			}
			if value == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --%s is required", idFlag)
				if idFlag == "app" {
					fmt.Fprint(d.IO.ErrOut, " (or set ASC_APP_ID)")
				}
				fmt.Fprintln(d.IO.ErrOut)
				return shared.MissingRequiredUsageError("--" + idFlag)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result json.RawMessage
			err = withWebSpinnerWith(d, "Fetching age ratings", func() error {
				var err error
				result, err = fetch(newWebClientFn(session), requestCtx, value)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, command)
			}
			return printWebAppAgeRatings(d, result, name == "territories", *output.Output, *output.Pretty)
		},
	}
}

func printWebAppAgeRatings(d *shared.Deps, result json.RawMessage, territories bool, output string, pretty bool) error {
	type relationship struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	var document struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Rating string `json:"appStoreAgeRating"`
				State  string `json:"state"`
			} `json:"attributes"`
			Relationships struct {
				Declaration relationship `json:"ageRatingDeclaration"`
				Territory   relationship `json:"territory"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &document); err != nil {
		return fmt.Errorf("failed to parse age ratings response: %w", err)
	}
	if document.Data == nil {
		return fmt.Errorf("invalid age ratings response: missing data array")
	}

	rows := make([][]string, 0, len(document.Data))
	if territories {
		for _, entry := range document.Data {
			rows = append(rows, []string{
				webAppValueOrUnknown(entry.Relationships.Territory.Data.ID),
				webAppValueOrUnknown(entry.Attributes.Rating),
			})
		}
		return d.PrintOutputRows(result, output, pretty, []string{"territory", "rating"}, rows)
	}
	for _, entry := range document.Data {
		rows = append(rows, []string{
			entry.ID,
			webAppValueOrUnknown(entry.Attributes.State),
			webAppValueOrUnknown(entry.Attributes.Rating),
			webAppValueOrUnknown(entry.Relationships.Declaration.Data.ID),
		})
	}
	return d.PrintOutputRows(result, output, pretty, []string{"app_info_id", "state", "rating", "declaration_id"}, rows)
}
