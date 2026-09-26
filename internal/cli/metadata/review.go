package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	defaultMetadataReviewDir  = ".asc/metadata/review"
	metadataPlanFileName      = "plan.json"
	metadataApprovalFileName  = "approved.json"
	metadataReviewSchemaV1    = 1
	metadataApprovalAllOption = "all"
)

type MetadataPlanArtifact struct {
	SchemaVersion int                 `json:"schemaVersion"`
	GeneratedAt   string              `json:"generatedAt"`
	Command       string              `json:"command"`
	ReviewDir     string              `json:"reviewDir"`
	PlanPath      string              `json:"planPath"`
	ApprovalPath  string              `json:"approvalPath"`
	PlanHash      string              `json:"planHash"`
	Options       metadataPlanOptions `json:"options"`
	Plan          PushPlanResult      `json:"plan"`
}

type MetadataApprovalArtifact struct {
	SchemaVersion int      `json:"schemaVersion"`
	ApprovedAt    string   `json:"approvedAt"`
	PlanHash      string   `json:"planHash"`
	ReviewDir     string   `json:"reviewDir"`
	PlanPath      string   `json:"planPath"`
	ApprovalPath  string   `json:"approvalPath"`
	Mode          string   `json:"mode"`
	Note          string   `json:"note,omitempty"`
	ApprovedKeys  []string `json:"approvedKeys"`
	PendingKeys   []string `json:"pendingKeys,omitempty"`
}

type MetadataReviewStatus struct {
	ReviewDir           string   `json:"reviewDir"`
	PlanPath            string   `json:"planPath"`
	ApprovalPath        string   `json:"approvalPath"`
	PlanHash            string   `json:"planHash"`
	ApprovalPlanHash    string   `json:"approvalPlanHash,omitempty"`
	ApprovalMatchesPlan bool     `json:"approvalMatchesPlan"`
	Ready               bool     `json:"ready"`
	TotalCount          int      `json:"totalCount"`
	ApprovedCount       int      `json:"approvedCount"`
	PendingCount        int      `json:"pendingCount"`
	ApprovedKeys        []string `json:"approvedKeys,omitempty"`
	PendingKeys         []string `json:"pendingKeys,omitempty"`
}

type metadataPlanOptions struct {
	AppID     string `json:"appId"`
	AppInfoID string `json:"appInfoId,omitempty"`
	Version   string `json:"version"`
	Platform  string `json:"platform,omitempty"`
	Dir       string `json:"dir"`
	// IfExists binds the reviewed conflict policy to the plan hash, so an
	// approved plan cannot be applied with a policy the reviewer never saw.
	// The default, fail, is rendered as the empty string and omitted, which
	// keeps plan artifacts written before --if-exists existed hash-identical.
	IfExists     string   `json:"ifExists,omitempty"`
	Includes     []string `json:"includes"`
	AllowDeletes bool     `json:"allowDeletes"`
}

type metadataPlanHashPayload struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Options       metadataPlanOptions `json:"options"`
	Plan          PushPlanResult      `json:"plan"`
}

// MetadataPlanCommand returns the metadata plan subcommand.
func MetadataPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata plan", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	include := fs.String("include", includeLocalizations, "Included metadata scopes (comma-separated)")
	allowDeletes := fs.Bool("allow-deletes", false, "Plan destructive delete operations (disables default locale fallback for missing locales)")
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	reviewDir := fs.String("review-dir", defaultMetadataReviewDir, "Directory for metadata review artifacts")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: `asc metadata plan --app "APP_ID" --version "1.2.3" --dir "./metadata" [--review-dir ".asc/metadata/review"]`,
		ShortHelp:  "Plan metadata changes and write a review artifact.",
		LongHelp: `Plan metadata changes from canonical files and write a review artifact.

Examples:
  asc metadata plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata plan --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --review-dir ".asc/metadata/review"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata plan does not accept positional arguments")
			}
			// See the note in push.go: the bound flag defaults to fail, so an
			// empty value is an explicitly supplied unsupported value.
			if _, err := shared.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate); err != nil {
				return err
			}
			artifact, warnings, err := ExecuteMetadataPlanWithWarnings(ctx, PushExecutionOptions{
				CommandName:  "plan",
				AppID:        *appID,
				AppInfoID:    *appInfoID,
				Version:      *version,
				Platform:     *platform,
				Dir:          *dir,
				Include:      *include,
				DryRun:       true,
				AllowDeletes: *allowDeletes,
				IfExists:     *ifExists,
			}, *reviewDir)
			if err != nil {
				return err
			}
			if err := shared.PrintOutputWithRenderers(
				artifact,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataPlanArtifactTable(artifact, false) },
				func() error { return printMetadataPlanArtifactTable(artifact, true) },
			); err != nil {
				return err
			}
			return shared.PrintSubmitReadinessCreateWarnings(os.Stderr, warnings)
		},
	}
}

// MetadataApproveCommand returns the metadata approve subcommand.
func MetadataApproveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata approve", flag.ExitOnError)

	reviewDir := fs.String("review-dir", defaultMetadataReviewDir, "Directory containing metadata review artifacts")
	all := fs.Bool("all", false, "Approve every planned metadata change")
	key := fs.String("key", "", "Approve specific plan key(s), comma-separated")
	scope := fs.String("scope", "", "Approve all changes in scope(s): app-info, version")
	note := fs.String("note", "", "Optional reviewer note written to approved.json")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "approve",
		ShortUsage: `asc metadata approve [--review-dir ".asc/metadata/review"] (--all | --key "KEY" | --scope app-info,version)`,
		ShortHelp:  "Approve a metadata review plan.",
		LongHelp: `Approve a metadata review plan by writing approved.json.

Examples:
  asc metadata approve --all
  asc metadata approve --review-dir ".asc/metadata/review" --scope app-info
  asc metadata approve --review-dir ".asc/metadata/review" --key "app-info:en-US:subtitle"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if len(args) > 0 {
				return shared.UsageError("metadata approve does not accept positional arguments")
			}
			approval, err := ExecuteMetadataApprove(MetadataApproveOptions{
				ReviewDir: *reviewDir,
				All:       *all,
				Key:       *key,
				Scope:     *scope,
				Note:      *note,
			})
			if err != nil {
				return err
			}
			return shared.PrintOutputWithRenderers(
				approval,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataApprovalTable(approval, false) },
				func() error { return printMetadataApprovalTable(approval, true) },
			)
		},
	}
}

// MetadataStatusCommand returns the metadata status subcommand.
func MetadataStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata status", flag.ExitOnError)

	reviewDir := fs.String("review-dir", defaultMetadataReviewDir, "Directory containing metadata review artifacts")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: `asc metadata status [--review-dir ".asc/metadata/review"]`,
		ShortHelp:  "Show local metadata review approval status.",
		LongHelp: `Show local metadata review approval status.

Examples:
  asc metadata status
  asc metadata status --review-dir ".asc/metadata/review" --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if len(args) > 0 {
				return shared.UsageError("metadata status does not accept positional arguments")
			}
			status, err := ExecuteMetadataReviewStatus(*reviewDir)
			if err != nil {
				return err
			}
			return shared.PrintOutputWithRenderers(
				status,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataReviewStatusTable(status, false) },
				func() error { return printMetadataReviewStatusTable(status, true) },
			)
		},
	}
}

type MetadataApproveOptions struct {
	ReviewDir string
	All       bool
	Key       string
	Scope     string
	Note      string
}

func ExecuteMetadataPlanWithWarnings(ctx context.Context, opts PushExecutionOptions, reviewDir string) (MetadataPlanArtifact, []shared.SubmitReadinessCreateWarning, error) {
	opts.DryRun = true
	result, warnings, err := ExecutePushWithWarnings(ctx, opts)
	if err != nil {
		return MetadataPlanArtifact{}, warnings, err
	}

	resolvedReviewDir := metadataReviewDir(reviewDir)
	planPath := filepath.Join(resolvedReviewDir, metadataPlanFileName)
	approvalPath := filepath.Join(resolvedReviewDir, metadataApprovalFileName)
	options := metadataPlanOptionsFromPush(opts, result)
	planHash, err := hashMetadataPlan(options, result)
	if err != nil {
		return MetadataPlanArtifact{}, warnings, fmt.Errorf("metadata plan: hash plan: %w", err)
	}

	artifact := MetadataPlanArtifact{
		SchemaVersion: metadataReviewSchemaV1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Command:       "metadata plan",
		ReviewDir:     resolvedReviewDir,
		PlanPath:      planPath,
		ApprovalPath:  approvalPath,
		PlanHash:      planHash,
		Options:       options,
		Plan:          result,
	}
	if err := writeMetadataReviewJSON(planPath, artifact); err != nil {
		return MetadataPlanArtifact{}, warnings, fmt.Errorf("metadata plan: write %s: %w", planPath, err)
	}
	return artifact, warnings, nil
}

func ExecuteMetadataApprove(opts MetadataApproveOptions) (MetadataApprovalArtifact, error) {
	selectedModes := 0
	if opts.All {
		selectedModes++
	}
	if strings.TrimSpace(opts.Key) != "" {
		selectedModes++
	}
	if strings.TrimSpace(opts.Scope) != "" {
		selectedModes++
	}
	if selectedModes == 0 {
		return MetadataApprovalArtifact{}, shared.UsageError("--all, --key, or --scope is required")
	}
	if selectedModes > 1 {
		return MetadataApprovalArtifact{}, shared.UsageError("--all, --key, and --scope cannot be combined")
	}

	reviewDir := metadataReviewDir(opts.ReviewDir)
	plan, err := readMetadataPlanArtifact(filepath.Join(reviewDir, metadataPlanFileName))
	if err != nil {
		return MetadataApprovalArtifact{}, err
	}
	allItems := metadataPlanItems(plan.Plan)
	allKeys := metadataPlanKeys(allItems)
	approvedKeys, mode, err := selectApprovedMetadataKeys(allItems, opts)
	if err != nil {
		return MetadataApprovalArtifact{}, err
	}
	planPath := filepath.Join(reviewDir, metadataPlanFileName)
	approvalPath := filepath.Join(reviewDir, metadataApprovalFileName)
	existingApproval, err := readMetadataApprovalArtifact(approvalPath)
	if err != nil && !os.IsNotExist(err) {
		return MetadataApprovalArtifact{}, err
	}
	if err == nil && existingApproval.PlanHash == plan.PlanHash {
		approvedKeys = mergeMetadataApprovalKeys(allKeys, existingApproval.ApprovedKeys, approvedKeys)
	}
	pendingKeys := diffMetadataKeys(allKeys, approvedKeys)
	approval := MetadataApprovalArtifact{
		SchemaVersion: metadataReviewSchemaV1,
		ApprovedAt:    time.Now().UTC().Format(time.RFC3339),
		PlanHash:      plan.PlanHash,
		ReviewDir:     reviewDir,
		PlanPath:      planPath,
		ApprovalPath:  approvalPath,
		Mode:          mode,
		Note:          strings.TrimSpace(opts.Note),
		ApprovedKeys:  approvedKeys,
		PendingKeys:   pendingKeys,
	}
	if err := writeMetadataReviewJSON(approvalPath, approval); err != nil {
		return MetadataApprovalArtifact{}, fmt.Errorf("metadata approve: write %s: %w", approvalPath, err)
	}
	return approval, nil
}

func ExecuteMetadataReviewStatus(reviewDir string) (MetadataReviewStatus, error) {
	resolvedReviewDir := metadataReviewDir(reviewDir)
	planPath := filepath.Join(resolvedReviewDir, metadataPlanFileName)
	approvalPath := filepath.Join(resolvedReviewDir, metadataApprovalFileName)
	plan, err := readMetadataPlanArtifact(planPath)
	if err != nil {
		return MetadataReviewStatus{}, err
	}
	allKeys := metadataPlanKeys(metadataPlanItems(plan.Plan))

	approval, err := readMetadataApprovalArtifact(approvalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return MetadataReviewStatus{
				ReviewDir:           resolvedReviewDir,
				PlanPath:            planPath,
				ApprovalPath:        approvalPath,
				PlanHash:            plan.PlanHash,
				ApprovalMatchesPlan: false,
				Ready:               len(allKeys) == 0,
				TotalCount:          len(allKeys),
				ApprovedCount:       0,
				PendingCount:        len(allKeys),
				PendingKeys:         allKeys,
			}, nil
		}
		return MetadataReviewStatus{}, err
	}

	approvedKeys := intersectMetadataKeys(allKeys, approval.ApprovedKeys)
	pendingKeys := diffMetadataKeys(allKeys, approvedKeys)
	matches := approval.PlanHash == plan.PlanHash
	return MetadataReviewStatus{
		ReviewDir:           resolvedReviewDir,
		PlanPath:            planPath,
		ApprovalPath:        approvalPath,
		PlanHash:            plan.PlanHash,
		ApprovalPlanHash:    approval.PlanHash,
		ApprovalMatchesPlan: matches,
		Ready:               len(allKeys) == 0 || (matches && len(pendingKeys) == 0),
		TotalCount:          len(allKeys),
		ApprovedCount:       len(approvedKeys),
		PendingCount:        len(pendingKeys),
		ApprovedKeys:        approvedKeys,
		PendingKeys:         pendingKeys,
	}, nil
}

func VerifyApprovedMetadataPlan(opts PushExecutionOptions, currentPlan PushPlanResult, reviewDir string) error {
	resolvedReviewDir := metadataReviewDir(reviewDir)
	plan, err := readMetadataPlanArtifact(filepath.Join(resolvedReviewDir, metadataPlanFileName))
	if err != nil {
		return err
	}
	hashPlan := currentPlan
	hashPlan.DryRun = true
	currentOptions := metadataPlanOptionsFromPush(opts, hashPlan)
	currentHash, err := hashMetadataPlan(currentOptions, hashPlan)
	if err != nil {
		return fmt.Errorf("metadata apply: hash current plan: %w", err)
	}
	planKeys := metadataPlanKeys(metadataPlanItems(plan.Plan))
	currentKeys := metadataPlanKeys(metadataPlanItems(currentPlan))
	if currentHash != plan.PlanHash {
		return shared.UsageError("approved metadata plan drifted; rerun asc metadata plan and asc metadata approve")
	}
	if len(planKeys) == 0 && len(currentKeys) == 0 {
		return nil
	}
	approval, err := readMetadataApprovalArtifact(filepath.Join(resolvedReviewDir, metadataApprovalFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return shared.UsageError("approved metadata plan is missing; run asc metadata approve first")
		}
		return err
	}
	if approval.PlanHash != plan.PlanHash {
		return shared.UsageError("approved metadata plan hash does not match plan.json; rerun asc metadata approve")
	}

	pendingKeys := diffMetadataKeys(currentKeys, approval.ApprovedKeys)
	if len(pendingKeys) > 0 {
		return shared.UsageErrorf("approved metadata plan is incomplete; pending key(s): %s", strings.Join(pendingKeys, ", "))
	}
	return nil
}

func ExecuteApprovedMetadataApplyWithWarnings(ctx context.Context, opts PushExecutionOptions, reviewDir string) (PushPlanResult, []shared.SubmitReadinessCreateWarning, error) {
	opts.ReviewDir = reviewDir
	return ExecutePushWithWarnings(ctx, opts)
}

func metadataReviewDir(reviewDir string) string {
	trimmed := strings.TrimSpace(reviewDir)
	if trimmed == "" {
		return defaultMetadataReviewDir
	}
	return trimmed
}

// normalizedIfExistsPlanValue renders the --if-exists mode for the review plan
// hash. fail, the default, renders as the empty string so a plan recorded
// before this flag existed keeps its hash; skip and update each change it, so
// switching the conflict policy invalidates the approval.
func normalizedIfExistsPlanValue(raw string) string {
	mode, err := shared.ParseOptionalIfExistsMode(raw, shared.IfExistsSkip, shared.IfExistsUpdate)
	if err != nil {
		// Unreachable in practice: an unsupported value is a usage error
		// before any plan is hashed. Keep the raw spelling so it can never
		// collide with a supported mode's hash.
		return strings.ToLower(strings.TrimSpace(raw))
	}
	if mode == shared.IfExistsFail {
		return ""
	}
	return string(mode)
}

func metadataPlanOptionsFromPush(opts PushExecutionOptions, result PushPlanResult) metadataPlanOptions {
	platform := strings.TrimSpace(opts.Platform)
	if platform != "" {
		if normalized, err := shared.NormalizeAppStoreVersionPlatform(platform); err == nil {
			platform = normalized
		}
	}
	return metadataPlanOptions{
		IfExists:     normalizedIfExistsPlanValue(opts.IfExists),
		AppID:        result.AppID,
		AppInfoID:    result.AppInfoID,
		Version:      result.Version,
		Platform:     platform,
		Dir:          result.Dir,
		Includes:     append([]string(nil), result.Includes...),
		AllowDeletes: opts.AllowDeletes,
	}
}

func hashMetadataPlan(options metadataPlanOptions, plan PushPlanResult) (string, error) {
	payload := metadataPlanHashPayload{
		SchemaVersion: metadataReviewSchemaV1,
		Options:       options,
		Plan:          plan,
	}
	data, err := encodeCanonicalJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeMetadataReviewJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileNoFollow(path, data)
}

func readMetadataPlanArtifact(path string) (MetadataPlanArtifact, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return MetadataPlanArtifact{}, shared.UsageErrorf("metadata plan artifact not found at %s; run asc metadata plan first", path)
		}
		return MetadataPlanArtifact{}, fmt.Errorf("metadata plan: read %s: %w", path, err)
	}
	var artifact MetadataPlanArtifact
	if err := decodeStrictJSON(data, &artifact); err != nil {
		return MetadataPlanArtifact{}, fmt.Errorf("metadata plan: parse %s: %w", path, err)
	}
	if artifact.SchemaVersion != metadataReviewSchemaV1 {
		return MetadataPlanArtifact{}, shared.UsageErrorf("unsupported metadata plan schema version %d", artifact.SchemaVersion)
	}
	if strings.TrimSpace(artifact.PlanHash) == "" {
		return MetadataPlanArtifact{}, shared.UsageError("metadata plan artifact is missing planHash")
	}
	actualHash, err := hashMetadataPlan(artifact.Options, artifact.Plan)
	if err != nil {
		return MetadataPlanArtifact{}, fmt.Errorf("metadata plan: hash %s: %w", path, err)
	}
	if actualHash != artifact.PlanHash {
		return MetadataPlanArtifact{}, shared.UsageError("metadata plan artifact planHash does not match its contents; rerun asc metadata plan")
	}
	return artifact, nil
}

func readMetadataApprovalArtifact(path string) (MetadataApprovalArtifact, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return MetadataApprovalArtifact{}, err
		}
		return MetadataApprovalArtifact{}, fmt.Errorf("metadata approve: read %s: %w", path, err)
	}
	var artifact MetadataApprovalArtifact
	if err := decodeStrictJSON(data, &artifact); err != nil {
		return MetadataApprovalArtifact{}, fmt.Errorf("metadata approve: parse %s: %w", path, err)
	}
	if artifact.SchemaVersion != metadataReviewSchemaV1 {
		return MetadataApprovalArtifact{}, shared.UsageErrorf("unsupported metadata approval schema version %d", artifact.SchemaVersion)
	}
	return artifact, nil
}

func metadataPlanItems(plan PushPlanResult) []PlanItem {
	items := make([]PlanItem, 0, len(plan.Adds)+len(plan.Updates)+len(plan.Deletes))
	items = append(items, plan.Adds...)
	items = append(items, plan.Updates...)
	items = append(items, plan.Deletes...)
	sortPlanItems(items)
	return items
}

func metadataPlanKeys(items []PlanItem) []string {
	keys := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func selectApprovedMetadataKeys(items []PlanItem, opts MetadataApproveOptions) ([]string, string, error) {
	allKeys := metadataPlanKeys(items)
	if opts.All {
		return allKeys, metadataApprovalAllOption, nil
	}

	itemByKey := make(map[string]PlanItem, len(items))
	for _, item := range items {
		itemByKey[item.Key] = item
	}
	if strings.TrimSpace(opts.Key) != "" {
		requested := splitReviewCSV(opts.Key)
		for _, key := range requested {
			if _, ok := itemByKey[key]; !ok {
				return nil, "", shared.UsageErrorf("metadata approve key %q was not found in plan", key)
			}
		}
		sort.Strings(requested)
		return requested, "key", nil
	}

	scopes := splitReviewCSV(opts.Scope)
	if len(scopes) == 0 {
		return nil, "", shared.UsageError("--scope must include at least one scope")
	}
	allowedScopes := map[string]struct{}{
		appInfoDirName: {},
		versionDirName: {},
	}
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, ok := allowedScopes[scope]; !ok {
			return nil, "", shared.UsageErrorf("--scope must be one of %s, %s", appInfoDirName, versionDirName)
		}
		scopeSet[scope] = struct{}{}
	}
	approved := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := scopeSet[item.Scope]; ok {
			approved = append(approved, item.Key)
		}
	}
	sort.Strings(approved)
	return approved, "scope", nil
}

func splitReviewCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func diffMetadataKeys(all []string, approved []string) []string {
	approvedSet := make(map[string]struct{}, len(approved))
	for _, key := range approved {
		approvedSet[key] = struct{}{}
	}
	pending := make([]string, 0)
	for _, key := range all {
		if _, ok := approvedSet[key]; !ok {
			pending = append(pending, key)
		}
	}
	sort.Strings(pending)
	return pending
}

func intersectMetadataKeys(all []string, approved []string) []string {
	allSet := make(map[string]struct{}, len(all))
	for _, key := range all {
		allSet[key] = struct{}{}
	}
	out := make([]string, 0)
	for _, key := range approved {
		if _, ok := allSet[key]; ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func mergeMetadataApprovalKeys(all []string, existing []string, selected []string) []string {
	allSet := make(map[string]struct{}, len(all))
	for _, key := range all {
		allSet[key] = struct{}{}
	}
	mergedSet := make(map[string]struct{}, len(existing)+len(selected))
	for _, key := range existing {
		if _, ok := allSet[key]; ok {
			mergedSet[key] = struct{}{}
		}
	}
	for _, key := range selected {
		if _, ok := allSet[key]; ok {
			mergedSet[key] = struct{}{}
		}
	}
	merged := make([]string, 0, len(mergedSet))
	for key := range mergedSet {
		merged = append(merged, key)
	}
	sort.Strings(merged)
	return merged
}

func printMetadataPlanArtifactTable(artifact MetadataPlanArtifact, markdown bool) error {
	fmt.Printf("Review Dir: %s\n", artifact.ReviewDir)
	fmt.Printf("Plan Path: %s\n", artifact.PlanPath)
	fmt.Printf("Approval Path: %s\n", artifact.ApprovalPath)
	fmt.Printf("Plan Hash: %s\n\n", artifact.PlanHash)
	if markdown {
		return printPushPlanMarkdown(artifact.Plan)
	}
	return printPushPlanTable(artifact.Plan)
}

func printMetadataApprovalTable(approval MetadataApprovalArtifact, markdown bool) error {
	headers := []string{"planHash", "mode", "approved", "pending", "approvalPath"}
	rows := [][]string{{
		approval.PlanHash,
		approval.Mode,
		fmt.Sprintf("%d", len(approval.ApprovedKeys)),
		fmt.Sprintf("%d", len(approval.PendingKeys)),
		approval.ApprovalPath,
	}}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return nil
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printMetadataReviewStatusTable(status MetadataReviewStatus, markdown bool) error {
	headers := []string{"ready", "planHash", "approvalMatchesPlan", "total", "approved", "pending"}
	rows := [][]string{{
		fmt.Sprintf("%t", status.Ready),
		status.PlanHash,
		fmt.Sprintf("%t", status.ApprovalMatchesPlan),
		fmt.Sprintf("%d", status.TotalCount),
		fmt.Sprintf("%d", status.ApprovedCount),
		fmt.Sprintf("%d", status.PendingCount),
	}}
	if markdown {
		asc.RenderMarkdown(headers, rows)
		return nil
	}
	asc.RenderTable(headers, rows)
	return nil
}
