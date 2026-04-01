package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
)

// RunASCCommand runs an arbitrary asc CLI command and returns the raw output.
// Args is a shell-style command string, e.g. `reviews list --app "123" --limit 10 --output json`.
func (a *App) RunASCCommand(args string) (ASCCommandResponse, error) {
	if strings.TrimSpace(args) == "" {
		return ASCCommandResponse{Error: "args required"}, nil
	}

	parts, err := parseASCCommandArgs(args)
	if err != nil {
		return ASCCommandResponse{Error: "Invalid command arguments: " + err.Error()}, nil
	}
	if !isAllowedStudioCommand(parts) {
		return ASCCommandResponse{Error: "Command is not allowed in ASC Studio"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ASCCommandResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()
	out, err := a.runASCCombinedOutput(ctx, ascPath, parts...)
	if err != nil {
		return ASCCommandResponse{Error: strings.TrimSpace(string(out))}, nil
	}
	return ASCCommandResponse{Data: string(out)}, nil
}

// GetFinanceRegions fetches finance report region codes.
func (a *App) GetFinanceRegions() (FinanceResponse, error) {
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return FinanceResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	out, err := a.runASCCombinedOutput(ctx, ascPath, "finance", "regions", "--output", "json")
	if err != nil {
		return FinanceResponse{Error: strings.TrimSpace(string(out))}, nil
	}
	regions, err := parseFinanceRegionsOutput(out)
	if err != nil {
		return FinanceResponse{Error: err.Error()}, nil
	}
	return FinanceResponse{Regions: regions}, nil
}

func parseASCCommandArgs(args string) ([]string, error) {
	return shellquote.Split(strings.TrimSpace(args))
}

func parseFinanceRegionsOutput(out []byte) ([]FinanceRegion, error) {
	var regionsEnvelope struct {
		Regions []FinanceRegion `json:"regions"`
	}
	if err := json.Unmarshal(out, &regionsEnvelope); err == nil && regionsEnvelope.Regions != nil {
		return regionsEnvelope.Regions, nil
	}

	var dataEnvelope struct {
		Data []FinanceRegion `json:"data"`
	}
	if err := json.Unmarshal(out, &dataEnvelope); err == nil && dataEnvelope.Data != nil {
		return dataEnvelope.Data, nil
	}

	return nil, errors.New("failed to parse finance regions")
}

func isAllowedStudioCommand(parts []string) bool {
	path := studioCommandPath(parts)
	if path == "" {
		return false
	}
	_, ok := allowedStudioCommandPaths[path]
	return ok
}

func studioCommandPath(parts []string) string {
	path := make([]string, 0, 4)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			break
		}
		path = append(path, part)
	}
	return strings.Join(path, " ")
}
