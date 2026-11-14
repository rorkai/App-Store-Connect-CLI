package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"image"
	"image/png"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDepsBindOutputFlagsDefaultFollowsStdoutTTY(t *testing.T) {
	resetDefaultOutput(t)
	clearCIEnvironment(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	os.Unsetenv("ASC_DEFAULT_OUTPUT")

	for _, tt := range []struct {
		stdoutTTY bool
		want      string
	}{
		{stdoutTTY: true, want: "table"},
		{stdoutTTY: false, want: "json"},
	} {
		d := &Deps{IO: &IO{StdoutTTY: tt.stdoutTTY}}
		output := d.BindOutputFlags(flag.NewFlagSet("list", flag.ContinueOnError))
		if *output.Output != tt.want {
			t.Fatalf("StdoutTTY=%t: default output = %q, want %q", tt.stdoutTTY, *output.Output, tt.want)
		}
	}
}

func TestDepsBindOutputFlagsReportsInvalidDefaultOnDepsStderr(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "xml")

	var stderr bytes.Buffer
	d := &Deps{IO: &IO{ErrOut: &stderr, StdoutTTY: true}}
	var output OutputFlags
	processStdout, processStderr := captureOutput(t, func() {
		output = d.BindOutputFlags(flag.NewFlagSet("list", flag.ContinueOnError))
	})

	if processStdout != "" || processStderr != "" {
		t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
	}
	if got, want := stderr.String(), "Warning: invalid ASC_DEFAULT_OUTPUT value \"xml\" (expected json, table, markdown, or md); using json\n"; got != want {
		t.Fatalf("deps stderr = %q, want %q", got, want)
	}
	if *output.Output != "json" {
		t.Fatalf("default output = %q, want json", *output.Output)
	}
}

func TestDepsPrintOutputMatchesPrintOutputOnDepsStreams(t *testing.T) {
	var page asc.ActorsResponse
	if err := json.Unmarshal([]byte(`{
		"data":[{"type":"actors","id":"actor-1","attributes":{"actorType":"USER","userEmail":"a@example.com"}}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/actors","next":"https://api.appstoreconnect.apple.com/v1/actors?cursor=abc"}
	}`), &page); err != nil {
		t.Fatalf("unmarshal page: %v", err)
	}

	for _, tt := range []struct {
		format string
		pretty bool
	}{
		{format: "json"},
		{format: "json", pretty: true},
		{format: "table"},
		{format: "markdown"},
	} {
		t.Run(tt.format, func(t *testing.T) {
			wantStdout, wantStderr := captureOutput(t, func() {
				if err := PrintOutput(&page, tt.format, tt.pretty); err != nil {
					t.Errorf("PrintOutput() error = %v", err)
				}
			})

			var stdout, stderr bytes.Buffer
			d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr}}
			processStdout, processStderr := captureOutput(t, func() {
				if err := d.PrintOutput(&page, tt.format, tt.pretty); err != nil {
					t.Errorf("Deps.PrintOutput() error = %v", err)
				}
			})

			if processStdout != "" || processStderr != "" {
				t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
			}
			if stdout.String() != wantStdout {
				t.Fatalf("deps stdout = %q, want %q", stdout.String(), wantStdout)
			}
			if stderr.String() != wantStderr {
				t.Fatalf("deps stderr = %q, want %q", stderr.String(), wantStderr)
			}
			if wantStdout == "" || wantStderr == "" {
				t.Fatalf("expected PrintOutput to write a page and a more-pages warning, got stdout %q, stderr %q", wantStdout, wantStderr)
			}
		})
	}
}

func TestDepsUsageErrorsMatchProcessVariantsOnDepsStderr(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/actors?cursor=AQ"
	nextWithLimit := func() *flag.FlagSet {
		fs := flag.NewFlagSet("list", flag.ContinueOnError)
		fs.Int("limit", 0, "")
		if err := fs.Parse([]string{"--limit", "5"}); err != nil {
			t.Fatalf("parse --limit: %v", err)
		}
		return fs
	}
	parsed := func(bind func(*flag.FlagSet), args ...string) *flag.FlagSet {
		fs := flag.NewFlagSet("command", flag.ContinueOnError)
		bind(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		return fs
	}
	rewriteCtx := ContextWithUsageMessageRewrites(context.Background(), UsageMessageRewrite{Old: "testflight", New: "beta"})
	ambiguous := errors.New("multiple apps match \"Outslept\"\n  app-1  Outslept\n  app-2  Outslept")
	relationshipTypes := []string{"builds", "betaGroups"}
	appInfosClient := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/appInfos/info-editable/appInfoLocalizations" {
			return appResolutionJSONResponse(`{"data":[]}`)
		}
		return appResolutionJSONResponse(`{"data":[
			{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_SALE"}},
			{"type":"appInfos","id":"info-editable","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
		]}`)
	})
	versionsClient := defaultVersionTestClient(t, map[string]string{
		"|" + defaultVersionEditableFilter + "|": `{"data":[
			{"type":"appStoreVersions","id":"ver-new","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":""}}`,
	}, nil)
	betaGroupPlan := BuildBetaGroupAssignmentPlan{GroupsToAdd: []ResolvedBetaGroup{{ID: "group-1"}}}
	var screenshot bytes.Buffer
	if err := png.Encode(&screenshot, image.NewGray(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatalf("encode screenshot: %v", err)
	}
	screenshotFile := bytes.NewReader(screenshot.Bytes())

	tests := []struct {
		name    string
		process func() (any, error)
		deps    func(*Deps) (any, error)
	}{
		{
			name:    "UsageError",
			process: func() (any, error) { return nil, UsageError("actors list: --limit must be between 1 and 200") },
			deps:    func(d *Deps) (any, error) { return nil, d.UsageError("actors list: --limit must be between 1 and 200") },
		},
		{
			name: "UsageErrorf",
			process: func() (any, error) {
				return nil, UsageErrorf("%s: --limit must be between 1 and %d", "actors view", 200)
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.UsageErrorf("%s: --limit must be between 1 and %d", "actors view", 200)
			},
		},
		{
			name:    "RequiredFlag",
			process: func() (any, error) { return nil, RequiredFlag("--id") },
			deps:    func(d *Deps) (any, error) { return nil, d.RequiredFlag("--id") },
		},
		{
			name: "RejectNextFlagConflicts",
			process: func() (any, error) {
				return nil, RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit")
			},
		},
		{
			name:    "UsageErrorCtx",
			process: func() (any, error) { return nil, UsageErrorCtx(rewriteCtx, "testflight list: --app is required") },
			deps: func(d *Deps) (any, error) {
				return nil, d.UsageErrorCtx(rewriteCtx, "testflight list: --app is required")
			},
		},
		{
			name: "UsageErrorfCtx",
			process: func() (any, error) {
				return nil, UsageErrorfCtx(rewriteCtx, "%s: --app is required", "testflight list")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.UsageErrorfCtx(rewriteCtx, "%s: --app is required", "testflight list")
			},
		},
		{
			name:    "AmbiguousUsageError",
			process: func() (any, error) { return nil, AmbiguousUsageError(ambiguous) },
			deps:    func(d *Deps) (any, error) { return nil, d.AmbiguousUsageError(ambiguous) },
		},
		{
			name:    "AmbiguousUsageErrorWithKind",
			process: func() (any, error) { return nil, AmbiguousUsageErrorWithKind(ambiguous, UsageErrorInvalidValue) },
			deps: func(d *Deps) (any, error) {
				return nil, d.AmbiguousUsageErrorWithKind(ambiguous, UsageErrorInvalidValue)
			},
		},
		{
			name:    "AppIDFlagValue",
			process: func() (any, error) { return AppIDFlagValue("https://api.appstoreconnect.apple.com/v1/builds/123") },
			deps: func(d *Deps) (any, error) {
				return d.AppIDFlagValue("https://api.appstoreconnect.apple.com/v1/builds/123")
			},
		},
		{
			name:    "RequireConfirmUnlessDryRun",
			process: func() (any, error) { return nil, RequireConfirmUnlessDryRun(false, false) },
			deps:    func(d *Deps) (any, error) { return nil, d.RequireConfirmUnlessDryRun(false, false) },
		},
		{
			name:    "ParseIfExistsMode",
			process: func() (any, error) { return ParseIfExistsMode("replace", IfExistsSkip) },
			deps:    func(d *Deps) (any, error) { return d.ParseIfExistsMode("replace", IfExistsSkip) },
		},
		{
			name:    "ParseOptionalIfExistsMode",
			process: func() (any, error) { return ParseOptionalIfExistsMode("replace", IfExistsUpdate) },
			deps:    func(d *Deps) (any, error) { return d.ParseOptionalIfExistsMode("replace", IfExistsUpdate) },
		},
		{
			name:    "MissingRelationshipTypeUsageError",
			process: func() (any, error) { return nil, MissingRelationshipTypeUsageError(relationshipTypes) },
			deps:    func(d *Deps) (any, error) { return nil, d.MissingRelationshipTypeUsageError(relationshipTypes) },
		},
		{
			name: "PrintInvalidRelationshipTypeError",
			process: func() (any, error) {
				PrintInvalidRelationshipTypeError("apps", relationshipTypes)
				return nil, nil
			},
			deps: func(d *Deps) (any, error) {
				d.PrintInvalidRelationshipTypeError("apps", relationshipTypes)
				return nil, nil
			},
		},
		{
			name:    "ValidateCertificateCreateType",
			process: func() (any, error) { return ValidateCertificateCreateType("--certificate-type", "BOGUS") },
			deps:    func(d *Deps) (any, error) { return d.ValidateCertificateCreateType("--certificate-type", "BOGUS") },
		},
		{
			name:    "ValidateCertificateCreateMerchantID",
			process: func() (any, error) { return nil, ValidateCertificateCreateMerchantID("APPLE_PAY", "") },
			deps:    func(d *Deps) (any, error) { return nil, d.ValidateCertificateCreateMerchantID("APPLE_PAY", "") },
		},
		{
			name:    "ReviewScreenshotUsageError",
			process: func() (any, error) { return nil, ReviewScreenshotUsageError("--file", "review screenshot is empty") },
			deps: func(d *Deps) (any, error) {
				return nil, d.ReviewScreenshotUsageError("--file", "review screenshot is empty")
			},
		},
		{
			name: "PreflightReviewScreenshot",
			process: func() (any, error) {
				return nil, PreflightReviewScreenshot("review.png", screenshotFile, screenshotFile.Size())
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.PreflightReviewScreenshot("review.png", screenshotFile, screenshotFile.Size())
			},
		},
		{
			name: "BindTargetSubscriptionPlanTypeFlag",
			process: func() (any, error) {
				var resolve func() (asc.SubscriptionPlanType, error)
				parsed(func(fs *flag.FlagSet) { resolve = BindTargetSubscriptionPlanTypeFlag(fs) }, "--target-subscription-plan-type", "WEEKLY")
				return resolve()
			},
			deps: func(d *Deps) (any, error) {
				var resolve func() (asc.SubscriptionPlanType, error)
				parsed(func(fs *flag.FlagSet) { resolve = d.BindTargetSubscriptionPlanTypeFlag(fs) }, "--target-subscription-plan-type", "WEEKLY")
				return resolve()
			},
		},
		{
			name: "NormalizeBuildProcessingStateFilter",
			process: func() (any, error) {
				return NormalizeBuildProcessingStateFilter("VALID,ALL", BuildProcessingStateFilterOptions{})
			},
			deps: func(d *Deps) (any, error) {
				return d.NormalizeBuildProcessingStateFilter("VALID,ALL", BuildProcessingStateFilterOptions{})
			},
		},
		{
			name: "NormalizeLatestBuildSelectionOptions",
			process: func() (any, error) {
				return NormalizeLatestBuildSelectionOptions("app-1", "1.0", "", "BOGUS", false)
			},
			deps: func(d *Deps) (any, error) {
				return d.NormalizeLatestBuildSelectionOptions("app-1", "1.0", "", "BOGUS", false)
			},
		},
		{
			name: "PPPTerritorySelection",
			process: func() (any, error) {
				var value *OnceCSVValue
				parsed(func(fs *flag.FlagSet) { value = BindOnceCSVFlag(fs, "territory", "") }, "--territory", ",")
				return PPPTerritorySelection(value)
			},
			deps: func(d *Deps) (any, error) {
				var value *OnceCSVValue
				parsed(func(fs *flag.FlagSet) { value = BindOnceCSVFlag(fs, "territory", "") }, "--territory", ",")
				return d.PPPTerritorySelection(value)
			},
		},
		{
			name:    "ParsePriceRounding",
			process: func() (any, error) { return ParsePriceRounding("sideways") },
			deps:    func(d *Deps) (any, error) { return d.ParsePriceRounding("sideways") },
		},
		{
			name: "RecoverBoolFlagTailArgs",
			process: func() (any, error) {
				return nil, RecoverBoolFlagTailArgs(flag.NewFlagSet("create", flag.ContinueOnError), []string{"stray"}, nil)
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.RecoverBoolFlagTailArgs(flag.NewFlagSet("create", flag.ContinueOnError), []string{"stray"}, nil)
			},
		},
		{
			name:    "RequireAppForStableSelector",
			process: func() (any, error) { return nil, RequireAppForStableSelector("", "Monthly Pro", "--subscription") },
			deps: func(d *Deps) (any, error) {
				return nil, d.RequireAppForStableSelector("", "Monthly Pro", "--subscription")
			},
		},
		{
			name: "IncludedFieldFlags.ResolveWith",
			process: func() (any, error) {
				var fields *IncludedFieldFlags
				parsed(func(fs *flag.FlagSet) {
					fields = BindIncludedFieldFlags(fs, map[string]string{"territory": "territories"})
				}, "--territory-fields", "currency")
				return fields.Resolve("equalizations list", "", nil)
			},
			deps: func(d *Deps) (any, error) {
				var fields *IncludedFieldFlags
				parsed(func(fs *flag.FlagSet) {
					fields = BindIncludedFieldFlags(fs, map[string]string{"territory": "territories"})
				}, "--territory-fields", "currency")
				return fields.ResolveWith(d, "equalizations list", "", nil)
			},
		},
		{
			name: "LocalizationIncludeFlags.ResolveWith",
			process: func() (any, error) {
				var include *LocalizationIncludeFlags
				parsed(func(fs *flag.FlagSet) {
					include = BindLocalizationIncludeFlags(fs, "Include", []string{"placements"})
				}, "--placements-limit", "99")
				values, limit, err := include.Resolve()
				return []any{values, limit}, err
			},
			deps: func(d *Deps) (any, error) {
				var include *LocalizationIncludeFlags
				parsed(func(fs *flag.FlagSet) {
					include = BindLocalizationIncludeFlags(fs, "Include", []string{"placements"})
				}, "--placements-limit", "99")
				values, limit, err := include.ResolveWith(d)
				return []any{values, limit}, err
			},
		},
		{
			name: "MediaPollIntervalFlag.ApplyWith",
			process: func() (any, error) {
				var interval *MediaPollIntervalFlag
				parsed(func(fs *flag.FlagSet) { interval = BindMediaPollIntervalFlag(fs) }, "--poll-interval", "10ms")
				return interval.Apply(context.Background())
			},
			deps: func(d *Deps) (any, error) {
				var interval *MediaPollIntervalFlag
				parsed(func(fs *flag.FlagSet) { interval = BindMediaPollIntervalFlag(fs) }, "--poll-interval", "10ms")
				return interval.ApplyWith(context.Background(), d)
			},
		},
		{
			name: "ResolveAndAnnounceDefaultAppStoreVersion",
			process: func() (any, error) {
				return ResolveAndAnnounceDefaultAppStoreVersion(context.Background(), versionsClient, "app-1", "", "--version")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAndAnnounceDefaultAppStoreVersion(context.Background(), versionsClient, "app-1", "", "--version")
			},
		},
		{
			name: "PreflightBuildBetaGroupAssignment",
			process: func() (any, error) {
				return nil, PreflightBuildBetaGroupAssignment(context.Background(), emptyBuildPreflightClient{}, "build-1", betaGroupPlan, BuildBetaGroupPreflightOptions{OperationName: "builds add-groups"})
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.PreflightBuildBetaGroupAssignment(context.Background(), emptyBuildPreflightClient{}, "build-1", betaGroupPlan, BuildBetaGroupPreflightOptions{OperationName: "builds add-groups"})
			},
		},
		{
			name: "ResolveAppInfoID",
			process: func() (any, error) {
				return ResolveAppInfoID(context.Background(), appInfosClient, "app-1", "")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAppInfoID(context.Background(), appInfosClient, "app-1", "")
			},
		},
		{
			name: "ResolveAppInfoIDWithFlag",
			process: func() (any, error) {
				return ResolveAppInfoIDWithFlag(context.Background(), appInfosClient, "app-1", "", "--app-info-id")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAppInfoIDWithFlag(context.Background(), appInfosClient, "app-1", "", "--app-info-id")
			},
		},
		{
			name: "ResolveOwnedAppInfoID",
			process: func() (any, error) {
				return ResolveOwnedAppInfoID(context.Background(), appInfosClient, "app-1", "")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveOwnedAppInfoID(context.Background(), appInfosClient, "app-1", "")
			},
		},
		{
			name: "PlanAppInfoLocalizationUpsert",
			process: func() (any, error) {
				return PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "app-1", "", "en-US", map[string]string{"name": "Outslept"})
			},
			deps: func(d *Deps) (any, error) {
				return d.PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "app-1", "", "en-US", map[string]string{"name": "Outslept"})
			},
		},
		{
			name: "WarnDeprecatedAppInfoFields",
			process: func() (any, error) {
				WarnDeprecatedAppInfoFields([]string{"kidsAgeBand"}, "")
				return nil, nil
			},
			deps: func(d *Deps) (any, error) {
				d.WarnDeprecatedAppInfoFields([]string{"kidsAgeBand"}, "")
				return nil, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wantValue any
			var wantErr error
			_, wantStderr := captureOutput(t, func() { wantValue, wantErr = test.process() })

			var stdout, stderr bytes.Buffer
			d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr}}
			var gotValue any
			var gotErr error
			processStdout, processStderr := captureOutput(t, func() { gotValue, gotErr = test.deps(d) })

			if processStdout != "" || processStderr != "" {
				t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
			}
			if stdout.String() != "" {
				t.Fatalf("deps stdout = %q, want empty", stdout.String())
			}
			if wantStderr == "" || stderr.String() != wantStderr {
				t.Fatalf("deps stderr = %q, want %q", stderr.String(), wantStderr)
			}
			if !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("error = %#v, want %#v", gotErr, wantErr)
			}
			if !reflect.DeepEqual(gotValue, wantValue) {
				t.Fatalf("value = %#v, want %#v", gotValue, wantValue)
			}
		})
	}
}

type emptyBuildPreflightClient struct{}

func (emptyBuildPreflightClient) GetBuild(context.Context, string, ...asc.BuildOption) (*asc.BuildResponse, error) {
	return nil, nil
}

func (emptyBuildPreflightClient) GetBuildBuildBetaDetail(context.Context, string) (*asc.BuildBetaDetailResponse, error) {
	return nil, nil
}
