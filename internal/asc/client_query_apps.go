package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type appClipDefaultExperienceQuery struct {
	includedFieldsQuery
	include []string
}

type nominationsQuery struct {
	listQuery
	includedFieldsQuery
	types                     []string
	states                    []string
	relatedApps               []string
	sort                      string
	fields                    []string
	include                   []string
	inAppEventsLimit          int
	relatedAppsLimit          int
	supportedTerritoriesLimit int
}

type appEncryptionDeclarationsQuery struct {
	listQuery
	includedFieldsQuery
	appID          string
	buildIDs       []string
	fields         []string
	documentFields []string
	include        []string
	buildLimit     int
}

func buildAppClipDefaultExperienceQuery(query *appClipDefaultExperienceQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildNominationsQuery(query *nominationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[type]", query.types)
	addCSV(values, "filter[state]", query.states)
	addCSV(values, "filter[relatedApps]", query.relatedApps)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[nominations]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.inAppEventsLimit > 0 {
		values.Set("limit[inAppEvents]", strconv.Itoa(query.inAppEventsLimit))
	}
	if query.relatedAppsLimit > 0 {
		values.Set("limit[relatedApps]", strconv.Itoa(query.relatedAppsLimit))
	}
	if query.supportedTerritoriesLimit > 0 {
		values.Set("limit[supportedTerritories]", strconv.Itoa(query.supportedTerritoriesLimit))
	}
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildNominationsDetailQuery(query *nominationsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[nominations]", query.fields)
	addCSV(values, "include", query.include)
	if query.inAppEventsLimit > 0 {
		values.Set("limit[inAppEvents]", strconv.Itoa(query.inAppEventsLimit))
	}
	if query.relatedAppsLimit > 0 {
		values.Set("limit[relatedApps]", strconv.Itoa(query.relatedAppsLimit))
	}
	if query.supportedTerritoriesLimit > 0 {
		values.Set("limit[supportedTerritories]", strconv.Itoa(query.supportedTerritoriesLimit))
	}
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAccessibilityDeclarationsFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[accessibilityDeclarations]", fields)
	return values.Encode()
}

func buildAppEncryptionDeclarationsQuery(query *appEncryptionDeclarationsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(query.appID))
	}
	addCSV(values, "filter[builds]", query.buildIDs)
	addCSV(values, "fields[appEncryptionDeclarations]", query.fields)
	addCSV(values, "fields[appEncryptionDeclarationDocuments]", query.documentFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.buildLimit > 0 {
		values.Set("limit[builds]", strconv.Itoa(query.buildLimit))
	}
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppEncryptionDeclarationDocumentFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[appEncryptionDeclarationDocuments]", fields)
	return values.Encode()
}

// AppClipDefaultExperienceOption is a functional option for GetAppClipDefaultExperience.
type AppClipDefaultExperienceOption func(*appClipDefaultExperienceQuery)

// NominationsOption is a functional option for nominations endpoints.
type NominationsOption func(*nominationsQuery)

// AppEncryptionDeclarationsOption is a functional option for encryption declarations.
type AppEncryptionDeclarationsOption func(*appEncryptionDeclarationsQuery)

// WithAppEncryptionDeclarationsBuildIDs filters declarations by build IDs.
func WithAppEncryptionDeclarationsBuildIDs(ids []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithAppEncryptionDeclarationsFields includes specific declaration fields.
func WithAppEncryptionDeclarationsFields(fields []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppEncryptionDeclarationsDocumentFields includes document fields when included.
func WithAppEncryptionDeclarationsDocumentFields(fields []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.documentFields = normalizeList(fields)
	}
}

// WithAppEncryptionDeclarationsInclude includes related resources.
func WithAppEncryptionDeclarationsInclude(include []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppEncryptionDeclarationsBuildLimit sets the max number of related builds when included.
func WithAppEncryptionDeclarationsBuildLimit(limit int) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		if limit > 0 {
			q.buildLimit = limit
		}
	}
}

// WithAppClipDefaultExperienceInclude sets include for default experience detail.
func WithAppClipDefaultExperienceInclude(include []string) AppClipDefaultExperienceOption {
	return func(q *appClipDefaultExperienceQuery) {
		q.include = normalizeList(include)
	}
}

// WithNominationsTypes filters nominations by type.
func WithNominationsTypes(types []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.types = normalizeUpperList(types)
	}
}

// WithNominationsStates filters nominations by state.
func WithNominationsStates(states []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithNominationsRelatedApps filters nominations by related app ID(s).
func WithNominationsRelatedApps(appIDs []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.relatedApps = normalizeList(appIDs)
	}
}

// WithNominationsSort sets the sort order for nominations.
func WithNominationsSort(sort string) NominationsOption {
	return func(q *nominationsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithNominationsFields sets fields[nominations] for nominations responses.
func WithNominationsFields(fields []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithNominationsInclude sets include for nominations responses.
func WithNominationsInclude(include []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithNominationsInAppEventsLimit sets limit[inAppEvents] for included in-app events.
func WithNominationsInAppEventsLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.inAppEventsLimit = limit
		}
	}
}

// WithNominationsRelatedAppsLimit sets limit[relatedApps] for included related apps.
func WithNominationsRelatedAppsLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.relatedAppsLimit = limit
		}
	}
}

// WithNominationsSupportedTerritoriesLimit sets limit[supportedTerritories] for included territories.
func WithNominationsSupportedTerritoriesLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.supportedTerritoriesLimit = limit
		}
	}
}
