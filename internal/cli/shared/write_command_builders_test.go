package shared

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type testWriteAttributes struct {
	ID      string  `json:"id"`
	Name    string  `json:"name,omitempty"`
	Note    *string `json:"note,omitempty"`
	Title   *string `json:"title,omitempty"`
	Label   string  `json:"label,omitempty"`
	Summary *string `json:"summary,omitempty"`
}

func TestBuildWriteCommands(t *testing.T) {
	t.Parallel()

	errClient := errors.New("no client")
	tests := []struct {
		name          string
		update        bool
		configure     func(*WriteCommandConfig, *testWriteAttributes)
		args          []string
		clientErr     bool
		sendErr       error
		wantErr       string
		wantParameter string
		wantStderr    string
		wantStdout    string
		wantFlagSet   string
		absentFlag    string
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
		{
			name: "create names its flag set and wraps the client error as configured",
			configure: func(config *WriteCommandConfig, _ *testWriteAttributes) {
				config.FlagSetName = "items create"
				config.ErrorPrefix = "things create"
			},
			args:        []string{"--parent-id", "parent-1", "--name", "Item"},
			clientErr:   true,
			wantErr:     "things create: no client",
			wantFlagSet: "items create",
		},
		{
			name: "create wraps the request error with the configured prefix",
			configure: func(config *WriteCommandConfig, _ *testWriteAttributes) {
				config.ErrorPrefix = "things create"
			},
			args:    []string{"--parent-id", "parent-1", "--name", "Item"},
			sendErr: errors.New("boom"),
			wantErr: "things create: failed to create: boom",
		},
		{
			name: "top-level create has no ID flag and sends an empty ID",
			configure: func(config *WriteCommandConfig, _ *testWriteAttributes) {
				config.TopLevel = true
			},
			args:       []string{"--name", " Item "},
			wantStdout: `{"id":"","name":"Item"}` + "\n",
			absentFlag: "parent-id",
		},
		{
			name: "create sends a plain field trimmed",
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields = append(config.Fields, StringFlag(&attrs.Label, "label", "Label"))
			},
			args:       []string{"--parent-id", "parent-1", "--name", "Item", "--label", " Tag "},
			wantStdout: `{"id":"parent-1","name":"Item","label":"Tag"}` + "\n",
		},
		{
			name:   "update does not count a blank plain field as given",
			update: true,
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields = append(config.Fields, StringFlag(&attrs.Label, "label", "Label"))
			},
			args:       []string{"--id", "item-1", "--label", "  "},
			wantErr:    "flag: help requested",
			wantStderr: "Error: at least one update flag is required (--note, --title, --label)\n",
		},
		{
			name:   "update sends an explicit field given blank",
			update: true,
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields = append(config.Fields, ExplicitStringFlag(&attrs.Summary, "summary", "Summary"))
			},
			args:       []string{"--id", "item-1", "--summary", "  "},
			wantStdout: `{"id":"item-1","summary":""}` + "\n",
		},
		{
			name:   "update does not count an explicit field left off the command line",
			update: true,
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields = append(config.Fields, ExplicitStringFlag(&attrs.Summary, "summary", "Summary"))
			},
			args:       []string{"--id", "item-1"},
			wantErr:    "flag: help requested",
			wantStderr: "Error: at least one update flag is required (--note, --title, --summary)\n",
		},
		{
			name: "create rejects a field longer than its maximum before any request",
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields[0] = RequiredStringFlag(&attrs.Name, "name", "Name").WithMaxLength(5)
			},
			args:       []string{"--parent-id", "parent-1", "--name", "Item 42"},
			wantErr:    "--name must be at most 5 characters (got 7)",
			wantStderr: "Error: --name must be at most 5 characters (got 7)\n",
		},
		{
			name: "create counts characters, not bytes, against the maximum",
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields[0] = RequiredStringFlag(&attrs.Name, "name", "Name").WithMaxLength(5)
			},
			args:       []string{"--parent-id", "parent-1", "--name", " 夏祭りの日 "},
			wantStdout: `{"id":"parent-1","name":"夏祭りの日"}` + "\n",
		},
		{
			name:   "update rejects an optional field longer than its maximum",
			update: true,
			configure: func(config *WriteCommandConfig, attrs *testWriteAttributes) {
				config.Fields[1] = OptionalStringFlag(&attrs.Title, "title", "Title").WithMaxLength(3)
			},
			args:       []string{"--id", "item-1", "--title", "Title"},
			wantErr:    "--title must be at most 3 characters (got 5)",
			wantStderr: "Error: --title must be at most 3 characters (got 5)\n",
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
			config := WriteCommandConfig{
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
			}
			build := BuildCreateCommand
			if tt.update {
				config = WriteCommandConfig{
					Name:       "update",
					ShortUsage: "asc items update --id ITEM_ID [flags]",
					IDUsage:    "Item ID",
					Fields: []AttributeFlag{
						OptionalStringFlag(&attrs.Note, "note", "Note"),
						OptionalStringFlag(&attrs.Title, "title", "Title"),
					},
					Deps: d,
					Send: send,
				}
				build = BuildUpdateCommand
			}
			if tt.configure != nil {
				tt.configure(&config, &attrs)
			}
			cmd := build(config)
			if tt.wantFlagSet != "" && cmd.FlagSet.Name() != tt.wantFlagSet {
				t.Fatalf("flag set name = %q, want %q", cmd.FlagSet.Name(), tt.wantFlagSet)
			}
			if tt.absentFlag != "" && cmd.FlagSet.Lookup(tt.absentFlag) != nil {
				t.Fatalf("flag --%s is registered, want none", tt.absentFlag)
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
