package apps

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/term"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

const (
	communityWallUpstreamOwner        = "rudrankriyam"
	communityWallUpstreamRepo         = "App-Store-Connect-CLI"
	communityWallUpstreamBranch       = "main"
	communityWallGitHubMaxBodyBytes   = 8192
	defaultCommunityWallSubmitOutput  = "json"
	defaultCommunityWallSubmitMessage = "Create the fork, branch, commit, and pull request"
)

var (
	communityWallGitHubAPIBase     = "https://api.github.com"
	communityWallAppStoreLookupURL = "https://itunes.apple.com"
	communityWallGitHubClient      = func() *http.Client {
		return &http.Client{Timeout: asc.ResolveTimeout()}
	}
	communityWallGHCommandRunner = defaultCommunityWallGHCommandRunner
	communityWallPromptEnabled   = func() bool {
		return term.IsTerminal(int(os.Stdin.Fd()))
	}
	communityWallNow = func() time.Time {
		return time.Now().UTC()
	}
	communityWallSleep            = time.Sleep
	communityWallLookupAppDetails = fetchCommunityWallAppDetails
)

var (
	errCommunityWallUsage               = errors.New("community wall usage")
	communityWallAppStoreIDPattern      = regexp.MustCompile(`/id(\d+)`)
	communityWallAppStoreCountryPattern = regexp.MustCompile(`^https?://apps\.apple\.com/([a-zA-Z]{2})/app(?:/|$)`)
	communityWallNumericIDPattern       = regexp.MustCompile(`^\d+$`)
)

type communityWallSubmitInput struct {
	AppID string
	Name  string
	Link  string
}

type communityWallSubmitRequest struct {
	Input       communityWallSubmitInput
	GitHubToken string
	GitHubLogin string
	DryRun      bool
}

type communityWallSubmitResult struct {
	Mode              string   `json:"mode"`
	AppID             string   `json:"appId,omitempty"`
	App               string   `json:"app"`
	Link              string   `json:"link"`
	UpstreamRepo      string   `json:"upstreamRepo"`
	ForkRepo          string   `json:"forkRepo"`
	Branch            string   `json:"branch"`
	ChangedFiles      []string `json:"changedFiles"`
	CommitMessage     string   `json:"commitMessage"`
	PullRequestTitle  string   `json:"pullRequestTitle"`
	PullRequestBody   string   `json:"pullRequestBody"`
	PullRequestURL    string   `json:"pullRequestUrl,omitempty"`
	PullRequestNumber int      `json:"pullRequestNumber,omitempty"`
	WillCreateFork    bool     `json:"willCreateFork,omitempty"`
	CreatedFork       bool     `json:"createdFork,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

type communityWallAppDetails struct {
	Name string
	Link string
	Icon string
}

type communityWallGitHubUser struct {
	Login string `json:"login"`
}

type communityWallGitHubRepo struct {
	FullName string                        `json:"full_name"`
	Fork     bool                          `json:"fork"`
	Parent   *communityWallGitHubRepoBrief `json:"parent,omitempty"`
}

type communityWallGitHubRepoBrief struct {
	FullName string `json:"full_name"`
}

type communityWallGitHubRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type communityWallGitHubContent struct {
	SHA      string `json:"sha"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

type communityWallGitHubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

type communityWallGitHubClientAPI struct {
	Token string
}

type communityWallUsageError struct {
	message string
}

func (e communityWallUsageError) Error() string {
	return e.message
}

func (e communityWallUsageError) Is(target error) bool {
	return target == errCommunityWallUsage
}

// AppsWallSubmitCommand returns the wall submit subcommand.
func AppsWallSubmitCommand(parentWallFlags *flag.FlagSet) *ffcli.Command {
	fs := flag.NewFlagSet("apps wall submit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store or App Store Connect app ID")
	name := fs.String("name", "", "Override the app name resolved from the App Store lookup")
	link := fs.String("link", "", "Manual app URL for non-App-Store entries")
	confirm := fs.Bool("confirm", false, defaultCommunityWallSubmitMessage)
	dryRun := fs.Bool("dry-run", false, "Preview the fork, branch, and pull request plan without creating anything")
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultCommunityWallSubmitOutput, "Output format: json (default)", "json")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc apps wall submit [flags]",
		ShortHelp:  "Open a Wall of Apps pull request using your GitHub CLI session.",
		LongHelp: `Open a Wall of Apps pull request using your authenticated GitHub CLI session.

Use --app for the normal flow: the command resolves the public App Store name, URL,
and icon from the app ID automatically. For entries that are not on the public App
Store yet, use --link with --name instead.

Prompts for missing fields when running interactively. The pull request only updates
docs/wall-of-apps.json so community submissions stay focused to a single file.

Examples:
  asc apps wall submit --app "1234567890" --dry-run
  asc apps wall submit --app "1234567890" --confirm
  asc apps wall submit --link "https://testflight.apple.com/join/ABCDEFG" --name "My Beta App" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: apps wall submit does not accept positional arguments")
				return flag.ErrHelp
			}
			if err := validateAppsWallSubmitParentFlags(parentWallFlags); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if !*dryRun && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required unless --dry-run is set")
				return flag.ErrHelp
			}

			token, ghLogin, err := resolveCommunityWallGitHubIdentity(ctx)
			if err != nil {
				return fmt.Errorf("apps wall submit: %w", err)
			}

			input, err := collectCommunityWallSubmitInput(*appID, *name, *link)
			if err != nil {
				if errors.Is(err, errCommunityWallUsage) {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
					return flag.ErrHelp
				}
				return fmt.Errorf("apps wall submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := submitCommunityWallEntry(requestCtx, communityWallSubmitRequest{
				Input:       input,
				GitHubToken: token,
				GitHubLogin: ghLogin,
				DryRun:      *dryRun,
			})
			if err != nil {
				if errors.Is(err, errCommunityWallUsage) {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
					return flag.ErrHelp
				}
				return fmt.Errorf("apps wall submit: %w", err)
			}

			for _, warning := range result.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
			if result.PullRequestURL != "" {
				fmt.Fprintf(os.Stderr, "Pull request created: #%d %s\n", result.PullRequestNumber, result.PullRequestURL)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func validateAppsWallSubmitParentFlags(parentWallFlags *flag.FlagSet) error {
	if parentWallFlags == nil {
		return nil
	}

	visited := make([]string, 0, 4)
	parentWallFlags.Visit(func(f *flag.Flag) {
		visited = append(visited, "--"+strings.TrimSpace(f.Name))
	})
	if len(visited) == 0 {
		return nil
	}

	sort.Strings(visited)
	return communityWallUsageError{
		message: fmt.Sprintf(
			"apps wall submit does not accept parent wall flags (%s); move flags after \"submit\" or use \"asc apps wall\" for listing",
			strings.Join(visited, ", "),
		),
	}
}

func resolveCommunityWallGitHubIdentity(ctx context.Context) (string, string, error) {
	if _, _, err := communityWallGHCommandRunner(ctx, "auth", "status"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", "", fmt.Errorf("gh CLI is required; install it from https://cli.github.com/")
		}
		return "", "", fmt.Errorf("gh auth status failed; run 'gh auth login' first")
	}

	stdout, stderr, err := communityWallGHCommandRunner(ctx, "auth", "token")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", "", fmt.Errorf("gh CLI is required; install it from https://cli.github.com/")
		}
		message := strings.TrimSpace(string(stderr))
		if message == "" {
			message = strings.TrimSpace(err.Error())
		}
		return "", "", fmt.Errorf("failed to read GitHub token from gh: %s", message)
	}

	token := strings.TrimSpace(string(stdout))
	if token == "" {
		return "", "", fmt.Errorf("gh auth token returned an empty token; run 'gh auth login' first")
	}

	client := communityWallGitHubClientAPI{Token: token}
	login, err := client.currentUserLogin(ctx)
	if err != nil {
		return "", "", err
	}
	return token, login, nil
}

func collectCommunityWallSubmitInput(appIDValue, nameValue, linkValue string) (communityWallSubmitInput, error) {
	canPrompt := communityWallPromptEnabled()

	appIDValue = normalizeCommunityWallAppID(appIDValue)
	nameValue = strings.TrimSpace(nameValue)
	linkValue = strings.TrimSpace(linkValue)

	if appIDValue != "" && linkValue != "" {
		return communityWallSubmitInput{}, communityWallUsageError{message: "use either --app or --link, not both"}
	}

	if appIDValue == "" && linkValue == "" {
		if !canPrompt {
			return communityWallSubmitInput{}, communityWallUsageError{message: "--app is required"}
		}
		prompted, err := promptCommunityWallSubmitText(
			"App ID:",
			"Paste the App Store or App Store Connect app ID. Leave blank to use a manual link instead.",
			validateCommunityWallOptionalAppIDValue,
		)
		if err != nil {
			return communityWallSubmitInput{}, err
		}
		appIDValue = normalizeCommunityWallAppID(prompted)
	}

	if appIDValue == "" && linkValue == "" {
		if !canPrompt {
			return communityWallSubmitInput{}, communityWallUsageError{message: "--app or --link is required"}
		}
		prompted, err := promptCommunityWallSubmitText(
			"Manual link:",
			"Paste the TestFlight, GitHub, or product URL for entries that are not on the public App Store yet.",
			validateCommunityWallLinkValue,
		)
		if err != nil {
			return communityWallSubmitInput{}, err
		}
		linkValue = prompted
	}

	if appIDValue != "" {
		if err := validateCommunityWallAppID(appIDValue); err != nil {
			return communityWallSubmitInput{}, communityWallUsageError{message: "--app must be a numeric app ID"}
		}
	} else {
		if err := validateCommunityWallHTTPURL(linkValue); err != nil {
			return communityWallSubmitInput{}, communityWallUsageError{message: "--link must be a valid http/https URL"}
		}
		if nameValue == "" {
			if !canPrompt {
				return communityWallSubmitInput{}, communityWallUsageError{message: "--name is required when --link is used"}
			}
			prompted, err := promptCommunityWallSubmitText(
				"App name:",
				"The name that should appear on the Wall of Apps",
				validateCommunityWallRequiredValue,
			)
			if err != nil {
				return communityWallSubmitInput{}, err
			}
			nameValue = prompted
		}
	}

	input := communityWallSubmitInput{
		AppID: appIDValue,
		Name:  nameValue,
		Link:  linkValue,
	}

	if input.AppID == "" && input.Link == "" {
		return communityWallSubmitInput{}, communityWallUsageError{message: "--app or --link is required"}
	}

	return input, nil
}

func promptCommunityWallSubmitText(message, help string, validator survey.Validator) (string, error) {
	answer := ""
	opts := []survey.AskOpt{}
	if validator != nil {
		opts = append(opts, survey.WithValidator(validator))
	}
	if err := survey.AskOne(&survey.Input{
		Message: message,
		Help:    help,
	}, &answer, opts...); err != nil {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}

func validateCommunityWallRequiredValue(ans interface{}) error {
	value, _ := ans.(string)
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

func validateCommunityWallLinkValue(ans interface{}) error {
	value, _ := ans.(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value is required")
	}
	if err := validateCommunityWallHTTPURL(value); err != nil {
		return fmt.Errorf("must be a valid http/https URL")
	}
	return nil
}

func validateCommunityWallOptionalAppIDValue(ans interface{}) error {
	value, _ := ans.(string)
	value = normalizeCommunityWallAppID(value)
	if value == "" {
		return nil
	}
	if err := validateCommunityWallAppID(value); err != nil {
		return fmt.Errorf("must be a numeric app ID")
	}
	return nil
}

func validateCommunityWallAppID(value string) error {
	if !communityWallNumericIDPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("invalid app ID")
	}
	return nil
}

func resolveCommunityWallCandidate(ctx context.Context, input communityWallSubmitInput) (communityWallEntry, []string, error) {
	candidate := communityWallEntry{}
	warnings := []string{}

	if input.AppID != "" {
		detailsByID, err := communityWallLookupAppDetails(ctx, []string{input.AppID})
		if err != nil {
			return communityWallEntry{}, nil, fmt.Errorf("failed to resolve app %q from the App Store lookup: %w", input.AppID, err)
		}
		details, ok := detailsByID[input.AppID]
		if !ok {
			return communityWallEntry{}, nil, fmt.Errorf("app %q was not found in the App Store lookup; use --link with --name for non-App-Store entries", input.AppID)
		}

		candidate.App = strings.TrimSpace(input.Name)
		if candidate.App == "" {
			candidate.App = strings.TrimSpace(details.Name)
		}
		candidate.Link = strings.TrimSpace(details.Link)
		if candidate.Link == "" {
			candidate.Link = communityWallAppStoreURL(input.AppID)
		}
		candidate.Icon = strings.TrimSpace(details.Icon)
	} else {
		candidate.App = strings.TrimSpace(input.Name)
		candidate.Link = strings.TrimSpace(input.Link)
		if iconURL, err := communityWallIconForLink(ctx, candidate.Link); err != nil {
			warnings = append(warnings, fmt.Sprintf("unable to refresh App Store icon: %v", err))
		} else if strings.TrimSpace(iconURL) != "" {
			candidate.Icon = iconURL
		}
	}

	if strings.TrimSpace(candidate.App) == "" {
		return communityWallEntry{}, nil, fmt.Errorf("app name could not be resolved")
	}
	if strings.TrimSpace(candidate.Link) == "" {
		return communityWallEntry{}, nil, fmt.Errorf("app link could not be resolved")
	}

	return candidate, warnings, nil
}

func submitCommunityWallEntry(ctx context.Context, req communityWallSubmitRequest) (*communityWallSubmitResult, error) {
	if strings.TrimSpace(req.GitHubToken) == "" {
		return nil, fmt.Errorf("missing GitHub token")
	}
	if strings.TrimSpace(req.GitHubLogin) == "" {
		return nil, fmt.Errorf("missing GitHub login")
	}

	client := communityWallGitHubClientAPI{Token: req.GitHubToken}
	candidate, warnings, err := resolveCommunityWallCandidate(ctx, req.Input)
	if err != nil {
		return nil, err
	}

	branch := communityWallBranchName(candidate.App, communityWallNow())
	changeTitle := communityWallChangeTitle(candidate.App)
	commitMessage := changeTitle
	prTitle := changeTitle
	prBody := communityWallPullRequestBody(req.Input, candidate)
	upstreamRepo := communityWallUpstreamOwner + "/" + communityWallUpstreamRepo
	forkRepo := req.GitHubLogin + "/" + communityWallUpstreamRepo

	result := &communityWallSubmitResult{
		Mode:             "dry-run",
		AppID:            req.Input.AppID,
		App:              candidate.App,
		Link:             candidate.Link,
		UpstreamRepo:     upstreamRepo,
		ForkRepo:         forkRepo,
		Branch:           branch,
		ChangedFiles:     []string{communityWallSourcePath},
		CommitMessage:    commitMessage,
		PullRequestTitle: prTitle,
		PullRequestBody:  prBody,
		Warnings:         warnings,
	}

	forkRepoInfo, err := client.repo(ctx, req.GitHubLogin, communityWallUpstreamRepo)
	if err != nil {
		return nil, err
	}
	forkExists := forkRepoInfo != nil
	result.WillCreateFork = !forkExists
	if forkRepoInfo != nil && !forkRepoInfo.isForkOf(upstreamRepo) {
		return nil, fmt.Errorf(
			"existing repo %s is not a fork of %s; rename or remove it before using apps wall submit",
			forkRepoInfo.displayName(req.GitHubLogin, communityWallUpstreamRepo),
			upstreamRepo,
		)
	}

	baseRefSHA, err := client.refSHA(ctx, communityWallUpstreamOwner, communityWallUpstreamRepo, "heads/"+communityWallUpstreamBranch)
	if err != nil {
		return nil, err
	}

	upstreamContent, upstreamFileSHA, err := client.fileContents(ctx, communityWallUpstreamOwner, communityWallUpstreamRepo, communityWallSourcePath, baseRefSHA)
	if err != nil {
		return nil, err
	}
	entries, err := parseCommunityWallSourceEntries(upstreamContent, communityWallSourcePath)
	if err != nil {
		return nil, err
	}

	updatedEntries, err := addCommunityWallSourceEntry(entries, candidate)
	if err != nil {
		return nil, err
	}
	updatedContent, err := renderCommunityWallSourceEntries(updatedEntries)
	if err != nil {
		return nil, err
	}

	if req.DryRun {
		return result, nil
	}

	if !forkExists {
		if err := client.createFork(ctx, communityWallUpstreamOwner, communityWallUpstreamRepo); err != nil {
			return nil, err
		}
		if err := client.waitForRepo(ctx, req.GitHubLogin, communityWallUpstreamRepo); err != nil {
			return nil, err
		}
		forkRepoInfo, err = client.repo(ctx, req.GitHubLogin, communityWallUpstreamRepo)
		if err != nil {
			return nil, err
		}
		if forkRepoInfo == nil || !forkRepoInfo.isForkOf(upstreamRepo) {
			return nil, fmt.Errorf("created repo %s/%s is not recognized as a fork of %s", req.GitHubLogin, communityWallUpstreamRepo, upstreamRepo)
		}
		result.CreatedFork = true
	}

	if err := client.createBranch(ctx, req.GitHubLogin, communityWallUpstreamRepo, branch, baseRefSHA); err != nil {
		return nil, err
	}
	if err := client.updateFile(ctx, req.GitHubLogin, communityWallUpstreamRepo, communityWallSourcePath, upstreamFileSHA, branch, commitMessage, []byte(updatedContent)); err != nil {
		return nil, err
	}

	pr, err := client.createPullRequest(ctx, communityWallUpstreamOwner, communityWallUpstreamRepo, prTitle, prBody, req.GitHubLogin+":"+branch, communityWallUpstreamBranch)
	if err != nil {
		return nil, err
	}

	result.Mode = "submitted"
	result.PullRequestNumber = pr.Number
	result.PullRequestURL = pr.HTMLURL
	return result, nil
}

func communityWallChangeTitle(appName string) string {
	return fmt.Sprintf("apps wall: add %s", strings.TrimSpace(appName))
}

func communityWallPullRequestBody(input communityWallSubmitInput, candidate communityWallEntry) string {
	var builder strings.Builder
	builder.WriteString("## Summary\n\n")
	builder.WriteString(fmt.Sprintf("- add `%s` to the Wall of Apps\n", strings.TrimSpace(candidate.App)))
	builder.WriteString("\n## Entry\n\n")
	if strings.TrimSpace(input.AppID) != "" {
		builder.WriteString(fmt.Sprintf("- App ID: %s\n", strings.TrimSpace(input.AppID)))
	}
	builder.WriteString(fmt.Sprintf("- App: %s\n", strings.TrimSpace(candidate.App)))
	builder.WriteString(fmt.Sprintf("- Link: %s\n", strings.TrimSpace(candidate.Link)))
	builder.WriteString("\n## Notes\n\n")
	builder.WriteString("- Submitted via `asc apps wall submit`\n")
	return builder.String()
}

func communityWallBranchName(appName string, now time.Time) string {
	slug := communityWallSlug(appName)
	if slug == "" {
		slug = "app"
	}
	return fmt.Sprintf("wall-of-apps/%s-%s", slug, now.UTC().Format("20060102150405"))
}

func communityWallSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func parseCommunityWallSourceEntries(raw []byte, source string) ([]communityWallEntry, error) {
	entries, err := decodeCommunityWallEntries(raw, source)
	if err != nil {
		return nil, err
	}

	normalized := make([]communityWallEntry, 0, len(entries))
	for idx, entry := range entries {
		item, err := normalizeCommunityWallSourceEntry(entry, idx+1)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, item)
	}

	sortCommunityWallSourceEntries(normalized)
	return normalized, nil
}

func normalizeCommunityWallSourceEntry(entry communityWallEntry, index int) (communityWallEntry, error) {
	entry.App = strings.TrimSpace(entry.App)
	entry.Link = strings.TrimSpace(entry.Link)
	entry.Icon = strings.TrimSpace(entry.Icon)

	if entry.App == "" {
		return communityWallEntry{}, fmt.Errorf("entry #%d: 'app' is required", index)
	}
	if entry.Link == "" {
		return communityWallEntry{}, fmt.Errorf("entry #%d: 'link' is required", index)
	}
	if err := validateCommunityWallHTTPURL(entry.Link); err != nil {
		return communityWallEntry{}, fmt.Errorf("entry #%d: 'link' must be a valid http/https URL", index)
	}
	if entry.Icon != "" {
		if err := validateCommunityWallHTTPURL(entry.Icon); err != nil {
			return communityWallEntry{}, fmt.Errorf("entry #%d: 'icon' must be a valid http/https URL", index)
		}
	}

	return entry, nil
}

func validateCommunityWallHTTPURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme")
	}
	return nil
}

func addCommunityWallSourceEntry(entries []communityWallEntry, candidate communityWallEntry) ([]communityWallEntry, error) {
	candidate, err := normalizeCommunityWallSourceEntry(candidate, len(entries)+1)
	if err != nil {
		return nil, err
	}

	candidateAppStoreID := extractCommunityWallAppStoreID(candidate.Link)
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.App), candidate.App) {
			return nil, fmt.Errorf("app %q already exists in %s", candidate.App, communityWallSourcePath)
		}
		if strings.EqualFold(strings.TrimSpace(entry.Link), candidate.Link) {
			return nil, fmt.Errorf("link %q already exists in %s", candidate.Link, communityWallSourcePath)
		}
		if candidateAppStoreID != "" && candidateAppStoreID == extractCommunityWallAppStoreID(entry.Link) {
			return nil, fmt.Errorf("app ID %q already exists in %s", candidateAppStoreID, communityWallSourcePath)
		}
	}

	updated := append(append([]communityWallEntry(nil), entries...), candidate)
	sortCommunityWallSourceEntries(updated)
	return updated, nil
}

func sortCommunityWallSourceEntries(entries []communityWallEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		leftApp := strings.ToLower(strings.TrimSpace(entries[i].App))
		rightApp := strings.ToLower(strings.TrimSpace(entries[j].App))
		if leftApp != rightApp {
			return leftApp < rightApp
		}
		return strings.ToLower(strings.TrimSpace(entries[i].Link)) < strings.ToLower(strings.TrimSpace(entries[j].Link))
	})
}

func renderCommunityWallSourceEntries(entries []communityWallEntry) (string, error) {
	var builder strings.Builder
	builder.WriteString("[\n")
	for index, entry := range entries {
		appValue, err := quoteCommunityWallJSON(entry.App)
		if err != nil {
			return "", err
		}
		linkValue, err := quoteCommunityWallJSON(entry.Link)
		if err != nil {
			return "", err
		}
		builder.WriteString("  {\n")
		fields := []string{
			"    \"app\": " + appValue,
			"    \"link\": " + linkValue,
		}
		if strings.TrimSpace(entry.Icon) != "" {
			iconValue, err := quoteCommunityWallJSON(strings.TrimSpace(entry.Icon))
			if err != nil {
				return "", err
			}
			fields = append(fields, "    \"icon\": "+iconValue)
		}
		builder.WriteString(strings.Join(fields, ",\n"))
		builder.WriteString("\n")
		builder.WriteString("  }")
		if index < len(entries)-1 {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("]\n")
	return builder.String(), nil
}

func quoteCommunityWallJSON(value string) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

func communityWallIconForLink(ctx context.Context, link string) (string, error) {
	appStoreID := extractCommunityWallAppStoreID(link)
	if appStoreID == "" {
		return "", nil
	}

	detailsByID, err := fetchCommunityWallAppDetailsWithOptions(ctx, []string{appStoreID}, itunes.LookupOptions{
		Country:               extractCommunityWallAppStoreCountry(link),
		IncludeSoftwareEntity: true,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(detailsByID[appStoreID].Icon), nil
}

func communityWallAppStoreURL(appStoreID string) string {
	return "https://apps.apple.com/app/id" + canonicalizeCommunityWallAppID(appStoreID)
}

func normalizeCommunityWallAppID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if matches := communityWallAppStoreIDPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return matches[1]
	}
	lowered := strings.ToLower(trimmed)
	if strings.HasPrefix(lowered, "id") {
		return strings.TrimSpace(trimmed[2:])
	}
	return trimmed
}

func canonicalizeCommunityWallAppID(value string) string {
	normalized := normalizeCommunityWallAppID(value)
	if normalized == "" {
		return ""
	}

	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return normalized
	}
	return strconv.FormatInt(parsed, 10)
}

func extractCommunityWallAppStoreID(link string) string {
	matches := communityWallAppStoreIDPattern.FindStringSubmatch(strings.TrimSpace(link))
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func extractCommunityWallAppStoreCountry(link string) string {
	matches := communityWallAppStoreCountryPattern.FindStringSubmatch(strings.TrimSpace(link))
	if len(matches) != 2 {
		return ""
	}

	country, err := itunes.NormalizeCountryCode(matches[1])
	if err != nil {
		return ""
	}
	return country
}

func fetchCommunityWallAppDetails(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
	return fetchCommunityWallAppDetailsWithOptions(ctx, ids, itunes.LookupOptions{
		IncludeSoftwareEntity: true,
	})
}

func fetchCommunityWallAppDetailsWithOptions(ctx context.Context, ids []string, opts itunes.LookupOptions) (map[string]communityWallAppDetails, error) {
	client := itunes.NewClient()
	client.HTTPClient = &http.Client{Timeout: asc.ResolveTimeout()}
	client.BaseURL = communityWallAppStoreLookupURL

	opts.IncludeSoftwareEntity = true
	apps, err := client.LookupApps(ctx, ids, opts)
	if err != nil {
		return nil, fmt.Errorf("app store lookup request failed: %w", err)
	}

	detailsByID := make(map[string]communityWallAppDetails, len(ids))
	for _, id := range ids {
		trimmedID := strings.TrimSpace(id)
		if trimmedID == "" {
			continue
		}
		app, ok := apps[trimmedID]
		if !ok {
			continue
		}
		detailsByID[trimmedID] = communityWallAppDetails{
			Name: app.Name,
			Link: app.URL,
			Icon: app.ArtworkURL,
		}
	}
	return detailsByID, nil
}

func defaultCommunityWallGHCommandRunner(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (client communityWallGitHubClientAPI) currentUserLogin(ctx context.Context) (string, error) {
	body, status, err := client.request(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("failed to fetch GitHub user: %s", strings.TrimSpace(string(body)))
	}

	var user communityWallGitHubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return "", fmt.Errorf("decode GitHub user response: %w", err)
	}
	if strings.TrimSpace(user.Login) == "" {
		return "", fmt.Errorf("GitHub user login was empty")
	}
	return strings.TrimSpace(user.Login), nil
}

func (client communityWallGitHubClientAPI) repo(ctx context.Context, owner, repo string) (*communityWallGitHubRepo, error) {
	body, status, err := client.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return nil, err
	}
	switch status {
	case http.StatusOK:
		var payload communityWallGitHubRepo
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("decode GitHub repo response: %w", err)
		}
		return &payload, nil
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to inspect fork %s/%s: %s", owner, repo, strings.TrimSpace(string(body)))
	}
}

func (client communityWallGitHubClientAPI) createFork(ctx context.Context, owner, repo string) error {
	body, status, err := client.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/forks", owner, repo), map[string]any{})
	if err != nil {
		return err
	}
	if status != http.StatusAccepted && status != http.StatusCreated {
		return fmt.Errorf("failed to create fork: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (client communityWallGitHubClientAPI) waitForRepo(ctx context.Context, owner, repo string) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timed out waiting for fork %s/%s: %w", owner, repo, err)
		}

		repoInfo, err := client.repo(ctx, owner, repo)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("timed out waiting for fork %s/%s: %w", owner, repo, ctxErr)
			}
			return err
		}
		if repoInfo != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for fork %s/%s: %w", owner, repo, ctx.Err())
		default:
			communityWallSleep(time.Second)
		}
	}
}

func (repo *communityWallGitHubRepo) isForkOf(upstreamFullName string) bool {
	if repo == nil || !repo.Fork || repo.Parent == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(repo.Parent.FullName), strings.TrimSpace(upstreamFullName))
}

func (repo *communityWallGitHubRepo) displayName(owner, name string) string {
	if repo == nil {
		return strings.TrimSpace(owner) + "/" + strings.TrimSpace(name)
	}
	if strings.TrimSpace(repo.FullName) != "" {
		return strings.TrimSpace(repo.FullName)
	}
	return strings.TrimSpace(owner) + "/" + strings.TrimSpace(name)
}

func (client communityWallGitHubClientAPI) refSHA(ctx context.Context, owner, repo, ref string) (string, error) {
	body, status, err := client.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/git/ref/%s", owner, repo, ref), nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("failed to fetch Git ref %s for %s/%s: %s", ref, owner, repo, strings.TrimSpace(string(body)))
	}

	var payload communityWallGitHubRef
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode Git ref response: %w", err)
	}
	if strings.TrimSpace(payload.Object.SHA) == "" {
		return "", fmt.Errorf("Git ref %s for %s/%s returned an empty SHA", ref, owner, repo)
	}
	return strings.TrimSpace(payload.Object.SHA), nil
}

func (client communityWallGitHubClientAPI) createBranch(ctx context.Context, owner, repo, branch, sha string) error {
	body, status, err := client.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo), map[string]any{
		"ref": "refs/heads/" + branch,
		"sha": sha,
	})
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("failed to create branch %s in %s/%s: %s", branch, owner, repo, strings.TrimSpace(string(body)))
	}
	return nil
}

func (client communityWallGitHubClientAPI) fileContents(ctx context.Context, owner, repo, path, ref string) ([]byte, string, error) {
	body, status, err := client.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, url.QueryEscape(ref)), nil)
	if err != nil {
		return nil, "", err
	}
	if status != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch %s from %s/%s: %s", path, owner, repo, strings.TrimSpace(string(body)))
	}

	var payload communityWallGitHubContent
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", fmt.Errorf("decode GitHub contents response: %w", err)
	}
	if payload.Encoding != "base64" {
		return nil, "", fmt.Errorf("unsupported GitHub contents encoding %q", payload.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return nil, "", fmt.Errorf("decode GitHub contents payload: %w", err)
	}
	return decoded, strings.TrimSpace(payload.SHA), nil
}

func (client communityWallGitHubClientAPI) updateFile(ctx context.Context, owner, repo, path, sha, branch, message string, content []byte) error {
	body, status, err := client.request(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path), map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"sha":     sha,
		"branch":  branch,
	})
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("failed to update %s in %s/%s: %s", path, owner, repo, strings.TrimSpace(string(body)))
	}
	return nil
}

func (client communityWallGitHubClientAPI) createPullRequest(ctx context.Context, owner, repo, title, bodyText, head, base string) (*communityWallGitHubPullRequest, error) {
	body, status, err := client.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), map[string]any{
		"title": title,
		"body":  bodyText,
		"head":  head,
		"base":  base,
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("failed to create pull request: %s", strings.TrimSpace(string(body)))
	}

	var pullRequest communityWallGitHubPullRequest
	if err := json.Unmarshal(body, &pullRequest); err != nil {
		return nil, fmt.Errorf("decode pull request response: %w", err)
	}
	return &pullRequest, nil
}

func (client communityWallGitHubClientAPI) request(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, communityWallGitHubAPIBase+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+client.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := communityWallGitHubClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := io.LimitReader(resp.Body, communityWallGitHubMaxBodyBytes)
		body, _ := io.ReadAll(limited)
		return body, resp.StatusCode, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}
