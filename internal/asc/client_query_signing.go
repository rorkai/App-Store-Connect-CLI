package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type bundleIDsQuery struct {
	listQuery
	identifier string
}

type merchantIDsQuery struct {
	listQuery
	name              string
	identifier        string
	sort              string
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type bundleIDCapabilitiesQuery struct {
	listQuery
}

type passTypeIDsQuery struct {
	listQuery
	ids               string
	identifier        string
	name              string
	sort              string
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type passTypeIDQuery struct {
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type passTypeIDCertificatesQuery struct {
	listQuery
	displayNames     []string
	certificateTypes []string
	serialNumbers    []string
	ids              []string
	sort             string
	fields           []string
	passTypeIDFields []string
	include          []string
}

type certificatesQuery struct {
	listQuery
	certificateTypes []string
	include          []string
}

type merchantIDCertificatesQuery struct {
	listQuery
	displayName     string
	certificateType string
	serialNumber    string
	ids             string
	sort            string
	fields          []string
	passTypeFields  []string
	include         []string
}

type profilesQuery struct {
	listQuery
	bundleID      string
	profileTypes  []string
	profileStates []string
	include       []string
}

type usersQuery struct {
	listQuery
	email   string
	roles   []string
	include []string
}

type profileCertificatesQuery struct {
	listQuery
}

type profileDevicesQuery struct {
	listQuery
}

type bundleIDProfilesQuery struct {
	listQuery
}

type userVisibleAppsQuery struct {
	listQuery
}

type userInvitationVisibleAppsQuery struct {
	listQuery
}

type actorsQuery struct {
	listQuery
	ids    []string
	fields []string
}

type devicesQuery struct {
	listQuery
	names     []string
	platforms []string
	status    string
	udids     []string
	ids       []string
	sort      string
	fields    []string
}

type userInvitationsQuery struct {
	listQuery
}

func buildBundleIDsQuery(query *bundleIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildMerchantIDsQuery(query *merchantIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.name) != "" {
		values.Set("filter[name]", strings.TrimSpace(query.name))
	}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[merchantIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPassTypeIDsQuery(query *passTypeIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.ids) != "" {
		values.Set("filter[id]", strings.TrimSpace(query.ids))
	}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	if strings.TrimSpace(query.name) != "" {
		values.Set("filter[name]", strings.TrimSpace(query.name))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[passTypeIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	return values.Encode()
}

func buildPassTypeIDQuery(query *passTypeIDQuery) string {
	values := url.Values{}
	addCSV(values, "fields[passTypeIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	return values.Encode()
}

func buildBundleIDCapabilitiesQuery(_ *bundleIDCapabilitiesQuery) string {
	// Bundle ID capabilities endpoint does not support limit/pagination params.
	return ""
}

func buildCertificatesQuery(query *certificatesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[certificateType]", query.certificateTypes)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildMerchantIDCertificatesQuery(query *merchantIDCertificatesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.displayName) != "" {
		values.Set("filter[displayName]", strings.TrimSpace(query.displayName))
	}
	if strings.TrimSpace(query.certificateType) != "" {
		values.Set("filter[certificateType]", strings.TrimSpace(query.certificateType))
	}
	if strings.TrimSpace(query.serialNumber) != "" {
		values.Set("filter[serialNumber]", strings.TrimSpace(query.serialNumber))
	}
	if strings.TrimSpace(query.ids) != "" {
		values.Set("filter[id]", strings.TrimSpace(query.ids))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[certificates]", query.fields)
	addCSV(values, "fields[passTypeIds]", query.passTypeFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPassTypeIDCertificatesQuery(query *passTypeIDCertificatesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[displayName]", query.displayNames)
	addCSV(values, "filter[certificateType]", query.certificateTypes)
	addCSV(values, "filter[serialNumber]", query.serialNumbers)
	addCSV(values, "filter[id]", query.ids)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[certificates]", query.fields)
	addCSV(values, "fields[passTypeIds]", query.passTypeIDFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfilesQuery(query *profilesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.bundleID) != "" {
		values.Set("filter[bundleId]", strings.TrimSpace(query.bundleID))
	}
	addCSV(values, "filter[profileType]", query.profileTypes)
	addCSV(values, "filter[profileState]", query.profileStates)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildUsersQuery(query *usersQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.email) != "" {
		values.Set("filter[username]", strings.TrimSpace(query.email))
	}
	addCSV(values, "filter[roles]", query.roles)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfileCertificatesQuery(query *profileCertificatesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfileDevicesQuery(query *profileDevicesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBundleIDProfilesQuery(query *bundleIDProfilesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildUserVisibleAppsQuery(query *userVisibleAppsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildUserInvitationVisibleAppsQuery(query *userInvitationVisibleAppsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildActorsQuery(query *actorsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[id]", query.ids)
	addCSV(values, "fields[actors]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildActorsFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[actors]", fields)
	return values.Encode()
}

func buildDevicesQuery(query *devicesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "filter[platform]", query.platforms)
	if strings.TrimSpace(query.status) != "" {
		values.Set("filter[status]", strings.TrimSpace(query.status))
	}
	addCSV(values, "filter[udid]", query.udids)
	addCSV(values, "filter[id]", query.ids)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addCSV(values, "fields[devices]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildDevicesFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[devices]", fields)
	return values.Encode()
}

func buildUserInvitationsQuery(query *userInvitationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// BundleIDsOption is a functional option for GetBundleIDs.
type BundleIDsOption func(*bundleIDsQuery)

// BundleIDCapabilitiesOption is a functional option for GetBundleIDCapabilities.
type BundleIDCapabilitiesOption func(*bundleIDCapabilitiesQuery)

// MerchantIDsOption is a functional option for GetMerchantIDs.
type MerchantIDsOption func(*merchantIDsQuery)

// MerchantIDCertificatesOption is a functional option for GetMerchantIDCertificates.
type MerchantIDCertificatesOption func(*merchantIDCertificatesQuery)

// PassTypeIDsOption is a functional option for GetPassTypeIDs.
type PassTypeIDsOption func(*passTypeIDsQuery)

// PassTypeIDOption is a functional option for GetPassTypeID.
type PassTypeIDOption func(*passTypeIDQuery)

// PassTypeIDCertificatesOption is a functional option for GetPassTypeIDCertificates.
type PassTypeIDCertificatesOption func(*passTypeIDCertificatesQuery)

// CertificatesOption is a functional option for GetCertificates.
type CertificatesOption func(*certificatesQuery)

// DevicesOption is a functional option for GetDevices.
type DevicesOption func(*devicesQuery)

// ProfilesOption is a functional option for GetProfiles.
type ProfilesOption func(*profilesQuery)

// BundleIDProfilesOption is a functional option for GetBundleIDProfiles.
type BundleIDProfilesOption func(*bundleIDProfilesQuery)

// UsersOption is a functional option for GetUsers.
type UsersOption func(*usersQuery)

// ProfileCertificatesOption is a functional option for GetProfileCertificates.
type ProfileCertificatesOption func(*profileCertificatesQuery)

// ProfileDevicesOption is a functional option for GetProfileDevices.
type ProfileDevicesOption func(*profileDevicesQuery)

// UserVisibleAppsOption is a functional option for GetUserVisibleApps.
type UserVisibleAppsOption func(*userVisibleAppsQuery)

// UserInvitationVisibleAppsOption is a functional option for GetUserInvitationVisibleApps.
type UserInvitationVisibleAppsOption func(*userInvitationVisibleAppsQuery)

// ActorsOption is a functional option for GetActors.
type ActorsOption func(*actorsQuery)

// UserInvitationsOption is a functional option for GetUserInvitations.
type UserInvitationsOption func(*userInvitationsQuery)

// WithBundleIDsLimit sets the max number of bundle IDs to return.
func WithBundleIDsLimit(limit int) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBundleIDsNextURL uses a next page URL directly.
func WithBundleIDsNextURL(next string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBundleIDProfilesLimit sets the max number of bundle ID profiles to return.
func WithBundleIDProfilesLimit(limit int) BundleIDProfilesOption {
	return func(q *bundleIDProfilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBundleIDProfilesNextURL uses a next page URL directly.
func WithBundleIDProfilesNextURL(next string) BundleIDProfilesOption {
	return func(q *bundleIDProfilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBundleIDsFilterIdentifier filters bundle IDs by identifier (supports CSV).
func WithBundleIDsFilterIdentifier(identifier string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithMerchantIDsLimit sets the max number of merchant IDs to return.
func WithMerchantIDsLimit(limit int) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithMerchantIDsNextURL uses a next page URL directly.
func WithMerchantIDsNextURL(next string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMerchantIDsFilterIdentifier filters merchant IDs by identifier (supports CSV).
func WithMerchantIDsFilterIdentifier(identifier string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithMerchantIDsFilterName filters merchant IDs by name (supports CSV).
func WithMerchantIDsFilterName(name string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		normalized := normalizeCSVString(name)
		if normalized != "" {
			q.name = normalized
		}
	}
}

// WithMerchantIDsSort sets the sort order for merchant IDs.
func WithMerchantIDsSort(sort string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithMerchantIDsFields sets fields[merchantIds] for merchant ID responses.
func WithMerchantIDsFields(fields []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithMerchantIDsCertificateFields sets fields[certificates] for included certificates.
func WithMerchantIDsCertificateFields(fields []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithMerchantIDsInclude sets include for merchant ID responses.
func WithMerchantIDsInclude(include []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.include = normalizeList(include)
	}
}

// WithMerchantIDsCertificatesLimit sets limit[certificates] for included certificates.
func WithMerchantIDsCertificatesLimit(limit int) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithMerchantIDCertificatesLimit sets the max number of certificates to return.
func WithMerchantIDCertificatesLimit(limit int) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithMerchantIDCertificatesNextURL uses a next page URL directly.
func WithMerchantIDCertificatesNextURL(next string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMerchantIDCertificatesFilterDisplayName filters certificates by display name (supports CSV).
func WithMerchantIDCertificatesFilterDisplayName(displayName string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(displayName)
		if normalized != "" {
			q.displayName = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterCertificateTypes filters certificates by type (supports CSV).
func WithMerchantIDCertificatesFilterCertificateTypes(types string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeUpperCSVString(types)
		if normalized != "" {
			q.certificateType = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterSerialNumbers filters certificates by serial number (supports CSV).
func WithMerchantIDCertificatesFilterSerialNumbers(serialNumbers string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(serialNumbers)
		if normalized != "" {
			q.serialNumber = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterIDs filters certificates by ID (supports CSV).
func WithMerchantIDCertificatesFilterIDs(ids string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(ids)
		if normalized != "" {
			q.ids = normalized
		}
	}
}

// WithMerchantIDCertificatesSort sets the sort order for merchant ID certificates.
func WithMerchantIDCertificatesSort(sort string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithMerchantIDCertificatesFields sets fields[certificates] for certificate responses.
func WithMerchantIDCertificatesFields(fields []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithMerchantIDCertificatesPassTypeFields sets fields[passTypeIds] for included pass type IDs.
func WithMerchantIDCertificatesPassTypeFields(fields []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.passTypeFields = normalizeList(fields)
	}
}

// WithMerchantIDCertificatesInclude sets include for merchant ID certificates responses.
func WithMerchantIDCertificatesInclude(include []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDsLimit sets the max number of pass type IDs to return.
func WithPassTypeIDsLimit(limit int) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPassTypeIDsNextURL uses a next page URL directly.
func WithPassTypeIDsNextURL(next string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPassTypeIDsFilterIDs filters pass type IDs by ID(s).
func WithPassTypeIDsFilterIDs(ids []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeList(ids)
		if len(normalized) > 0 {
			q.ids = strings.Join(normalized, ",")
		}
	}
}

// WithPassTypeIDsFilterName filters pass type IDs by name (supports CSV).
func WithPassTypeIDsFilterName(name string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeCSVString(name)
		if normalized != "" {
			q.name = normalized
		}
	}
}

// WithPassTypeIDsFilterIdentifier filters pass type IDs by identifier (supports CSV).
func WithPassTypeIDsFilterIdentifier(identifier string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithPassTypeIDsSort sets the sort order for pass type IDs.
func WithPassTypeIDsSort(sort string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithPassTypeIDsFields sets fields[passTypeIds] for pass type ID responses.
func WithPassTypeIDsFields(fields []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDsCertificateFields sets fields[certificates] for included certificates.
func WithPassTypeIDsCertificateFields(fields []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithPassTypeIDsInclude sets include for pass type ID responses.
func WithPassTypeIDsInclude(include []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDsCertificatesLimit sets limit[certificates] for included certificates.
func WithPassTypeIDsCertificatesLimit(limit int) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithPassTypeIDFields sets fields[passTypeIds] for pass type ID responses.
func WithPassTypeIDFields(fields []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificateFields sets fields[certificates] for included certificates.
func WithPassTypeIDCertificateFields(fields []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithPassTypeIDInclude sets include for pass type ID responses.
func WithPassTypeIDInclude(include []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDCertificatesIncludeLimit sets limit[certificates] for included certificates.
func WithPassTypeIDCertificatesIncludeLimit(limit int) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithPassTypeIDCertificatesLimit sets the max number of certificates to return.
func WithPassTypeIDCertificatesLimit(limit int) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPassTypeIDCertificatesNextURL uses a next page URL directly.
func WithPassTypeIDCertificatesNextURL(next string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPassTypeIDCertificatesFilterDisplayNames filters certificates by display name(s).
func WithPassTypeIDCertificatesFilterDisplayNames(names []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.displayNames = normalizeList(names)
	}
}

// WithPassTypeIDCertificatesFilterCertificateTypes filters certificates by type(s).
func WithPassTypeIDCertificatesFilterCertificateTypes(types []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.certificateTypes = normalizeUpperList(types)
	}
}

// WithPassTypeIDCertificatesFilterSerialNumbers filters certificates by serial number(s).
func WithPassTypeIDCertificatesFilterSerialNumbers(serials []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.serialNumbers = normalizeList(serials)
	}
}

// WithPassTypeIDCertificatesFilterIDs filters certificates by ID(s).
func WithPassTypeIDCertificatesFilterIDs(ids []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithPassTypeIDCertificatesSort sets the sort order for certificates.
func WithPassTypeIDCertificatesSort(sort string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithPassTypeIDCertificatesFields sets fields[certificates] for certificate responses.
func WithPassTypeIDCertificatesFields(fields []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificatesPassTypeIDFields sets fields[passTypeIds] for included pass type IDs.
func WithPassTypeIDCertificatesPassTypeIDFields(fields []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.passTypeIDFields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificatesInclude sets include for certificate responses.
func WithPassTypeIDCertificatesInclude(include []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithBundleIDCapabilitiesNextURL uses a next page URL directly.
func WithBundleIDCapabilitiesNextURL(next string) BundleIDCapabilitiesOption {
	return func(q *bundleIDCapabilitiesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCertificatesLimit sets the max number of certificates to return.
func WithCertificatesLimit(limit int) CertificatesOption {
	return func(q *certificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCertificatesNextURL uses a next page URL directly.
func WithCertificatesNextURL(next string) CertificatesOption {
	return func(q *certificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCertificatesTypes filters certificates by type.
func WithCertificatesTypes(types []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.certificateTypes = normalizeUpperList(types)
	}
}

// WithCertificatesInclude sets include for certificate responses.
func WithCertificatesInclude(include []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithCertificatesFilterType filters certificates by certificate type (supports CSV).
func WithCertificatesFilterType(certType string) CertificatesOption {
	return func(q *certificatesQuery) {
		normalized := normalizeUpperCSVString(certType)
		if normalized == "" {
			return
		}
		q.certificateTypes = strings.Split(normalized, ",")
	}
}

// WithProfilesLimit sets the max number of profiles to return.
func WithProfilesLimit(limit int) ProfilesOption {
	return func(q *profilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfilesNextURL uses a next page URL directly.
func WithProfilesNextURL(next string) ProfilesOption {
	return func(q *profilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithProfilesTypes filters profiles by profile type.
func WithProfilesTypes(types []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.profileTypes = normalizeUpperList(types)
	}
}

// WithProfilesStates filters profiles by profile state.
func WithProfilesStates(states []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.profileStates = normalizeUpperList(states)
	}
}

// WithProfilesInclude sets include for profile responses.
func WithProfilesInclude(include []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.include = normalizeList(include)
	}
}

// WithProfilesFilterType filters profiles by profile type (supports CSV).
func WithProfilesFilterType(profileType string) ProfilesOption {
	return func(q *profilesQuery) {
		normalized := normalizeUpperCSVString(profileType)
		if normalized == "" {
			return
		}
		q.profileTypes = strings.Split(normalized, ",")
	}
}

// WithProfileCertificatesLimit sets the max number of profile certificates to return.
func WithProfileCertificatesLimit(limit int) ProfileCertificatesOption {
	return func(q *profileCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfileCertificatesNextURL uses a next page URL directly.
func WithProfileCertificatesNextURL(next string) ProfileCertificatesOption {
	return func(q *profileCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithProfileDevicesLimit sets the max number of profile devices to return.
func WithProfileDevicesLimit(limit int) ProfileDevicesOption {
	return func(q *profileDevicesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfileDevicesNextURL uses a next page URL directly.
func WithProfileDevicesNextURL(next string) ProfileDevicesOption {
	return func(q *profileDevicesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithUsersLimit sets the max number of users to return.
func WithUsersLimit(limit int) UsersOption {
	return func(q *usersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUsersNextURL uses a next page URL directly.
func WithUsersNextURL(next string) UsersOption {
	return func(q *usersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithUsersEmail filters users by email/username.
func WithUsersEmail(email string) UsersOption {
	return func(q *usersQuery) {
		q.email = strings.TrimSpace(email)
	}
}

// WithUsersRoles filters users by roles.
func WithUsersRoles(roles []string) UsersOption {
	return func(q *usersQuery) {
		q.roles = normalizeList(roles)
	}
}

// WithUsersInclude sets include for user responses.
func WithUsersInclude(include []string) UsersOption {
	return func(q *usersQuery) {
		q.include = normalizeList(include)
	}
}

// WithUserVisibleAppsLimit sets the max number of visible apps to return.
func WithUserVisibleAppsLimit(limit int) UserVisibleAppsOption {
	return func(q *userVisibleAppsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUserVisibleAppsNextURL uses a next page URL directly.
func WithUserVisibleAppsNextURL(next string) UserVisibleAppsOption {
	return func(q *userVisibleAppsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithUserInvitationVisibleAppsLimit sets the max number of invitation-visible apps to return.
func WithUserInvitationVisibleAppsLimit(limit int) UserInvitationVisibleAppsOption {
	return func(q *userInvitationVisibleAppsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUserInvitationVisibleAppsNextURL uses a next page URL directly.
func WithUserInvitationVisibleAppsNextURL(next string) UserInvitationVisibleAppsOption {
	return func(q *userInvitationVisibleAppsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithActorsLimit sets the max number of actors to return.
func WithActorsLimit(limit int) ActorsOption {
	return func(q *actorsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithActorsNextURL uses a next page URL directly.
func WithActorsNextURL(next string) ActorsOption {
	return func(q *actorsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithActorsIDs filters actors by id(s).
func WithActorsIDs(ids []string) ActorsOption {
	return func(q *actorsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithActorsFields limits actor fields in the response.
func WithActorsFields(fields []string) ActorsOption {
	return func(q *actorsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithDevicesLimit sets the max number of devices to return.
func WithDevicesLimit(limit int) DevicesOption {
	return func(q *devicesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithDevicesFilterUDIDs filters devices by UDID(s).
func WithDevicesFilterUDIDs(udids []string) DevicesOption {
	return WithDevicesUDIDs(udids)
}

// WithDevicesFilterPlatforms filters devices by platform(s).
func WithDevicesFilterPlatforms(platforms []string) DevicesOption {
	return func(q *devicesQuery) {
		normalized := normalizeUpperList(platforms)
		if len(normalized) == 0 {
			return
		}
		q.platforms = normalized
	}
}

// WithDevicesFilterStatuses filters devices by status (e.g., ENABLED, DISABLED).
func WithDevicesFilterStatuses(statuses []string) DevicesOption {
	return func(q *devicesQuery) {
		normalized := normalizeUpperList(statuses)
		if len(normalized) == 0 {
			return
		}
		q.status = strings.Join(normalized, ",")
	}
}

// WithDevicesNextURL uses a next page URL directly.
func WithDevicesNextURL(next string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithDevicesNames filters devices by name(s).
func WithDevicesNames(names []string) DevicesOption {
	return func(q *devicesQuery) {
		q.names = normalizeList(names)
	}
}

// WithDevicesPlatform filters devices by platform.
func WithDevicesPlatform(platform string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(platform) != "" {
			q.platforms = []string{strings.ToUpper(strings.TrimSpace(platform))}
		}
	}
}

// WithDevicesPlatforms filters devices by platform(s).
func WithDevicesPlatforms(platforms []string) DevicesOption {
	return func(q *devicesQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithDevicesStatus filters devices by status.
func WithDevicesStatus(status string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(status) != "" {
			q.status = strings.ToUpper(strings.TrimSpace(status))
		}
	}
}

// WithDevicesUDIDs filters devices by UDID(s).
func WithDevicesUDIDs(udids []string) DevicesOption {
	return func(q *devicesQuery) {
		q.udids = normalizeList(udids)
	}
}

// WithDevicesIDs filters devices by ID(s).
func WithDevicesIDs(ids []string) DevicesOption {
	return func(q *devicesQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithDevicesSort sets the sort order for devices.
func WithDevicesSort(sort string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithDevicesFields sets fields[devices] for device responses.
func WithDevicesFields(fields []string) DevicesOption {
	return func(q *devicesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithUserInvitationsLimit sets the max number of invitations to return.
func WithUserInvitationsLimit(limit int) UserInvitationsOption {
	return func(q *userInvitationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUserInvitationsNextURL uses a next page URL directly.
func WithUserInvitationsNextURL(next string) UserInvitationsOption {
	return func(q *userInvitationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}
