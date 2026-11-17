package validate

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type stubDeepWebClient struct {
	privacy          *webcore.AppDataUsagesPublishState
	privacyErr       error
	subscriptions    []webcore.ReviewSubscription
	subscriptionsErr error
	agreements       *asc.WebAgreementsStatusResult
	agreementsErr    error
}

func (c *stubDeepWebClient) GetAppDataUsagesPublishState(context.Context, string) (*webcore.AppDataUsagesPublishState, error) {
	return c.privacy, c.privacyErr
}

func (c *stubDeepWebClient) ListReviewSubscriptions(context.Context, string) ([]webcore.ReviewSubscription, error) {
	return c.subscriptions, c.subscriptionsErr
}

func (c *stubDeepWebClient) GetAgreementsStatus(context.Context) (*asc.WebAgreementsStatusResult, error) {
	return c.agreements, c.agreementsErr
}

func TestBuildDeepValidationVerifiedBlockersAndDerivedPublicChecks(t *testing.T) {
	base := validation.Report{
		AppID:        "app-1",
		VersionID:    "version-1",
		VersionState: "PREPARE_FOR_SUBMISSION",
		Checks: []validation.CheckResult{
			{ID: "availability.missing", Severity: validation.SeverityError, Remediation: "Configure availability"},
		},
	}
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: false, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{
			ID:                                 "sub-1",
			State:                              "READY_TO_SUBMIT",
			SubmitWithNextAppStoreVersion:      false,
			SubmitWithNextAppStoreVersionKnown: true,
		}, {
			ID:                                 "sub-2",
			State:                              "READY_TO_SUBMIT",
			SubmitWithNextAppStoreVersion:      false,
			SubmitWithNextAppStoreVersionKnown: true,
		}},
		agreements: &asc.WebAgreementsStatusResult{Pending: true, Agreements: []asc.WebAgreement{{
			AgreementID:               "agreement-1",
			Status:                    "active",
			IsProgramLicenseAgreement: true,
			Pending:                   true,
		}}},
	}

	deep, findings := buildDeepValidation(context.Background(), base, client, validation.DeepSessionCached)
	if len(deep.Checks) != 5 {
		t.Fatalf("deep checks = %d, want 5: %#v", len(deep.Checks), deep.Checks)
	}
	for _, id := range []string{
		validation.DeepCheckPrivacyPublishState,
		validation.DeepCheckSubscriptionAttachment,
		validation.DeepCheckAgreementsActive,
		validation.DeepCheckAvailabilityConfigured,
		validation.DeepCheckReviewInformationComplete,
	} {
		if got := deepCheckByID(t, deep, id); got.Status == "" {
			t.Fatalf("deep check %q has empty status", id)
		}
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckPrivacyPublishState); got.Status != validation.DeepStatusBlocked || got.Resolution.Fixability != validation.FixabilityWebFixable {
		t.Fatalf("privacy check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusBlocked || got.Resolution.Fixability != validation.FixabilityManual || len(got.Resolution.Commands) != 1 {
		t.Fatalf("subscription check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive); got.Status != validation.DeepStatusBlocked || got.Resolution.Fixability != validation.FixabilityWebFixable || len(got.Resolution.Commands) != 1 {
		t.Fatalf("agreement check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckAvailabilityConfigured); got.Status != validation.DeepStatusBlocked || got.Source != validation.DeepSourcePublicAPI || got.Resolution.Fixability != validation.FixabilityManual {
		t.Fatalf("availability check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckReviewInformationComplete); got.Status != validation.DeepStatusPassed {
		t.Fatalf("review information check = %#v", got)
	}
	if len(findings) != 3 {
		t.Fatalf("new web findings = %d, want 3: %#v", len(findings), findings)
	}
}

func TestBuildDeepValidationDoesNotTreatMissingPrivateAttributesAsFalse(t *testing.T) {
	base := validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1"},
		subscriptions: []webcore.ReviewSubscription{{
			ID:    "sub-1",
			State: "READY_TO_SUBMIT",
		}},
		agreements: nil,
	}

	deep, findings := buildDeepValidation(context.Background(), base, client, validation.DeepSessionCached)
	for _, id := range []string{
		validation.DeepCheckPrivacyPublishState,
		validation.DeepCheckSubscriptionAttachment,
		validation.DeepCheckAgreementsActive,
	} {
		if got := deepCheckByID(t, deep, id); got.Status != validation.DeepStatusUnverified {
			t.Fatalf("deep check %q = %#v, want unverified", id, got)
		}
	}
	if len(findings) != 3 {
		t.Fatalf("unverified findings = %d, want 3", len(findings))
	}
	for _, finding := range findings {
		if finding.Severity != validation.SeverityWarning {
			t.Fatalf("unverified finding severity = %q, want warning", finding.Severity)
		}
	}
}

func TestBuildDeepValidationKeepsIndependentResultsOnEndpointFailure(t *testing.T) {
	base := validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}
	client := &stubDeepWebClient{
		privacy:          &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptionsErr: errors.New("provider response with sensitive details"),
		agreements: &asc.WebAgreementsStatusResult{Pending: false, Agreements: []asc.WebAgreement{{
			AgreementID:               "agreement-1",
			Status:                    "active",
			IsProgramLicenseAgreement: true,
		}}},
	}

	deep, _ := buildDeepValidation(context.Background(), base, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckPrivacyPublishState); got.Status != validation.DeepStatusPassed {
		t.Fatalf("privacy check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusUnverified || got.Message == "provider response with sensitive details" {
		t.Fatalf("subscription check = %#v", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive); got.Status != validation.DeepStatusPassed {
		t.Fatalf("agreement check = %#v", got)
	}
}

func TestBuildDeepValidationTreatsApprovedSubscriptionAsSubsequentSubmission(t *testing.T) {
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{
			{ID: "approved", State: "APPROVED", SubmitWithNextAppStoreVersionKnown: true},
			{ID: "new", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersionKnown: true},
		},
		agreements: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusNotApplicable {
		t.Fatalf("subscription check = %#v, want subsequent submission notApplicable", got)
	}
	for _, finding := range findings {
		if finding.ID == "subscriptions.first_type_app_version_attachment.missing" {
			t.Fatalf("subsequent subscription produced first-of-type blocker: %#v", findings)
		}
	}
}

func TestBuildDeepValidationProvidesMutationOnlyForOneUnambiguousSubscription(t *testing.T) {
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{
			ID:                                 "sub-1",
			State:                              "READY_TO_SUBMIT",
			SubmitWithNextAppStoreVersionKnown: true,
		}},
		agreements: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, _ := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}, client, validation.DeepSessionCached)
	got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment)
	if got.Resolution == nil || got.Resolution.Fixability != validation.FixabilityWebFixable || len(got.Resolution.Commands) != 1 {
		t.Fatalf("subscription check = %#v, want one web-fixable command", got)
	}
	if want := `asc web review subscriptions attach --app "app-1" --subscription-id "sub-1" --confirm`; got.Resolution.Commands[0] != want {
		t.Fatalf("subscription command = %q, want %q", got.Resolution.Commands[0], want)
	}
}

func TestBuildDeepValidationTreatsUnknownSubscriptionIdentityOrStateAsUnverified(t *testing.T) {
	client := &stubDeepWebClient{
		privacy:       &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{ID: "", State: "", SubmitWithNextAppStoreVersion: true, SubmitWithNextAppStoreVersionKnown: true}},
		agreements:    &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, _ := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusUnverified {
		t.Fatalf("subscription check = %#v, want unverified", got)
	}
}

func TestBuildDeepValidationTreatsAnyIncompleteSubscriptionRowAsUnverified(t *testing.T) {
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{
			{},
			{ID: "sub-1", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersionKnown: true},
		},
		agreements: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, _ := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusUnverified {
		t.Fatalf("subscription check = %#v, want unverified with any incomplete row", got)
	}
}

func TestBuildDeepValidationUsesAgreementStatusAndMonetizationRelevance(t *testing.T) {
	tests := []struct {
		name                  string
		hasActiveMonetization bool
		hasPaidAppPrice       bool
		appPricingKnown       bool
		monetizationUnknown   bool
		status                *asc.WebAgreementsStatusResult
		want                  validation.DeepStatus
		wantCommand           string
		wantFixability        validation.Fixability
	}{
		{
			name:   "expired program agreement blocks",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "Expired", IsProgramLicenseAgreement: true}}},
			want:   validation.DeepStatusBlocked,
		},
		{
			name:   "unknown status is unverified",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "mystery", IsProgramLicenseAgreement: true}}},
			want:   validation.DeepStatusUnverified,
		},
		{
			name:   "active pending user remains in effect",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "paid", Status: "Active (Pending User)", IsProgramLicenseAgreement: true}}},
			want:   validation.DeepStatusPassed,
		},
		{
			name:   "processing is a manual blocker",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "paid", Status: "Processing", IsProgramLicenseAgreement: true}}},
			want:   validation.DeepStatusBlocked,
		},
		{
			name: "superseded expired agreement does not block current active agreement",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "current", Title: "Apple Developer Program License Agreement", Version: "5031", DateEffective: "2026-08-01T00:00:00Z", Status: "Active", IsProgramLicenseAgreement: true},
				{AgreementID: "old", Title: "Apple Developer Program License Agreement", Version: "5030", DateEffective: "2026-01-01T00:00:00Z", Status: "Expired", IsProgramLicenseAgreement: true},
			}},
			want: validation.DeepStatusPassed,
		},
		{
			name: "outstanding agreement without metadata cannot pass a free app",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "current", DateEffective: "2026-08-01T00:00:00Z", Status: "Active", IsProgramLicenseAgreement: true},
				{AgreementID: "outstanding", Pending: true, ProgramLicenseClassificationUnknown: true},
			}},
			appPricingKnown: true,
			want:            validation.DeepStatusUnverified,
		},
		{
			name: "dated outstanding agreement without classification cannot pass a free app",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "current", DateEffective: "2026-08-01T00:00:00Z", Status: "Active", IsProgramLicenseAgreement: true},
				{AgreementID: "outstanding", DateEffective: "2026-10-29T00:00:00Z", Pending: true, ProgramLicenseClassificationUnknown: true},
			}},
			appPricingKnown: true,
			want:            validation.DeepStatusUnverified,
		},
		{
			name: "undated known pending agreement is not hidden by accepted history",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "old", DateEffective: "2026-08-01T00:00:00Z", Status: "Active", IsProgramLicenseAgreement: true},
				{AgreementID: "new", Pending: true, IsProgramLicenseAgreement: true},
			}},
			want:           validation.DeepStatusBlocked,
			wantCommand:    `asc web agreements accept --agreement-id "new" --confirm`,
			wantFixability: validation.FixabilityWebFixable,
		},
		{
			name: "known and unclassified pending agreements require manual review",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "known", Pending: true, IsProgramLicenseAgreement: true},
				{AgreementID: "unknown", Pending: true, ProgramLicenseClassificationUnknown: true},
			}},
			want:           validation.DeepStatusBlocked,
			wantCommand:    `asc web agreements accept --agreement-id "known" --confirm`,
			wantFixability: validation.FixabilityManual,
		},
		{
			name: "new agreement supersedes active agreement",
			status: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{
				{AgreementID: "old", Title: "Apple Developer Program License Agreement", Version: "5030", DateEffective: "2026-01-01T00:00:00Z", Status: "Active", IsProgramLicenseAgreement: true},
				{AgreementID: "new", Title: "Apple Developer Program License Agreement", Version: "5031", DateEffective: "2026-08-01T00:00:00Z", Status: "New", IsProgramLicenseAgreement: true},
			}},
			want: validation.DeepStatusBlocked,
		},
		{
			name:            "paid agreement is irrelevant for free app",
			status:          &asc.WebAgreementsStatusResult{Warnings: []string{"App Store Connect alert has no pending agreement in Developer Portal"}, Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "active", IsProgramLicenseAgreement: true}}, ContractMessages: []asc.WebAgreementContractMessage{{Subject: "Paid Apps Agreement update"}}},
			appPricingKnown: true,
			want:            validation.DeepStatusPassed,
		},
		{
			name:            "paid agreement remains relevant for upfront paid app",
			hasPaidAppPrice: true,
			appPricingKnown: true,
			status:          &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "active", IsProgramLicenseAgreement: true}}, ContractMessages: []asc.WebAgreementContractMessage{{Subject: "Paid Apps Agreement update"}}},
			want:            validation.DeepStatusBlocked,
		},
		{
			name:                "paid agreement relevance is unverified when pricing or monetization is unknown",
			monetizationUnknown: true,
			status:              &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "active", IsProgramLicenseAgreement: true}}, ContractMessages: []asc.WebAgreementContractMessage{{Subject: "Paid Apps Agreement update"}}},
			want:                validation.DeepStatusUnverified,
		},
		{
			name:                  "paid agreement blocks monetized app",
			hasActiveMonetization: true,
			status:                &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{AgreementID: "pla", Status: "active", IsProgramLicenseAgreement: true}}, ContractMessages: []asc.WebAgreementContractMessage{{Subject: "Paid Apps Agreement update"}}},
			want:                  validation.DeepStatusBlocked,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &stubDeepWebClient{
				privacy:       &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
				subscriptions: []webcore.ReviewSubscription{},
				agreements:    test.status,
			}
			report := validation.Report{
				AppID:                 "app-1",
				VersionState:          "PREPARE_FOR_SUBMISSION",
				HasActiveMonetization: test.hasActiveMonetization,
				MonetizationKnown:     !test.monetizationUnknown,
				HasPaidAppPrice:       test.hasPaidAppPrice,
				AppPricingKnown:       test.appPricingKnown,
			}
			deep, _ := buildDeepValidation(context.Background(), report, client, validation.DeepSessionCached)
			if got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive); got.Status != test.want {
				t.Fatalf("agreement check = %#v, want %q", got, test.want)
			}
			if test.wantCommand != "" {
				got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive)
				if got.Resolution == nil || len(got.Resolution.Commands) != 1 || got.Resolution.Commands[0] != test.wantCommand || got.Resolution.Fixability != test.wantFixability {
					t.Fatalf("agreement resolution = %#v, want %q with fixability %q", got.Resolution, test.wantCommand, test.wantFixability)
				}
			}
			if len(test.status.ContractMessages) > 0 && test.want == validation.DeepStatusBlocked {
				got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive)
				if got.Message != "App Store Connect reports an agreement alert that needs review" || got.Resolution == nil || len(got.Resolution.Commands) != 0 {
					t.Fatalf("banner-only check = %#v, want alert without acceptance command", got)
				}
			}
		})
	}
}

func TestBuildDeepValidationChecksSubscriptionAttachmentInReadyForReview(t *testing.T) {
	client := &stubDeepWebClient{
		privacy:       &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{ID: "sub-1", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersionKnown: true}},
		agreements:    &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, _ := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "READY_FOR_REVIEW"}, client, validation.DeepSessionCached)
	got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment)
	if got.Status != validation.DeepStatusBlocked || got.Resolution == nil || got.Resolution.Fixability != validation.FixabilityManual {
		t.Fatalf("READY_FOR_REVIEW subscription check = %#v, want manual blocker", got)
	}
	for _, command := range got.Resolution.Commands {
		if strings.Contains(command, " attach ") {
			t.Fatalf("READY_FOR_REVIEW resolution offers unsafe mutation: %#v", got.Resolution.Commands)
		}
	}
}

func TestBuildDeepValidationPreservesAgreementBannerWhenHistoryFails(t *testing.T) {
	client := &stubDeepWebClient{
		privacy:       &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{},
		agreements: &asc.WebAgreementsStatusResult{
			Pending: true,
			ContractMessages: []asc.WebAgreementContractMessage{{
				Subject: "Apple Developer Program License Agreement Updated",
				Message: "The agreement needs to be reviewed.",
			}},
		},
		agreementsErr: errors.New("agreement history unavailable"),
	}

	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "PREPARE_FOR_SUBMISSION"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckAgreementsActive); got.Status != validation.DeepStatusBlocked {
		t.Fatalf("agreement check = %#v, want preserved blocker", got)
	}
	found := false
	for _, finding := range findings {
		if finding.ID == "agreements.alert" {
			if finding.Severity != validation.SeverityError || finding.Resolution == nil || len(finding.Resolution.Commands) != 0 || strings.Contains(finding.Remediation, "accept") {
				t.Fatalf("alert finding must fail closed without acceptance advice: %#v", finding)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("findings = %#v, want agreements.alert", findings)
	}
}

func TestBuildDeepValidationDoesNotInspectNextVersionAttachmentForTerminalSelectedVersion(t *testing.T) {
	client := &stubDeepWebClient{
		privacy:       &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{ID: "sub-1", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersionKnown: true}},
		agreements:    &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "READY_FOR_SALE"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusNotApplicable {
		t.Fatalf("subscription check = %#v, want notApplicable for terminal selected version", got)
	}
	for _, finding := range findings {
		if strings.Contains(finding.ID, "first_type_app_version_attachment") {
			t.Fatalf("terminal selected version produced attachment finding: %#v", finding)
		}
	}
}

func TestBuildDeepValidationRequiresSelectedVersionStateForAttachmentCheck(t *testing.T) {
	client := &stubDeepWebClient{
		privacy:    &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		agreements: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1"}, client, validation.DeepSessionCached)
	if got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment); got.Status != validation.DeepStatusUnverified {
		t.Fatalf("subscription check = %#v, want unverified without selected version state", got)
	}
	if len(findings) != 1 || findings[0].ID != "deep.subscriptions.first_type_app_version_attachment.unverified" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestBuildDeepValidationDoesNotOfferAttachMutationForVersionAlreadyInReview(t *testing.T) {
	client := &stubDeepWebClient{
		privacy: &webcore.AppDataUsagesPublishState{ID: "publish-1", Published: true, PublishedKnown: true},
		subscriptions: []webcore.ReviewSubscription{{
			ID:                                 "sub-1",
			State:                              "READY_TO_SUBMIT",
			SubmitWithNextAppStoreVersionKnown: true,
		}},
		agreements: &asc.WebAgreementsStatusResult{Agreements: []asc.WebAgreement{{Status: "active", IsProgramLicenseAgreement: true}}},
	}

	deep, _ := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1", VersionState: "IN_REVIEW"}, client, validation.DeepSessionCached)
	got := deepCheckByID(t, deep, validation.DeepCheckSubscriptionAttachment)
	if got.Status != validation.DeepStatusBlocked || got.Resolution == nil || got.Resolution.Fixability != validation.FixabilityManual {
		t.Fatalf("subscription check = %#v, want manual blocker", got)
	}
	if len(got.Resolution.Commands) != 1 || strings.Contains(got.Resolution.Commands[0], " attach ") {
		t.Fatalf("unsafe in-review resolution commands = %#v", got.Resolution.Commands)
	}
}

func TestScopeDeepResolutionCommandsPinsTheValidatedAppleAccount(t *testing.T) {
	sharedResolution := &validation.Resolution{
		Fixability: validation.FixabilityWebFixable,
		Commands:   []string{`asc web privacy publish --app "app-1" --confirm`},
	}
	deep := validation.DeepReport{Checks: []validation.DeepCheck{{Resolution: sharedResolution}}}
	findings := []validation.CheckResult{{Resolution: sharedResolution}}

	scopeDeepResolutionCommands(&deep, findings, "user@example.com")

	want := `asc web privacy publish --app "app-1" --apple-id "user@example.com" --confirm`
	if got := sharedResolution.Commands[0]; got != want {
		t.Fatalf("scoped command = %q, want %q", got, want)
	}
}

func TestBuildDeepValidationClassifiesSessionValidationFailureSeparately(t *testing.T) {
	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1"}, nil, validation.DeepSessionValidationFailed)
	if deep.SessionStatus != validation.DeepSessionValidationFailed {
		t.Fatalf("session status = %q", deep.SessionStatus)
	}
	if len(findings) != 1 || findings[0].ID != "deep.web_session.validation_failed" {
		t.Fatalf("findings = %#v", findings)
	}
	scopeDeepResolutionCommands(&deep, findings, "user@example.com")
	if findings[0].Resolution == nil || len(findings[0].Resolution.Commands) != 2 {
		t.Fatalf("scoped session resolution = %#v", findings[0].Resolution)
	}
	if !strings.Contains(findings[0].Remediation, `--apple-id "user@example.com"`) {
		t.Fatalf("scoped session remediation = %q", findings[0].Remediation)
	}
}

func TestRequiredAgreementFallbackReportKeepsDeepPublicGatesUnverified(t *testing.T) {
	report := requiredAgreementFallbackReport(validateOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Strict:    true,
	})
	if report.Summary.Errors != 1 || report.Summary.Warnings != 3 || report.Summary.Blocking != 4 {
		t.Fatalf("fallback summary = %#v", report.Summary)
	}
	deep, _ := buildDeepValidation(context.Background(), report, nil, validation.DeepSessionUnavailable)
	if got := deepCheckByID(t, deep, validation.DeepCheckIAPAttachment); got.Status != validation.DeepStatusUnverified || got.Source != validation.DeepSourcePublicAPI {
		t.Fatalf("IAP attachment check = %#v, want unverified public API evidence", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckAvailabilityConfigured); got.Status != validation.DeepStatusUnverified {
		t.Fatalf("availability check = %#v, want unverified", got)
	}
	if got := deepCheckByID(t, deep, validation.DeepCheckReviewInformationComplete); got.Status != validation.DeepStatusUnverified {
		t.Fatalf("review check = %#v, want unverified", got)
	}
}

func TestBuildDeepValidationWithoutSessionProducesUnverifiedWebChecks(t *testing.T) {
	deep, findings := buildDeepValidation(context.Background(), validation.Report{AppID: "app-1"}, nil, validation.DeepSessionUnavailable)
	if deep.SessionStatus != validation.DeepSessionUnavailable {
		t.Fatalf("session status = %q", deep.SessionStatus)
	}
	for _, id := range []string{
		validation.DeepCheckPrivacyPublishState,
		validation.DeepCheckSubscriptionAttachment,
		validation.DeepCheckAgreementsActive,
	} {
		if got := deepCheckByID(t, deep, id); got.Status != validation.DeepStatusUnverified {
			t.Fatalf("deep check %q = %#v, want unverified", id, got)
		}
	}
	if len(findings) != 1 || findings[0].ID != "deep.web_session.unavailable" {
		t.Fatalf("findings = %#v, want one session warning", findings)
	}
}

func deepCheckByID(t *testing.T, report validation.DeepReport, id string) validation.DeepCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("deep check %q not found in %#v", id, report.Checks)
	return validation.DeepCheck{}
}

func TestCollectDeepValidationLoadsSessionOnDepsTransport(t *testing.T) {
	transport := &http.Transport{}
	var loaderTransport http.RoundTripper
	previous := loadDeepSessionFn
	loadDeepSessionFn = func(_ context.Context, rt http.RoundTripper, _ string) (*webcore.AuthSession, bool, error) {
		loaderTransport = rt
		return nil, false, nil
	}
	t.Cleanup(func() { loadDeepSessionFn = previous })

	d := &shared.Deps{HTTPTransport: func() http.RoundTripper { return transport }}
	if _, _, err := collectDeepValidation(context.Background(), d, validation.Report{}, ""); err != nil {
		t.Fatalf("collectDeepValidation() error: %v", err)
	}
	if loaderTransport != transport {
		t.Fatalf("session loader transport = %v, want the deps transport", loaderTransport)
	}
}

// Under a web session broker the cached session is not validate's to use: it
// is neither read nor sent to Apple.
func TestLoadDeepSessionLeavesTheCacheAloneUnderAWebBroker(t *testing.T) {
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(&url.URL{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"}, []*http.Cookie{{Name: "myacinfo", Value: "cached", Path: "/"}})
	if err := webcore.PersistSession(&webcore.AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); err != nil {
		t.Fatalf("seed cached session: %v", err)
	}
	t.Setenv(webcore.WebBrokerURLEnv, "https://broker.example.com")
	t.Setenv(webcore.WebBrokerTokenEnv, "broker-token")

	requests := 0
	transport := buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("no request expected")
	})
	session, ok, err := loadDeepSession(context.Background(), transport, "user@example.com")
	if session != nil || ok || err != nil || requests != 0 {
		t.Fatalf("session=%v ok=%v err=%v requests=%d", session, ok, err, requests)
	}
	// The finding names no sign-in command: login is refused under a broker.
	d := &shared.Deps{HTTPTransport: func() http.RoundTripper { return transport }}
	_, findings, err := collectDeepValidation(context.Background(), d, validation.Report{AppID: "app-1"}, "user@example.com")
	if err != nil || len(findings) != 1 || requests != 0 {
		t.Fatalf("findings=%#v err=%v requests=%d", findings, err, requests)
	}
	if advice := findings[0].Remediation + strings.Join(findings[0].Resolution.Commands, " "); !strings.Contains(advice, webcore.WebBrokerURLEnv) || strings.Contains(advice, "asc web auth") {
		t.Fatalf("advice=%q", advice)
	}
}
