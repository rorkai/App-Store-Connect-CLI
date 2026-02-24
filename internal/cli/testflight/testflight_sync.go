package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"gopkg.in/yaml.v3"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestFlightConfig is the YAML export schema for TestFlight config.
type TestFlightConfig struct {
	App     TestFlightAppConfig      `yaml:"app"`
	Groups  []TestFlightGroupConfig  `yaml:"groups"`
	Builds  []TestFlightBuildConfig  `yaml:"builds,omitempty"`
	Testers []TestFlightTesterConfig `yaml:"testers,omitempty"`
}

// TestFlightAppConfig describes the app metadata.
type TestFlightAppConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	BundleID string `yaml:"bundleId"`
}

// TestFlightGroupConfig describes TestFlight beta groups.
type TestFlightGroupConfig struct {
	ID                string                       `yaml:"id"`
	Name              string                       `yaml:"name"`
	IsInternalGroup   bool                         `yaml:"isInternalGroup"`
	PublicLinkEnabled bool                         `yaml:"publicLinkEnabled,omitempty"`
	PublicLinkLimit   *int                         `yaml:"publicLinkLimit,omitempty"`
	FeedbackEnabled   bool                         `yaml:"feedbackEnabled"`
	Builds            []string                     `yaml:"builds,omitempty"`
	BuildDetails      []TestFlightGroupBuildDetail `yaml:"buildDetails,omitempty"`
}

// TestFlightGroupBuildDetail describes build identifiers with human-readable metadata.
type TestFlightGroupBuildDetail struct {
	ID              string `yaml:"id"`
	BuildNumber     string `yaml:"buildNumber"`
	ProcessingState string `yaml:"processingState,omitempty"`
	UploadedDate    string `yaml:"uploadedDate,omitempty"`
}

// TestFlightBuildConfig describes build metadata and group assignments.
type TestFlightBuildConfig struct {
	ID              string   `yaml:"id"`
	Version         string   `yaml:"version"`
	UploadedDate    string   `yaml:"uploadedDate"`
	ProcessingState string   `yaml:"processingState"`
	Groups          []string `yaml:"groups,omitempty"`
}

// TestFlightTesterConfig describes tester metadata and group memberships.
type TestFlightTesterConfig struct {
	ID     string   `yaml:"id"`
	Email  string   `yaml:"email,omitempty"`
	Name   string   `yaml:"name,omitempty"`
	State  string   `yaml:"state"`
	Groups []string `yaml:"groups,omitempty"`
}

type testFlightSyncSummary struct {
	File    string `json:"file"`
	App     string `json:"app"`
	Groups  int    `json:"groups"`
	Builds  int    `json:"builds"`
	Testers int    `json:"testers"`
}

type testFlightPullOptions struct {
	includeBuilds  bool
	includeTesters bool
	groupFilter    string
	buildFilters   []string
	testerFilters  []string
}

type testFlightSyncClient interface {
	GetApp(ctx context.Context, appID string) (*asc.AppResponse, error)
	GetBetaGroups(ctx context.Context, appID string, opts ...asc.BetaGroupsOption) (*asc.BetaGroupsResponse, error)
	GetBetaGroupBuilds(ctx context.Context, groupID string, opts ...asc.BetaGroupBuildsOption) (*asc.BuildsResponse, error)
	GetBetaGroupTesters(ctx context.Context, groupID string, opts ...asc.BetaGroupTestersOption) (*asc.BetaTestersResponse, error)
}

// TestFlightSyncCommand returns the testflight sync command group.
func TestFlightSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc testflight sync <subcommand> [flags]",
		ShortHelp:  "Sync TestFlight configuration.",
		LongHelp: `Sync TestFlight configuration.

Examples:
  asc testflight sync pull --app "APP_ID" --output "./testflight.yaml"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightSyncPullCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// TestFlightSyncPullCommand exports TestFlight config to YAML.
func TestFlightSyncPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID env)")
	output := fs.String("output", "", "Output file path for YAML (required)")
	includeBuilds := fs.Bool("include-builds", false, "Include builds and group assignments")
	includeTesters := fs.Bool("include-testers", false, "Include testers and group memberships")
	groupFilter := fs.String("group", "", "Filter to a specific beta group (name or ID)")
	buildFilter := fs.String("build", "", "Filter to build ID(s), comma-separated")
	testerFilter := fs.String("tester", "", "Filter to tester ID(s) or emails, comma-separated")
	pretty := shared.BindPrettyJSONFlag(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc testflight sync pull [flags]",
		ShortHelp:  "Export TestFlight configuration to YAML.",
		LongHelp: `Export TestFlight configuration to YAML.

Examples:
  asc testflight sync pull --app "APP_ID" --output "./testflight.yaml"
  asc testflight sync pull --app "APP_ID" --output "./testflight.yaml" --include-builds
  asc testflight sync pull --app "APP_ID" --output "./testflight.yaml" --include-builds --include-testers`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			outputValue := strings.TrimSpace(*output)
			if outputValue == "" {
				fmt.Fprintf(os.Stderr, "Error: --output is required\n\n")
				return flag.ErrHelp
			}

			buildFilters := shared.SplitCSV(*buildFilter)
			testerFilters := shared.SplitCSV(*testerFilter)
			if len(buildFilters) > 0 && !*includeBuilds {
				fmt.Fprintf(os.Stderr, "Error: --build requires --include-builds\n\n")
				return flag.ErrHelp
			}
			if len(testerFilters) > 0 && !*includeTesters {
				fmt.Fprintf(os.Stderr, "Error: --tester requires --include-testers\n\n")
				return flag.ErrHelp
			}

			resolvedOutputPath, err := resolveTestFlightOutputPath(outputValue)
			if err != nil {
				return fmt.Errorf("testflight sync pull: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight sync pull: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("testflight sync pull: %w", err)
			}

			options := testFlightPullOptions{
				includeBuilds:  *includeBuilds,
				includeTesters: *includeTesters,
				groupFilter:    strings.TrimSpace(*groupFilter),
				buildFilters:   buildFilters,
				testerFilters:  testerFilters,
			}

			config, err := pullTestFlightConfig(requestCtx, client, resolvedAppID, options)
			if err != nil {
				return fmt.Errorf("testflight sync pull: %w", err)
			}

			if err := writeTestFlightConfigYAML(resolvedOutputPath, config); err != nil {
				return fmt.Errorf("testflight sync pull: %w", err)
			}

			summary := testFlightSyncSummary{
				File:    filepath.Clean(outputValue),
				App:     config.App.Name,
				Groups:  len(config.Groups),
				Builds:  len(config.Builds),
				Testers: len(config.Testers),
			}

			return shared.PrintOutput(summary, "json", *pretty)
		},
	}
}

func pullTestFlightConfig(ctx context.Context, client testFlightSyncClient, appID string, opts testFlightPullOptions) (*TestFlightConfig, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	appResp, err := client.GetApp(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("fetch app: %w", err)
	}
	appConfig := TestFlightAppConfig{
		ID:       appResp.Data.ID,
		Name:     appResp.Data.Attributes.Name,
		BundleID: appResp.Data.Attributes.BundleID,
	}

	groupFirstPage, err := client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsLimit(200))
	if err != nil {
		return nil, fmt.Errorf("fetch beta groups: %w", err)
	}
	groupResp, err := paginateBetaGroups(ctx, client, appID, groupFirstPage)
	if err != nil {
		return nil, fmt.Errorf("fetch beta groups: %w", err)
	}
	filteredGroups, err := filterBetaGroups(groupResp.Data, opts.groupFilter)
	if err != nil {
		return nil, err
	}

	buildConfigs := make(map[string]*TestFlightBuildConfig)
	groupBuilds := make(map[string][]string)
	if opts.includeBuilds {
		for _, group := range filteredGroups {
			buildFirstPage, err := client.GetBetaGroupBuilds(ctx, group.ID, asc.WithBetaGroupBuildsLimit(200))
			if err != nil {
				return nil, fmt.Errorf("fetch beta group builds: %w", err)
			}
			buildResp, err := paginateBetaGroupBuilds(ctx, client, group.ID, buildFirstPage)
			if err != nil {
				return nil, fmt.Errorf("fetch beta group builds: %w", err)
			}
			for _, build := range buildResp.Data {
				groupBuilds[group.ID] = append(groupBuilds[group.ID], build.ID)
				cfg := buildConfigs[build.ID]
				if cfg == nil {
					cfg = &TestFlightBuildConfig{
						ID:              build.ID,
						Version:         build.Attributes.Version,
						UploadedDate:    build.Attributes.UploadedDate,
						ProcessingState: build.Attributes.ProcessingState,
					}
					buildConfigs[build.ID] = cfg
				}
				cfg.Groups = append(cfg.Groups, group.ID)
			}
		}

		if err := applyBuildFilter(buildConfigs, groupBuilds, opts.buildFilters); err != nil {
			return nil, err
		}
	}

	testerConfigs := make(map[string]*TestFlightTesterConfig)
	if opts.includeTesters {
		for _, group := range filteredGroups {
			testerFirstPage, err := client.GetBetaGroupTesters(ctx, group.ID, asc.WithBetaGroupTestersLimit(200))
			if err != nil {
				return nil, fmt.Errorf("fetch beta group testers: %w", err)
			}
			testerResp, err := paginateBetaGroupTesters(ctx, client, group.ID, testerFirstPage)
			if err != nil {
				return nil, fmt.Errorf("fetch beta group testers: %w", err)
			}
			for _, tester := range testerResp.Data {
				cfg := testerConfigs[tester.ID]
				if cfg == nil {
					cfg = &TestFlightTesterConfig{
						ID:    tester.ID,
						Email: tester.Attributes.Email,
						Name:  formatTesterName(tester.Attributes.FirstName, tester.Attributes.LastName),
						State: string(tester.Attributes.State),
					}
					testerConfigs[tester.ID] = cfg
				}
				cfg.Groups = append(cfg.Groups, group.ID)
			}
		}

		if err := applyTesterFilter(testerConfigs, opts.testerFilters); err != nil {
			return nil, err
		}
	}

	groupConfigs := make([]TestFlightGroupConfig, 0, len(filteredGroups))
	for _, group := range filteredGroups {
		attrs := group.Attributes
		cfg := TestFlightGroupConfig{
			ID:                group.ID,
			Name:              attrs.Name,
			IsInternalGroup:   attrs.IsInternalGroup,
			PublicLinkEnabled: attrs.PublicLinkEnabled,
			FeedbackEnabled:   attrs.FeedbackEnabled,
		}
		if attrs.PublicLinkLimitEnabled && attrs.PublicLinkLimit > 0 {
			limit := attrs.PublicLinkLimit
			cfg.PublicLinkLimit = &limit
		}
		if opts.includeBuilds {
			cfg.Builds = uniqueSortedStrings(groupBuilds[group.ID])
			cfg.BuildDetails = buildDetailsForIDs(cfg.Builds, buildConfigs)
		}
		groupConfigs = append(groupConfigs, cfg)
	}
	sort.Slice(groupConfigs, func(i, j int) bool {
		if groupConfigs[i].Name == groupConfigs[j].Name {
			return groupConfigs[i].ID < groupConfigs[j].ID
		}
		return groupConfigs[i].Name < groupConfigs[j].Name
	})

	config := &TestFlightConfig{
		App:    appConfig,
		Groups: groupConfigs,
	}

	if opts.includeBuilds {
		config.Builds = buildConfigsFromMap(buildConfigs)
	}
	if opts.includeTesters {
		config.Testers = testerConfigsFromMap(testerConfigs)
	}

	return config, nil
}

func paginateBetaGroups(ctx context.Context, client testFlightSyncClient, appID string, firstPage *asc.BetaGroupsResponse) (*asc.BetaGroupsResponse, error) {
	if firstPage == nil {
		return &asc.BetaGroupsResponse{}, nil
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	if allPages == nil {
		return &asc.BetaGroupsResponse{}, nil
	}
	resp, ok := allPages.(*asc.BetaGroupsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta groups response type")
	}
	return resp, nil
}

func paginateBetaGroupBuilds(ctx context.Context, client testFlightSyncClient, groupID string, firstPage *asc.BuildsResponse) (*asc.BuildsResponse, error) {
	if firstPage == nil {
		return &asc.BuildsResponse{}, nil
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaGroupBuilds(ctx, groupID, asc.WithBetaGroupBuildsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	if allPages == nil {
		return &asc.BuildsResponse{}, nil
	}
	resp, ok := allPages.(*asc.BuildsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta group builds response type")
	}
	return resp, nil
}

func paginateBetaGroupTesters(ctx context.Context, client testFlightSyncClient, groupID string, firstPage *asc.BetaTestersResponse) (*asc.BetaTestersResponse, error) {
	if firstPage == nil {
		return &asc.BetaTestersResponse{}, nil
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaGroupTesters(ctx, groupID, asc.WithBetaGroupTestersNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	if allPages == nil {
		return &asc.BetaTestersResponse{}, nil
	}
	resp, ok := allPages.(*asc.BetaTestersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta group testers response type")
	}
	return resp, nil
}

func filterBetaGroups(groups []asc.Resource[asc.BetaGroupAttributes], filter string) ([]asc.Resource[asc.BetaGroupAttributes], error) {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return groups, nil
	}

	for _, group := range groups {
		if group.ID == trimmed {
			return []asc.Resource[asc.BetaGroupAttributes]{group}, nil
		}
	}

	matches := make([]asc.Resource[asc.BetaGroupAttributes], 0, 1)
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.Attributes.Name), trimmed) {
			matches = append(matches, group)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("beta group %q not found", trimmed)
	case 1:
		return matches, nil
	default:
		return nil, fmt.Errorf("multiple beta groups named %q; use group ID", trimmed)
	}
}

func applyBuildFilter(builds map[string]*TestFlightBuildConfig, groupBuilds map[string][]string, filters []string) error {
	filters = normalizeFilters(filters)
	if len(filters) == 0 {
		return nil
	}

	filterSet := make(map[string]struct{}, len(filters))
	for _, value := range filters {
		filterSet[value] = struct{}{}
	}

	missing := make([]string, 0)
	for value := range filterSet {
		if _, ok := builds[value]; !ok {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("build filter not found: %s", strings.Join(missing, ", "))
	}

	for id := range builds {
		if _, ok := filterSet[id]; !ok {
			delete(builds, id)
		}
	}

	for groupID, ids := range groupBuilds {
		groupBuilds[groupID] = filterStringsBySet(ids, filterSet)
	}

	return nil
}

func applyTesterFilter(testers map[string]*TestFlightTesterConfig, filters []string) error {
	filters = normalizeFilters(filters)
	if len(filters) == 0 {
		return nil
	}

	filterSet := make(map[string]struct{}, len(filters))
	for _, value := range filters {
		filterSet[strings.ToLower(value)] = struct{}{}
	}

	matched := make(map[string]struct{})
	for _, tester := range testers {
		if testerMatchesFilter(tester, filterSet) {
			matched[strings.ToLower(tester.ID)] = struct{}{}
			if tester.Email != "" {
				matched[strings.ToLower(tester.Email)] = struct{}{}
			}
		}
	}

	for id, tester := range testers {
		if testerMatchesFilter(tester, filterSet) {
			continue
		}
		delete(testers, id)
	}

	missing := make([]string, 0)
	for value := range filterSet {
		if _, ok := matched[value]; !ok {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("tester filter not found: %s", strings.Join(missing, ", "))
	}

	return nil
}

func testerMatchesFilter(tester *TestFlightTesterConfig, filterSet map[string]struct{}) bool {
	if tester == nil {
		return false
	}
	if _, ok := filterSet[strings.ToLower(tester.ID)]; ok {
		return true
	}
	if tester.Email != "" {
		if _, ok := filterSet[strings.ToLower(tester.Email)]; ok {
			return true
		}
	}
	return false
}

func buildConfigsFromMap(builds map[string]*TestFlightBuildConfig) []TestFlightBuildConfig {
	if len(builds) == 0 {
		return nil
	}
	ids := make([]string, 0, len(builds))
	for id := range builds {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]TestFlightBuildConfig, 0, len(ids))
	for _, id := range ids {
		cfg := builds[id]
		cfg.Groups = uniqueSortedStrings(cfg.Groups)
		result = append(result, *cfg)
	}
	return result
}

func buildDetailsForIDs(buildIDs []string, builds map[string]*TestFlightBuildConfig) []TestFlightGroupBuildDetail {
	if len(buildIDs) == 0 || len(builds) == 0 {
		return nil
	}
	details := make([]TestFlightGroupBuildDetail, 0, len(buildIDs))
	for _, buildID := range buildIDs {
		cfg, ok := builds[buildID]
		if !ok || cfg == nil {
			details = append(details, TestFlightGroupBuildDetail{ID: buildID})
			continue
		}
		details = append(details, TestFlightGroupBuildDetail{
			ID:              buildID,
			BuildNumber:     cfg.Version,
			ProcessingState: cfg.ProcessingState,
			UploadedDate:    cfg.UploadedDate,
		})
	}
	return details
}

func testerConfigsFromMap(testers map[string]*TestFlightTesterConfig) []TestFlightTesterConfig {
	if len(testers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(testers))
	for id := range testers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]TestFlightTesterConfig, 0, len(ids))
	for _, id := range ids {
		cfg := testers[id]
		cfg.Groups = uniqueSortedStrings(cfg.Groups)
		result = append(result, *cfg)
	}
	return result
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{})
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func filterStringsBySet(values []string, allowed map[string]struct{}) []string {
	if len(values) == 0 || len(allowed) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; ok {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func normalizeFilters(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func formatTesterName(firstName, lastName string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	switch {
	case first != "" && last != "":
		return first + " " + last
	case first != "":
		return first
	case last != "":
		return last
	default:
		return ""
	}
}

func resolveTestFlightOutputPath(outputPath string) (string, error) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return "", fmt.Errorf("output path is required")
	}
	if strings.HasSuffix(trimmed, string(filepath.Separator)) {
		return "", fmt.Errorf("output path must be a file")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}

	if !filepath.IsAbs(trimmed) {
		resolved := filepath.Clean(filepath.Join(cwd, trimmed))
		rel, err := filepath.Rel(cwd, resolved)
		if err != nil {
			return "", fmt.Errorf("resolve output path: %w", err)
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return "", fmt.Errorf("output path must be within the current directory")
		}
		return resolved, nil
	}

	return filepath.Clean(trimmed), nil
}

func marshalTestFlightConfigYAML(config *TestFlightConfig) ([]byte, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	return yaml.Marshal(config)
}

func writeTestFlightConfigYAML(outputPath string, config *TestFlightConfig) error {
	if config == nil {
		return fmt.Errorf("config is required")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	data, err := marshalTestFlightConfigYAML(config)
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), ".testflight-*.yaml")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()
	committed := false
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
		if !committed {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		return err
	}
	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	tempFile = nil
	if err := os.Rename(tempName, outputPath); err != nil {
		if info, statErr := os.Stat(outputPath); statErr == nil && info.IsDir() {
			return fmt.Errorf("output path is a directory")
		}
		return err
	}
	committed = true

	return nil
}
