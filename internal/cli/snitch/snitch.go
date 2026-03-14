package snitch

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	githubTokenEnvVar    = "GITHUB_TOKEN"
	githubTokenGHEnvVar  = "GH_TOKEN"
	defaultOwner         = "rudrankriyam"
	defaultRepo          = "App-Store-Connect-CLI"
	maxSearchResults     = 5
	maxResponseBodyBytes = 8192
)

// githubAPIBase is a variable so tests can override it with httptest servers.
var githubAPIBase = "https://api.github.com"

// setGitHubAPIBase is used by tests to point at httptest servers.
func setGitHubAPIBase(base string) {
	githubAPIBase = base
}

var validSeverities = []string{"bug", "friction", "feature-request"}

// githubHTTPClient is a package-level var for testability.
var githubHTTPClient = func() *http.Client {
	return &http.Client{Timeout: asc.ResolveTimeout()}
}

// SnitchCommand returns the top-level snitch command.
func SnitchCommand(version string) *ffcli.Command {
	fs := flag.NewFlagSet("snitch", flag.ExitOnError)

	repro := fs.String("repro", "", "Reproduction command (e.g., the exact asc command that failed)")
	expected := fs.String("expected", "", "Expected behavior")
	actual := fs.String("actual", "", "Actual behavior or error message")
	severity := fs.String("severity", "bug", "Severity: bug, friction, or feature-request")
	dryRun := fs.Bool("dry-run", false, "Search for duplicates and preview without filing")
	local := fs.Bool("local", false, "Log to .asc/snitch.log instead of filing on GitHub")
	confirm := fs.Bool("confirm", false, "Create the GitHub issue after duplicate search")

	return &ffcli.Command{
		Name:       "snitch",
		ShortUsage: `asc snitch [flags] "description"`,
		ShortHelp:  "Report CLI friction as a GitHub issue.",
		LongHelp: `Report CLI friction directly from the terminal.

Searches for duplicate issues when GITHUB_TOKEN or GH_TOKEN is available.
Without --confirm, snitch prints a preview only. Use --local to log friction
offline for later review with "asc snitch flush".
Place flags before the description. If the description itself contains text
that looks like a flag (for example, "--app"), wrap the full description in quotes.

Examples:
  asc snitch --repro 'asc crashes --app "com.example"' --expected "Should resolve bundle ID" --actual "Error: AppId is invalid" --confirm "crashes --app doesn't support bundle ID"
  asc snitch --dry-run "group name ambiguity"
  asc snitch --local "status command needs bundle ID support"
  asc snitch flush
  asc snitch flush --file .asc/snitch.log`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			flushCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return shared.UsageError("description is required")
			}

			description, err := descriptionFromArgs(args, fs)
			if err != nil {
				return err
			}

			sev := strings.TrimSpace(strings.ToLower(*severity))
			if !isValidSeverity(sev) {
				return shared.UsageErrorf("--severity must be one of: %s", strings.Join(validSeverities, ", "))
			}

			entry := LogEntry{
				Description: description,
				Repro:       strings.TrimSpace(*repro),
				Expected:    strings.TrimSpace(*expected),
				Actual:      strings.TrimSpace(*actual),
				Severity:    sev,
				Timestamp:   time.Now().UTC(),
				ASCVersion:  version,
				OS:          fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			}

			if *local && !*dryRun {
				return writeLocalLog(entry)
			}

			token := resolveGitHubToken()
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var duplicates []GitHubIssue
			if token != "" {
				var err error
				duplicates, err = searchIssues(requestCtx, token, issueTitle(entry))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: duplicate search failed: %v\n", err)
				}
			} else {
				fmt.Fprintln(os.Stderr, "Note: skipping duplicate search because GITHUB_TOKEN or GH_TOKEN is not set.")
			}

			printPotentialDuplicates(duplicates)

			if *dryRun || !*confirm {
				printPreview(entry, *dryRun)
				return nil
			}

			if token == "" {
				return fmt.Errorf("snitch: GITHUB_TOKEN or GH_TOKEN is required to create issues")
			}

			issue, err := createIssue(requestCtx, token, entry)
			if err != nil {
				return fmt.Errorf("snitch: failed to create issue: %w", err)
			}
			if labels := issueLabels(entry); len(labels) > 0 {
				if err := addIssueLabels(requestCtx, token, issue.Number, labels); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: issue created, but labels could not be applied: %v\n", err)
				}
			}

			fmt.Fprintf(os.Stderr, "Issue created: #%d %s\n", issue.Number, issue.HTMLURL)
			result := map[string]any{
				"number":   issue.Number,
				"html_url": issue.HTMLURL,
				"title":    issue.Title,
			}
			return json.NewEncoder(os.Stdout).Encode(result)
		},
	}
}

func descriptionFromArgs(args []string, fs *flag.FlagSet) (string, error) {
	parts := make([]string, 0, len(args))
	for i, raw := range args {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}

		if i > 0 {
			if misplacedFlag := trailingSnitchFlag(part, fs); misplacedFlag != "" {
				return "", shared.UsageErrorf(
					"flags must appear before the description; move %s before the description or quote it if it is part of the report text",
					misplacedFlag,
				)
			}
		}

		parts = append(parts, part)
	}

	description := strings.TrimSpace(strings.Join(parts, " "))
	if description == "" {
		return "", shared.UsageError("description must not be empty")
	}

	return description, nil
}

func trailingSnitchFlag(token string, fs *flag.FlagSet) string {
	if token == "" || token == "--" {
		return ""
	}

	if strings.HasPrefix(token, "--") {
		name := token[2:]
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			name = name[:idx]
		}
		if fs.Lookup(name) != nil {
			return "--" + name
		}
		return ""
	}

	if strings.HasPrefix(token, "-") {
		name := token[1:]
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			name = name[:idx]
		}
		if fs.Lookup(name) != nil {
			return "-" + name
		}
	}

	return ""
}

func flushCommand() *ffcli.Command {
	fs := flag.NewFlagSet("snitch flush", flag.ExitOnError)
	logFile := fs.String("file", "", "Path to snitch log file (default: .asc/snitch.log)")

	return &ffcli.Command{
		Name:       "flush",
		ShortUsage: "asc snitch flush [--file PATH]",
		ShortHelp:  "Review locally logged friction entries.",
		LongHelp: `Review friction entries logged with --local.

Prints all entries from .asc/snitch.log (or --file path) in a readable format.
Filing from flush is manual: copy the description and rerun "asc snitch"
with --confirm when you're ready to create the issue.

Examples:
  asc snitch flush
  asc snitch flush --file .asc/snitch.log`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("snitch flush does not accept positional arguments; use --file PATH to specify a log file")
			}

			path := strings.TrimSpace(*logFile)
			if path == "" {
				path = filepath.Join(".asc", "snitch.log")
			}

			entries, err := readLocalLog(path)
			if os.IsNotExist(err) {
				fmt.Fprintln(os.Stderr, "No local snitch entries found.")
				return nil
			}
			if err != nil {
				return fmt.Errorf("snitch flush: %w", err)
			}

			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "No local snitch entries found.")
				return nil
			}

			fmt.Fprint(os.Stdout, formatLocalEntries(entries))
			return nil
		},
	}
}

// LogEntry represents a friction report.
type LogEntry struct {
	Description string    `json:"description"`
	Repro       string    `json:"repro,omitempty"`
	Expected    string    `json:"expected,omitempty"`
	Actual      string    `json:"actual,omitempty"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	ASCVersion  string    `json:"asc_version"`
	OS          string    `json:"os"`
}

// GitHubIssue represents a GitHub issue (search result or creation response).
type GitHubIssue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
}

func isValidSeverity(s string) bool {
	for _, v := range validSeverities {
		if s == v {
			return true
		}
	}
	return false
}

func resolveGitHubToken() string {
	if v := strings.TrimSpace(os.Getenv(githubTokenEnvVar)); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(githubTokenGHEnvVar)); v != "" {
		return v
	}
	return ""
}

func issueTitle(e LogEntry) string {
	prefix := ""
	switch e.Severity {
	case "friction":
		prefix = "Friction: "
	case "feature-request":
		prefix = "Feature: "
	}
	return prefix + e.Description
}

func issueBody(e LogEntry) string {
	var b strings.Builder

	b.WriteString("## Summary\n\n")
	b.WriteString(e.Description)
	b.WriteString("\n")

	if e.Repro != "" {
		b.WriteString("\n## Reproduction\n\n```bash\n")
		b.WriteString(e.Repro)
		b.WriteString("\n```\n")
	}

	if e.Expected != "" {
		b.WriteString("\n## Expected behavior\n\n")
		b.WriteString(e.Expected)
		b.WriteString("\n")
	}

	if e.Actual != "" {
		b.WriteString("\n## Actual behavior\n\n```\n")
		b.WriteString(e.Actual)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n## Environment\n\n")
	b.WriteString(fmt.Sprintf("- **asc version:** %s\n", e.ASCVersion))
	b.WriteString(fmt.Sprintf("- **OS:** %s\n", e.OS))
	b.WriteString(fmt.Sprintf("- **Filed via:** `asc snitch`\n"))

	return b.String()
}

func issueLabels(e LogEntry) []string {
	labels := []string{"asc-snitch"}
	switch e.Severity {
	case "bug":
		labels = append(labels, "bug")
	case "feature-request":
		labels = append(labels, "enhancement")
	}
	return labels
}

func printPotentialDuplicates(duplicates []GitHubIssue) {
	if len(duplicates) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "Potentially related issues (%d):\n", len(duplicates))
	for _, dup := range duplicates {
		fmt.Fprintf(os.Stderr, "  #%d %s\n       %s\n", dup.Number, dup.Title, dup.HTMLURL)
	}
	fmt.Fprintln(os.Stderr)
}

func printPreview(entry LogEntry, dryRun bool) {
	if dryRun {
		fmt.Fprintln(os.Stderr, "--- Dry run: would create issue ---")
	} else {
		fmt.Fprintln(os.Stderr, "--- Preview only: rerun with --confirm to create issue ---")
	}
	fmt.Fprintf(os.Stderr, "Title: %s\n", issueTitle(entry))
	fmt.Fprintf(os.Stderr, "Body:\n%s\n", issueBody(entry))
}

func readLocalLog(path string) ([]LogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	lines := strings.Split(trimmed, "\n")
	entries := make([]LogEntry, 0, len(lines))
	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("invalid log entry on line %d: %w", i+1, err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func formatLocalEntries(entries []LogEntry) string {
	var b strings.Builder

	for i, entry := range entries {
		fmt.Fprintf(&b, "[%d] %s: %s\n", i+1, entry.Severity, entry.Description)
		if !entry.Timestamp.IsZero() {
			fmt.Fprintf(&b, "Timestamp: %s\n", entry.Timestamp.Format(time.RFC3339))
		}
		if entry.ASCVersion != "" {
			fmt.Fprintf(&b, "ASC version: %s\n", entry.ASCVersion)
		}
		if entry.OS != "" {
			fmt.Fprintf(&b, "OS: %s\n", entry.OS)
		}
		if entry.Repro != "" {
			fmt.Fprintf(&b, "Reproduction:\n%s\n", entry.Repro)
		}
		if entry.Expected != "" {
			fmt.Fprintf(&b, "Expected:\n%s\n", entry.Expected)
		}
		if entry.Actual != "" {
			fmt.Fprintf(&b, "Actual:\n%s\n", entry.Actual)
		}
		if i < len(entries)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func searchIssues(ctx context.Context, token string, query string) ([]GitHubIssue, error) {
	// Search open issue titles first to reduce noisy matches from generic terms.
	q := fmt.Sprintf("repo:%s/%s is:issue is:open in:title %q", defaultOwner, defaultRepo, strings.TrimSpace(query))
	searchURL := fmt.Sprintf("%s/search/issues?q=%s&per_page=%d",
		githubAPIBase, url.QueryEscape(q), maxSearchResults)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := githubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		limited := io.LimitReader(resp.Body, maxResponseBodyBytes)
		body, _ := io.ReadAll(limited)
		return nil, fmt.Errorf("GitHub search returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Items []GitHubIssue `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return result.Items, nil
}

func createIssue(ctx context.Context, token string, entry LogEntry) (*GitHubIssue, error) {
	issueURL := fmt.Sprintf("%s/repos/%s/%s/issues", githubAPIBase, defaultOwner, defaultRepo)

	payload := map[string]any{
		"title": issueTitle(entry),
		"body":  issueBody(entry),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", issueURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := githubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, readGitHubAPIError(resp)
	}

	var issue GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode issue response: %w", err)
	}

	return &issue, nil
}

func addIssueLabels(ctx context.Context, token string, issueNumber int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	labelsURL := fmt.Sprintf(
		"%s/repos/%s/%s/issues/%d/labels",
		githubAPIBase,
		defaultOwner,
		defaultRepo,
		issueNumber,
	)

	payload := map[string]any{
		"labels": labels,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", labelsURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := githubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return readGitHubAPIError(resp)
	}

	return nil
}

func readGitHubAPIError(resp *http.Response) error {
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes)
	respBody, _ := io.ReadAll(limited)
	return fmt.Errorf("GitHub returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
}

func writeLocalLog(entry LogEntry) error {
	dir := ".asc"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("snitch: failed to create %s: %w", dir, err)
	}

	path := filepath.Join(dir, "snitch.log")

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("snitch: failed to marshal entry: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("snitch: failed to open %s: %w", path, err)
	}
	defer f.Close()

	if err := f.Chmod(0o600); err != nil {
		return fmt.Errorf("snitch: failed to set secure permissions on %s: %w", path, err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("snitch: failed to write entry: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Friction logged to %s\n", path)
	return nil
}
