package validate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const agreementsAppStoreConnectURL = "https://appstoreconnect.apple.com/agreements/#/"

type deepWebClient interface {
	GetAppDataUsagesPublishState(context.Context, string) (*webcore.AppDataUsagesPublishState, error)
	ListReviewSubscriptions(context.Context, string) ([]webcore.ReviewSubscription, error)
	GetAgreementsStatus(context.Context) (*asc.WebAgreementsStatusResult, error)
}

var (
	loadDeepSessionFn    = loadDeepSession
	deepWebClientFactory = func(session *webcore.AuthSession) deepWebClient {
		return webcore.NewClient(session)
	}
)

func loadDeepSession(ctx context.Context, transport http.RoundTripper, appleID string) (*webcore.AuthSession, bool, error) {
	requestCtx, cancel := readinessRequestContext(ctx)
	defer cancel()
	var session *webcore.AuthSession
	var ok bool
	var err error
	if strings.TrimSpace(appleID) != "" {
		session, ok, err = webcore.ResumeCachedSessionWithoutPersist(requestCtx, strings.TrimSpace(appleID), transport)
	} else {
		session, ok, err = webcore.ResumeLastCachedSessionWithoutPersist(requestCtx, transport)
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return session, false, webcore.ErrCachedSessionValidationFailed
	}
	return session, ok, err
}

func collectDeepValidation(ctx context.Context, d *shared.Deps, report validation.Report, appleID string) (validation.DeepReport, []validation.CheckResult, error) {
	session, ok, err := loadDeepSessionFn(ctx, d.HTTPTransport(), strings.TrimSpace(appleID))
	selectedAppleID := strings.TrimSpace(appleID)
	if session != nil && strings.TrimSpace(session.UserEmail) != "" {
		selectedAppleID = strings.TrimSpace(session.UserEmail)
	}
	if ctx.Err() != nil {
		return validation.DeepReport{}, nil, ctx.Err()
	}
	if errors.Is(err, webcore.ErrCachedSessionExpired) {
		deep, findings := buildDeepValidation(ctx, report, nil, validation.DeepSessionExpired)
		scopeDeepResolutionCommands(&deep, findings, selectedAppleID)
		return deep, findings, nil
	}
	if errors.Is(err, webcore.ErrCachedSessionValidationFailed) {
		deep, findings := buildDeepValidation(ctx, report, nil, validation.DeepSessionValidationFailed)
		scopeDeepResolutionCommands(&deep, findings, selectedAppleID)
		return deep, findings, nil
	}
	if err != nil || !ok || session == nil || session.Client == nil {
		deep, findings := buildDeepValidation(ctx, report, nil, validation.DeepSessionUnavailable)
		scopeDeepResolutionCommands(&deep, findings, selectedAppleID)
		return deep, findings, nil
	}
	deep, findings := buildDeepValidation(ctx, report, deepWebClientFactory(session), validation.DeepSessionCached)
	if ctx.Err() != nil {
		return validation.DeepReport{}, nil, ctx.Err()
	}
	scopeDeepResolutionCommands(&deep, findings, selectedAppleID)
	return deep, findings, nil
}

func buildDeepValidation(ctx context.Context, report validation.Report, client deepWebClient, sessionStatus validation.DeepSessionStatus) (validation.DeepReport, []validation.CheckResult) {
	deep := validation.DeepReport{
		SessionStatus: sessionStatus,
		Checks:        make([]validation.DeepCheck, 0, 5),
	}
	if report.Deep != nil {
		deep.Checks = append(deep.Checks, report.Deep.Checks...)
	}
	findings := make([]validation.CheckResult, 0, 4)

	if client == nil || sessionStatus != validation.DeepSessionCached {
		sessionReason := "without a cached Apple web session"
		findingID := "deep.web_session.unavailable"
		findingMessage := "deep validation requires an existing cached Apple web session"
		switch sessionStatus {
		case validation.DeepSessionExpired:
			sessionReason = "because the cached Apple web session is expired"
			findingID = "deep.web_session.expired"
			findingMessage = "the cached Apple web session is expired"
		case validation.DeepSessionValidationFailed:
			sessionReason = "because Apple or the network did not return a valid session response"
			findingID = "deep.web_session.validation_failed"
			findingMessage = "the cached Apple web session could not be validated because of an Apple or network response"
		}
		deep.Checks = append(
			deep.Checks,
			unverifiedWebCheck(validation.DeepCheckPrivacyPublishState, "App Privacy publication could not be verified "+sessionReason, appStoreConnectURL(report.AppID, "appPrivacy")),
			unverifiedWebCheck(validation.DeepCheckSubscriptionAttachment, "First auto-renewable subscription attachment could not be verified "+sessionReason, appStoreConnectURL(report.AppID, "appstore/review")),
			unverifiedWebCheck(validation.DeepCheckAgreementsActive, "Required agreements could not be verified "+sessionReason, agreementsAppStoreConnectURL),
		)
		findings = append(findings, validation.CheckResult{
			ID:           findingID,
			Severity:     validation.SeverityWarning,
			Message:      findingMessage,
			Remediation:  "Inspect cached sessions with `asc web auth status`, or authenticate separately with `asc web auth login --apple-id EMAIL` and retry",
			ResourceType: "webSession",
			Resolution: &validation.Resolution{
				Fixability: validation.FixabilityManual,
				Commands:   []string{"asc web auth status"},
			},
		})
	} else {
		privacy, privacyFinding := collectPrivacyDeepCheck(ctx, client, report.AppID)
		deep.Checks = append(deep.Checks, privacy)
		if privacyFinding != nil {
			findings = append(findings, *privacyFinding)
		}

		subscriptions, subscriptionFinding := collectSubscriptionDeepCheck(ctx, client, report.AppID, report.VersionState)
		deep.Checks = append(deep.Checks, subscriptions)
		if subscriptionFinding != nil {
			findings = append(findings, *subscriptionFinding)
		}

		requiresPaidAgreement := report.HasActiveMonetization || report.HasPaidAppPrice
		paidAgreementRelevanceKnown := requiresPaidAgreement || (report.MonetizationKnown && report.AppPricingKnown)
		agreements, agreementFinding := collectAgreementsDeepCheck(ctx, client, requiresPaidAgreement, paidAgreementRelevanceKnown)
		deep.Checks = append(deep.Checks, agreements)
		if agreementFinding != nil {
			findings = append(findings, *agreementFinding)
		}
	}

	deep.Checks = append(
		deep.Checks,
		derivedPublicCheck(report, validation.DeepCheckAvailabilityConfigured, "availability.", "The public API confirms app availability and at least one territory", "App availability has blocking public-API findings", appStoreConnectURL(report.AppID, "appstore/pricing")),
		derivedPublicCheck(report, validation.DeepCheckReviewInformationComplete, "review_details.", "Required App Review fields are present", "Required App Review fields have blocking findings", appStoreConnectURL(report.AppID, "appstore/review")),
	)
	deep.Summary = validation.SummarizeDeepChecks(deep.Checks)
	return deep, findings
}

func scopeDeepResolutionCommands(deep *validation.DeepReport, findings []validation.CheckResult, appleID string) {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return
	}
	for i := range deep.Checks {
		scopeResolutionCommands(deep.Checks[i].Resolution, appleID)
	}
	for i := range findings {
		if findings[i].ID == "deep.web_session.unavailable" ||
			findings[i].ID == "deep.web_session.expired" ||
			findings[i].ID == "deep.web_session.validation_failed" {
			if findings[i].Resolution != nil {
				findings[i].Remediation = fmt.Sprintf(
					"Inspect the selected cache with `asc web auth status --apple-id %q`, or authenticate separately with `asc web auth login --apple-id %q` and retry",
					appleID,
					appleID,
				)
				findings[i].Resolution.Commands = append(
					findings[i].Resolution.Commands,
					fmt.Sprintf("asc web auth login --apple-id %q", appleID),
				)
			}
		}
		scopeResolutionCommands(findings[i].Resolution, appleID)
	}
}

func scopeResolutionCommands(resolution *validation.Resolution, appleID string) {
	if resolution == nil || strings.TrimSpace(appleID) == "" {
		return
	}
	selector := fmt.Sprintf(" --apple-id %q", strings.TrimSpace(appleID))
	for i, command := range resolution.Commands {
		if !strings.HasPrefix(command, "asc web ") || strings.Contains(command, "--apple-id") {
			continue
		}
		if confirmIndex := strings.Index(command, " --confirm"); confirmIndex >= 0 {
			resolution.Commands[i] = command[:confirmIndex] + selector + command[confirmIndex:]
			continue
		}
		resolution.Commands[i] = command + selector
	}
}

func collectPrivacyDeepCheck(ctx context.Context, client deepWebClient, appID string) (validation.DeepCheck, *validation.CheckResult) {
	requestCtx, cancel := readinessRequestContext(ctx)
	defer cancel()
	state, err := client.GetAppDataUsagesPublishState(requestCtx, strings.TrimSpace(appID))
	link := appStoreConnectURL(appID, "appPrivacy")
	if err != nil || state == nil || strings.TrimSpace(state.ID) == "" || !state.PublishedKnown {
		check := unverifiedWebCheck(validation.DeepCheckPrivacyPublishState, "App Privacy publication could not be verified through the cached Apple web session", link)
		return check, unverifiedFinding("deep.privacy.publish_state.unverified", check.Message, "Retry after confirming the cached Apple web session can access this app", "appPrivacy", appID, check.Resolution)
	}
	if state.Published {
		return validation.DeepCheck{
			ID:      validation.DeepCheckPrivacyPublishState,
			Status:  validation.DeepStatusPassed,
			Source:  validation.DeepSourceWebSession,
			Message: "App Privacy answers are published",
		}, nil
	}
	resolution := &validation.Resolution{
		Fixability:         validation.FixabilityWebFixable,
		Commands:           []string{fmt.Sprintf("asc web privacy publish --app %q --confirm", strings.TrimSpace(appID))},
		AppStoreConnectURL: link,
	}
	check := validation.DeepCheck{
		ID:         validation.DeepCheckPrivacyPublishState,
		Status:     validation.DeepStatusBlocked,
		Source:     validation.DeepSourceWebSession,
		Message:    "App Privacy answers are not published",
		Resolution: resolution,
	}
	return check, &validation.CheckResult{
		ID:           "privacy.publish_state.unpublished",
		Severity:     validation.SeverityError,
		Message:      check.Message,
		Remediation:  "Publish the App Privacy answers before submission",
		ResourceType: "appPrivacy",
		ResourceID:   strings.TrimSpace(appID),
		Resolution:   resolution,
	}
}

func collectSubscriptionDeepCheck(ctx context.Context, client deepWebClient, appID, versionState string) (validation.DeepCheck, *validation.CheckResult) {
	link := appStoreConnectURL(appID, "appstore/review")
	normalizedVersionState := strings.ToUpper(strings.TrimSpace(versionState))
	if normalizedVersionState == "" {
		check := unverifiedWebCheck(validation.DeepCheckSubscriptionAttachment, "The selected app version state is unavailable, so its first-subscription attachment could not be verified", link)
		return check, unverifiedFinding("deep.subscriptions.first_type_app_version_attachment.unverified", check.Message, "Retry after the public API returns the selected app version state", "subscription", "", check.Resolution)
	}
	if !isSubscriptionAttachmentVersionState(normalizedVersionState) {
		return validation.DeepCheck{
			ID:      validation.DeepCheckSubscriptionAttachment,
			Status:  validation.DeepStatusNotApplicable,
			Source:  validation.DeepSourceWebSession,
			Message: "The selected version is not the app's current review candidate; Apple's attachment flag describes the next app version",
		}, nil
	}

	requestCtx, cancel := readinessRequestContext(ctx)
	defer cancel()
	subscriptions, err := client.ListReviewSubscriptions(requestCtx, strings.TrimSpace(appID))
	if err != nil {
		check := unverifiedWebCheck(validation.DeepCheckSubscriptionAttachment, "First auto-renewable subscription attachment could not be verified through the cached Apple web session", link)
		return check, unverifiedFinding("deep.subscriptions.first_type_app_version_attachment.unverified", check.Message, "Retry after confirming the cached Apple web session can access subscriptions for this app", "subscription", "", check.Resolution)
	}
	if len(subscriptions) == 0 {
		return validation.DeepCheck{
			ID:      validation.DeepCheckSubscriptionAttachment,
			Status:  validation.DeepStatusNotApplicable,
			Source:  validation.DeepSourceWebSession,
			Message: "No subscriptions exist for this app",
		}, nil
	}

	ready := make([]webcore.ReviewSubscription, 0, len(subscriptions))
	hasApproved := false
	hasUnknownState := false
	for _, subscription := range subscriptions {
		state := strings.ToUpper(strings.TrimSpace(subscription.State))
		if state == "" || strings.TrimSpace(subscription.ID) == "" {
			hasUnknownState = true
		}
		if state == "APPROVED" {
			hasApproved = true
		}
		if state == "READY_TO_SUBMIT" {
			ready = append(ready, subscription)
		}
		if subscription.IsAppStoreReviewInProgress && strings.TrimSpace(subscription.ID) != "" {
			return validation.DeepCheck{
				ID:      validation.DeepCheckSubscriptionAttachment,
				Status:  validation.DeepStatusPassed,
				Source:  validation.DeepSourceWebSession,
				Message: "A subscription is already in App Store review",
			}, nil
		}
		if subscription.SubmitWithNextAppStoreVersionKnown && subscription.SubmitWithNextAppStoreVersion {
			if strings.TrimSpace(subscription.ID) == "" {
				hasUnknownState = true
				continue
			}
			return validation.DeepCheck{
				ID:      validation.DeepCheckSubscriptionAttachment,
				Status:  validation.DeepStatusPassed,
				Source:  validation.DeepSourceWebSession,
				Message: "A subscription is attached to the next app version review",
			}, nil
		}
	}
	// Apple's app-version rule is app-wide for the first item of each purchase
	// type, not one item per subscription group. New groups still need at least
	// one subscription in their submission, but after the first auto-renewable
	// subscription is approved they no longer require an app version.
	if hasApproved {
		return validation.DeepCheck{
			ID:      validation.DeepCheckSubscriptionAttachment,
			Status:  validation.DeepStatusNotApplicable,
			Source:  validation.DeepSourceWebSession,
			Message: "An auto-renewable subscription is already approved, so later subscriptions do not require app-version attachment",
		}, nil
	}
	if len(ready) == 0 {
		if hasUnknownState {
			check := unverifiedWebCheck(validation.DeepCheckSubscriptionAttachment, "Apple did not return enough subscription identity and state data to verify first-of-type attachment", link)
			return check, unverifiedFinding("deep.subscriptions.first_type_app_version_attachment.unverified", check.Message, "Inspect subscription readiness and attachment in App Store Connect", "subscription", "", check.Resolution)
		}
		return validation.DeepCheck{
			ID:      validation.DeepCheckSubscriptionAttachment,
			Status:  validation.DeepStatusNotApplicable,
			Source:  validation.DeepSourceWebSession,
			Message: "No subscription is currently READY_TO_SUBMIT; public readiness findings cover incomplete subscription metadata",
		}, nil
	}
	sort.SliceStable(ready, func(i, j int) bool { return strings.TrimSpace(ready[i].ID) < strings.TrimSpace(ready[j].ID) })
	unknown := hasUnknownState
	for _, subscription := range ready {
		if !subscription.SubmitWithNextAppStoreVersionKnown || strings.TrimSpace(subscription.ID) == "" {
			unknown = true
		}
	}
	if unknown {
		check := unverifiedWebCheck(validation.DeepCheckSubscriptionAttachment, "Apple did not return a reliable first-of-type attachment state", link)
		return check, unverifiedFinding("deep.subscriptions.first_type_app_version_attachment.unverified", check.Message, "Inspect subscription review attachment in App Store Connect", "subscription", "", check.Resolution)
	}

	commands := []string{fmt.Sprintf("asc web review subscriptions list --app %q", strings.TrimSpace(appID))}
	fixability := validation.FixabilityManual
	remediation := "Choose the intended first subscription in App Store Connect and attach it to this app version"
	if len(ready) == 1 && isSubscriptionAttachmentEditableVersionState(normalizedVersionState) {
		commands = []string{fmt.Sprintf(
			"asc web review subscriptions attach --app %q --subscription-id %q --confirm",
			strings.TrimSpace(appID),
			strings.TrimSpace(ready[0].ID),
		)}
		fixability = validation.FixabilityWebFixable
		remediation = "Attach the only READY_TO_SUBMIT subscription to this app version"
	}
	resolution := &validation.Resolution{
		Fixability:         fixability,
		Commands:           commands,
		AppStoreConnectURL: link,
	}
	check := validation.DeepCheck{
		ID:         validation.DeepCheckSubscriptionAttachment,
		Status:     validation.DeepStatusBlocked,
		Source:     validation.DeepSourceWebSession,
		Message:    "No READY_TO_SUBMIT subscription is attached to the next app version review",
		Resolution: resolution,
	}
	return check, &validation.CheckResult{
		ID:           "subscriptions.first_type_app_version_attachment.missing",
		Severity:     validation.SeverityError,
		Message:      check.Message,
		Remediation:  remediation,
		ResourceType: "subscription",
		Resolution:   resolution,
	}
}

func isSubscriptionAttachmentVersionState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY", "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW":
		return true
	default:
		return false
	}
}

func isSubscriptionAttachmentEditableVersionState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY":
		return true
	default:
		return false
	}
}

func collectAgreementsDeepCheck(ctx context.Context, client deepWebClient, requiresPaidAgreement, paidAgreementRelevanceKnown bool) (validation.DeepCheck, *validation.CheckResult) {
	requestCtx, cancel := readinessRequestContext(ctx)
	defer cancel()
	status, err := client.GetAgreementsStatus(requestCtx)
	if status == nil {
		check := unverifiedWebCheck(validation.DeepCheckAgreementsActive, "Required agreements could not be verified through the cached Apple web session", agreementsAppStoreConnectURL)
		return check, unverifiedFinding("deep.agreements.active.unverified", check.Message, "Retry after confirming the cached Apple web session has access to agreements", "agreement", "", check.Resolution)
	}
	acceptIDs := make([]string, 0, len(status.Agreements))
	hasPending := false
	hasAlert := false
	hasManualBlocker := false
	hasUnknown := false
	hasRelevantEvidence := false
	for _, message := range status.ContractMessages {
		messageText := strings.ToLower(strings.Join([]string{message.Group, message.Subject, message.Message}, " "))
		if strings.Contains(messageText, "paid") {
			switch {
			case paidAgreementRelevanceKnown && !requiresPaidAgreement:
				continue
			case !paidAgreementRelevanceKnown:
				hasRelevantEvidence = true
				hasUnknown = true
				continue
			}
		}
		if strings.Contains(messageText, "agreement") || strings.Contains(messageText, "contract") {
			hasRelevantEvidence = true
			hasAlert = true
			hasManualBlocker = true
			continue
		}
		hasUnknown = true
	}
	seenIDs := map[string]struct{}{}
	agreements := make([]asc.WebAgreement, 0, len(status.Agreements))
	for _, agreement := range status.Agreements {
		if agreement.Pending && agreement.ProgramLicenseClassificationUnknown {
			hasUnknown = true
			continue
		}
		agreements = append(agreements, agreement)
	}
	for _, agreement := range currentAgreementHistory(agreements) {
		if !agreement.IsProgramLicenseAgreement {
			switch {
			case paidAgreementRelevanceKnown && !requiresPaidAgreement:
				continue
			case !paidAgreementRelevanceKnown:
				hasRelevantEvidence = true
				hasUnknown = true
				continue
			}
		}
		hasRelevantEvidence = true
		agreementStatus := strings.ToLower(strings.TrimSpace(agreement.Status))
		needsAcceptance := agreement.Pending || agreementStatus == "new" || strings.Contains(agreementStatus, "new agreement available")
		switch {
		case needsAcceptance:
			hasPending = true
			agreementID := strings.TrimSpace(agreement.AgreementID)
			if agreementID == "" {
				hasManualBlocker = true
				continue
			}
			if _, seen := seenIDs[agreementID]; seen {
				continue
			}
			seenIDs[agreementID] = struct{}{}
			acceptIDs = append(acceptIDs, agreementID)
		case agreementStatus == "active" || agreementStatus == "active (pending user)":
			// Apple documents both states as in effect. The latter still needs
			// account information, but it does not make the agreement inactive.
		case agreementStatus == "pending user info" || agreementStatus == "processing" || agreementStatus == "verifying" || strings.HasPrefix(agreementStatus, "pending (") || agreementStatus == "expired" || agreementStatus == "disabled":
			hasPending = true
			hasManualBlocker = true
		default:
			hasUnknown = true
		}
	}
	if !hasRelevantEvidence {
		hasUnknown = true
	}
	if err != nil {
		hasUnknown = true
	}
	if !hasPending && !hasAlert && !hasUnknown {
		return validation.DeepCheck{
			ID:      validation.DeepCheckAgreementsActive,
			Status:  validation.DeepStatusPassed,
			Source:  validation.DeepSourceWebSession,
			Message: "Required agreements are active",
		}, nil
	}
	if !hasPending && !hasAlert {
		check := unverifiedWebCheck(validation.DeepCheckAgreementsActive, "Apple returned an empty or unknown agreement status", agreementsAppStoreConnectURL)
		return check, unverifiedFinding("deep.agreements.active.unverified", check.Message, "Inspect current agreement status with `asc web agreements status`", "agreement", "", check.Resolution)
	}
	sort.Strings(acceptIDs)
	commands := make([]string, 0, len(acceptIDs))
	for _, agreementID := range acceptIDs {
		commands = append(commands, fmt.Sprintf("asc web agreements accept --agreement-id %q --confirm", agreementID))
	}
	fixability := validation.FixabilityManual
	if len(commands) > 0 && !hasManualBlocker && !hasUnknown {
		fixability = validation.FixabilityWebFixable
	}
	resolution := &validation.Resolution{
		Fixability:         fixability,
		Commands:           commands,
		AppStoreConnectURL: agreementsAppStoreConnectURL,
	}
	check := validation.DeepCheck{
		ID:         validation.DeepCheckAgreementsActive,
		Status:     validation.DeepStatusBlocked,
		Source:     validation.DeepSourceWebSession,
		Message:    "A required agreement is pending",
		Resolution: resolution,
	}
	findingID := "agreements.pending"
	remediation := "Have the Account Holder review and accept the pending agreement"
	if !hasPending {
		check.Message = "App Store Connect reports an agreement alert that needs review"
		findingID = "agreements.alert"
		remediation = "Inspect the alert in App Store Connect and review the Apple Developer account; agreement history does not confirm a pending agreement"
	}
	return check, &validation.CheckResult{
		ID:           findingID,
		Severity:     validation.SeverityError,
		Message:      check.Message,
		Remediation:  remediation,
		ResourceType: "agreement",
		Resolution:   resolution,
	}
}

func currentAgreementHistory(agreements []asc.WebAgreement) []asc.WebAgreement {
	current := make(map[string]asc.WebAgreement)
	order := make([]string, 0, len(agreements))
	for index, agreement := range agreements {
		key := strings.ToLower(strings.TrimSpace(agreement.Title))
		if agreement.IsProgramLicenseAgreement {
			key = "program-license-agreement"
		}
		if agreement.Pending && strings.TrimSpace(agreement.DateEffective) == "" {
			// Dated history cannot supersede an undated outstanding agreement.
			key = ""
		}
		if key == "" {
			agreementID := strings.TrimSpace(agreement.AgreementID)
			if agreementID == "" {
				key = fmt.Sprintf("unknown-agreement:%d", index)
			} else {
				key = "agreement-id:" + agreementID
			}
		}
		candidate, ok := current[key]
		if !ok {
			current[key] = agreement
			order = append(order, key)
			continue
		}
		if agreementHistoryRecordIsNewer(agreement, candidate) {
			current[key] = agreement
		}
	}

	result := make([]asc.WebAgreement, 0, len(order))
	for _, key := range order {
		result = append(result, current[key])
	}
	return result
}

func agreementHistoryRecordIsNewer(candidate, current asc.WebAgreement) bool {
	candidateEffective := strings.TrimSpace(candidate.DateEffective)
	currentEffective := strings.TrimSpace(current.DateEffective)
	if candidateEffective != currentEffective {
		return candidateEffective > currentEffective
	}
	candidateVersion := strings.TrimSpace(candidate.Version)
	currentVersion := strings.TrimSpace(current.Version)
	if candidateVersion != currentVersion {
		if len(candidateVersion) != len(currentVersion) {
			return len(candidateVersion) > len(currentVersion)
		}
		return candidateVersion > currentVersion
	}
	return agreementCurrentStatusRank(candidate.Status) > agreementCurrentStatusRank(current.Status)
}

func agreementCurrentStatusRank(status string) int {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case normalized == "new", strings.Contains(normalized, "new agreement available"):
		return 3
	case normalized == "active", normalized == "active (pending user)":
		return 2
	case normalized == "pending user info", normalized == "processing", normalized == "verifying", strings.HasPrefix(normalized, "pending ("):
		return 1
	default:
		return 0
	}
}

func derivedPublicCheck(report validation.Report, id, prefix, passedMessage, blockedMessage, link string) validation.DeepCheck {
	status := validation.DeepStatusPassed
	message := passedMessage
	var resolution *validation.Resolution
	for _, check := range report.Checks {
		if !strings.HasPrefix(check.ID, prefix) {
			continue
		}
		switch check.Severity {
		case validation.SeverityError:
			status = validation.DeepStatusBlocked
			message = blockedMessage
		case validation.SeverityWarning:
			if status != validation.DeepStatusBlocked && strings.Contains(check.ID, "unverified") {
				status = validation.DeepStatusUnverified
				message = strings.TrimSpace(check.Message)
			}
		}
		if status != validation.DeepStatusPassed {
			resolution = &validation.Resolution{
				Fixability:         validation.FixabilityManual,
				AppStoreConnectURL: link,
			}
		}
	}
	return validation.DeepCheck{
		ID:         id,
		Status:     status,
		Source:     validation.DeepSourcePublicAPI,
		Message:    message,
		Resolution: resolution,
	}
}

func unverifiedWebCheck(id, message, link string) validation.DeepCheck {
	return validation.DeepCheck{
		ID:      id,
		Status:  validation.DeepStatusUnverified,
		Source:  validation.DeepSourceWebSession,
		Message: message,
		Resolution: &validation.Resolution{
			Fixability:         validation.FixabilityManual,
			AppStoreConnectURL: link,
		},
	}
}

func unverifiedFinding(id, message, remediation, resourceType, resourceID string, resolution *validation.Resolution) *validation.CheckResult {
	return &validation.CheckResult{
		ID:           id,
		Severity:     validation.SeverityWarning,
		Message:      message,
		Remediation:  remediation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Resolution:   resolution,
	}
}

func appStoreConnectURL(appID, suffix string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	base := "https://appstoreconnect.apple.com/apps/" + url.PathEscape(appID)
	if strings.TrimSpace(suffix) == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(strings.TrimSpace(suffix), "/")
}
