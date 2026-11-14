package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

const (
	reviewSubmissionsInclude = "appStoreVersionForReview,items,lastUpdatedByActor,submittedByActor,createdByActor"
	// iris GET /reviewSubmissions/{id}/items rejects appStoreVersionExperimentV2
	// with PARAMETER_ERROR.INVALID (verified live 2026-09-03) even though the
	// public OpenAPI lists it.
	reviewSubmissionsItemsInclude = "appStoreVersion,appCustomProductPageVersion,appStoreVersionExperiment,appEvent,backgroundAssetVersion,gameCenterAchievementVersion,gameCenterActivityVersion,gameCenterChallengeVersion,gameCenterLeaderboardSetVersion,gameCenterLeaderboardVersion,inAppPurchaseVersion,subscriptionVersion,subscriptionGroupVersion"
	reviewMessagesInclude         = "fromActor,rejections,resolutionCenterMessageAttachments"
	reviewDraftMessageInclude     = "resolutionCenterMessageAttachments,fromActor"
	reviewAppThreadsInclude       = "appStoreVersions,app,appMessageThreadDetail,build,betaBackgroundAssetReviewSubmission"
	// reviewAppThreadTypes is the thread-type set the App Store Connect review
	// center requests for an app. It is sent verbatim rather than narrowed
	// here: the captured query always carries it, and unsupported include or
	// filter shapes on this surface answer 400.
	reviewAppThreadTypes    = "REJECTION_BINARY,REJECTION_METADATA,REJECTION_REVIEW_SUBMISSION,APP_MESSAGE_ARC,APP_MESSAGE_ARB,APP_MESSAGE_COMM,APP_MESSAGE_INFORMATIONAL"
	reviewRejectionsInclude = "appCustomProductPageVersion,appEvent,appStoreVersion,appStoreVersionExperiment,backgroundAssetVersions,gameCenterAchievementVersions,gameCenterLeaderboardVersions,gameCenterLeaderboardSetVersions,gameCenterChallengeVersions,gameCenterActivityVersions,build,appBundleVersion,rejectionAttachments"
	attachmentHostsEnv      = "ASC_WEB_ALLOWED_ATTACHMENT_HOSTS"
)

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

var skippedReviewRelatedRelationships = map[string]struct{}{
	"rejectionAttachments": {},
}

var defaultAttachmentHostSuffixes = []string{
	".apple.com",
	".mzstatic.com",
	".amazonaws.com",
	".cloudfront.net",
}

type jsonAPIRelationship struct {
	Data json.RawMessage `json:"data"`
}

type jsonAPIResource struct {
	ID            string                         `json:"id"`
	Type          string                         `json:"type"`
	Attributes    map[string]any                 `json:"attributes"`
	Relationships map[string]jsonAPIRelationship `json:"relationships"`
}

type jsonAPIListPayload struct {
	Data     []jsonAPIResource `json:"data"`
	Included []jsonAPIResource `json:"included"`
	Links    map[string]any    `json:"links"`
}

type jsonAPISingleResourcePayload struct {
	Data     json.RawMessage   `json:"data"`
	Included []jsonAPIResource `json:"included"`
}

type resourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AppStoreVersionForReview describes app version context attached to review data.
type AppStoreVersionForReview struct {
	ID       string `json:"id"`
	Version  string `json:"version,omitempty"`
	Platform string `json:"platform,omitempty"`
}

// ReviewActor describes actor metadata from included relationships.
type ReviewActor struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	ActorType string `json:"actorType,omitempty"`
	Name      string `json:"name,omitempty"`
}

// ReviewSubmission captures high-level review submission metadata.
type ReviewSubmission struct {
	ID                       string                    `json:"id"`
	State                    string                    `json:"state,omitempty"`
	SubmittedDate            string                    `json:"submittedDate,omitempty"`
	Platform                 string                    `json:"platform,omitempty"`
	AppStoreVersionForReview *AppStoreVersionForReview `json:"appStoreVersionForReview,omitempty"`
	SubmittedByActor         *ReviewActor              `json:"submittedByActor,omitempty"`
	LastUpdatedByActor       *ReviewActor              `json:"lastUpdatedByActor,omitempty"`
	CreatedByActor           *ReviewActor              `json:"createdByActor,omitempty"`
}

// ReviewRelatedResource is an included JSON:API resource decoded for display.
type ReviewRelatedResource struct {
	Relationship string `json:"relationship,omitempty"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Label        string `json:"label,omitempty"`
}

// ReviewSubmissionItemRelation links a submission item to related resources.
type ReviewSubmissionItemRelation struct {
	Relationship string `json:"relationship"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Label        string `json:"label,omitempty"`
}

// ReviewSubmissionItem models review submission item relationships.
type ReviewSubmissionItem struct {
	ID      string                         `json:"id"`
	Type    string                         `json:"type"`
	State   string                         `json:"state,omitempty"`
	Related []ReviewSubmissionItemRelation `json:"related,omitempty"`
}

// ResolutionCenterThread models thread metadata for app review issues.
type ResolutionCenterThread struct {
	ID                      string   `json:"id"`
	ThreadType              string   `json:"threadType,omitempty"`
	State                   string   `json:"state,omitempty"`
	CreatedDate             string   `json:"createdDate,omitempty"`
	LastMessageResponseDate string   `json:"lastMessageResponseDate,omitempty"`
	CanDeveloperAddNote     bool     `json:"canDeveloperAddNote"`
	AppStoreVersionIDs      []string `json:"appStoreVersionIds,omitempty"`
	ReviewSubmissionID      string   `json:"reviewSubmissionId,omitempty"`
}

// ResolutionCenterMessage models a single message in a resolution center thread.
type ResolutionCenterMessage struct {
	ID               string       `json:"id"`
	CreatedDate      string       `json:"createdDate,omitempty"`
	MessageBody      string       `json:"messageBody,omitempty"`
	MessageBodyPlain string       `json:"messageBodyPlain,omitempty"`
	FromActor        *ReviewActor `json:"fromActor,omitempty"`
	RejectionIDs     []string     `json:"rejectionIds,omitempty"`
	AttachmentIDs    []string     `json:"attachmentIds,omitempty"`
}

// ResolutionCenterDraftMessage models the unsent draft reply Apple keeps on a
// resolution center thread. Attachment download URLs are never populated: the
// draft surface is read-only and its signed URLs are not offered for download.
type ResolutionCenterDraftMessage struct {
	ID               string             `json:"id"`
	ThreadID         string             `json:"threadId,omitempty"`
	CreatedDate      string             `json:"createdDate,omitempty"`
	MessageBody      string             `json:"messageBody,omitempty"`
	MessageBodyPlain string             `json:"messageBodyPlain,omitempty"`
	FromActor        *ReviewActor       `json:"fromActor,omitempty"`
	Attachments      []ReviewAttachment `json:"attachments,omitempty"`
}

// ReviewRejectionReason captures normalized review rejection reason fields.
type ReviewRejectionReason struct {
	ReasonSection     string `json:"reasonSection,omitempty"`
	ReasonDescription string `json:"reasonDescription,omitempty"`
	ReasonCode        string `json:"reasonCode,omitempty"`
}

// ReviewRejection models rejection records linked to resolution center.
type ReviewRejection struct {
	ID            string                  `json:"id"`
	Reasons       []ReviewRejectionReason `json:"reasons,omitempty"`
	AttachmentIDs []string                `json:"attachmentIds,omitempty"`
	Related       []ReviewRelatedResource `json:"related,omitempty"`
}

// ReviewAttachment models message/rejection attachment metadata.
type ReviewAttachment struct {
	AttachmentID       string `json:"attachmentId"`
	SourceType         string `json:"sourceType"`
	FileName           string `json:"fileName,omitempty"`
	FileSize           int64  `json:"fileSize,omitempty"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Downloadable       bool   `json:"downloadable"`
	DownloadURL        string `json:"downloadUrl,omitempty"`
	ThreadID           string `json:"threadId,omitempty"`
	MessageID          string `json:"messageId,omitempty"`
	ReviewRejectionID  string `json:"reviewRejectionId,omitempty"`
}

// ReviewThreadDetails bundles per-thread review records from shared API calls.
type ReviewThreadDetails struct {
	Messages    []ResolutionCenterMessage `json:"messages,omitempty"`
	Rejections  []ReviewRejection         `json:"rejections,omitempty"`
	Attachments []ReviewAttachment        `json:"attachments,omitempty"`
}

func jsonAPIResourceKey(resourceType, id string) string {
	return strings.TrimSpace(resourceType) + "#" + strings.TrimSpace(id)
}

func buildIncludedMap(included []jsonAPIResource) map[string]jsonAPIResource {
	result := make(map[string]jsonAPIResource, len(included))
	for _, resource := range included {
		if strings.TrimSpace(resource.ID) == "" || strings.TrimSpace(resource.Type) == "" {
			continue
		}
		result[jsonAPIResourceKey(resource.Type, resource.ID)] = resource
	}
	return result
}

func queryPath(path string, values url.Values) string {
	if values == nil {
		return path
	}
	encoded := values.Encode()
	if encoded == "" {
		return path
	}
	return path + "?" + encoded
}

func parseRelationshipRefs(raw json.RawMessage) []resourceRef {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	switch trimmed[0] {
	case '{':
		var ref resourceRef
		if err := json.Unmarshal(trimmed, &ref); err != nil {
			return nil
		}
		if strings.TrimSpace(ref.ID) == "" || strings.TrimSpace(ref.Type) == "" {
			return nil
		}
		return []resourceRef{ref}
	case '[':
		var refs []resourceRef
		if err := json.Unmarshal(trimmed, &refs); err != nil {
			return nil
		}
		out := make([]resourceRef, 0, len(refs))
		for _, ref := range refs {
			if strings.TrimSpace(ref.ID) == "" || strings.TrimSpace(ref.Type) == "" {
				continue
			}
			out = append(out, ref)
		}
		return out
	default:
		return nil
	}
}

func relationshipRefs(resource jsonAPIResource, relationshipName string) []resourceRef {
	if resource.Relationships == nil {
		return nil
	}
	relationship, ok := resource.Relationships[relationshipName]
	if !ok {
		return nil
	}
	return parseRelationshipRefs(relationship.Data)
}

func firstRelationshipRef(resource jsonAPIResource, relationshipName string) *resourceRef {
	refs := relationshipRefs(resource, relationshipName)
	if len(refs) == 0 {
		return nil
	}
	return &refs[0]
}

// validateReviewSubmissionRelationship validates a to-one relationship when
// an attachment submission response includes it. A missing relationship is
// distinct from a present-but-malformed relationship: callers may fall back
// to the requested resource ID only in the former case.
func validateReviewSubmissionRelationship(resource jsonAPIResource, relationshipName, expectedType, expectedID string) (string, bool, error) {
	relationship, present := resource.Relationships[relationshipName]
	if !present {
		return "", false, nil
	}

	refs := parseRelationshipRefs(relationship.Data)
	if len(refs) == 0 {
		return "", true, fmt.Errorf("submission response %s relationship is present but has no resource data", relationshipName)
	}
	if len(refs) != 1 {
		return "", true, fmt.Errorf("submission response %s relationship contains %d resources, want one", relationshipName, len(refs))
	}

	ref := refs[0]
	if strings.TrimSpace(ref.Type) != expectedType {
		return "", true, fmt.Errorf("submission response %s relationship has unexpected resource type %q, want %q", relationshipName, ref.Type, expectedType)
	}
	if strings.TrimSpace(ref.ID) != strings.TrimSpace(expectedID) {
		return "", true, fmt.Errorf("submission response %s relationship refers to %q, want %q", relationshipName, ref.ID, expectedID)
	}
	return strings.TrimSpace(ref.ID), true, nil
}

func stringAttr(attrs map[string]any, keys ...string) string {
	if attrs == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := attrs[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		}
	}
	return ""
}

func stringAttrPreserveWhitespace(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	value, ok := attrs[key]
	if !ok {
		return ""
	}
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func boolAttr(attrs map[string]any, keys ...string) bool {
	if attrs == nil {
		return false
	}
	for _, key := range keys {
		value, ok := attrs[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		}
	}
	return false
}

func int64Attr(attrs map[string]any, keys ...string) int64 {
	if attrs == nil {
		return 0
	}
	for _, key := range keys {
		value, ok := attrs[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case int:
			return int64(typed)
		case int32:
			return int64(typed)
		case int64:
			return typed
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func htmlToPlainText(value string) string {
	withoutTags := htmlTagPattern.ReplaceAllString(value, " ")
	decoded := html.UnescapeString(withoutTags)
	return strings.Join(strings.Fields(decoded), " ")
}

func actorFromRef(ref *resourceRef, included map[string]jsonAPIResource) *ReviewActor {
	if ref == nil {
		return nil
	}
	actor := &ReviewActor{
		ID:   strings.TrimSpace(ref.ID),
		Type: strings.TrimSpace(ref.Type),
	}
	if included == nil {
		return actor
	}
	resource, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
	if !ok {
		return actor
	}
	actor.ActorType = stringAttr(resource.Attributes, "actorType")
	actor.Name = stringAttr(resource.Attributes, "name", "displayName")
	return actor
}

func relatedResourceLabel(resource jsonAPIResource) string {
	return attrLabel(
		resource.Attributes,
		"versionString",
		"version",
		"buildNumber",
		"name",
		"referenceName",
		"displayName",
		"title",
		"bundleId",
		"identifier",
	)
}

func attrLabel(attrs map[string]any, keys ...string) string {
	if label := stringAttr(attrs, keys...); label != "" {
		return label
	}
	if attrs == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := attrs[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case json.Number:
			if label := strings.TrimSpace(typed.String()); label != "" {
				return label
			}
		case int:
			return strconv.Itoa(typed)
		case int32:
			return strconv.FormatInt(int64(typed), 10)
		case int64:
			return strconv.FormatInt(typed, 10)
		}
	}
	return ""
}

func includedResourceLabel(included map[string]jsonAPIResource, ref resourceRef) string {
	if included == nil {
		return ""
	}
	resource, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
	if !ok {
		return ""
	}
	return relatedResourceLabel(resource)
}

func relatedResourcesFrom(resource jsonAPIResource, included map[string]jsonAPIResource, skip map[string]struct{}) []ReviewRelatedResource {
	if len(resource.Relationships) == 0 {
		return nil
	}
	names := make([]string, 0, len(resource.Relationships))
	for relationshipName := range resource.Relationships {
		if _, skipped := skip[relationshipName]; skipped {
			continue
		}
		names = append(names, relationshipName)
	}
	sort.Strings(names)

	var related []ReviewRelatedResource
	for _, relationshipName := range names {
		for _, ref := range relationshipRefs(resource, relationshipName) {
			related = append(related, ReviewRelatedResource{
				Relationship: relationshipName,
				Type:         strings.TrimSpace(ref.Type),
				ID:           strings.TrimSpace(ref.ID),
				Label:        includedResourceLabel(included, ref),
			})
		}
	}
	return related
}

func appStoreVersionFromRef(ref *resourceRef, included map[string]jsonAPIResource) *AppStoreVersionForReview {
	if ref == nil {
		return nil
	}
	version := &AppStoreVersionForReview{ID: strings.TrimSpace(ref.ID)}
	if included == nil {
		return version
	}
	resource, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
	if !ok {
		return version
	}
	version.Version = stringAttr(resource.Attributes, "versionString")
	version.Platform = stringAttr(resource.Attributes, "platform")
	return version
}

func decodeReviewSubmissions(resources []jsonAPIResource, included []jsonAPIResource) []ReviewSubmission {
	if len(resources) == 0 {
		return []ReviewSubmission{}
	}
	includedMap := buildIncludedMap(included)
	result := make([]ReviewSubmission, 0, len(resources))
	for _, resource := range resources {
		submission := ReviewSubmission{
			ID:            strings.TrimSpace(resource.ID),
			State:         stringAttr(resource.Attributes, "state"),
			SubmittedDate: stringAttr(resource.Attributes, "submittedDate"),
			Platform:      stringAttr(resource.Attributes, "platform"),
		}
		submission.AppStoreVersionForReview = appStoreVersionFromRef(firstRelationshipRef(resource, "appStoreVersionForReview"), includedMap)
		if submission.Platform == "" && submission.AppStoreVersionForReview != nil {
			submission.Platform = strings.TrimSpace(submission.AppStoreVersionForReview.Platform)
		}
		submission.SubmittedByActor = actorFromRef(firstRelationshipRef(resource, "submittedByActor"), includedMap)
		submission.LastUpdatedByActor = actorFromRef(firstRelationshipRef(resource, "lastUpdatedByActor"), includedMap)
		submission.CreatedByActor = actorFromRef(firstRelationshipRef(resource, "createdByActor"), includedMap)
		result = append(result, submission)
	}
	return result
}

// ListReviewSubmissions lists review submissions for a specific app ID.
func (c *Client) ListReviewSubmissions(ctx context.Context, appID string) ([]ReviewSubmission, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	query := url.Values{}
	query.Set("include", reviewSubmissionsInclude)
	query.Set("limit", "2000")
	query.Set("limit[items]", "0")
	path := queryPath("/apps/"+url.PathEscape(appID)+"/reviewSubmissions", query)

	payload, err := c.fetchJSONAPIPages(ctx, path, "review submissions")
	if err != nil {
		return nil, err
	}
	return decodeReviewSubmissions(payload.Data, payload.Included), nil
}

// ListReviewSubmissionItems returns submission items for a review submission.
func (c *Client) ListReviewSubmissionItems(ctx context.Context, reviewSubmissionID string) ([]ReviewSubmissionItem, error) {
	reviewSubmissionID = strings.TrimSpace(reviewSubmissionID)
	if reviewSubmissionID == "" {
		return nil, fmt.Errorf("review submission id is required")
	}
	query := url.Values{}
	query.Set("include", reviewSubmissionsItemsInclude)
	query.Set("limit", "200")
	path := queryPath("/reviewSubmissions/"+url.PathEscape(reviewSubmissionID)+"/items", query)

	payload, err := c.fetchJSONAPIPages(ctx, path, "review submission items")
	if err != nil {
		return nil, err
	}
	includedMap := buildIncludedMap(payload.Included)
	items := make([]ReviewSubmissionItem, 0, len(payload.Data))
	for _, resource := range payload.Data {
		item := ReviewSubmissionItem{
			ID:    strings.TrimSpace(resource.ID),
			Type:  strings.TrimSpace(resource.Type),
			State: stringAttr(resource.Attributes, "state"),
		}
		names := make([]string, 0, len(resource.Relationships))
		for relationshipName := range resource.Relationships {
			names = append(names, relationshipName)
		}
		sort.Strings(names)
		for _, relationshipName := range names {
			refs := relationshipRefs(resource, relationshipName)
			for _, ref := range refs {
				item.Related = append(item.Related, ReviewSubmissionItemRelation{
					Relationship: relationshipName,
					Type:         strings.TrimSpace(ref.Type),
					ID:           strings.TrimSpace(ref.ID),
					Label:        includedResourceLabel(includedMap, ref),
				})
			}
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return []ReviewSubmissionItem{}, nil
	}
	return items, nil
}

func decodeResolutionCenterThreads(resources []jsonAPIResource) []ResolutionCenterThread {
	if len(resources) == 0 {
		return []ResolutionCenterThread{}
	}
	threads := make([]ResolutionCenterThread, 0, len(resources))
	for _, resource := range resources {
		thread := ResolutionCenterThread{
			ID:                      strings.TrimSpace(resource.ID),
			ThreadType:              stringAttr(resource.Attributes, "threadType"),
			State:                   stringAttr(resource.Attributes, "state"),
			CreatedDate:             stringAttr(resource.Attributes, "createdDate"),
			LastMessageResponseDate: stringAttr(resource.Attributes, "lastMessageResponseDate"),
			CanDeveloperAddNote:     boolAttr(resource.Attributes, "canDeveloperAddNote", "canDeveloperAddNode"),
		}
		versionRefs := relationshipRefs(resource, "appStoreVersions")
		if len(versionRefs) > 0 {
			thread.AppStoreVersionIDs = make([]string, 0, len(versionRefs))
			for _, ref := range versionRefs {
				if strings.TrimSpace(ref.ID) != "" {
					thread.AppStoreVersionIDs = append(thread.AppStoreVersionIDs, strings.TrimSpace(ref.ID))
				}
			}
		}
		reviewSubmissionRef := firstRelationshipRef(resource, "reviewSubmission")
		if reviewSubmissionRef != nil {
			thread.ReviewSubmissionID = strings.TrimSpace(reviewSubmissionRef.ID)
		}
		threads = append(threads, thread)
	}
	return threads
}

// ListResolutionCenterThreadsBySubmission lists threads for a review submission.
func (c *Client) ListResolutionCenterThreadsBySubmission(ctx context.Context, reviewSubmissionID string) ([]ResolutionCenterThread, error) {
	reviewSubmissionID = strings.TrimSpace(reviewSubmissionID)
	if reviewSubmissionID == "" {
		return nil, fmt.Errorf("review submission id is required")
	}
	query := url.Values{}
	query.Set("filter[reviewSubmission]", reviewSubmissionID)
	query.Set("include", "reviewSubmission")
	path := queryPath("/resolutionCenterThreads", query)

	payload, err := c.fetchJSONAPIPages(ctx, path, "resolution center threads")
	if err != nil {
		return nil, err
	}
	return decodeResolutionCenterThreads(payload.Data), nil
}

// ListResolutionCenterThreadsByApp lists every resolution center thread on an
// app, including threads that are not attached to a review submission and are
// therefore invisible to ListResolutionCenterThreadsBySubmission.
func (c *Client) ListResolutionCenterThreadsByApp(ctx context.Context, appID string) ([]ResolutionCenterThread, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	query := url.Values{}
	query.Set("include", reviewAppThreadsInclude)
	query.Set("limit[appStoreVersions]", "2000")
	query.Set("filter[threadType]", reviewAppThreadTypes)
	path := queryPath("/apps/"+url.PathEscape(appID)+"/resolutionCenterThreads", query)

	payload, err := c.fetchJSONAPIPages(ctx, path, "resolution center threads")
	if err != nil {
		return nil, err
	}
	return decodeResolutionCenterThreads(payload.Data), nil
}

// GetResolutionCenterDraftMessage returns the unsent draft reply on a thread,
// or nil when the thread has no draft. Apple reports an absent draft either as
// a null data member or as a 404 on the relationship, so both mean "no draft"
// rather than an error.
func (c *Client) GetResolutionCenterDraftMessage(ctx context.Context, threadID string, plainText bool) (*ResolutionCenterDraftMessage, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, fmt.Errorf("thread id is required")
	}
	query := url.Values{}
	query.Set("include", reviewDraftMessageInclude)
	query.Set("limit[resolutionCenterMessageAttachments]", "1000")
	path := queryPath("/resolutionCenterThreads/"+url.PathEscape(threadID)+"/resolutionCenterDraftMessage", query)

	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	var payload jsonAPISingleResourcePayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse resolution center draft message response: %w", err)
	}
	trimmedData := bytes.TrimSpace(payload.Data)
	if bytes.Equal(trimmedData, []byte("null")) {
		return nil, nil
	}
	if len(trimmedData) == 0 {
		return nil, fmt.Errorf("resolution center draft message response did not include data")
	}
	var resource jsonAPIResource
	if err := json.Unmarshal(trimmedData, &resource); err != nil {
		return nil, fmt.Errorf("failed to parse resolution center draft message response: %w", err)
	}
	return decodeResolutionCenterDraftMessage(resource, payload.Included, threadID, plainText), nil
}

func validateResolutionCenterMessageBody(messageBody string) (string, error) {
	if strings.TrimSpace(messageBody) == "" {
		return "", fmt.Errorf("message body is required")
	}
	return messageBody, nil
}

func decodeResolutionCenterWriteResource(responseBody []byte, operation, expectedType string) (jsonAPIResource, []jsonAPIResource, error) {
	var payload jsonAPISingleResourcePayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return jsonAPIResource{}, nil, fmt.Errorf("failed to parse %s response: %w", operation, err)
	}
	trimmedData := bytes.TrimSpace(payload.Data)
	if len(trimmedData) == 0 || bytes.Equal(trimmedData, []byte("null")) {
		return jsonAPIResource{}, nil, fmt.Errorf("%s response did not include a resource", operation)
	}
	var resource jsonAPIResource
	if err := json.Unmarshal(trimmedData, &resource); err != nil {
		return jsonAPIResource{}, nil, fmt.Errorf("failed to parse %s response: %w", operation, err)
	}
	if strings.TrimSpace(resource.ID) == "" {
		return jsonAPIResource{}, nil, fmt.Errorf("%s response did not include a resource id", operation)
	}
	if expectedType != "" && strings.TrimSpace(resource.Type) != expectedType {
		return jsonAPIResource{}, nil, fmt.Errorf("%s response returned unexpected resource type", operation)
	}
	return resource, payload.Included, nil
}

// CreateResolutionCenterDraftMessage creates the unsent draft that Apple uses
// as the source for a Resolution Center reply. The request shape mirrors the
// App Store Connect web client; the caller must explicitly decide when to send
// the resulting draft.
func (c *Client) CreateResolutionCenterDraftMessage(ctx context.Context, threadID, messageBody string) (*ResolutionCenterDraftMessage, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, fmt.Errorf("thread id is required")
	}
	messageBody, err := validateResolutionCenterMessageBody(messageBody)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "resolutionCenterDraftMessages",
			"attributes": map[string]any{
				"messageBody": messageBody,
			},
			"relationships": map[string]any{
				"resolutionCenterThread": map[string]any{
					"data": map[string]string{
						"type": "resolutionCenterThreads",
						"id":   threadID,
					},
				},
			},
		},
	}
	responseBody, err := c.doRequest(ctx, http.MethodPost, "/resolutionCenterDraftMessages", requestBody)
	if err != nil {
		return nil, err
	}
	resource, included, err := decodeResolutionCenterWriteResource(responseBody, "resolution center draft create", "resolutionCenterDraftMessages")
	if err != nil {
		return nil, err
	}
	return decodeResolutionCenterDraftMessage(resource, included, threadID, false), nil
}

// UpdateResolutionCenterDraftMessage updates an existing unsent Resolution
// Center draft. It is intentionally separate from sending so callers cannot
// accidentally turn a draft edit into a message.
func (c *Client) UpdateResolutionCenterDraftMessage(ctx context.Context, draftID, messageBody string) (*ResolutionCenterDraftMessage, error) {
	draftID = strings.TrimSpace(draftID)
	if draftID == "" {
		return nil, fmt.Errorf("draft id is required")
	}
	messageBody, err := validateResolutionCenterMessageBody(messageBody)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "resolutionCenterDraftMessages",
			"id":   draftID,
			"attributes": map[string]any{
				"messageBody": messageBody,
			},
		},
	}
	responseBody, err := c.doRequest(ctx, http.MethodPatch, "/resolutionCenterDraftMessages/"+url.PathEscape(draftID), requestBody)
	if err != nil {
		return nil, err
	}
	resource, included, err := decodeResolutionCenterWriteResource(responseBody, "resolution center draft update", "resolutionCenterDraftMessages")
	if err != nil {
		return nil, err
	}
	threadID := ""
	if ref := firstRelationshipRef(resource, "resolutionCenterThread"); ref != nil {
		threadID = strings.TrimSpace(ref.ID)
	}
	return decodeResolutionCenterDraftMessage(resource, included, threadID, false), nil
}

// SendResolutionCenterDraftMessage publishes a draft as a Resolution Center
// message. The response must contain the created message resource and ID so a
// caller can re-read and verify it. This method never retries because a failed
// response may follow a successful server-side send.
func (c *Client) SendResolutionCenterDraftMessage(ctx context.Context, draftID string) (*ResolutionCenterMessage, error) {
	draftID = strings.TrimSpace(draftID)
	if draftID == "" {
		return nil, fmt.Errorf("draft id is required")
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "resolutionCenterMessages",
			"relationships": map[string]any{
				"createFromDraftMessage": map[string]any{
					"data": map[string]string{
						"type": "resolutionCenterDraftMessages",
						"id":   draftID,
					},
				},
			},
		},
	}
	responseBody, err := c.doRequest(ctx, http.MethodPost, "/resolutionCenterMessages", requestBody)
	if err != nil {
		return nil, err
	}
	resource, included, err := decodeResolutionCenterWriteResource(responseBody, "resolution center message send", "resolutionCenterMessages")
	if err != nil {
		return nil, err
	}
	message := decodeResolutionCenterMessage(resource, buildIncludedMap(included), false)
	return &message, nil
}

func decodeResolutionCenterDraftMessage(resource jsonAPIResource, included []jsonAPIResource, threadID string, plainText bool) *ResolutionCenterDraftMessage {
	includedMap := buildIncludedMap(included)
	draft := &ResolutionCenterDraftMessage{
		ID:          strings.TrimSpace(resource.ID),
		ThreadID:    strings.TrimSpace(threadID),
		CreatedDate: stringAttr(resource.Attributes, "createdDate"),
		MessageBody: stringAttrPreserveWhitespace(resource.Attributes, "messageBody"),
		FromActor:   actorFromRef(firstRelationshipRef(resource, "fromActor"), includedMap),
	}
	if plainText {
		draft.MessageBodyPlain = htmlToPlainText(draft.MessageBody)
	}
	attachments := make([]ReviewAttachment, 0)
	for _, ref := range relationshipRefs(resource, "resolutionCenterMessageAttachments") {
		attachmentResource, ok := includedMap[jsonAPIResourceKey(ref.Type, ref.ID)]
		if !ok {
			attachmentResource = jsonAPIResource{ID: ref.ID, Type: ref.Type}
		}
		attachment := attachmentFromResource(attachmentResource, false)
		attachment.ThreadID = draft.ThreadID
		attachment.MessageID = draft.ID
		attachments = append(attachments, attachment)
	}
	if len(attachments) > 0 {
		draft.Attachments = attachments
	}
	return draft
}

func decodeResolutionCenterMessages(resources []jsonAPIResource, included []jsonAPIResource, plainText bool) []ResolutionCenterMessage {
	if len(resources) == 0 {
		return []ResolutionCenterMessage{}
	}
	includedMap := buildIncludedMap(included)
	messages := make([]ResolutionCenterMessage, 0, len(resources))
	for _, resource := range resources {
		messages = append(messages, decodeResolutionCenterMessage(resource, includedMap, plainText))
	}
	return messages
}

func decodeResolutionCenterMessage(resource jsonAPIResource, includedMap map[string]jsonAPIResource, plainText bool) ResolutionCenterMessage {
	message := ResolutionCenterMessage{
		ID:          strings.TrimSpace(resource.ID),
		CreatedDate: stringAttr(resource.Attributes, "createdDate"),
		MessageBody: stringAttr(resource.Attributes, "messageBody"),
	}
	if plainText {
		message.MessageBodyPlain = htmlToPlainText(message.MessageBody)
	}
	message.FromActor = actorFromRef(firstRelationshipRef(resource, "fromActor"), includedMap)
	rejectionRefs := relationshipRefs(resource, "rejections")
	if len(rejectionRefs) > 0 {
		message.RejectionIDs = make([]string, 0, len(rejectionRefs))
		for _, ref := range rejectionRefs {
			message.RejectionIDs = append(message.RejectionIDs, strings.TrimSpace(ref.ID))
		}
	}
	attachmentRefs := relationshipRefs(resource, "resolutionCenterMessageAttachments")
	if len(attachmentRefs) > 0 {
		message.AttachmentIDs = make([]string, 0, len(attachmentRefs))
		for _, ref := range attachmentRefs {
			message.AttachmentIDs = append(message.AttachmentIDs, strings.TrimSpace(ref.ID))
		}
	}
	return message
}

func (c *Client) listResolutionCenterMessagesPayload(ctx context.Context, threadID string) (jsonAPIListPayload, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return jsonAPIListPayload{}, fmt.Errorf("thread id is required")
	}
	query := url.Values{}
	query.Set("include", reviewMessagesInclude)
	query.Set("limit[rejections]", "2000")
	query.Set("limit[resolutionCenterMessageAttachments]", "1000")
	path := queryPath("/resolutionCenterThreads/"+url.PathEscape(threadID)+"/resolutionCenterMessages", query)

	return c.fetchJSONAPIPages(ctx, path, "resolution center messages")
}

// ListResolutionCenterMessages lists thread messages and optional plain text body.
func (c *Client) ListResolutionCenterMessages(ctx context.Context, threadID string, plainText bool) ([]ResolutionCenterMessage, error) {
	payload, err := c.listResolutionCenterMessagesPayload(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return decodeResolutionCenterMessages(payload.Data, payload.Included, plainText), nil
}

func parseRejectionReasons(attributes map[string]any) []ReviewRejectionReason {
	var rawReasons any
	switch {
	case attributes == nil:
		return nil
	case attributes["reasons"] != nil:
		rawReasons = attributes["reasons"]
	case attributes["reviewRejectionReasons"] != nil:
		rawReasons = attributes["reviewRejectionReasons"]
	default:
		return nil
	}
	array, ok := rawReasons.([]any)
	if !ok {
		return nil
	}
	reasons := make([]ReviewRejectionReason, 0, len(array))
	for _, entry := range array {
		reasonMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		reason := ReviewRejectionReason{
			ReasonSection:     stringAttr(reasonMap, "reasonSection"),
			ReasonDescription: stringAttr(reasonMap, "reasonDescription"),
			ReasonCode:        stringAttr(reasonMap, "reasonCode"),
		}
		if reason == (ReviewRejectionReason{}) {
			continue
		}
		reasons = append(reasons, reason)
	}
	return reasons
}

// listReviewRejectionsPayload fetches raw review rejection JSON:API payload for a thread.
func (c *Client) listReviewRejectionsPayload(ctx context.Context, threadID string) (jsonAPIListPayload, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return jsonAPIListPayload{}, fmt.Errorf("thread id is required")
	}
	query := url.Values{}
	query.Set("filter[resolutionCenterMessage.resolutionCenterThread]", threadID)
	query.Set("include", reviewRejectionsInclude)
	query.Set("limit", "2000")
	query.Set("limit[rejectionAttachments]", "1000")
	path := queryPath("/reviewRejections", query)

	return c.fetchJSONAPIPages(ctx, path, "review rejections")
}

func decodeReviewRejections(resources []jsonAPIResource, included []jsonAPIResource) []ReviewRejection {
	if len(resources) == 0 {
		return []ReviewRejection{}
	}
	includedMap := buildIncludedMap(included)
	rejections := make([]ReviewRejection, 0, len(resources))
	for _, resource := range resources {
		rejection := ReviewRejection{
			ID:      strings.TrimSpace(resource.ID),
			Reasons: parseRejectionReasons(resource.Attributes),
			Related: relatedResourcesFrom(resource, includedMap, skippedReviewRelatedRelationships),
		}
		attachmentRefs := relationshipRefs(resource, "rejectionAttachments")
		if len(attachmentRefs) > 0 {
			rejection.AttachmentIDs = make([]string, 0, len(attachmentRefs))
			for _, ref := range attachmentRefs {
				rejection.AttachmentIDs = append(rejection.AttachmentIDs, strings.TrimSpace(ref.ID))
			}
		}
		rejections = append(rejections, rejection)
	}
	return rejections
}

// ListReviewRejections lists review rejections associated with a thread.
func (c *Client) ListReviewRejections(ctx context.Context, threadID string) ([]ReviewRejection, error) {
	payload, err := c.listReviewRejectionsPayload(ctx, threadID)
	if err != nil {
		return nil, err
	}
	rejections := decodeReviewRejections(payload.Data, payload.Included)
	if len(rejections) == 0 {
		return []ReviewRejection{}, nil
	}
	return rejections, nil
}

func attachmentFromResource(resource jsonAPIResource, includeURL bool) ReviewAttachment {
	downloadURL := stringAttr(resource.Attributes, "downloadUrl", "downloadURL")
	attachment := ReviewAttachment{
		AttachmentID:       strings.TrimSpace(resource.ID),
		SourceType:         strings.TrimSpace(resource.Type),
		FileName:           stringAttr(resource.Attributes, "fileName"),
		FileSize:           int64Attr(resource.Attributes, "fileSize"),
		AssetDeliveryState: stringAttr(resource.Attributes, "assetDeliveryState"),
		Downloadable:       strings.TrimSpace(downloadURL) != "",
	}
	if includeURL {
		attachment.DownloadURL = downloadURL
	}
	return attachment
}

func hostMatchesPattern(host, pattern string) bool {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	if pattern == "" {
		return false
	}
	pattern = strings.TrimPrefix(pattern, "*")
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return false
	}
	if strings.HasPrefix(pattern, ".") {
		trimmed := strings.TrimPrefix(pattern, ".")
		return host == trimmed || strings.HasSuffix(host, pattern)
	}
	return host == pattern
}

func isAllowedAttachmentHost(host string) bool {
	for _, pattern := range defaultAttachmentHostSuffixes {
		if hostMatchesPattern(host, pattern) {
			return true
		}
	}
	extraHosts := strings.Split(os.Getenv(attachmentHostsEnv), ",")
	for _, pattern := range extraHosts {
		if hostMatchesPattern(host, pattern) {
			return true
		}
	}
	return false
}

func appendAttachmentUnique(attachments []ReviewAttachment, seen map[string]struct{}, attachment ReviewAttachment) []ReviewAttachment {
	key := strings.Join([]string{
		attachment.SourceType,
		attachment.AttachmentID,
		attachment.ThreadID,
		attachment.MessageID,
		attachment.ReviewRejectionID,
	}, "|")
	if _, ok := seen[key]; ok {
		return attachments
	}
	seen[key] = struct{}{}
	return append(attachments, attachment)
}

func attachmentsFromMessagesPayload(payload jsonAPIListPayload, threadID string, includeURL bool) []ReviewAttachment {
	attachments := make([]ReviewAttachment, 0)
	included := buildIncludedMap(payload.Included)
	for _, message := range payload.Data {
		for _, ref := range relationshipRefs(message, "resolutionCenterMessageAttachments") {
			resource, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
			if !ok {
				resource = jsonAPIResource{ID: ref.ID, Type: ref.Type}
			}
			attachment := attachmentFromResource(resource, includeURL)
			attachment.ThreadID = threadID
			attachment.MessageID = strings.TrimSpace(message.ID)
			attachments = append(attachments, attachment)
		}
	}
	return attachments
}

func attachmentsFromRejectionsPayload(payload jsonAPIListPayload, threadID string, includeURL bool) []ReviewAttachment {
	attachments := make([]ReviewAttachment, 0)
	included := buildIncludedMap(payload.Included)
	for _, rejection := range payload.Data {
		for _, ref := range relationshipRefs(rejection, "rejectionAttachments") {
			resource, ok := included[jsonAPIResourceKey(ref.Type, ref.ID)]
			if !ok {
				resource = jsonAPIResource{ID: ref.ID, Type: ref.Type}
			}
			attachment := attachmentFromResource(resource, includeURL)
			attachment.ThreadID = threadID
			attachment.ReviewRejectionID = strings.TrimSpace(rejection.ID)
			attachments = append(attachments, attachment)
		}
	}
	return attachments
}

// ListReviewThreadDetails fetches messages, rejections, and attachments for a thread in one pass.
func (c *Client) ListReviewThreadDetails(ctx context.Context, threadID string, plainText bool, includeURL bool) (ReviewThreadDetails, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return ReviewThreadDetails{}, fmt.Errorf("thread id is required")
	}
	messagesPayload, err := c.listResolutionCenterMessagesPayload(ctx, threadID)
	if err != nil {
		return ReviewThreadDetails{}, err
	}
	rejectionsPayload, err := c.listReviewRejectionsPayload(ctx, threadID)
	if err != nil {
		return ReviewThreadDetails{}, err
	}

	details := ReviewThreadDetails{
		Messages:   decodeResolutionCenterMessages(messagesPayload.Data, messagesPayload.Included, plainText),
		Rejections: decodeReviewRejections(rejectionsPayload.Data, rejectionsPayload.Included),
	}
	attachments := make([]ReviewAttachment, 0)
	seen := map[string]struct{}{}
	for _, attachment := range attachmentsFromMessagesPayload(messagesPayload, threadID, includeURL) {
		attachments = appendAttachmentUnique(attachments, seen, attachment)
	}
	for _, attachment := range attachmentsFromRejectionsPayload(rejectionsPayload, threadID, includeURL) {
		attachments = appendAttachmentUnique(attachments, seen, attachment)
	}
	details.Attachments = attachments
	return details, nil
}

// ListReviewAttachmentsByThread lists message and rejection attachments for a thread.
func (c *Client) ListReviewAttachmentsByThread(ctx context.Context, threadID string, includeURL bool) ([]ReviewAttachment, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, fmt.Errorf("thread id is required")
	}
	attachments := make([]ReviewAttachment, 0)
	seen := map[string]struct{}{}

	messagesPayload, err := c.listResolutionCenterMessagesPayload(ctx, threadID)
	if err != nil {
		return nil, err
	}
	for _, attachment := range attachmentsFromMessagesPayload(messagesPayload, threadID, includeURL) {
		attachments = appendAttachmentUnique(attachments, seen, attachment)
	}

	rejectionsPayload, err := c.listReviewRejectionsPayload(ctx, threadID)
	if err != nil {
		return nil, err
	}
	for _, attachment := range attachmentsFromRejectionsPayload(rejectionsPayload, threadID, includeURL) {
		attachments = appendAttachmentUnique(attachments, seen, attachment)
	}

	if len(attachments) == 0 {
		return []ReviewAttachment{}, nil
	}
	return attachments, nil
}

// ListReviewAttachmentsBySubmission aggregates attachments across submission threads.
func (c *Client) ListReviewAttachmentsBySubmission(ctx context.Context, reviewSubmissionID string, includeURL bool) ([]ReviewAttachment, error) {
	reviewSubmissionID = strings.TrimSpace(reviewSubmissionID)
	if reviewSubmissionID == "" {
		return nil, fmt.Errorf("review submission id is required")
	}
	threads, err := c.ListResolutionCenterThreadsBySubmission(ctx, reviewSubmissionID)
	if err != nil {
		return nil, err
	}
	all := make([]ReviewAttachment, 0)
	seen := map[string]struct{}{}
	for _, thread := range threads {
		threadAttachments, err := c.ListReviewAttachmentsByThread(ctx, thread.ID, includeURL)
		if err != nil {
			return nil, err
		}
		for _, attachment := range threadAttachments {
			all = appendAttachmentUnique(all, seen, attachment)
		}
	}
	if len(all) == 0 {
		return []ReviewAttachment{}, nil
	}
	return all, nil
}

// DownloadAttachment downloads binary attachment payload from a signed URL.
func (c *Client) DownloadAttachment(ctx context.Context, signedURL string) ([]byte, int, error) {
	signedURL = strings.TrimSpace(signedURL)
	if signedURL == "" {
		return nil, 0, fmt.Errorf("download url is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.httpClient == nil {
		return nil, 0, fmt.Errorf("web client is not initialized")
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, 0, err
	}
	parsedURL, err := url.ParseRequestURI(signedURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid download url")
	}
	if !strings.EqualFold(parsedURL.Scheme, "https") {
		return nil, 0, fmt.Errorf("download url must use https")
	}
	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	if host == "" {
		return nil, 0, fmt.Errorf("download url host is required")
	}
	if !isAllowedAttachmentHost(host) {
		return nil, 0, fmt.Errorf("download url host is not allowed")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create download request")
	}
	request.Header.Set("Accept", "*/*")
	copyJarCookiesToHeader(c.httpClient, request)

	httpClient := *c.httpClient
	// The initial request already has the authenticated session cookie copied
	// explicitly. Disable the jar on this one-purpose client so it neither
	// duplicates that cookie nor attaches session state to redirect targets.
	httpClient.Jar = nil
	previousCheckRedirect := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
		stripReviewAttachmentRedirectCredentials(redirect)
		if len(via) >= 10 {
			return &reviewAttachmentRedirectError{message: "download stopped after 10 redirects"}
		}
		if err := validateReviewAttachmentDownloadTarget(redirect.URL, "redirect"); err != nil {
			return err
		}
		if previousCheckRedirect != nil {
			if err := previousCheckRedirect(redirect, via); err != nil {
				return err
			}
			stripReviewAttachmentRedirectCredentials(redirect)
			// The wrapped policy receives the mutable upcoming request and may
			// have rewritten its URL; never send the request to an unchecked host.
			return validateReviewAttachmentDownloadTarget(redirect.URL, "redirect")
		}
		return nil
	}

	response, err := httpClient.Do(request)
	if err != nil {
		var redirectErr *reviewAttachmentRedirectError
		if errors.As(err, &redirectErr) {
			return nil, 0, redirectErr
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if errors.Is(urlErr.Err, context.Canceled) || errors.Is(urlErr.Err, context.DeadlineExceeded) {
				return nil, 0, urlErr.Err
			}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, 0, err
		}
		return nil, 0, urlsanitize.NewTransportError(
			"attachment download request",
			urlsanitize.RedactURLForError(signedURL),
			err,
		)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("failed to read download response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, response.StatusCode, fmt.Errorf("attachment download failed with status %d", response.StatusCode)
	}
	return body, response.StatusCode, nil
}

type reviewAttachmentRedirectError struct {
	message string
}

func (e *reviewAttachmentRedirectError) Error() string {
	return e.message
}

func stripReviewAttachmentRedirectCredentials(request *http.Request) {
	if request == nil {
		return
	}
	for name := range request.Header {
		if strings.EqualFold(name, "Cookie") || strings.EqualFold(name, "Referer") {
			delete(request.Header, name)
		}
	}
}

// validateReviewAttachmentDownloadTarget validates a redirect target without
// echoing its potentially signed URL in diagnostics.
func validateReviewAttachmentDownloadTarget(target *url.URL, kind string) error {
	if target == nil || strings.TrimSpace(target.Hostname()) == "" {
		return &reviewAttachmentRedirectError{message: fmt.Sprintf("download %s host is required", kind)}
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return &reviewAttachmentRedirectError{message: fmt.Sprintf("download %s must use https", kind)}
	}
	if !isAllowedAttachmentHost(target.Hostname()) {
		return &reviewAttachmentRedirectError{message: fmt.Sprintf("download %s host is not allowed", kind)}
	}
	return nil
}
