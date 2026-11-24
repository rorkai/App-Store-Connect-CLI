package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

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
				if err := DefaultDeps().PrintOutput(&page, tt.format, tt.pretty); err != nil {
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
	// buildsClient matches version "1.2.0" only as "1.2" and serves one
	// processed build whose number is not a positive integer.
	buildsClient := func() *asc.Client {
		ResetEquivalentVersionNotesForTest()
		return newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/v1/preReleaseVersions":
				if req.URL.Query().Get("filter[version]") == "1.2" {
					return buildWaitJSONResponse(`{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2","platform":"IOS"}}],"links":{}}`)
				}
			case "/v1/builds":
				return buildWaitJSONResponse(`{"data":[{"type":"builds","id":"build-1","attributes":{"version":"abc","uploadedDate":"2026-01-02T00:00:00Z","processingState":"VALID"}}],"links":{}}`)
			case "/v1/apps/app-1/appAvailabilityV2":
				return buildWaitJSONResponse(`{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`)
			case "/v2/appAvailabilities/availability-1/territoryAvailabilities":
				return buildWaitJSONResponse(`{"data":[{"type":"territoryAvailabilities","id":"eyJ0IjoiVVNBIn0","attributes":{"available":true}}],"links":{}}`)
			}
			return buildWaitJSONResponse(`{"data":[],"links":{}}`)
		})
	}
	blockedCacheParent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedCacheParent, nil, 0o600); err != nil {
		t.Fatalf("write %s: %v", blockedCacheParent, err)
	}
	tiersClient := func() *asc.Client {
		tierCacheDirOverrideMu.Lock()
		previous := tierCacheDirOverride
		tierCacheDirOverride = filepath.Join(blockedCacheParent, "cache")
		tierCacheDirOverrideMu.Unlock()
		t.Cleanup(func() {
			tierCacheDirOverrideMu.Lock()
			tierCacheDirOverride = previous
			tierCacheDirOverrideMu.Unlock()
		})
		return newBuildWaitTestClient(t, func(*http.Request) (*http.Response, error) {
			return buildWaitJSONResponse(`{"data":[{"type":"pricePoints","id":"point-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}],"links":{}}`)
		})
	}
	whatsNewClient := func() *stubVersionLocalizationClient {
		return &stubVersionLocalizationClient{
			getResp: &asc.AppStoreVersionLocalizationsResponse{Data: []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
				{ID: "existing-loc", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US"}},
			}},
			updateErrs: []error{errors.New("An attribute value is not acceptable for the current resource state. The attribute 'whatsNew' is not editable.")},
		}
	}
	whatsNew := map[string]map[string]string{"en-US": {"description": "A description", "whatsNew": "Bug fixes"}}
	latestBuild := LatestBuildSelectionOptions{AppID: "123456789", Version: "1.2.0", Platform: "IOS"}
	nextBuild := NextBuildNumberOptions{LatestBuildSelectionOptions: latestBuild, InitialBuildNumber: 1}
	availabilityUpdate := TerritoryAvailabilityUpdateRequest{AppID: "app-1", AllTerritories: true, Available: true, ErrorPrefix: "pricing availability edit"}
	// availabilityClient serves USA available and FRA unavailable until FRA
	// is patched.
	availabilityClient := func() *asc.Client {
		patched := false
		return newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
			switch {
			case req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
				return buildWaitJSONResponse(`{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`)
			case req.Method == http.MethodPatch && req.URL.Path == "/v1/territoryAvailabilities/eyJ0IjoiRlJBIn0":
				patched = true
				return buildWaitJSONResponse(`{"data":{"type":"territoryAvailabilities","id":"eyJ0IjoiRlJBIn0","attributes":{"available":true}}}`)
			case req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
				return buildWaitJSONResponse(fmt.Sprintf(`{"data":[{"type":"territoryAvailabilities","id":"eyJ0IjoiVVNBIn0","attributes":{"available":true}},{"type":"territoryAvailabilities","id":"eyJ0IjoiRlJBIn0","attributes":{"available":%t}}],"links":{}}`, patched))
			}
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		})
	}
	runBuilder := func(cmd *ffcli.Command, args ...string) error {
		if err := cmd.FlagSet.Parse(args); err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		return cmd.Exec(context.Background(), nil)
	}
	pricingSet := PricingSetCommandConfig{FlagSetName: "pricing set", CommandName: "set", ErrorPrefix: "pricing set", Deps: DefaultDeps()}
	availabilitySet := AvailabilitySetCommandConfig{FlagSetName: "pricing availability edit", CommandName: "edit", ErrorPrefix: "pricing availability edit", Deps: DefaultDeps()}
	t.Setenv("ASC_APP_ID", "")
	ppp := PriceSchedulePPPCommandConfig{
		Deps:        DefaultDeps(),
		FlagSetName: "pricing ppp",
		ErrorPrefix: "pricing ppp",
		BindResource: func(*flag.FlagSet) PriceSchedulePPPResource {
			return PriceSchedulePPPResource{Validate: func() error { return nil }}
		},
	}
	pppProgress := ppp
	pppProgress.BindResource = func(*flag.FlagSet) PriceSchedulePPPResource {
		return PriceSchedulePPPResource{
			Validate: func() error { return nil },
			Resolve: func(context.Context, *asc.Client) (PriceSchedulePPPSurface, error) {
				return PriceSchedulePPPSurface{CurrentPrices: func(context.Context, time.Time) ([]ResolvedPriceRow, error) {
					return nil, errors.New("prices unavailable")
				}}, nil
			},
		}
	}
	t.Cleanup(SetASCClientFactoryForTesting(func() (*asc.Client, error) { return buildsClient(), nil }))

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
				return nil, DefaultDeps().RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit")
			},
		},
		{
			name: "UsageErrorCtx",
			process: func() (any, error) {
				return nil, DefaultDeps().UsageErrorCtx(rewriteCtx, "testflight list: --app is required")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.UsageErrorCtx(rewriteCtx, "testflight list: --app is required")
			},
		},
		{
			name: "UsageErrorfCtx",
			process: func() (any, error) {
				return nil, DefaultDeps().UsageErrorfCtx(rewriteCtx, "%s: --app is required", "testflight list")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.UsageErrorfCtx(rewriteCtx, "%s: --app is required", "testflight list")
			},
		},
		{
			name:    "AmbiguousUsageError",
			process: func() (any, error) { return nil, DefaultDeps().AmbiguousUsageError(ambiguous) },
			deps:    func(d *Deps) (any, error) { return nil, d.AmbiguousUsageError(ambiguous) },
		},
		{
			name: "AmbiguousUsageErrorWithKind",
			process: func() (any, error) {
				return nil, DefaultDeps().AmbiguousUsageErrorWithKind(ambiguous, UsageErrorInvalidValue)
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.AmbiguousUsageErrorWithKind(ambiguous, UsageErrorInvalidValue)
			},
		},
		{
			name: "AppIDFlagValue",
			process: func() (any, error) {
				return DefaultDeps().AppIDFlagValue("https://api.appstoreconnect.apple.com/v1/builds/123")
			},
			deps: func(d *Deps) (any, error) {
				return d.AppIDFlagValue("https://api.appstoreconnect.apple.com/v1/builds/123")
			},
		},
		{
			name:    "RequireConfirmUnlessDryRun",
			process: func() (any, error) { return nil, DefaultDeps().RequireConfirmUnlessDryRun(false, false) },
			deps:    func(d *Deps) (any, error) { return nil, d.RequireConfirmUnlessDryRun(false, false) },
		},
		{
			name:    "ParseIfExistsMode",
			process: func() (any, error) { return DefaultDeps().ParseIfExistsMode("replace", IfExistsSkip) },
			deps:    func(d *Deps) (any, error) { return d.ParseIfExistsMode("replace", IfExistsSkip) },
		},
		{
			name:    "ParseOptionalIfExistsMode",
			process: func() (any, error) { return DefaultDeps().ParseOptionalIfExistsMode("replace", IfExistsUpdate) },
			deps:    func(d *Deps) (any, error) { return d.ParseOptionalIfExistsMode("replace", IfExistsUpdate) },
		},
		{
			name:    "MissingRelationshipTypeUsageError",
			process: func() (any, error) { return nil, DefaultDeps().MissingRelationshipTypeUsageError(relationshipTypes) },
			deps:    func(d *Deps) (any, error) { return nil, d.MissingRelationshipTypeUsageError(relationshipTypes) },
		},
		{
			name: "PrintInvalidRelationshipTypeError",
			process: func() (any, error) {
				DefaultDeps().PrintInvalidRelationshipTypeError("apps", relationshipTypes)
				return nil, nil
			},
			deps: func(d *Deps) (any, error) {
				d.PrintInvalidRelationshipTypeError("apps", relationshipTypes)
				return nil, nil
			},
		},
		{
			name:    "ValidateCertificateCreateType",
			process: func() (any, error) { return DefaultDeps().ValidateCertificateCreateType("--certificate-type", "BOGUS") },
			deps:    func(d *Deps) (any, error) { return d.ValidateCertificateCreateType("--certificate-type", "BOGUS") },
		},
		{
			name:    "ValidateCertificateCreateMerchantID",
			process: func() (any, error) { return nil, DefaultDeps().ValidateCertificateCreateMerchantID("APPLE_PAY", "") },
			deps:    func(d *Deps) (any, error) { return nil, d.ValidateCertificateCreateMerchantID("APPLE_PAY", "") },
		},
		{
			name: "ReviewScreenshotUsageError",
			process: func() (any, error) {
				return nil, DefaultDeps().ReviewScreenshotUsageError("--file", "review screenshot is empty")
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.ReviewScreenshotUsageError("--file", "review screenshot is empty")
			},
		},
		{
			name: "PreflightReviewScreenshot",
			process: func() (any, error) {
				return nil, DefaultDeps().PreflightReviewScreenshot("review.png", screenshotFile, screenshotFile.Size())
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.PreflightReviewScreenshot("review.png", screenshotFile, screenshotFile.Size())
			},
		},
		{
			name: "BindTargetSubscriptionPlanTypeFlag",
			process: func() (any, error) {
				var resolve func() (asc.SubscriptionPlanType, error)
				parsed(func(fs *flag.FlagSet) { resolve = DefaultDeps().BindTargetSubscriptionPlanTypeFlag(fs) }, "--target-subscription-plan-type", "WEEKLY")
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
				return DefaultDeps().NormalizeBuildProcessingStateFilter("VALID,ALL", BuildProcessingStateFilterOptions{})
			},
			deps: func(d *Deps) (any, error) {
				return d.NormalizeBuildProcessingStateFilter("VALID,ALL", BuildProcessingStateFilterOptions{})
			},
		},
		{
			name: "NormalizeLatestBuildSelectionOptions",
			process: func() (any, error) {
				return DefaultDeps().NormalizeLatestBuildSelectionOptions("app-1", "1.0", "", "BOGUS", false)
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
				return DefaultDeps().PPPTerritorySelection(value)
			},
			deps: func(d *Deps) (any, error) {
				var value *OnceCSVValue
				parsed(func(fs *flag.FlagSet) { value = BindOnceCSVFlag(fs, "territory", "") }, "--territory", ",")
				return d.PPPTerritorySelection(value)
			},
		},
		{
			name:    "ParsePriceRounding",
			process: func() (any, error) { return DefaultDeps().ParsePriceRounding("sideways") },
			deps:    func(d *Deps) (any, error) { return d.ParsePriceRounding("sideways") },
		},
		{
			name: "RecoverBoolFlagTailArgs",
			process: func() (any, error) {
				return nil, DefaultDeps().RecoverBoolFlagTailArgs(flag.NewFlagSet("create", flag.ContinueOnError), []string{"stray"}, nil)
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.RecoverBoolFlagTailArgs(flag.NewFlagSet("create", flag.ContinueOnError), []string{"stray"}, nil)
			},
		},
		{
			name: "RequireAppForStableSelector",
			process: func() (any, error) {
				return nil, DefaultDeps().RequireAppForStableSelector("", "Monthly Pro", "--subscription")
			},
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
				return DefaultDeps().ResolveAndAnnounceDefaultAppStoreVersion(context.Background(), versionsClient, "app-1", "", "--version")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAndAnnounceDefaultAppStoreVersion(context.Background(), versionsClient, "app-1", "", "--version")
			},
		},
		{
			name: "PreflightBuildBetaGroupAssignment",
			process: func() (any, error) {
				return nil, DefaultDeps().PreflightBuildBetaGroupAssignment(context.Background(), emptyBuildPreflightClient{}, "build-1", betaGroupPlan, BuildBetaGroupPreflightOptions{OperationName: "builds add-groups"})
			},
			deps: func(d *Deps) (any, error) {
				return nil, d.PreflightBuildBetaGroupAssignment(context.Background(), emptyBuildPreflightClient{}, "build-1", betaGroupPlan, BuildBetaGroupPreflightOptions{OperationName: "builds add-groups"})
			},
		},
		{
			name: "ResolveAppInfoID",
			process: func() (any, error) {
				return DefaultDeps().ResolveAppInfoID(context.Background(), appInfosClient, "123456789", "")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAppInfoID(context.Background(), appInfosClient, "123456789", "")
			},
		},
		{
			name: "ResolveAppInfoIDWithFlag",
			process: func() (any, error) {
				return DefaultDeps().ResolveAppInfoIDWithFlag(context.Background(), appInfosClient, "123456789", "", "--app-info-id")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveAppInfoIDWithFlag(context.Background(), appInfosClient, "123456789", "", "--app-info-id")
			},
		},
		{
			name: "ResolveOwnedAppInfoID",
			process: func() (any, error) {
				return DefaultDeps().ResolveOwnedAppInfoID(context.Background(), appInfosClient, "123456789", "")
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveOwnedAppInfoID(context.Background(), appInfosClient, "123456789", "")
			},
		},
		{
			name: "PlanAppInfoLocalizationUpsert",
			process: func() (any, error) {
				return DefaultDeps().PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "123456789", "", "en-US", map[string]string{"name": "Outslept"})
			},
			deps: func(d *Deps) (any, error) {
				return d.PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "123456789", "", "en-US", map[string]string{"name": "Outslept"})
			},
		},
		{
			name: "PlanAppInfoLocalizationUpsert without a name to create",
			process: func() (any, error) {
				return DefaultDeps().PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "123456789", "", "en-US", map[string]string{"subtitle": "Sleep better"})
			},
			deps: func(d *Deps) (any, error) {
				return d.PlanAppInfoLocalizationUpsert(context.Background(), appInfosClient, "123456789", "", "en-US", map[string]string{"subtitle": "Sleep better"})
			},
		},
		{
			name:    "RejectOutputFormatAsPath",
			process: func() (any, error) { return nil, DefaultDeps().RejectOutputFormatAsPath("json", "--output-format") },
			deps:    func(d *Deps) (any, error) { return nil, d.RejectOutputFormatAsPath("json", "--output-format") },
		},
		{
			name:    "RejectOutputFormatAsPath without a format flag",
			process: func() (any, error) { return nil, DefaultDeps().RejectOutputFormatAsPath("table", "") },
			deps:    func(d *Deps) (any, error) { return nil, d.RejectOutputFormatAsPath("table", "") },
		},
		{
			name: "FindPreReleaseVersionIDs",
			process: func() (any, error) {
				return DefaultDeps().FindPreReleaseVersionIDs(context.Background(), buildsClient(), "app-1", "1.2.0", "IOS")
			},
			deps: func(d *Deps) (any, error) {
				return d.FindPreReleaseVersionIDs(context.Background(), buildsClient(), "app-1", "1.2.0", "IOS")
			},
		},
		{
			name: "FindPreReleaseVersionIDsWithMaxPages",
			process: func() (any, error) {
				return DefaultDeps().FindPreReleaseVersionIDsWithMaxPages(context.Background(), buildsClient(), "app-1", "1.2.0", "IOS", 1)
			},
			deps: func(d *Deps) (any, error) {
				return d.FindPreReleaseVersionIDsWithMaxPages(context.Background(), buildsClient(), "app-1", "1.2.0", "IOS", 1)
			},
		},
		{
			name: "ResolveLatestBuild",
			process: func() (any, error) {
				return DefaultDeps().ResolveLatestBuild(context.Background(), buildsClient(), latestBuild, true)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveLatestBuild(context.Background(), buildsClient(), latestBuild, true)
			},
		},
		{
			name: "ResolveNextBuildNumber",
			process: func() (any, error) {
				return DefaultDeps().ResolveNextBuildNumber(context.Background(), buildsClient(), nextBuild)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveNextBuildNumber(context.Background(), buildsClient(), nextBuild)
			},
		},
		{
			name: "ResolveLatestBuild without an app",
			process: func() (any, error) {
				return DefaultDeps().ResolveLatestBuild(context.Background(), buildsClient(), LatestBuildSelectionOptions{}, true)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveLatestBuild(context.Background(), buildsClient(), LatestBuildSelectionOptions{}, true)
			},
		},
		{
			name: "ResolveNextBuildNumber with an invalid initial number",
			process: func() (any, error) {
				return DefaultDeps().ResolveNextBuildNumber(context.Background(), buildsClient(), NextBuildNumberOptions{LatestBuildSelectionOptions: latestBuild})
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveNextBuildNumber(context.Background(), buildsClient(), NextBuildNumberOptions{LatestBuildSelectionOptions: latestBuild})
			},
		},
		{
			name: "WaitForBuildByNumberOrUploadFailure",
			process: func() (any, error) {
				return DefaultDeps().WaitForBuildByNumberOrUploadFailure(context.Background(), buildsClient(), "app-1", "", "1.2.0", "abc", "IOS", time.Millisecond)
			},
			deps: func(d *Deps) (any, error) {
				return d.WaitForBuildByNumberOrUploadFailure(context.Background(), buildsClient(), "app-1", "", "1.2.0", "abc", "IOS", time.Millisecond)
			},
		},
		{
			name: "ResolveTiers",
			process: func() (any, error) {
				return DefaultDeps().ResolveTiers(context.Background(), tiersClient(), "app-1", "USA", true)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveTiers(context.Background(), tiersClient(), "app-1", "USA", true)
			},
		},
		{
			name: "ResolveSubscriptionTiers",
			process: func() (any, error) {
				return DefaultDeps().ResolveSubscriptionTiers(context.Background(), tiersClient(), "sub-1", "USA", true)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveSubscriptionTiers(context.Background(), tiersClient(), "sub-1", "USA", true)
			},
		},
		{
			name: "ResolveIAPTiers",
			process: func() (any, error) {
				return DefaultDeps().ResolveIAPTiers(context.Background(), tiersClient(), "iap-1", "USA", true)
			},
			deps: func(d *Deps) (any, error) {
				return d.ResolveIAPTiers(context.Background(), tiersClient(), "iap-1", "USA", true)
			},
		},
		{
			name: "UploadVersionLocalizations",
			process: func() (any, error) {
				return DefaultDeps().UploadVersionLocalizations(context.Background(), whatsNewClient(), "version-1", whatsNew, false)
			},
			deps: func(d *Deps) (any, error) {
				return d.UploadVersionLocalizations(context.Background(), whatsNewClient(), "version-1", whatsNew, false)
			},
		},
		{
			name: "UploadVersionLocalizationsWithWarnings",
			process: func() (any, error) {
				results, warnings, err := DefaultDeps().UploadVersionLocalizationsWithWarnings(context.Background(), whatsNewClient(), "version-1", whatsNew, false, SubmitReadinessOptions{})
				return []any{results, warnings}, err
			},
			deps: func(d *Deps) (any, error) {
				results, warnings, err := d.UploadVersionLocalizationsWithWarnings(context.Background(), whatsNewClient(), "version-1", whatsNew, false, SubmitReadinessOptions{})
				return []any{results, warnings}, err
			},
		},
		{
			name: "UploadPrevalidatedVersionLocalizationsWithWarnings",
			process: func() (any, error) {
				results, warnings, err := uploadPrevalidatedVersionLocalizationsWithWarningsTo(context.Background(), os.Stderr, whatsNewClient(), "version-1", whatsNew, false, SubmitReadinessOptions{})
				return []any{results, warnings}, err
			},
			deps: func(d *Deps) (any, error) {
				results, warnings, err := d.UploadPrevalidatedVersionLocalizationsWithWarnings(context.Background(), whatsNewClient(), "version-1", whatsNew, false, SubmitReadinessOptions{})
				return []any{results, warnings}, err
			},
		},
		{
			name: "ApplyTerritoryAvailabilityUpdate",
			process: func() (any, error) {
				resp, changed, err := DefaultDeps().ApplyTerritoryAvailabilityUpdate(context.Background(), buildsClient(), availabilityUpdate)
				return []any{resp, changed}, err
			},
			deps: func(d *Deps) (any, error) {
				resp, changed, err := d.ApplyTerritoryAvailabilityUpdate(context.Background(), buildsClient(), availabilityUpdate)
				return []any{resp, changed}, err
			},
		},
		{
			name: "ApplyTerritoryAvailabilityUpdate with a territory to update",
			process: func() (any, error) {
				resp, changed, err := DefaultDeps().ApplyTerritoryAvailabilityUpdate(context.Background(), availabilityClient(), availabilityUpdate)
				return []any{resp, changed}, err
			},
			deps: func(d *Deps) (any, error) {
				resp, changed, err := d.ApplyTerritoryAvailabilityUpdate(context.Background(), availabilityClient(), availabilityUpdate)
				return []any{resp, changed}, err
			},
		},
		{
			name: "NewAvailabilitySetCommand",
			process: func() (any, error) {
				return nil, runBuilder(NewAvailabilitySetCommand(availabilitySet), "--app", "app-1", "--available", "true")
			},
			deps: func(d *Deps) (any, error) {
				config := availabilitySet
				config.Deps = d
				return nil, runBuilder(NewAvailabilitySetCommand(config), "--app", "app-1", "--available", "true")
			},
		},
		{
			name: "NewAvailabilityRemoveFromSaleCommand",
			process: func() (any, error) {
				return nil, runBuilder(NewAvailabilityRemoveFromSaleCommand(AvailabilityRemoveFromSaleCommandConfig{Deps: DefaultDeps()}), "--app", "app-1")
			},
			deps: func(d *Deps) (any, error) {
				return nil, runBuilder(NewAvailabilityRemoveFromSaleCommand(AvailabilityRemoveFromSaleCommandConfig{Deps: d}), "--app", "app-1")
			},
		},
		{
			name:    "NewPricingSetCommand",
			process: func() (any, error) { return nil, runBuilder(NewPricingSetCommand(pricingSet), "--app", "app-1") },
			deps: func(d *Deps) (any, error) {
				config := pricingSet
				d.Now = time.Now
				config.Deps = d
				return nil, runBuilder(NewPricingSetCommand(config), "--app", "app-1")
			},
		},
		{
			name:    "NewPriceSchedulePPPCommand",
			process: func() (any, error) { return nil, runBuilder(NewPriceSchedulePPPCommand(ppp)) },
			deps: func(d *Deps) (any, error) {
				config := ppp
				config.Deps = d
				return nil, runBuilder(NewPriceSchedulePPPCommand(config))
			},
		},
		{
			name: "NewPriceSchedulePPPCommand progress",
			process: func() (any, error) {
				return nil, runBuilder(NewPriceSchedulePPPCommand(pppProgress), "--base-price", "1", "--confirm")
			},
			deps: func(d *Deps) (any, error) {
				config := pppProgress
				usedDepsClient := false
				d.Client = func() (*asc.Client, error) {
					usedDepsClient = true
					return buildsClient(), nil
				}
				d.Now = time.Now
				config.Deps = d
				err := runBuilder(NewPriceSchedulePPPCommand(config), "--base-price", "1", "--confirm")
				if !usedDepsClient {
					return nil, errors.New("the builder did not take its client from d")
				}
				return nil, err
			},
		},
		{
			name: "WarnDeprecatedAppInfoFields",
			process: func() (any, error) {
				DefaultDeps().WarnDeprecatedAppInfoFields([]string{"kidsAgeBand"}, "")
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
			d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr}, LookupEnv: os.LookupEnv}
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

func TestDepsOutputHelpersMatchProcessVariantsOnDepsStdout(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")

	var page asc.ActorsResponse
	if err := json.Unmarshal([]byte(`{
		"data":[{"type":"actors","id":"actor-1","attributes":{"actorType":"USER","userEmail":"a@example.com"}}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/actors","next":"https://api.appstoreconnect.apple.com/v1/actors?cursor=abc"}
	}`), &page); err != nil {
		t.Fatalf("unmarshal page: %v", err)
	}
	var planAvailabilities asc.SubscriptionPlanAvailabilitiesResponse
	if err := json.Unmarshal([]byte(`{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}}]}`), &planAvailabilities); err != nil {
		t.Fatalf("unmarshal plan availabilities: %v", err)
	}
	headers := []string{"ID", "Name"}
	rows := [][]string{{"actor-1", "Ada | Lovelace"}, {"actor-2", "Grace"}}
	manual := true
	prices := &ResolvedPricesResult{Prices: []ResolvedPriceRow{
		{Territory: "USA", PriceID: "price-2", PricePointID: "point-2", CustomerPrice: "0.99", Currency: "USD", Manual: &manual},
		{Territory: "FRA", PriceID: "price-1", PricePointID: "point-1"},
	}}
	upload := &asc.LocalizationUploadResult{Type: "version", InputPath: "./metadata", Total: 2, Succeeded: 1, Failed: 1, Results: []asc.LocalizationUploadLocaleResult{
		{Locale: "en-US", Action: "update", Status: "succeeded", LocalizationID: "loc-1"},
		{Locale: "fr-FR", Action: "create", Status: "failed", Error: "conflict"},
	}}
	published := &asc.AppsPublishedReport{
		AuditedAppCount:   2,
		PublishedAppCount: 1,
		Apps:              []asc.PublishedApp{{ID: "app-1", Name: "Healthy", BundleID: "com.example.healthy", SKU: "HEALTHY", AvailabilityID: "availability-1", PublishedTerritoryCount: 2}},
		Failures:          []asc.PublishedAppFailure{{ID: "app-2", Name: "Broken", Error: "request failed"}},
	}

	type outputFormat struct {
		format string
		pretty bool
	}
	jsonOnly := []outputFormat{{format: "json"}}
	human := []outputFormat{{format: "table"}, {format: "markdown"}}
	all := []outputFormat{{format: "json"}, {format: "json", pretty: true}, {format: "table"}, {format: "markdown"}}
	boldTitle := func(title string) func(string) string {
		return func(stdout string) string {
			return strings.Replace(stdout, title+"\n", bold+title+reset+"\n", 1)
		}
	}

	tests := []struct {
		name      string
		formats   []outputFormat
		stderrTTY bool
		process   func(format string, pretty bool) error
		deps      func(d *Deps, format string, pretty bool) error
		// wantStdout derives the deps stdout from the process stdout when
		// they differ by design; nil means identical.
		wantStdout func(processStdout string) string
	}{
		{
			name:    "PrintOutputWithRenderers",
			formats: all,
			process: func(format string, pretty bool) error {
				return DefaultDeps().PrintOutputWithRenderers(&page, format, pretty,
					func() error { return asc.WriteTable(headers, rows) },
					func() error { return asc.WriteMarkdown(headers, rows) })
			},
			deps: func(d *Deps, format string, pretty bool) error {
				return d.PrintOutputWithRenderers(&page, format, pretty,
					func() error { return asc.WriteTableTo(d.IO.Out, headers, rows) },
					func() error { return asc.WriteMarkdownTo(d.IO.Out, headers, rows) })
			},
		},
		{
			name:    "PrintOutputWithRenderers with RenderTable and RenderMarkdown",
			formats: human,
			process: func(format string, pretty bool) error {
				return DefaultDeps().PrintOutputWithRenderers(&page, format, pretty,
					func() error { asc.RenderTable(headers, rows); return nil },
					func() error { asc.RenderMarkdown(headers, rows); return nil })
			},
			deps: func(d *Deps, format string, pretty bool) error {
				return d.PrintOutputWithRenderers(&page, format, pretty,
					func() error { asc.RenderTableTo(d.IO.Out, headers, rows); return nil },
					func() error { asc.RenderMarkdownTo(d.IO.Out, headers, rows); return nil })
			},
		},
		{
			name:    "PrintOutputWithRenderers with the plan availability renderers",
			formats: human,
			process: func(format string, pretty bool) error {
				return DefaultDeps().PrintOutputWithRenderers(&planAvailabilities, format, pretty,
					func() error { return asc.PrintSubscriptionPlanAvailabilityShowTable(&planAvailabilities) },
					func() error { return asc.PrintSubscriptionPlanAvailabilityShowMarkdown(&planAvailabilities) })
			},
			deps: func(d *Deps, format string, pretty bool) error {
				return d.PrintOutputWithRenderers(&planAvailabilities, format, pretty,
					func() error { return asc.PrintSubscriptionPlanAvailabilityShowTableTo(d.IO.Out, &planAvailabilities) },
					func() error {
						return asc.PrintSubscriptionPlanAvailabilityShowMarkdownTo(d.IO.Out, &planAvailabilities)
					})
			},
		},
		{
			name:    "PrintOutputRows",
			formats: all,
			process: func(format string, pretty bool) error {
				return DefaultDeps().PrintOutputRows(&page, format, pretty, headers, rows)
			},
			deps: func(d *Deps, format string, pretty bool) error {
				return d.PrintOutputRows(&page, format, pretty, headers, rows)
			},
		},
		{
			name:    "RenderSection",
			formats: human,
			process: func(format string, _ bool) error {
				DefaultDeps().RenderSection("Summary", headers, rows, format == "markdown")
				return nil
			},
			deps: func(d *Deps, format string, _ bool) error {
				d.RenderSection("Summary", headers, rows, format == "markdown")
				return nil
			},
		},
		{
			name:      "RenderSection on a terminal stderr",
			formats:   human,
			stderrTTY: true,
			process: func(format string, _ bool) error {
				DefaultDeps().RenderSection("Summary", headers, rows, format == "markdown")
				return nil
			},
			deps: func(d *Deps, format string, _ bool) error {
				d.RenderSection("Summary", headers, rows, format == "markdown")
				return nil
			},
			wantStdout: boldTitle("SUMMARY"),
		},
		{
			name:    "PrintStreamPage",
			formats: jsonOnly,
			process: func(string, bool) error { return DefaultDeps().PrintStreamPage(&page) },
			deps:    func(d *Deps, _ string, _ bool) error { return d.PrintStreamPage(&page) },
		},
		{
			name:    "PrintResolvedPrices",
			formats: all,
			process: func(format string, pretty bool) error {
				return DefaultDeps().PrintResolvedPrices(prices, format, pretty)
			},
			deps: func(d *Deps, format string, pretty bool) error { return d.PrintResolvedPrices(prices, format, pretty) },
		},
		{
			name:    "RenderLocalizationUploadResult",
			formats: human,
			process: func(format string, _ bool) error {
				return DefaultDeps().RenderLocalizationUploadResult(upload, format == "markdown")
			},
			deps: func(d *Deps, format string, _ bool) error {
				return d.RenderLocalizationUploadResult(upload, format == "markdown")
			},
		},
		{
			name:    "PrintOutput of the published apps report",
			formats: all,
			process: func(format string, pretty bool) error { return DefaultDeps().PrintOutput(published, format, pretty) },
			deps:    func(d *Deps, format string, pretty bool) error { return d.PrintOutput(published, format, pretty) },
		},
	}

	for _, test := range tests {
		for _, f := range test.formats {
			name := test.name + "/" + f.format
			if f.pretty {
				name += " pretty"
			}
			t.Run(name, func(t *testing.T) {
				var wantErr error
				wantStdout, wantStderr := captureOutput(t, func() { wantErr = test.process(f.format, f.pretty) })
				if test.wantStdout != nil {
					wantStdout = test.wantStdout(wantStdout)
				}

				var stdout, stderr bytes.Buffer
				d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr, StderrTTY: test.stderrTTY}}
				var gotErr error
				processStdout, processStderr := captureOutput(t, func() { gotErr = test.deps(d, f.format, f.pretty) })

				if processStdout != "" || processStderr != "" {
					t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
				}
				if wantStdout == "" || stdout.String() != wantStdout {
					t.Fatalf("deps stdout = %q, want %q", stdout.String(), wantStdout)
				}
				if stderr.String() != wantStderr {
					t.Fatalf("deps stderr = %q, want %q", stderr.String(), wantStderr)
				}
				if !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("error = %#v, want %#v", gotErr, wantErr)
				}
			})
		}
	}
}

func TestDepsTerminalHelpersMatchProcessVariantsOnDepsStreams(t *testing.T) {
	resetSpinnerTestState(t)
	t.Setenv(spinnerDisabledEnvVar, "0")

	// A spinner draws its first frame when it starts and clears the line when
	// fn returns, so an fn that returns at once still shows where it draws.
	work := func() error { return nil }
	fetch := func(context.Context) (asc.PaginatedResponse, error) {
		return &testPaginatedResponse{Data: []map[string]string{{"id": "item-1"}}}, nil
	}
	next := func(context.Context, string) (asc.PaginatedResponse, error) {
		return nil, errors.New("unexpected next page")
	}
	t.Cleanup(SetASCClientFactoryForTesting(func() (*asc.Client, error) { return nil, nil }))
	// paginatedList runs a builder list command with --paginate on d.
	paginatedList := func(d *Deps) error {
		cmd := BuildPaginatedListCommand(PaginatedListCommandConfig{
			Name:       "list",
			ShortUsage: "asc items list [flags]",
			TopLevel:   true,
			Deps:       d,
			FetchPage: func(ctx context.Context, _ *asc.Client, _ string, _ int, _ string) (asc.PaginatedResponse, error) {
				return fetch(ctx)
			},
		})
		if err := cmd.FlagSet.Parse([]string{"--paginate", "--output", "json"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}
		return cmd.Exec(context.Background(), nil)
	}

	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadPath, []byte(`{"data":{"type":"apps"}}`), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	text := func(s string) *string { return &s }
	// stubStdin makes the process stdin a pipe holding payload, a terminal
	// when tty is true.
	stubStdin := func(t *testing.T, payload string, tty bool) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		if _, err := w.WriteString(payload); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
		_ = w.Close()
		previousStdin, previousIsTerminal := os.Stdin, isTerminal
		stdinFD := int(r.Fd())
		os.Stdin = r
		isTerminal = func(fd int) bool {
			if fd == stdinFD {
				return tty
			}
			return previousIsTerminal(fd)
		}
		t.Cleanup(func() {
			isTerminal = previousIsTerminal
			os.Stdin = previousStdin
			_ = r.Close()
		})
	}

	tests := []struct {
		name string
		// spinner rows draw on stderr only when stdout and stderr are both
		// terminals.
		spinner bool
		// stdin rows run with this on stdin, as a pipe and as a terminal.
		stdin   *string
		process func() (any, error)
		deps    func(*Deps) (any, error)
	}{
		{
			name:    "ReadJSONFilePayload from stdin",
			stdin:   text(`{"data":{"type":"apps"}}`),
			process: func() (any, error) { return DefaultDeps().ReadJSONFilePayload("-") },
			deps:    func(d *Deps) (any, error) { return d.ReadJSONFilePayload("-") },
		},
		{
			name:    "ReadJSONFilePayloadKind from stdin with the wrong shape",
			stdin:   text(`{"data":{"type":"apps"}}`),
			process: func() (any, error) { return DefaultDeps().ReadJSONFilePayloadKind("-", JSONPayloadArray) },
			deps:    func(d *Deps) (any, error) { return d.ReadJSONFilePayloadKind("-", JSONPayloadArray) },
		},
		{
			name:    "ReadJSONFilePayloadKind from a file",
			stdin:   text(""),
			process: func() (any, error) { return DefaultDeps().ReadJSONFilePayloadKind(payloadPath, JSONPayloadAny) },
			deps:    func(d *Deps) (any, error) { return d.ReadJSONFilePayloadKind(payloadPath, JSONPayloadAny) },
		},
		{
			name:    "ProgressEnabled",
			process: func() (any, error) { return DefaultDeps().ProgressEnabled(), nil },
			deps:    func(d *Deps) (any, error) { return d.ProgressEnabled(), nil },
		},
		{
			name:    "SpinnerEnabled",
			process: func() (any, error) { return SpinnerEnabled(), nil },
			deps:    func(d *Deps) (any, error) { return d.SpinnerEnabled(), nil },
		},
		{
			name:    "WithSpinner",
			spinner: true,
			process: func() (any, error) { return nil, DefaultDeps().WithSpinner("Working", work) },
			deps:    func(d *Deps) (any, error) { return nil, d.WithSpinner("Working", work) },
		},
		{
			name:    "WithSpinnerDelayed when fn returns before the delay",
			process: func() (any, error) { return nil, WithSpinnerDelayed("Working", time.Second, work) },
			deps:    func(d *Deps) (any, error) { return nil, d.WithSpinnerDelayed("Working", time.Second, work) },
		},
		{
			name:    "WithSpinnerDelayed without a delay",
			spinner: true,
			process: func() (any, error) { return nil, WithSpinnerDelayed("Working", 0, work) },
			deps:    func(d *Deps) (any, error) { return nil, d.WithSpinnerDelayed("Working", 0, work) },
		},
		{
			name:    "PaginateWithSpinner",
			spinner: true,
			process: func() (any, error) { return DefaultDeps().PaginateWithSpinner(context.Background(), fetch, next) },
			deps:    func(d *Deps) (any, error) { return d.PaginateWithSpinner(context.Background(), fetch, next) },
		},
		{
			name:    "BuildPaginatedListCommand with --paginate",
			spinner: true,
			process: func() (any, error) { return nil, paginatedList(DefaultDeps()) },
			deps:    func(d *Deps) (any, error) { return nil, paginatedList(d) },
		},
	}

	for _, test := range tests {
		stdinTTYs := []bool{false}
		if test.stdin != nil {
			stdinTTYs = []bool{false, true}
		}
		for _, tty := range []struct{ stdout, stderr bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
			for _, stdinTTY := range stdinTTYs {
				name := fmt.Sprintf("%s/stdout_tty=%t/stderr_tty=%t", test.name, tty.stdout, tty.stderr)
				if test.stdin != nil {
					name += fmt.Sprintf("/stdin_tty=%t", stdinTTY)
				}
				t.Run(name, func(t *testing.T) {
					var wantValue, gotValue any
					var wantErr, gotErr error
					stdin := ""
					if test.stdin != nil {
						stdin = *test.stdin
					}
					wantStdout, wantStderr := captureOutput(t, func() {
						withTTYStub(t, tty.stdout, tty.stderr)
						stubStdin(t, stdin, stdinTTY)
						synctest.Test(t, func(*testing.T) { wantValue, wantErr = test.process() })
					})

					var stdout, stderr bytes.Buffer
					d := &Deps{
						Client: func() (*asc.Client, error) { return nil, nil },
						IO: &IO{
							In: strings.NewReader(stdin), Out: &stdout, ErrOut: &stderr,
							StdinTTY: stdinTTY, StdoutTTY: tty.stdout, StderrTTY: tty.stderr,
						},
						Timeout: func(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
						After:   time.After,
					}
					processStdout, processStderr := captureOutput(t, func() {
						withTTYStub(t, false, false)
						stubStdin(t, "", false)
						synctest.Test(t, func(*testing.T) { gotValue, gotErr = test.deps(d) })
					})

					if processStdout != "" || processStderr != "" {
						t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
					}
					if stdout.String() != wantStdout {
						t.Fatalf("deps stdout = %q, want %q", stdout.String(), wantStdout)
					}
					if drawn := test.spinner && tty.stdout && tty.stderr; drawn != (wantStderr != "") {
						t.Fatalf("process stderr = %q, want a spinner: %t", wantStderr, drawn)
					}
					if stderr.String() != wantStderr {
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
	}
}

func TestDefaultDepsStdinMatchesProcessStdin(t *testing.T) {
	const payload = `{"data":{"type":"apps"}}`
	// useStdin makes the process stdin a pipe holding payload, a terminal
	// when tty is true.
	useStdin := func(t *testing.T, tty bool) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		if _, err := w.WriteString(payload); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
		_ = w.Close()
		previousStdin, previousIsTerminal := os.Stdin, isTerminal
		stdinFD := int(r.Fd())
		os.Stdin = r
		isTerminal = func(fd int) bool {
			if fd == stdinFD {
				return tty
			}
			return previousIsTerminal(fd)
		}
		t.Cleanup(func() {
			isTerminal = previousIsTerminal
			os.Stdin = previousStdin
			_ = r.Close()
		})
	}

	// built is the stdin when DefaultDeps is called and read the stdin when
	// the payload is read: a caller may replace os.Stdin in between.
	for _, tty := range []struct{ built, read bool }{{false, false}, {true, true}, {true, false}, {false, true}} {
		t.Run(fmt.Sprintf("built_tty=%t/read_tty=%t", tty.built, tty.read), func(t *testing.T) {
			useStdin(t, tty.read)
			want, wantErr := DefaultDeps().ReadJSONFilePayloadKind("-", JSONPayloadAny)

			useStdin(t, tty.built)
			d := DefaultDeps()
			if d.IO.StdinTTY != tty.built {
				t.Fatalf("DefaultDeps().IO.StdinTTY = %t, want %t", d.IO.StdinTTY, tty.built)
			}
			useStdin(t, tty.read)
			got, gotErr := d.ReadJSONFilePayloadKind("-", JSONPayloadAny)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("deps read = %q, %v; want %q, %v", got, gotErr, want, wantErr)
			}
		})
	}
}

func TestDefaultDepsTimeoutsMatchProcessVariants(t *testing.T) {
	timeoutEnv := []string{"ASC_TIMEOUT", "ASC_TIMEOUT_SECONDS", "ASC_UPLOAD_TIMEOUT", "ASC_UPLOAD_TIMEOUT_SECONDS"}
	const commandDefault, mediaBytes = 2 * time.Minute, 3 << 20
	ifSet := func(set bool, setting, fallback time.Duration) time.Duration {
		if set {
			return setting
		}
		return fallback
	}

	// requestSet and uploadSet say whether the setting is given with a valid
	// value, in which case it replaces a command's own default too.
	tests := []struct {
		name        string
		env         map[string]string
		config      string
		wantRequest time.Duration
		wantUpload  time.Duration
		requestSet  bool
		uploadSet   bool
	}{
		{name: "unset", wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout},
		{name: "ASC_TIMEOUT", env: map[string]string{"ASC_TIMEOUT": "7s"}, wantRequest: 7 * time.Second, wantUpload: asc.DefaultUploadTimeout, requestSet: true},
		{name: "ASC_TIMEOUT_SECONDS", env: map[string]string{"ASC_TIMEOUT_SECONDS": "9"}, wantRequest: 9 * time.Second, wantUpload: asc.DefaultUploadTimeout, requestSet: true},
		{name: "ASC_TIMEOUT equal to the default", env: map[string]string{"ASC_TIMEOUT": asc.DefaultTimeout.String()}, wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout, requestSet: true},
		{name: "invalid ASC_TIMEOUT keeps the default", env: map[string]string{"ASC_TIMEOUT": "soon"}, wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout},
		{name: "ASC_UPLOAD_TIMEOUT", env: map[string]string{"ASC_UPLOAD_TIMEOUT": "90s"}, wantRequest: asc.DefaultTimeout, wantUpload: 90 * time.Second, uploadSet: true},
		{name: "ASC_UPLOAD_TIMEOUT_SECONDS", env: map[string]string{"ASC_UPLOAD_TIMEOUT_SECONDS": "45"}, wantRequest: asc.DefaultTimeout, wantUpload: 45 * time.Second, uploadSet: true},
		{name: "ASC_UPLOAD_TIMEOUT equal to the default", env: map[string]string{"ASC_UPLOAD_TIMEOUT": asc.DefaultUploadTimeout.String()}, wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout, uploadSet: true},
		{name: "ASC_UPLOAD_TIMEOUT wins over its seconds form", env: map[string]string{"ASC_UPLOAD_TIMEOUT": "90s", "ASC_UPLOAD_TIMEOUT_SECONDS": "45"}, wantRequest: asc.DefaultTimeout, wantUpload: 90 * time.Second, uploadSet: true},
		{name: "invalid ASC_UPLOAD_TIMEOUT keeps the default", env: map[string]string{"ASC_UPLOAD_TIMEOUT": "soon", "ASC_UPLOAD_TIMEOUT_SECONDS": "45"}, wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout},
		{name: "empty ASC_UPLOAD_TIMEOUT keeps the default", env: map[string]string{"ASC_UPLOAD_TIMEOUT": "", "ASC_UPLOAD_TIMEOUT_SECONDS": "45"}, wantRequest: asc.DefaultTimeout, wantUpload: asc.DefaultUploadTimeout},
		{name: "config timeouts", config: `{"timeout":"11s","upload_timeout":"75s"}`, wantRequest: 11 * time.Second, wantUpload: 75 * time.Second, requestSet: true, uploadSet: true},
		{name: "config seconds timeouts", config: `{"timeout_seconds":"13","upload_timeout_seconds":"50"}`, wantRequest: 13 * time.Second, wantUpload: 50 * time.Second, requestSet: true, uploadSet: true},
		{name: "environment wins over config", env: map[string]string{"ASC_TIMEOUT": "7s", "ASC_UPLOAD_TIMEOUT": "90s"}, config: `{"timeout":"11s","upload_timeout":"75s"}`, wantRequest: 7 * time.Second, wantUpload: 90 * time.Second, requestSet: true, uploadSet: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, name := range timeoutEnv {
				t.Setenv(name, "")
				os.Unsetenv(name)
			}
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			configPath := filepath.Join(t.TempDir(), "config.json")
			if test.config != "" {
				if err := os.WriteFile(configPath, []byte(test.config), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}
			t.Setenv("ASC_CONFIG_PATH", configPath)
			asc.ResetConfigCacheForTest()
			t.Cleanup(asc.ResetConfigCacheForTest)

			d := DefaultDeps()
			pairs := []struct {
				name    string
				process func(context.Context) (context.Context, context.CancelFunc)
				deps    func(context.Context) (context.Context, context.CancelFunc)
				want    time.Duration
			}{
				{"Timeout", ContextWithTimeout, d.Timeout, test.wantRequest},
				{"UploadTimeout", ContextWithUploadTimeout, d.UploadTimeout, test.wantUpload},
				{"DownloadTimeout", ContextWithUploadTimeout, d.DownloadTimeout, test.wantUpload},
				{
					"ResolvedTimeout",
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return ContextWithResolvedTimeout(ctx, commandDefault)
					},
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return d.ResolvedTimeout(ctx, commandDefault)
					},
					ifSet(test.requestSet, test.wantRequest, commandDefault),
				},
				{
					"ResolvedUploadTimeout",
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return withTimeoutContext(ctx, ResolveTimeoutSetting(UploadTimeoutSetting, commandDefault))
					},
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return d.ResolvedUploadTimeout(ctx, commandDefault)
					},
					ifSet(test.uploadSet, test.wantUpload, commandDefault),
				},
				{
					"MediaUploadTimeout",
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return withTimeoutContext(ctx, ResolveTimeoutSetting(UploadTimeoutSetting, MediaUploadTimeoutForWork(1, mediaBytes)))
					},
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return d.MediaUploadTimeout(ctx, mediaBytes)
					},
					ifSet(test.uploadSet, test.wantUpload, MediaUploadTimeoutForWork(1, mediaBytes)),
				},
				{
					"AvailabilityTimeout for all territories",
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return ContextWithResolvedTimeout(ctx, bulkAvailabilityTimeout)
					},
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return d.AvailabilityTimeout(ctx, true)
					},
					ifSet(test.requestSet, test.wantRequest, bulkAvailabilityTimeout),
				},
				{
					"AvailabilityTimeout for some territories",
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return ContextWithTimeout(ctx)
					},
					func(ctx context.Context) (context.Context, context.CancelFunc) {
						return d.AvailabilityTimeout(ctx, false)
					},
					test.wantRequest,
				},
			}
			synctest.Test(t, func(t *testing.T) {
				want := func(d time.Duration) time.Time { return time.Now().Add(d) }
				for _, pair := range pairs {
					processCtx, processCancel := pair.process(context.Background())
					depsCtx, depsCancel := pair.deps(context.Background())
					processDeadline, processOK := processCtx.Deadline()
					depsDeadline, depsOK := depsCtx.Deadline()
					_, liftedOK := ContextWithoutTimeout(depsCtx).Deadline()
					processCancel()
					depsCancel()

					if !processOK || !processDeadline.Equal(want(pair.want)) {
						t.Fatalf("%s: process deadline = %v (set = %v), want %v", pair.name, processDeadline, processOK, want(pair.want))
					}
					if !depsOK || !depsDeadline.Equal(processDeadline) {
						t.Fatalf("%s: deps deadline = %v (set = %v), want the process deadline %v", pair.name, depsDeadline, depsOK, processDeadline)
					}
					if liftedOK {
						t.Fatalf("%s: ContextWithoutTimeout kept the deps deadline", pair.name)
					}
				}
			})
		})
	}

	t.Run("per-call defaults use d.ResolveTimeout", func(t *testing.T) {
		const injected = 4 * time.Minute
		type call struct {
			setting        TimeoutSetting
			defaultTimeout time.Duration
		}
		var calls []call
		d := &Deps{
			ResolveTimeout: func(setting TimeoutSetting, defaultTimeout time.Duration) time.Duration {
				calls = append(calls, call{setting, defaultTimeout})
				return injected
			},
			Timeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return ContextWithTimeoutDuration(ctx, injected)
			},
		}
		synctest.Test(t, func(t *testing.T) {
			for _, build := range []func(context.Context) (context.Context, context.CancelFunc){
				func(ctx context.Context) (context.Context, context.CancelFunc) {
					return d.ResolvedTimeout(ctx, commandDefault)
				},
				func(ctx context.Context) (context.Context, context.CancelFunc) {
					return d.ResolvedUploadTimeout(ctx, commandDefault)
				},
				func(ctx context.Context) (context.Context, context.CancelFunc) {
					return d.MediaUploadTimeout(ctx, mediaBytes)
				},
				func(ctx context.Context) (context.Context, context.CancelFunc) {
					return d.AvailabilityTimeout(ctx, true)
				},
				func(ctx context.Context) (context.Context, context.CancelFunc) {
					return d.AvailabilityTimeout(ctx, false)
				},
			} {
				ctx, cancel := build(context.Background())
				deadline, ok := ctx.Deadline()
				_, liftedOK := ContextWithoutTimeout(ctx).Deadline()
				cancel()
				if !ok || !deadline.Equal(time.Now().Add(injected)) {
					t.Fatalf("deadline = %v (set = %v), want %v", deadline, ok, time.Now().Add(injected))
				}
				if liftedOK {
					t.Fatal("ContextWithoutTimeout kept the deadline")
				}
			}
		})
		want := []call{
			{RequestTimeoutSetting, commandDefault},
			{UploadTimeoutSetting, commandDefault},
			{UploadTimeoutSetting, MediaUploadTimeoutForWork(1, mediaBytes)},
			{RequestTimeoutSetting, bulkAvailabilityTimeout},
		}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("ResolveTimeout calls = %v, want %v", calls, want)
		}
	})

	t.Run("DownloadTimeout uses d.UploadTimeout", func(t *testing.T) {
		const injected = 3 * time.Minute
		d := &Deps{UploadTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return ContextWithTimeoutDuration(ctx, injected)
		}}
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := d.DownloadTimeout(context.Background())
			defer cancel()
			if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(time.Now().Add(injected)) {
				t.Fatalf("deadline = %v (set = %v), want %v", deadline, ok, time.Now().Add(injected))
			}
		})
	})

	t.Run("ClientWithTimeout bounds each request with the given timeout", func(t *testing.T) {
		const given = 7 * time.Minute
		keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
		writeECDSAPEM(t, keyPath)
		for name, value := range map[string]string{
			"ASC_BYPASS_KEYCHAIN": "1", "ASC_KEY_ID": "KEY_ID", "ASC_ISSUER_ID": "ISSUER_ID", "ASC_PRIVATE_KEY_PATH": keyPath,
			"ASC_PRIVATE_KEY": "", "ASC_PRIVATE_KEY_B64": "", "ASC_BEARER_TOKEN": "", "ASC_BEARER_TOKEN_COMMAND": "", "ASC_PROFILE": "",
			"ASC_TIMEOUT": "", "ASC_TIMEOUT_SECONDS": "", "ASC_MAX_RETRIES": "0",
			"ASC_CONFIG_PATH": filepath.Join(t.TempDir(), "missing.json"),
		} {
			t.Setenv(name, value)
		}
		asc.ResetConfigCacheForTest()
		t.Cleanup(asc.ResetConfigCacheForTest)
		original := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = original })
		http.DefaultTransport = unansweredTransport{}
		d := DefaultDeps()

		for _, test := range []struct {
			name   string
			client func() (*asc.Client, error)
			want   time.Duration
		}{
			{name: "process", client: func() (*asc.Client, error) { return GetASCClientWithTimeout(given) }, want: given},
			{name: "DefaultDeps", client: func() (*asc.Client, error) { return d.ClientWithTimeout(given) }, want: given},
			{name: "request-timeout client", client: d.Client, want: asc.DefaultTimeout},
		} {
			synctest.Test(t, func(t *testing.T) {
				client, err := test.client()
				if err != nil {
					t.Fatalf("%s client: %v", test.name, err)
				}
				start := time.Now()
				if _, err := client.GetApps(context.Background()); err == nil {
					t.Fatalf("%s client: request to an unanswered transport succeeded", test.name)
				}
				if elapsed := time.Since(start); elapsed != test.want {
					t.Fatalf("%s client gave up after %v, want %v", test.name, elapsed, test.want)
				}
			})
		}
	})
}

func TestDepsRequestContextHelpersMatchProcessVariants(t *testing.T) {
	for _, name := range []string{"ASC_TIMEOUT_SECONDS", "ASC_MAX_DELAY"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("ASC_TIMEOUT", "7s")
	t.Setenv("ASC_MAX_RETRIES", "2")
	t.Setenv("ASC_BASE_DELAY", "1s")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	const processTimeout, injectedTimeout = 7 * time.Second, 3 * time.Minute
	injected := &Deps{Timeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
		return ContextWithTimeoutDuration(ctx, injectedTimeout)
	}, After: time.After}

	budget := func(ctx context.Context) time.Duration {
		deadline, ok := ctx.Deadline()
		if !ok {
			return -1
		}
		return time.Until(deadline)
	}
	expire := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)
	var reads []time.Duration
	transport := buildUploadsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		reads = append(reads, budget(req.Context()))
		body := `{"data":{"type":"reviewSubmissions","id":"sub-1","attributes":{"state":"COMPLETE"}}}`
		switch req.URL.Path {
		case "/v1/appAssetLibraryPlacements":
			body = `{"data":{"type":"appAssetLibraryPlacements","id":"placement-1","attributes":{"state":"PROCESSING"}}}`
		case "/v1/appAssetLibraryPlacementOrderingRequests":
			body = `{"data":{"type":"appAssetLibraryPlacementOrderingRequests","id":"order-1"}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	// The client is built inside each synctest bubble: it keeps a channel
	// that limits concurrent mutations.
	var client *asc.Client
	var err error

	// Each run uses the process form when d is nil and the Deps form otherwise,
	// and returns the budget of every request context the helper made.
	tests := []struct {
		name     string
		attempts int
		run      func(t *testing.T, d *Deps) []time.Duration
	}{
		{name: "RetryReadWithFreshTimeout", attempts: 3, run: func(t *testing.T, d *Deps) []time.Duration {
			var budgets []time.Duration
			read := func(ctx context.Context) (int, error) {
				budgets = append(budgets, budget(ctx))
				return 0, expire(ctx)
			}
			if d == nil {
				_, err = RetryReadWithFreshTimeout(context.Background(), read)
			} else {
				_, err = RetryReadWithFreshTimeoutWith(context.Background(), d, read)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want the last attempt's deadline", err)
			}
			return budgets
		}},
		{name: "CollectPricePointsByTerritory", attempts: 3, run: func(t *testing.T, d *Deps) []time.Duration {
			var budgets []time.Duration
			fetch := func(ctx context.Context, _ string) (*asc.Response[asc.AppPricePointV3Attributes], error) {
				budgets = append(budgets, budget(ctx))
				return nil, expire(ctx)
			}
			customerPrice := func(attributes asc.AppPricePointV3Attributes) string { return attributes.CustomerPrice }
			if d == nil {
				_, err = CollectPricePointsByTerritoryWith(context.Background(), DefaultDeps(), nil, fetch, customerPrice)
			} else {
				_, err = CollectPricePointsByTerritoryWith(context.Background(), d, nil, fetch, customerPrice)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want the last attempt's deadline", err)
			}
			return budgets
		}},
		{name: "RunReconciledMutation", attempts: 3, run: func(t *testing.T, d *Deps) []time.Duration {
			var budgets []time.Duration
			mutate := func(ctx context.Context) (int, error) {
				budgets = append(budgets, budget(ctx))
				return 0, expire(ctx)
			}
			readback := func(context.Context) (int, bool, error) { return 0, false, nil }
			if d == nil {
				_, _, err = RunReconciledMutation(context.Background(), mutate, readback)
			} else {
				_, _, err = RunReconciledMutationWith(context.Background(), d, mutate, readback)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want the last attempt's deadline", err)
			}
			return budgets
		}},
		{name: "RunReconciledMutationNoReplay", attempts: 1, run: func(t *testing.T, d *Deps) []time.Duration {
			var budgets []time.Duration
			mutate := func(ctx context.Context) (int, error) {
				budgets = append(budgets, budget(ctx))
				return 0, expire(ctx)
			}
			readback := func(context.Context) (int, bool, error) { return 0, false, nil }
			if d == nil {
				_, _, err = RunReconciledMutationNoReplayWith(context.Background(), DefaultDeps(), mutate, readback)
			} else {
				_, _, err = RunReconciledMutationNoReplayWith(context.Background(), d, mutate, readback)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want the mutation's deadline", err)
			}
			return budgets
		}},
		{name: "ExplainReviewSubmissionNotCancellable", attempts: 1, run: func(t *testing.T, d *Deps) []time.Duration {
			reads = nil
			if d == nil {
				err = DefaultDeps().ExplainReviewSubmissionNotCancellable(context.Background(), client, "sub-1", notCancellableAPIError())
			} else {
				err = d.ExplainReviewSubmissionNotCancellable(context.Background(), client, "sub-1", notCancellableAPIError())
			}
			if explained, ok := errors.AsType[*ReviewSubmissionNotCancellableError](err); !ok || explained.State != asc.ReviewSubmissionStateComplete {
				t.Fatalf("error = %v, want the state read back", err)
			}
			return reads
		}},
		{name: "CreateLibraryPlacement", attempts: 1, run: func(t *testing.T, d *Deps) []time.Duration {
			reads = nil
			var id string
			if d == nil {
				id, _, err = DefaultDeps().CreateLibraryPlacement(context.Background(), client, "appStoreVersionLocalizations", "loc-1", "image", "image-1", "APP_SCREENSHOT", "group-1")
			} else {
				id, _, err = d.CreateLibraryPlacement(context.Background(), client, "appStoreVersionLocalizations", "loc-1", "image", "image-1", "APP_SCREENSHOT", "group-1")
			}
			if err != nil || id != "placement-1" {
				t.Fatalf("placement = %q, error = %v", id, err)
			}
			return reads
		}},
		{name: "OrderLibraryPlacements", attempts: 1, run: func(t *testing.T, d *Deps) []time.Duration {
			reads = nil
			var id string
			if d == nil {
				id, err = orderLibraryPlacements(context.Background(), ContextWithTimeout, client, "appStoreVersionLocalizations", "loc-1", "group-1", []string{"placement-1", "placement-2"})
			} else {
				id, err = d.OrderLibraryPlacements(context.Background(), client, "appStoreVersionLocalizations", "loc-1", "group-1", []string{"placement-1", "placement-2"})
			}
			if err != nil || id != "order-1" {
				t.Fatalf("ordering request = %q, error = %v", id, err)
			}
			return reads
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				for _, variant := range []struct {
					name   string
					d      *Deps
					budget time.Duration
				}{
					{"process", nil, processTimeout},
					{"DefaultDeps", DefaultDeps(), processTimeout},
					{"injected Timeout", injected, injectedTimeout},
				} {
					client, err = asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
					if err != nil {
						t.Fatalf("NewClientWithHTTPClient() error: %v", err)
					}
					got := test.run(t, variant.d)
					want := make([]time.Duration, test.attempts)
					for i := range want {
						want[i] = variant.budget
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s: request budgets = %v, want %v", variant.name, got, want)
					}
				}
			})
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

// unansweredTransport never answers: a request ends only when its context
// does, as with a server that accepts and stays silent.
type unansweredTransport struct{}

func (unansweredTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}
