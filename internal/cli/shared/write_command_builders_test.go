package shared

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type testWriteAttributes struct {
	ID    string  `json:"id"`
	Name  string  `json:"name,omitempty"`
	Note  *string `json:"note,omitempty"`
	Title *string `json:"title,omitempty"`
}

func TestBuildWriteCommands(t *testing.T) {
	t.Parallel()

	errClient := errors.New("no client")
	tests := []struct {
		name          string
		update        bool
		args          []string
		clientErr     bool
		sendErr       error
		wantErr       string
		wantParameter string
		wantStderr    string
		wantStdout    string
	}{
		{
			name:          "create reports a missing ID first",
			args:          []string{"--name", "Item"},
			wantErr:       "--parent-id",
			wantParameter: "--parent-id",
			wantStderr:    "Error: --parent-id is required\n",
		},
		{
			name:          "create reports a blank required field",
			args:          []string{"--parent-id", "parent-1", "--name", "  ", "--note", "Note"},
			wantErr:       "--name",
			wantParameter: "--name",
			wantStderr:    "Error: --name is required\n",
		},
		{
			name:       "create sends trimmed values and leaves a blank optional field nil",
			args:       []string{"--parent-id", " parent-1 ", "--name", " Item ", "--note", "  "},
			wantStdout: `{"id":"parent-1","name":"Item"}` + "\n",
		},
		{
			name:       "create sends a given optional field",
			args:       []string{"--parent-id", "parent-1", "--name", "Item", "--note", " Note "},
			wantStdout: `{"id":"parent-1","name":"Item","note":"Note"}` + "\n",
		},
		{
			name:      "create wraps the client error",
			args:      []string{"--parent-id", "parent-1", "--name", "Item"},
			clientErr: true,
			wantErr:   "items create: no client",
		},
		{
			name:    "create wraps the request error",
			args:    []string{"--parent-id", "parent-1", "--name", "Item"},
			sendErr: errors.New("boom"),
			wantErr: "items create: failed to create: boom",
		},
		{
			name:       "update requires one of its fields",
			update:     true,
			args:       []string{"--id", "item-1", "--note", " ", "--title", ""},
			wantErr:    "flag: help requested",
			wantStderr: "Error: at least one update flag is required (--note, --title)\n",
		},
		{
			name:       "update sends only the given fields",
			update:     true,
			args:       []string{"--id", "item-1", "--title", " Title "},
			wantStdout: `{"id":"item-1","title":"Title"}` + "\n",
		},
		{
			name:    "update wraps the request error",
			update:  true,
			args:    []string{"--id", "item-1", "--note", "Note"},
			sendErr: errors.New("boom"),
			wantErr: "items update: failed to update: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			d := newBuilderTestDeps(&stdout, &stderr)
			if tt.clientErr {
				d.Client = func() (*asc.Client, error) { return nil, errClient }
			}
			var attrs testWriteAttributes
			send := func(_ context.Context, _ *asc.Client, id string) (any, error) {
				attrs.ID = id
				return attrs, tt.sendErr
			}
			var cmd *ffcli.Command
			if tt.update {
				cmd = BuildUpdateCommand(WriteCommandConfig{
					Name:       "update",
					ShortUsage: "asc items update --id ITEM_ID [flags]",
					IDUsage:    "Item ID",
					Fields: []AttributeFlag{
						OptionalStringFlag(&attrs.Note, "note", "Note"),
						OptionalStringFlag(&attrs.Title, "title", "Title"),
					},
					Deps: d,
					Send: send,
				})
			} else {
				cmd = BuildCreateCommand(WriteCommandConfig{
					Name:       "create",
					ShortUsage: "asc items create --parent-id PARENT_ID [flags]",
					IDFlag:     "parent-id",
					IDUsage:    "Parent ID",
					Fields: []AttributeFlag{
						RequiredStringFlag(&attrs.Name, "name", "Name"),
						OptionalStringFlag(&attrs.Note, "note", "Note"),
					},
					Deps: d,
					Send: send,
				})
			}
			if err := cmd.FlagSet.Parse(append(tt.args, "--output", "json")); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Fatalf("Exec() error = %v, want %q", err, tt.wantErr)
			}
			if tt.clientErr && !errors.Is(err, errClient) {
				t.Fatalf("Exec() error = %v, want it to wrap %v", err, errClient)
			}
			if tt.sendErr != nil && !errors.Is(err, tt.sendErr) {
				t.Fatalf("Exec() error = %v, want it to wrap %v", err, tt.sendErr)
			}
			if tt.wantParameter != "" {
				diagnostic, ok := DiagnosticFromError(err)
				if !ok || diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != tt.wantParameter {
					t.Fatalf("diagnostic = %+v, %v; want required %s", diagnostic, ok, tt.wantParameter)
				}
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
		})
	}
}
