package shared

import (
	"bytes"
	"context"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestGivenEmptyAppFlagOnAGroupIsRejected(t *testing.T) {
	var stderr bytes.Buffer
	d := &Deps{IO: &IO{Out: &bytes.Buffer{}, ErrOut: &stderr}}

	groupFlags := flag.NewFlagSet("group", flag.ContinueOnError)
	groupFlags.String("app", "", "App")
	ran := false
	group := &ffcli.Command{
		Name:    "group",
		FlagSet: groupFlags,
		Subcommands: []*ffcli.Command{{
			Name:    "child",
			FlagSet: flag.NewFlagSet("child", flag.ContinueOnError),
			Exec:    func(context.Context, []string) error { ran = true; return nil },
		}},
	}
	WrapCommandOutputValidation(group, d)

	err := group.ParseAndRun(context.Background(), []string{"--app", " ", "child"})
	if err == nil || ran || !strings.HasPrefix(stderr.String(), "Error: --app must not be empty\n") {
		t.Fatalf("err=%v ran=%v stderr=%q, want the child refused because the group's --app is blank", err, ran, stderr.String())
	}
	diagnostic, ok := DiagnosticFromError(err)
	if !ok || diagnostic.Code != DiagnosticInvalidInput || diagnostic.Parameter != "--app" {
		t.Fatalf("diagnostic = %+v (ok=%v), want invalid_input on --app", diagnostic, ok)
	}
}
