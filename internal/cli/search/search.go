package search

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

const defaultLimit = 10

var tokenPattern = regexp.MustCompile(`[a-z0-9][a-z0-9-]*`)

// SearchResponse is the machine-readable output for command discovery.
type SearchResponse struct {
	Query   string         `json:"query"`
	Count   int            `json:"count"`
	Results []SearchResult `json:"results"`
}

// SearchResult describes a matching CLI command.
type SearchResult struct {
	Command  string   `json:"command"`
	Summary  string   `json:"summary"`
	Usage    string   `json:"usage,omitempty"`
	Score    int      `json:"score"`
	Matched  []string `json:"matched"`
	Examples []string `json:"examples,omitempty"`
}

type commandDoc struct {
	Command     string
	Summary     string
	Usage       string
	LongHelp    string
	Examples    []string
	Flags       []string
	PathTokens  []string
	TextTokens  []string
	FlagTokens  []string
	AllTokens   []string
	CommandRank int
}

type scoredResult struct {
	result SearchResult
	rank   int
}

// SearchCommand returns the command-discovery search command.
func SearchCommand(commands func() []*ffcli.Command) *ffcli.Command {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", defaultLimit, "Maximum number of results to return")

	return &ffcli.Command{
		Name:       "search",
		ShortUsage: "asc search [flags] <query>",
		ShortHelp:  "Search asc commands and examples for agent-oriented command discovery.",
		LongHelp: `Search asc commands and examples for agent-oriented command discovery.

Search is local and deterministic. It indexes the registered command tree,
including command paths, summaries, usage strings, examples, and flag names.
It does not search App Store Connect data.

Examples:
  asc search "external testers"
  asc search --output json "submit app for review"
  asc search --output table "upload build"
  asc search --limit 5 "cert profiles"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			defer resetFlagSet(fs)

			args, err := parseInterspersedSearchFlags(fs, args)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return flag.ErrHelp
			}
			if len(args) == 0 {
				fmt.Fprintln(os.Stderr, "Error: search query is required")
				return flag.ErrHelp
			}
			if *limit <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --limit must be greater than 0")
				return flag.ErrHelp
			}

			query := strings.Join(args, " ")
			if strings.TrimSpace(query) == "" {
				fmt.Fprintln(os.Stderr, "Error: search query is required")
				return flag.ErrHelp
			}
			selectedOutput := *output.Output
			selectedPretty := *output.Pretty
			selectedLimit := *limit

			response := SearchCommands(commands(), query, selectedLimit)
			return shared.PrintOutputWithRenderers(
				response,
				selectedOutput,
				selectedPretty,
				func() error {
					asc.RenderTable([]string{"score", "command", "summary", "matched"}, searchRows(response.Results))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"score", "command", "summary", "matched"}, searchRows(response.Results))
					return nil
				},
			)
		},
	}
}

func resetFlagSet(fs *flag.FlagSet) {
	if fs == nil {
		return
	}
	fs.VisitAll(func(f *flag.Flag) {
		if f == nil {
			return
		}
		_ = f.Value.Set(f.DefValue)
	})
}

func parseInterspersedSearchFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	if fs == nil || len(args) == 0 {
		return args, nil
	}

	queryArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			queryArgs = append(queryArgs, args[i+1:]...)
			break
		}

		name, value, hasValue, ok := splitSearchFlagArg(arg)
		if !ok {
			queryArgs = append(queryArgs, arg)
			continue
		}

		f := fs.Lookup(name)
		if f == nil {
			queryArgs = append(queryArgs, arg)
			continue
		}

		if isBoolSearchFlag(f) && !hasValue {
			if err := f.Value.Set("true"); err != nil {
				return nil, fmt.Errorf("invalid value %q for --%s: %w", "true", name, err)
			}
			continue
		}

		if !hasValue {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: --%s", name)
			}
			i++
			value = args[i]
		}

		if err := f.Value.Set(value); err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: %w", value, name, err)
		}
	}

	return queryArgs, nil
}

func splitSearchFlagArg(arg string) (name, value string, hasValue, ok bool) {
	if arg == "" || arg == "-" || !strings.HasPrefix(arg, "-") {
		return "", "", false, false
	}

	trimmed := strings.TrimPrefix(arg, "--")
	if trimmed == arg {
		trimmed = strings.TrimPrefix(arg, "-")
	}
	if trimmed == "" {
		return "", "", false, false
	}

	name, value, hasValue = strings.Cut(trimmed, "=")
	if name == "" {
		return "", "", false, false
	}
	return name, value, hasValue, true
}

func isBoolSearchFlag(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}

	if f == nil {
		return false
	}
	boolValue, ok := f.Value.(boolFlag)
	return ok && boolValue.IsBoolFlag()
}

// SearchCommands searches a command tree and returns ranked results.
func SearchCommands(commands []*ffcli.Command, query string, limit int) SearchResponse {
	normalizedQuery := normalizeQuery(query)
	if limit <= 0 {
		limit = defaultLimit
	}
	docs := collectCommandDocs(commands)
	scored := scoreCommandDocs(docs, normalizedQuery)
	if len(scored) > limit {
		scored = scored[:limit]
	}

	results := make([]SearchResult, 0, len(scored))
	for _, item := range scored {
		results = append(results, item.result)
	}
	return SearchResponse{
		Query:   normalizedQuery,
		Count:   len(results),
		Results: results,
	}
}

func collectCommandDocs(commands []*ffcli.Command) []commandDoc {
	docs := make([]commandDoc, 0)
	for _, cmd := range commands {
		collectCommandDoc(&docs, cmd, nil)
	}
	return docs
}

func collectCommandDoc(docs *[]commandDoc, cmd *ffcli.Command, parents []string) {
	if cmd == nil || hiddenCommand(cmd) {
		return
	}

	pathParts := append(append([]string{"asc"}, parents...), cmd.Name)
	command := strings.Join(pathParts, " ")
	usage := strings.TrimSpace(cmd.ShortUsage)
	if usage == "" {
		usage = command
	}
	summary := strings.TrimSpace(cmd.ShortHelp)
	longHelp := strings.TrimSpace(cmd.LongHelp)
	examples := extractExamples(longHelp)
	flags := commandFlags(cmd)
	pathTokens := uniqueTokens(strings.Join(pathParts[1:], " "))
	textTokens := uniqueTokens(strings.Join([]string{summary, usage, longHelp, strings.Join(examples, " ")}, " "))
	flagTokens := uniqueTokens(strings.Join(flags, " "))

	all := append(append(append([]string{}, pathTokens...), textTokens...), flagTokens...)
	*docs = append(*docs, commandDoc{
		Command:     command,
		Summary:     summary,
		Usage:       usage,
		LongHelp:    longHelp,
		Examples:    examples,
		Flags:       flags,
		PathTokens:  pathTokens,
		TextTokens:  textTokens,
		FlagTokens:  flagTokens,
		AllTokens:   uniqueStrings(all),
		CommandRank: len(pathParts),
	})

	nextParents := append(append([]string{}, parents...), cmd.Name)
	for _, sub := range cmd.Subcommands {
		collectCommandDoc(docs, sub, nextParents)
	}
}

func scoreCommandDocs(docs []commandDoc, query string) []scoredResult {
	queryTokens := dropLeadingRootToken(uniqueTokens(query))
	if len(queryTokens) == 0 {
		return nil
	}

	results := make([]scoredResult, 0)
	for _, doc := range docs {
		score, matched := scoreCommandDoc(doc, queryTokens)
		if score <= 0 {
			continue
		}
		results = append(results, scoredResult{
			result: SearchResult{
				Command:  doc.Command,
				Summary:  doc.Summary,
				Usage:    doc.Usage,
				Score:    score,
				Matched:  matched,
				Examples: doc.Examples,
			},
			rank: doc.CommandRank,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].result.Score != results[j].result.Score {
			return results[i].result.Score > results[j].result.Score
		}
		if results[i].rank != results[j].rank {
			return results[i].rank < results[j].rank
		}
		return results[i].result.Command < results[j].result.Command
	})
	return results
}

func dropLeadingRootToken(tokens []string) []string {
	if len(tokens) == 0 || tokens[0] != "asc" {
		return tokens
	}
	return tokens[1:]
}

func scoreCommandDoc(doc commandDoc, queryTokens []string) (int, []string) {
	score := 0
	reasons := make([]string, 0)
	seenReasons := make(map[string]struct{})

	for _, token := range queryTokens {
		tokenScore, tokenReasons := scoreTerm(doc, token, "query:"+token)
		score += tokenScore
		for _, reason := range tokenReasons {
			addReason(&reasons, seenReasons, reason)
		}

		for _, alias := range aliasesFor(token) {
			if alias == token {
				continue
			}
			aliasScore, aliasReasons := scoreTerm(doc, alias, "alias:"+token)
			if aliasScore == 0 {
				continue
			}
			score += max(1, aliasScore/2)
			addReason(&reasons, seenReasons, "alias:"+token)
			for _, reason := range aliasReasons {
				addReason(&reasons, seenReasons, reason)
			}
		}
	}

	return score, reasons
}

func scoreTerm(doc commandDoc, term, reason string) (int, []string) {
	if strings.TrimSpace(term) == "" {
		return 0, nil
	}

	score := 0
	reasons := make([]string, 0, 4)
	commandWithoutASC := strings.TrimPrefix(doc.Command, "asc ")

	exactCommandMatch := commandWithoutASC == term || doc.Command == term
	if exactCommandMatch {
		return 120, []string{reason, "command:" + term}
	}
	if !exactCommandMatch && tokenContains(doc.PathTokens, term) {
		score += 60
		reasons = append(reasons, reason, "command:"+term)
	}
	if strings.Contains(strings.ToLower(doc.Command), term) && !tokenContains(doc.PathTokens, term) {
		score += 40
		reasons = append(reasons, reason, "command:"+term)
	}
	if strings.Contains(strings.ToLower(doc.Summary), term) {
		score += 35
		reasons = append(reasons, reason, "summary:"+term)
	}
	if strings.Contains(strings.ToLower(doc.Usage), term) {
		score += 25
		reasons = append(reasons, reason, "usage:"+term)
	}
	if tokenContains(doc.FlagTokens, term) {
		score += 20
		reasons = append(reasons, reason, "flag:"+term)
	}
	if strings.Contains(strings.ToLower(strings.Join(doc.Examples, "\n")), term) {
		score += 18
		reasons = append(reasons, reason, "example:"+term)
	}
	if strings.Contains(strings.ToLower(doc.LongHelp), term) {
		score += 10
		reasons = append(reasons, reason, "help:"+term)
	}

	if score == 0 && len(term) >= 4 && !strings.Contains(term, "-") {
		for _, suggestion := range suggest.Commands(term, doc.AllTokens) {
			if tokenContains(doc.AllTokens, suggestion) {
				score += 8
				reasons = append(reasons, reason, "fuzzy:"+suggestion)
				break
			}
		}
	}

	return score, uniqueStrings(reasons)
}

func aliasesFor(token string) []string {
	switch token {
	case "ship", "shipping":
		return []string{"publish", "submit", "release"}
	case "submission", "submissions":
		return []string{"submit", "review", "appstore"}
	case "review":
		return []string{"submit", "submission", "appstore"}
	case "beta":
		return []string{"testflight"}
	case "tester", "testers", "user", "users":
		return []string{"tester", "testers", "beta-testers", "testflight"}
	case "external":
		return []string{"testflight", "beta", "tester", "testers", "group", "groups"}
	case "cert", "certs":
		return []string{"certificate", "certificates"}
	case "provisioning":
		return []string{"profiles", "profile"}
	case "ipa", "binary":
		return []string{"build", "upload", "uploads"}
	case "appstore", "store":
		return []string{"appstore", "app", "review", "publish"}
	default:
		return nil
	}
}

func commandFlags(cmd *ffcli.Command) []string {
	if cmd.FlagSet == nil {
		return nil
	}
	flags := make([]string, 0)
	cmd.FlagSet.VisitAll(func(f *flag.Flag) {
		if f == nil {
			return
		}
		flags = append(flags, f.Name)
		if strings.TrimSpace(f.Usage) != "" {
			flags = append(flags, f.Usage)
		}
	})
	return flags
}

func extractExamples(longHelp string) []string {
	lines := strings.Split(longHelp, "\n")
	examples := make([]string, 0)
	inExamples := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "Examples:") {
			inExamples = true
			continue
		}
		if !inExamples {
			continue
		}
		if trimmed == "" {
			if len(examples) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "asc ") {
			examples = append(examples, trimmed)
		}
	}
	return examples
}

func hiddenCommand(cmd *ffcli.Command) bool {
	shortHelp := strings.TrimSpace(cmd.ShortHelp)
	return strings.HasPrefix(shortHelp, "DEPRECATED:") ||
		strings.HasPrefix(shortHelp, "REMOVED:") ||
		strings.HasPrefix(shortHelp, "Compatibility alias:")
}

func searchRows(results []SearchResult) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			fmt.Sprintf("%d", result.Score),
			result.Command,
			result.Summary,
			summarizeMatches(result.Matched),
		})
	}
	return rows
}

func summarizeMatches(matches []string) string {
	const maxMatches = 6
	if len(matches) <= maxMatches {
		return strings.Join(matches, ", ")
	}
	return strings.Join(matches[:maxMatches], ", ") + fmt.Sprintf(", +%d more", len(matches)-maxMatches)
}

func normalizeQuery(query string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
}

func uniqueTokens(text string) []string {
	matches := tokenPattern.FindAllString(strings.ToLower(text), -1)
	return uniqueStrings(matches)
}

func tokenContains(tokens []string, term string) bool {
	for _, token := range tokens {
		if token == term {
			return true
		}
		if len(term) >= 3 && strings.Contains(token, term) {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func addReason(reasons *[]string, seen map[string]struct{}, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	if _, ok := seen[reason]; ok {
		return
	}
	seen[reason] = struct{}{}
	*reasons = append(*reasons, reason)
}
