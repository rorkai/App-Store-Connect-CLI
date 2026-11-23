package web

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	listWebAppComplianceRequirementsFn = func(ctx context.Context, client *webcore.Client, accountID, appID string) ([]webcore.AppComplianceRequirement, error) {
		return client.ListAppComplianceRequirements(ctx, accountID, appID)
	}
	setWebAppComplianceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declaration webcore.AppComplianceDeclaration) (*webcore.AppComplianceDeclarationResult, error) {
		return client.SetAppComplianceDeclaration(ctx, accountID, appID, declaration)
	}
)

const (
	complianceSecurityAssessment = "SECURITY_ASSESSMENT"
	complianceAIGeneratedContent = "AI_GENERATED_CONTENT"

	// maxAppComplianceFileSize bounds one evidence file. The file travels
	// base64-encoded inside the JSON request, so it is held in memory.
	maxAppComplianceFileSize = 20 << 20
)

const complianceLongHelp = `WEB SESSION WORKFLOWS

Read and answer the requirements App Store Connect tracks for an app on its App
Compliance page, such as the Security Assessment and the AI Generated or
Synthetic Content declaration for China mainland. Apple does not expose these
requirements on the public App Store Connect API, so this command uses the same
web-session endpoints the website uses.

Every requirement is listed whether or not it was answered. ` + "`PENDING_COLLECT`" + `
means it has no answer yet, ` + "`COLLECTED_VERIFY_PENDING`" + ` means Apple has not
verified the answer, and ` + "`COLLECTED_VERIFY_NOT_NECESSARY`" + ` means the answer
needs no verification. Countries and regions are ISO 3166-1 alpha-3 codes.

The regulated medical device declaration is a separate requirement; read it
with ` + "`asc web apps declarations list`" + `.

`

// newWebAppsComplianceCommand returns the App Compliance command group.
func newWebAppsComplianceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compliance", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "compliance",
		ShortUsage: "asc web apps compliance <subcommand> [flags]",
		ShortHelp:  "Read and answer App Compliance requirements via web sessions.",
		LongHelp:   complianceLongHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAppsComplianceListCommand(d),
			newWebAppsComplianceSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAppsComplianceListCommand lists the App Compliance requirements for an app.
func newWebAppsComplianceListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compliance list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web apps compliance list --app APP_ID [flags]",
		ShortHelp:  "List App Compliance requirements for an app.",
		LongHelp: complianceLongHelp + `Examples:
  asc web apps compliance list --app "6748252780"
  asc web apps compliance list --app "6748252780" --output json

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, d, authFlags, "web apps compliance list")
			defer cancel()
			if err != nil {
				return err
			}

			var requirements []webcore.AppComplianceRequirement
			err = withWebSpinnerWith(d, "Fetching app compliance requirements", func() error {
				var err error
				requirements, err = listWebAppComplianceRequirementsFn(requestCtx, client, accountID, resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps compliance list")
			}

			outputList := make(asc.WebAppComplianceRequirementList, 0, len(requirements))
			for _, requirement := range requirements {
				outputList = append(outputList, asc.WebAppComplianceRequirement(requirement))
			}
			return d.PrintOutput(&outputList, *output.Output, *output.Pretty)
		},
	}
}

// newWebAppsComplianceSetCommand answers one App Compliance requirement.
func newWebAppsComplianceSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compliance set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	requirement := fs.String("requirement", "", "Requirement to answer: SECURITY_ASSESSMENT or AI_GENERATED_CONTENT")
	var declared shared.OptionalBool
	fs.Var(&declared, "declared", "Whether the requirement applies to the app in China mainland: true or false")
	proof := fs.String("proof", "", "Proof of a completed security assessment (file; SECURITY_ASSESSMENT with --declared true)")
	report := fs.String("report", "", "Security assessment report (file; SECURITY_ASSESSMENT with --declared true)")
	filingType := fs.String("filing-type", "", "Filing to provide: LLM or ALGORITHM (AI_GENERATED_CONTENT with --declared true)")
	filingName := fs.String("filing-name", "", "Name of the filed LLM or algorithm (AI_GENERATED_CONTENT with --declared true)")
	filingNumber := fs.String("filing-number", "", "Filing number of the LLM or algorithm (AI_GENERATED_CONTENT with --declared true)")
	var thirdParty shared.OptionalBool
	fs.Var(&thirdParty, "third-party-technology", "Whether third-party technology powers the app's AI features: true or false (AI_GENERATED_CONTENT with --declared true)")
	authorization := fs.String("third-party-authorization", "", "Authorization or usage agreement from the third-party technology (file; with --third-party-technology true)")
	dryRun := fs.Bool("dry-run", false, "Check the answer and its files locally without sending them to Apple")
	confirm := fs.Bool("confirm", false, "Confirm saving the declaration (required unless --dry-run)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps compliance set --app APP_ID --requirement REQUIREMENT --declared true|false [evidence flags] [--dry-run | --confirm] [flags]",
		ShortHelp:  "Answer an App Compliance requirement for an app.",
		LongHelp: `WEB SESSION WORKFLOWS

Answer one App Compliance requirement for China mainland through the same
web-session endpoints the App Store Connect website uses.

` + "`--declared false`" + ` states that the requirement does not apply to the app in
China mainland and takes no evidence. ` + "`--declared true`" + ` needs the evidence
Apple's form asks for:

  SECURITY_ASSESSMENT   --proof and --report
  AI_GENERATED_CONTENT  --filing-type, --filing-name, --filing-number and
                        --third-party-technology, plus
                        --third-party-authorization when that is true

Evidence files are .doc, .docx, .jpeg, .jpg, .pdf or .png, up to 20 MB each.
An answer with files runs under the upload timeout (ASC_UPLOAD_TIMEOUT) instead
of the request timeout. If the save fails without an answer from Apple, the
outcome is unknown; check ` + "`asc web apps compliance list`" + ` before retrying.

The command fills Apple's form step by step and saves only when it can answer
every field the form asks for; otherwise it stops without saving. A requirement
at ` + "`COLLECTED_VERIFY_PENDING`" + ` is read-only until Apple finishes verifying it.
Use ` + "`--dry-run`" + ` to check the requirement's state, the answer and its files
without sending the answer or any file to Apple: Apple's form takes them with
every step, so a dry run does not fill it and cannot report fields only the
form asks for. Read-only mode (` + "`ASC_READ_ONLY`" + `) refuses this command before
the form. Apple verifies an affirmative answer, so its status stays
` + "`COLLECTED_VERIFY_PENDING`" + ` after a save.

Examples:
  asc web apps compliance set --app "6748252780" --requirement SECURITY_ASSESSMENT --declared false --confirm
  asc web apps compliance set --app "6748252780" --requirement SECURITY_ASSESSMENT --declared true --proof "./proof.pdf" --report "./report.pdf" --confirm
  asc web apps compliance set --app "6748252780" --requirement AI_GENERATED_CONTENT --declared true --filing-type LLM --filing-name "Example Model" --filing-number "Example-0000" --third-party-technology false --dry-run

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}
			declaration := webcore.AppComplianceDeclaration{
				Requirement:          strings.ToUpper(strings.TrimSpace(*requirement)),
				Declared:             declared.Value(),
				FilingType:           strings.ToUpper(strings.TrimSpace(*filingType)),
				FilingName:           strings.TrimSpace(*filingName),
				FilingNumber:         strings.TrimSpace(*filingNumber),
				ThirdPartyTechnology: thirdParty.Value(),
				DryRun:               *dryRun,
			}
			if declaration.Requirement != complianceSecurityAssessment && declaration.Requirement != complianceAIGeneratedContent {
				return d.UsageError("--requirement must be one of: SECURITY_ASSESSMENT, AI_GENERATED_CONTENT")
			}
			if !declared.IsSet() {
				return d.UsageError("--declared is required (supported values: true, false)")
			}

			// Each answer takes exactly the evidence Apple's form asks for.
			var required []string
			switch {
			case !declaration.Declared:
			case declaration.Requirement == complianceSecurityAssessment:
				required = []string{"proof", "report"}
			default:
				required = []string{"filing-type", "filing-name", "filing-number", "third-party-technology"}
			}
			answer := fmt.Sprintf("%s with --declared %t", declaration.Requirement, declaration.Declared)
			provided := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { provided[f.Name] = strings.TrimSpace(f.Value.String()) != "" })
			for _, name := range []string{"proof", "report", "filing-type", "filing-name", "filing-number", "third-party-technology"} {
				switch {
				case slices.Contains(required, name) && !provided[name]:
					return d.UsageErrorf("--%s is required for %s", name, answer)
				case !slices.Contains(required, name) && provided[name]:
					return d.UsageErrorf("--%s does not apply to %s", name, answer)
				}
			}
			if len(required) == 4 && declaration.FilingType != "LLM" && declaration.FilingType != "ALGORITHM" {
				return d.UsageError("--filing-type must be one of: LLM, ALGORITHM")
			}
			if provided["third-party-authorization"] != declaration.ThirdPartyTechnology {
				if declaration.ThirdPartyTechnology {
					return d.UsageError("--third-party-authorization is required with --third-party-technology true")
				}
				return d.UsageError("--third-party-authorization does not apply with --third-party-technology false")
			}
			if err := d.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			for _, evidence := range []struct {
				flag string
				path string
				file **webcore.AppComplianceFile
			}{
				{"--proof", *proof, &declaration.Proof},
				{"--report", *report, &declaration.Report},
				{"--third-party-authorization", *authorization, &declaration.ThirdPartyAuthorization},
			} {
				if strings.TrimSpace(evidence.path) == "" {
					continue
				}
				file, err := readWebAppComplianceFile(evidence.flag, evidence.path)
				if err != nil {
					return err
				}
				*evidence.file = file
			}

			accountID, client, requestCtx, cancel, err := resolveWebComplianceClient(ctx, d, authFlags, "web apps compliance set")
			defer cancel()
			if err != nil {
				return err
			}
			hasEvidence := declaration.Proof != nil || declaration.Report != nil || declaration.ThirdPartyAuthorization != nil
			// Evidence files are uploaded twice (the last form step and the
			// save), so they get the upload budget instead of the request one.
			// A dry run uploads nothing.
			if hasEvidence && !*dryRun {
				uploadCtx, uploadCancel := d.UploadTimeout(shared.ContextWithoutTimeout(ctx))
				defer uploadCancel()
				requestCtx = uploadCtx
			}

			label := "Saving app compliance declaration"
			if *dryRun {
				label = "Checking app compliance declaration"
			}
			var result *webcore.AppComplianceDeclarationResult
			err = withWebSpinnerWith(d, label, func() error {
				var err error
				result, err = setWebAppComplianceDeclarationFn(requestCtx, client, accountID, resolvedAppID, declaration)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps compliance set")
			}
			if result == nil {
				return fmt.Errorf("web apps compliance set failed: missing declaration result")
			}

			if *dryRun {
				// Apple's form takes the answer and its evidence with every step,
				// so a dry run checks them locally and sends neither.
				note := "Dry run: nothing was sent to Apple; the answer was checked locally."
				if hasEvidence {
					note = "Dry run: nothing was sent to Apple; the answer and its evidence files were checked locally and not uploaded."
				}
				fmt.Fprintln(d.IO.ErrOut, note)
			}
			receipt := asc.WebAppComplianceDeclarationResult(*result)
			return d.PrintOutput(&receipt, *output.Output, *output.Pretty)
		},
	}
}

func readWebAppComplianceFile(flagName, path string) (*webcore.AppComplianceFile, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", flagName, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAppComplianceFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", flagName, err)
	}
	if len(data) > maxAppComplianceFileSize {
		return nil, fmt.Errorf("%s exceeds the %d MB limit", flagName, maxAppComplianceFileSize>>20)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", flagName)
	}
	return &webcore.AppComplianceFile{Name: filepath.Base(path), Data: data}, nil
}
