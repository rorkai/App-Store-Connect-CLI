package asc

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type bundleIDsQuery struct {
	listQuery
	names                      []string
	platforms                  []string
	identifier                 string
	seedIDs                    []string
	ids                        []string
	sort                       string
	fields                     []string
	profilesFields             []string
	bundleIDCapabilitiesFields []string
	appFields                  []string
	include                    []string
	profilesLimit              int
	bundleIDCapabilitiesLimit  int
	splitPagination            bool
	splitPaginationSet         bool
}

type bundleIDCapabilitiesQuery struct {
	listQuery
}

type passTypeIDCertificatesQuery struct {
	listQuery
	displayNames     []string
	certificateTypes []string
	serialNumbers    []string
	ids              []string
	sort             string
	fields           []string
}

type certificatesQuery struct {
	listQuery
	includedFieldsQuery
	displayNames     []string
	certificateTypes []string
	serialNumbers    []string
	ids              []string
	sort             string
	fields           []string
	passTypeIDFields []string
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
	includedFieldsQuery
	bundleID          string
	name              string
	ids               string
	profileTypes      []string
	profileStates     []string
	sort              string
	fields            []string
	bundleIDFields    []string
	deviceFields      []string
	certificateFields []string
	include           []string
	devicesLimit      int
	certificatesLimit int
}

type usersQuery struct {
	listQuery
	includedFieldsQuery
	email            string
	roles            []string
	visibleAppIDs    []string
	sort             string
	fields           []string
	appFields        []string
	include          []string
	visibleAppsLimit int
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

func buildBundleIDsQuery(query *bundleIDsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "filter[platform]", query.platforms)
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	addCSV(values, "filter[seedId]", query.seedIDs)
	addCSV(values, "filter[id]", query.ids)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[bundleIds]", query.fields)
	addCSV(values, "fields[profiles]", query.profilesFields)
	addCSV(values, "fields[bundleIdCapabilities]", query.bundleIDCapabilitiesFields)
	addCSV(values, "fields[apps]", query.appFields)
	addCSV(values, "include", query.include)
	if query.profilesLimit > 0 {
		values.Set("limit[profiles]", strconv.Itoa(query.profilesLimit))
	}
	if query.bundleIDCapabilitiesLimit > 0 {
		values.Set("limit[bundleIdCapabilities]", strconv.Itoa(query.bundleIDCapabilitiesLimit))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBundleIDCapabilitiesQuery(_ *bundleIDCapabilitiesQuery) string {
	// The live relationship endpoint rejects limit despite the OpenAPI entry.
	return ""
}

func buildCertificatesQuery(query *certificatesQuery) string {
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

func buildCertificateDetailQuery(query *certificatesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[certificates]", query.fields)
	addCSV(values, "fields[passTypeIds]", query.passTypeIDFields)
	addCSV(values, "include", query.include)
	query.includedFields.AddTo(values)
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfilesQuery(query *profilesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.name) != "" {
		values.Set("filter[name]", strings.TrimSpace(query.name))
	}
	if strings.TrimSpace(query.ids) != "" {
		values.Set("filter[id]", strings.TrimSpace(query.ids))
	}
	if strings.TrimSpace(query.bundleID) != "" {
		values.Set("filter[bundleId]", strings.TrimSpace(query.bundleID))
	}
	addCSV(values, "filter[profileType]", query.profileTypes)
	addCSV(values, "filter[profileState]", query.profileStates)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[profiles]", query.fields)
	addCSV(values, "fields[bundleIds]", query.bundleIDFields)
	addCSV(values, "fields[devices]", query.deviceFields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	if query.devicesLimit > 0 {
		values.Set("limit[devices]", strconv.Itoa(query.devicesLimit))
	}
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildUsersQuery(query *usersQuery) string {
	values := url.Values{}
	include := normalizeUniqueList(query.include)
	fields := query.fields
	if len(fields) > 0 {
		fields = normalizeUniqueList(append(append([]string{}, fields...), include...))
	}
	if strings.TrimSpace(query.email) != "" {
		values.Set("filter[username]", strings.TrimSpace(query.email))
	}
	addCSV(values, "filter[roles]", query.roles)
	addCSV(values, "filter[visibleApps]", query.visibleAppIDs)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[users]", fields)
	addCSV(values, "fields[apps]", query.appFields)
	addCSV(values, "include", include)
	if query.visibleAppsLimit > 0 {
		values.Set("limit[visibleApps]", strconv.Itoa(query.visibleAppsLimit))
	}
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
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
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
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

// BundleIDsOption is a functional option for GetBundleIDs.
type BundleIDsOption func(*bundleIDsQuery)

// BundleIDCapabilitiesOption is a functional option for GetBundleIDCapabilities.
type BundleIDCapabilitiesOption func(*bundleIDCapabilitiesQuery)

// MerchantIDCertificatesOption is a functional option for GetMerchantIDCertificates.
type MerchantIDCertificatesOption func(*merchantIDCertificatesQuery)

// PassTypeIDCertificatesOption is a functional option for GetPassTypeIDCertificates.
type PassTypeIDCertificatesOption func(*passTypeIDCertificatesQuery)

// CertificatesOption is a functional option for GetCertificates.
type CertificatesOption func(*certificatesQuery)

// DevicesOption is a functional option for GetDevices.
type DevicesOption func(*devicesQuery)

// ProfilesOption is a functional option for GetProfiles.
type ProfilesOption func(*profilesQuery)

// UsersOption is a functional option for GetUsers.
type UsersOption func(*usersQuery)

// WithBundleIDsSplitPagination controls whether split identifier requests follow links.next.
// It defaults to enabled for direct client callers.
func WithBundleIDsSplitPagination(enabled bool) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.splitPagination = enabled
		q.splitPaginationSet = true
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

// WithBundleIDsFilterNames filters bundle IDs by name(s).
func WithBundleIDsFilterNames(names []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.names = normalizeList(names)
	}
}

// WithBundleIDsFilterPlatforms filters bundle IDs by platform(s).
func WithBundleIDsFilterPlatforms(platforms []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithBundleIDsFilterSeedIDs filters bundle IDs by seed ID(s).
func WithBundleIDsFilterSeedIDs(seedIDs []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.seedIDs = normalizeList(seedIDs)
	}
}

// WithBundleIDsFilterIDs filters bundle IDs by ID(s).
func WithBundleIDsFilterIDs(ids []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithBundleIDsSort sets the sort order for bundle IDs.
func WithBundleIDsSort(sort string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBundleIDsFields sets fields[bundleIds] for bundle ID responses.
func WithBundleIDsFields(fields []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithBundleIDsProfilesFields sets fields[profiles] for included profiles.
func WithBundleIDsProfilesFields(fields []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.profilesFields = normalizeList(fields)
	}
}

// WithBundleIDsCapabilitiesFields sets fields[bundleIdCapabilities] for included capabilities.
func WithBundleIDsCapabilitiesFields(fields []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.bundleIDCapabilitiesFields = normalizeList(fields)
	}
}

// WithBundleIDsAppFields sets fields[apps] for the included app.
func WithBundleIDsAppFields(fields []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithBundleIDsInclude sets include for bundle ID responses.
func WithBundleIDsInclude(include []string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		q.include = normalizeList(include)
	}
}

// WithBundleIDsProfilesLimit sets the maximum number of included profiles.
func WithBundleIDsProfilesLimit(limit int) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if limit > 0 {
			q.profilesLimit = limit
		}
	}
}

// WithBundleIDsCapabilitiesLimit sets the maximum number of included capabilities.
func WithBundleIDsCapabilitiesLimit(limit int) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if limit > 0 {
			q.bundleIDCapabilitiesLimit = limit
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

// CertificateSerialKey returns the form of a certificate serial number for
// comparing serials, which ignores case and leading zeros.
func CertificateSerialKey(serial string) string {
	return strings.ToUpper(strings.TrimLeft(strings.TrimSpace(serial), "0"))
}

// withCertificateSerialsWithoutLeadingZeros adds the zero-stripped form of each
// serial number that starts with a zero, because a listed serial and the value
// its filter matches can differ by leading zeros. Lists without such a serial
// are returned unchanged.
func withCertificateSerialsWithoutLeadingZeros(serials []string) []string {
	hasLeadingZero := func(serial string) bool { return strings.HasPrefix(serial, "0") }
	if !slices.ContainsFunc(serials, hasLeadingZero) {
		return serials
	}
	seen := make(map[string]struct{}, 2*len(serials))
	expanded := make([]string, 0, 2*len(serials))
	for _, serial := range serials {
		for _, candidate := range []string{serial, strings.TrimLeft(serial, "0")} {
			if _, ok := seen[candidate]; ok || candidate == "" {
				continue
			}
			seen[candidate] = struct{}{}
			expanded = append(expanded, candidate)
		}
	}
	return expanded
}

// WithMerchantIDCertificatesFilterSerialNumbers filters certificates by serial number (supports CSV).
func WithMerchantIDCertificatesFilterSerialNumbers(serialNumbers string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(serialNumbers)
		if normalized != "" {
			q.serialNumber = strings.Join(withCertificateSerialsWithoutLeadingZeros(strings.Split(normalized, ",")), ",")
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
		q.serialNumbers = withCertificateSerialsWithoutLeadingZeros(normalizeList(serials))
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

// WithCertificatesTypes filters certificates by type.
func WithCertificatesTypes(types []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.certificateTypes = normalizeUpperList(types)
	}
}

// WithCertificatesFilterDisplayNames filters certificates by display name(s).
func WithCertificatesFilterDisplayNames(names []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.displayNames = normalizeList(names)
	}
}

// WithCertificatesFilterSerialNumbers filters certificates by serial number(s).
func WithCertificatesFilterSerialNumbers(serialNumbers []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.serialNumbers = withCertificateSerialsWithoutLeadingZeros(normalizeList(serialNumbers))
	}
}

// WithCertificatesFilterIDs filters certificates by ID(s).
func WithCertificatesFilterIDs(ids []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithCertificatesSort sets the certificate sort expression.
func WithCertificatesSort(sort string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.sort = strings.TrimSpace(sort)
	}
}

// WithCertificatesFields sets fields[certificates] for certificate responses.
func WithCertificatesFields(fields []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithCertificatesPassTypeIDFields sets fields[passTypeIds] for included pass type IDs.
func WithCertificatesPassTypeIDFields(fields []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.passTypeIDFields = normalizeList(fields)
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

// WithProfilesFilterName filters profiles by name (supports CSV).
func WithProfilesFilterName(name string) ProfilesOption {
	return func(q *profilesQuery) {
		normalized := normalizeCSVString(name)
		if normalized != "" {
			q.name = normalized
		}
	}
}

// WithProfilesFilterIDs filters profiles by ID(s).
func WithProfilesFilterIDs(ids []string) ProfilesOption {
	return func(q *profilesQuery) {
		normalized := normalizeList(ids)
		if len(normalized) > 0 {
			q.ids = strings.Join(normalized, ",")
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

// WithProfilesSort sets the sort order for profiles.
func WithProfilesSort(sort string) ProfilesOption {
	return func(q *profilesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithProfilesFields sets fields[profiles] for profile responses.
func WithProfilesFields(fields []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithProfilesBundleIDFields sets fields[bundleIds] for included bundle IDs.
func WithProfilesBundleIDFields(fields []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.bundleIDFields = normalizeList(fields)
	}
}

// WithProfilesDeviceFields sets fields[devices] for included devices.
func WithProfilesDeviceFields(fields []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.deviceFields = normalizeList(fields)
	}
}

// WithProfilesCertificateFields sets fields[certificates] for included certificates.
func WithProfilesCertificateFields(fields []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithProfilesInclude sets include for profile responses.
func WithProfilesInclude(include []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.include = normalizeList(include)
	}
}

// WithProfilesDevicesLimit sets limit[devices] for included devices.
func WithProfilesDevicesLimit(limit int) ProfilesOption {
	return func(q *profilesQuery) {
		if limit > 0 {
			q.devicesLimit = limit
		}
	}
}

// WithProfilesCertificatesLimit sets limit[certificates] for included certificates.
func WithProfilesCertificatesLimit(limit int) ProfilesOption {
	return func(q *profilesQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
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

// WithUsersEmail filters users by email/username.
func WithUsersEmail(email string) UsersOption {
	return func(q *usersQuery) {
		q.email = strings.TrimSpace(email)
	}
}

// WithUsersRoles filters users by roles.
func WithUsersRoles(roles []string) UsersOption {
	return func(q *usersQuery) {
		q.roles = normalizeUpperList(roles)
	}
}

// WithUsersVisibleAppIDs filters users by visible app ID(s).
func WithUsersVisibleAppIDs(appIDs []string) UsersOption {
	return func(q *usersQuery) {
		q.visibleAppIDs = normalizeList(appIDs)
	}
}

// WithUsersSort sets the user list sort order.
func WithUsersSort(sort string) UsersOption {
	return func(q *usersQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithUsersFields sets fields[users] for user responses.
func WithUsersFields(fields []string) UsersOption {
	return func(q *usersQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithUsersAppFields sets fields[apps] for included app responses.
func WithUsersAppFields(fields []string) UsersOption {
	return func(q *usersQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithUsersInclude sets include for user responses.
func WithUsersInclude(include []string) UsersOption {
	return func(q *usersQuery) {
		q.include = normalizeList(include)
	}
}

// WithUsersVisibleAppsLimit sets the maximum number of included visible apps.
func WithUsersVisibleAppsLimit(limit int) UsersOption {
	return func(q *usersQuery) {
		if limit > 0 {
			q.visibleAppsLimit = limit
		}
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

// WithDevicesNames filters devices by name(s).
func WithDevicesNames(names []string) DevicesOption {
	return func(q *devicesQuery) {
		q.names = normalizeList(names)
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
