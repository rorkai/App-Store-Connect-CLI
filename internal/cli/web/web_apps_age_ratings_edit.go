package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func newWebAgeRatingsEditCommand(d *shared.Deps) *ffcli.Command {
	const command = "web apps age-ratings edit"
	fs := d.NewFlagSet(command, flag.ExitOnError)
	id := fs.String("id", "", "Declaration ID from asc web apps age-ratings list")
	input := fs.String("input", "", "Path to a JSON object containing age-rating fields")
	confirm := fs.Bool("confirm", false, "Confirm updating the age rating declaration")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc web apps age-ratings edit --id DECLARATION_ID --input PATH --confirm [flags]",
		ShortHelp:  "Edit an age rating declaration via a web session.",
		LongHelp: `Pass a flat JSON object using App Store Connect field names.

  {"advertising":false,"contests":"NONE","kidsAgeBand":null}

The PATCH includes only the supplied fields. Set developerAgeRatingInfoUrl
to null to clear the Age Suitability URL.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return d.UsageError("unexpected positional arguments")
			}
			declarationID := strings.TrimSpace(*id)
			if declarationID == "" {
				return d.RequiredFlag("--id")
			}
			if declarationID == "." || declarationID == ".." {
				return d.UsageError("--id must be a declaration ID")
			}
			path := strings.TrimSpace(*input)
			if path == "" {
				return d.RequiredFlag("--input")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			edit, err := readWebAgeRatingEditInput(d.OSPath(path))
			if err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result json.RawMessage
			err = withWebSpinnerWith(d, "Saving age rating declaration", func() error {
				var err error
				result, err = newWebClientFn(session).UpdateAgeRatingDeclaration(requestCtx, declarationID, edit)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, command)
			}
			if len(result) == 0 {
				return d.PrintOutput(&asc.WebAgeRatingEditResult{DeclarationID: declarationID}, *output.Output, *output.Pretty)
			}
			return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"declaration_id"}, [][]string{{declarationID}})
		},
	}
}

func readWebAgeRatingEditInput(path string) (webcore.AgeRatingDeclarationEdit, error) {
	const maxInputSize = 1 << 20
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return webcore.AgeRatingDeclarationEdit{}, fmt.Errorf("open --input: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxInputSize+1))
	if err != nil {
		return webcore.AgeRatingDeclarationEdit{}, fmt.Errorf("read --input: %w", err)
	}
	if len(data) > maxInputSize {
		return webcore.AgeRatingDeclarationEdit{}, fmt.Errorf("--input exceeds %d-byte limit", maxInputSize)
	}
	return webcore.ParseAgeRatingDeclarationEdit(data)
}
