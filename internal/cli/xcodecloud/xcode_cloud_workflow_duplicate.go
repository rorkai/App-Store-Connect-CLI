package xcodecloud

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ciWorkflowCreateRelationships lists the relationships POST /v1/ciWorkflows requires.
var ciWorkflowCreateRelationships = []string{"product", "repository", "xcodeVersion", "macOsVersion"}

// ciWorkflowDuplicateDroppedAttributes lists attributes that a copy must not inherit:
// lastModifiedDate is response-only and CiWorkflowCreateRequest rejects it, and a copy is
// created unlocked so the operator can edit it, which is the point of duplicating.
var ciWorkflowDuplicateDroppedAttributes = []string{"lastModifiedDate", "isLockedForEditing"}

type ciWorkflowDuplicateOptions struct {
	name             string
	description      string
	overrideDescr    bool
	enabled          bool
	sourceWorkflowID string
}

// buildCiWorkflowDuplicatePayload turns a GET /v1/ciWorkflows/{id} envelope into a
// CiWorkflowCreateRequest body that recreates the same configuration under a new name.
func buildCiWorkflowDuplicatePayload(source json.RawMessage, opts ciWorkflowDuplicateOptions) (json.RawMessage, error) {
	var envelope struct {
		Data struct {
			Attributes    map[string]json.RawMessage `json:"attributes"`
			Relationships map[string]json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(source, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse source workflow: %w", err)
	}

	attributes := map[string]json.RawMessage{}
	for key, value := range envelope.Data.Attributes {
		attributes[key] = value
	}
	for _, key := range ciWorkflowDuplicateDroppedAttributes {
		delete(attributes, key)
	}

	for _, key := range []string{"containerFilePath", "actions"} {
		if isEmptyJSON(attributes[key]) {
			return nil, fmt.Errorf("source workflow %s is missing required attribute %q", opts.sourceWorkflowID, key)
		}
	}

	name, err := json.Marshal(opts.name)
	if err != nil {
		return nil, fmt.Errorf("failed to encode --name: %w", err)
	}
	attributes["name"] = name

	if opts.overrideDescr {
		description, err := json.Marshal(opts.description)
		if err != nil {
			return nil, fmt.Errorf("failed to encode --description: %w", err)
		}
		attributes["description"] = description
	} else if len(attributes["description"]) == 0 {
		attributes["description"] = json.RawMessage(`""`)
	}

	if opts.enabled {
		attributes["isEnabled"] = json.RawMessage("true")
	} else {
		attributes["isEnabled"] = json.RawMessage("false")
	}
	if len(attributes["clean"]) == 0 {
		attributes["clean"] = json.RawMessage("false")
	}

	cleaned := map[string]json.RawMessage{}
	for key, value := range attributes {
		omitted, keep, err := omitJSONNulls(value)
		if err != nil {
			return nil, fmt.Errorf("failed to copy attribute %q: %w", key, err)
		}
		if !keep {
			continue
		}
		cleaned[key] = omitted
	}
	attributes = cleaned

	relationships := map[string]json.RawMessage{}
	var missing []string
	for _, name := range ciWorkflowCreateRelationships {
		var relationship struct {
			Data *struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"data"`
		}
		raw, ok := envelope.Data.Relationships[name]
		if ok {
			if err := json.Unmarshal(raw, &relationship); err != nil {
				return nil, fmt.Errorf("failed to parse source workflow relationship %q: %w", name, err)
			}
		}
		if relationship.Data == nil || strings.TrimSpace(relationship.Data.ID) == "" {
			missing = append(missing, name)
			continue
		}
		linkage, err := json.Marshal(map[string]any{
			"data": map[string]string{"type": relationship.Data.Type, "id": relationship.Data.ID},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to encode relationship %q: %w", name, err)
		}
		relationships[name] = linkage
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf(
			"source workflow %s did not include the %s relationship linkage required to create a copy",
			opts.sourceWorkflowID,
			strings.Join(missing, ", "),
		)
	}

	payload, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"type":          "ciWorkflows",
			"attributes":    attributes,
			"relationships": relationships,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode workflow payload: %w", err)
	}

	return payload, nil
}

func isEmptyJSON(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func omitJSONNulls(raw json.RawMessage) (json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}

	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, false, err
	}
	omitted, keep := omitNulls(value)
	if !keep {
		return nil, false, nil
	}
	encoded, err := json.Marshal(omitted)
	if err != nil {
		return nil, false, err
	}
	return encoded, true, nil
}

func omitNulls(value any) (any, bool) {
	if value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			next, keep := omitNulls(item)
			if !keep {
				continue
			}
			out[key] = next
		}
		return out, true
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			next, keep := omitNulls(item)
			if !keep {
				continue
			}
			out = append(out, next)
		}
		return out, true
	default:
		return typed, true
	}
}

// XcodeCloudWorkflowsDuplicateCommand returns the xcode-cloud workflows duplicate subcommand.
func XcodeCloudWorkflowsDuplicateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("duplicate", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "ciWorkflows", "Source workflow ID to copy")
	name := fs.String("name", "", "Name for the new workflow")
	description := fs.String("description", "", "Description for the new workflow (default: copied from the source workflow)")
	enabled := fs.Bool("enabled", false, "Enable the new workflow immediately; the copy is created disabled by default")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "duplicate",
		ShortUsage: "asc xcode-cloud workflows duplicate --id \"WORKFLOW_ID\" --name \"New Workflow\"",
		ShortHelp:  "Duplicate a workflow under a new name.",
		LongHelp: `Duplicate a workflow under a new name.

Reads the source workflow, then creates a new workflow in the same product with the
same start conditions, actions, clean setting, Xcode version, macOS version, and
repository. The copy is created disabled so it cannot start builds before it is
reviewed, and unlocked so it can be edited; pass --enabled to create it enabled.

TestFlight post-actions and workflow environment variables are not part of the public
workflow schema, so they are not copied. Configure those with
asc web xcode-cloud workflows edit and asc web xcode-cloud env-vars.

Examples:
  asc xcode-cloud workflows duplicate --id "WORKFLOW_ID" --name "Nightly"
  asc xcode-cloud workflows duplicate --id "WORKFLOW_ID" --name "Nightly" --description "Nightly build"
  asc xcode-cloud workflows duplicate --id "WORKFLOW_ID" --name "Nightly" --enabled`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return shared.RequiredFlag("--id")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return shared.RequiredFlag("--name")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows duplicate: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			source, err := client.GetCiWorkflowRaw(requestCtx, idValue, ciWorkflowCreateRelationships...)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows duplicate: failed to read source workflow: %w", err)
			}

			payload, err := buildCiWorkflowDuplicatePayload(source, ciWorkflowDuplicateOptions{
				name:             nameValue,
				description:      *description,
				overrideDescr:    flagProvided(fs, "description"),
				enabled:          *enabled,
				sourceWorkflowID: idValue,
			})
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows duplicate: %w", err)
			}

			data, err := client.CreateCiWorkflowRaw(requestCtx, payload)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows duplicate: failed to create copy: %w", err)
			}

			return printCiWorkflowDuplicateOutput(data, *output.Output, *output.Pretty)
		},
	}
}

func printCiWorkflowDuplicateOutput(data json.RawMessage, format string, pretty bool) error {
	resolved, err := shared.ValidateOutputFormat(format, pretty)
	if err != nil {
		return err
	}
	if resolved == "json" {
		return shared.PrintOutput(json.RawMessage(data), format, pretty)
	}

	var resp asc.CiWorkflowResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse created workflow: %w", err)
	}
	return shared.PrintOutput(&resp, format, pretty)
}
