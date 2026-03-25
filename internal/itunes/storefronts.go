// Package itunes provides a client for the iTunes Lookup API.
package itunes

import (
	"sort"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Storefronts maps a subset of public App Store country codes to Apple storefront IDs.
// These IDs are required for the X-Apple-Store-Front header when fetching ratings histograms.
var Storefronts = map[string]string{
	"ae": "143481", // United Arab Emirates
	"ai": "143538", // Anguilla
	"am": "143524", // Armenia
	"ao": "143564", // Angola
	"ar": "143505", // Argentina
	"at": "143445", // Austria
	"au": "143460", // Australia
	"az": "143568", // Azerbaijan
	"bb": "143541", // Barbados
	"be": "143446", // Belgium
	"bg": "143526", // Bulgaria
	"bh": "143559", // Bahrain
	"bm": "143542", // Bermuda
	"bn": "143560", // Brunei
	"bo": "143556", // Bolivia
	"br": "143503", // Brazil
	"bw": "143525", // Botswana
	"by": "143565", // Belarus
	"bz": "143555", // Belize
	"ca": "143455", // Canada
	"ch": "143459", // Switzerland
	"cl": "143483", // Chile
	"cn": "143465", // China
	"co": "143501", // Colombia
	"cr": "143495", // Costa Rica
	"cy": "143557", // Cyprus
	"cz": "143489", // Czech Republic
	"de": "143443", // Germany
	"dk": "143458", // Denmark
	"dm": "143545", // Dominica
	"dz": "143563", // Algeria
	"ec": "143509", // Ecuador
	"ee": "143518", // Estonia
	"eg": "143516", // Egypt
	"es": "143454", // Spain
	"fi": "143447", // Finland
	"fr": "143442", // France
	"gb": "143444", // United Kingdom
	"gd": "143546", // Grenada
	"gh": "143573", // Ghana
	"gr": "143448", // Greece
	"gt": "143504", // Guatemala
	"gy": "143553", // Guyana
	"hk": "143463", // Hong Kong
	"hn": "143510", // Honduras
	"hr": "143494", // Croatia
	"hu": "143482", // Hungary
	"id": "143476", // Indonesia
	"ie": "143449", // Ireland
	"il": "143491", // Israel
	"in": "143467", // India
	"is": "143558", // Iceland
	"it": "143450", // Italy
	"jm": "143511", // Jamaica
	"jo": "143528", // Jordan
	"jp": "143462", // Japan
	"ke": "143529", // Kenya
	"kr": "143466", // South Korea
	"kw": "143493", // Kuwait
	"ky": "143544", // Cayman Islands
	"lb": "143497", // Lebanon
	"lk": "143486", // Sri Lanka
	"lt": "143520", // Lithuania
	"lu": "143451", // Luxembourg
	"lv": "143519", // Latvia
	"mg": "143531", // Madagascar
	"mk": "143530", // Macedonia
	"ml": "143532", // Mali
	"mo": "143515", // Macao
	"ms": "143547", // Montserrat
	"mt": "143521", // Malta
	"mu": "143533", // Mauritius
	"mx": "143468", // Mexico
	"my": "143473", // Malaysia
	"ne": "143534", // Niger
	"ng": "143561", // Nigeria
	"ni": "143512", // Nicaragua
	"nl": "143452", // Netherlands
	"no": "143457", // Norway
	"np": "143484", // Nepal
	"nz": "143461", // New Zealand
	"om": "143562", // Oman
	"pa": "143485", // Panama
	"pe": "143507", // Peru
	"ph": "143474", // Philippines
	"pk": "143477", // Pakistan
	"pl": "143478", // Poland
	"pt": "143453", // Portugal
	"py": "143513", // Paraguay
	"qa": "143498", // Qatar
	"ro": "143487", // Romania
	"ru": "143469", // Russia
	"sa": "143479", // Saudi Arabia
	"se": "143456", // Sweden
	"sg": "143464", // Singapore
	"si": "143499", // Slovenia
	"sk": "143496", // Slovakia
	"sn": "143535", // Senegal
	"sr": "143554", // Suriname
	"sv": "143506", // El Salvador
	"th": "143475", // Thailand
	"tn": "143536", // Tunisia
	"tr": "143480", // Turkey
	"tw": "143470", // Taiwan
	"tz": "143572", // Tanzania
	"ua": "143492", // Ukraine
	"ug": "143537", // Uganda
	"us": "143441", // United States
	"uy": "143514", // Uruguay
	"uz": "143566", // Uzbekistan
	"ve": "143502", // Venezuela
	"vg": "143543", // British Virgin Islands
	"vn": "143471", // Vietnam
	"ye": "143571", // Yemen
	"za": "143472", // South Africa
}

// CountryNames maps country codes with custom display-name handling.
// Public-only countries not listed here fall back to CLDR English names.
var CountryNames = map[string]string{
	"ae": "UAE",
	"ai": "Anguilla",
	"am": "Armenia",
	"ao": "Angola",
	"ar": "Argentina",
	"at": "Austria",
	"au": "Australia",
	"az": "Azerbaijan",
	"bb": "Barbados",
	"be": "Belgium",
	"bg": "Bulgaria",
	"bh": "Bahrain",
	"bm": "Bermuda",
	"bn": "Brunei",
	"bo": "Bolivia",
	"br": "Brazil",
	"bw": "Botswana",
	"by": "Belarus",
	"bz": "Belize",
	"ca": "Canada",
	"ch": "Switzerland",
	"cl": "Chile",
	"cn": "China",
	"co": "Colombia",
	"cr": "Costa Rica",
	"cy": "Cyprus",
	"cz": "Czech Republic",
	"de": "Germany",
	"dk": "Denmark",
	"dm": "Dominica",
	"dz": "Algeria",
	"ec": "Ecuador",
	"ee": "Estonia",
	"eg": "Egypt",
	"es": "Spain",
	"fi": "Finland",
	"fr": "France",
	"gb": "United Kingdom",
	"gd": "Grenada",
	"gh": "Ghana",
	"gr": "Greece",
	"gt": "Guatemala",
	"gy": "Guyana",
	"hk": "Hong Kong",
	"hn": "Honduras",
	"hr": "Croatia",
	"hu": "Hungary",
	"id": "Indonesia",
	"ie": "Ireland",
	"il": "Israel",
	"in": "India",
	"is": "Iceland",
	"it": "Italy",
	"jm": "Jamaica",
	"jo": "Jordan",
	"jp": "Japan",
	"ke": "Kenya",
	"kr": "South Korea",
	"kw": "Kuwait",
	"ky": "Cayman Islands",
	"lb": "Lebanon",
	"lk": "Sri Lanka",
	"lt": "Lithuania",
	"lu": "Luxembourg",
	"lv": "Latvia",
	"mg": "Madagascar",
	"mk": "Macedonia",
	"ml": "Mali",
	"mo": "Macao",
	"ms": "Montserrat",
	"mt": "Malta",
	"mu": "Mauritius",
	"mx": "Mexico",
	"my": "Malaysia",
	"ne": "Niger",
	"ng": "Nigeria",
	"ni": "Nicaragua",
	"nl": "Netherlands",
	"no": "Norway",
	"np": "Nepal",
	"nz": "New Zealand",
	"om": "Oman",
	"pa": "Panama",
	"pe": "Peru",
	"ph": "Philippines",
	"pk": "Pakistan",
	"pl": "Poland",
	"pt": "Portugal",
	"py": "Paraguay",
	"qa": "Qatar",
	"ro": "Romania",
	"ru": "Russia",
	"sa": "Saudi Arabia",
	"se": "Sweden",
	"sg": "Singapore",
	"si": "Slovenia",
	"sk": "Slovakia",
	"sn": "Senegal",
	"sr": "Suriname",
	"sv": "El Salvador",
	"th": "Thailand",
	"tn": "Tunisia",
	"tr": "Turkey",
	"tw": "Taiwan",
	"tz": "Tanzania",
	"ua": "Ukraine",
	"ug": "Uganda",
	"us": "United States",
	"uy": "Uruguay",
	"uz": "Uzbekistan",
	"ve": "Venezuela",
	"vg": "British Virgin Islands",
	"vn": "Vietnam",
	"ye": "Yemen",
	"za": "South Africa",
}

// publicCountryCodes lists the public App Store country codes accepted by lookup/search.
var publicCountryCodes = []string{
	"ae", "af", "ag", "ai", "al", "am", "ao", "ar", "at", "au", "az", "ba", "bb", "be", "bf",
	"bg", "bh", "bj", "bm", "bn", "bo", "br", "bs", "bt", "bw", "by", "bz", "ca", "cd", "cg",
	"ch", "ci", "cl", "cm", "cn", "co", "cr", "cv", "cy", "cz", "de", "dk", "dm", "do", "dz",
	"ec", "ee", "eg", "es", "fi", "fj", "fm", "fr", "ga", "gb", "gd", "ge", "gh", "gm", "gr",
	"gt", "gw", "gy", "hk", "hn", "hr", "hu", "id", "ie", "il", "in", "iq", "is", "it", "jm",
	"jo", "jp", "ke", "kg", "kh", "kn", "kr", "kw", "ky", "kz", "la", "lb", "lc", "lk", "lr",
	"lt", "lu", "lv", "ly", "ma", "md", "me", "mg", "mk", "ml", "mm", "mn", "mo", "mr", "ms",
	"mt", "mu", "mv", "mw", "mx", "my", "mz", "na", "ne", "ng", "ni", "nl", "no", "np", "nr",
	"nz", "om", "pa", "pe", "pg", "ph", "pk", "pl", "pt", "pw", "py", "qa", "ro", "rs", "ru",
	"rw", "sa", "sb", "sc", "se", "sg", "si", "sk", "sl", "sn", "sr", "st", "sv", "sz", "tc",
	"td", "th", "tj", "tm", "tn", "to", "tr", "tt", "tw", "tz", "ua", "ug", "us", "uy", "uz",
	"vc", "ve", "vg", "vn", "vu", "xk", "ye", "za", "zm", "zw",
}

var publicCountrySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(publicCountryCodes))
	for _, code := range publicCountryCodes {
		set[code] = struct{}{}
	}
	return set
}()

func publicCountryName(country string) string {
	country = strings.ToLower(strings.TrimSpace(country))
	if name, ok := CountryNames[country]; ok {
		return name
	}

	region, err := language.ParseRegion(strings.ToUpper(country))
	if err != nil {
		return ""
	}
	region = region.Canonicalize()
	if !region.IsCountry() {
		return ""
	}

	name := display.English.Regions().Name(region)
	if name == "" || name == region.String() || name == "Unknown Region" {
		return ""
	}
	return name
}

// Storefront contains public storefront metadata for a single country.
type Storefront struct {
	Country      string `json:"country"`
	CountryName  string `json:"countryName"`
	StorefrontID string `json:"storefrontId"`
}

// AllCountries returns a sorted list of the histogram storefront countries.
func AllCountries() []string {
	countries := make([]string, 0, len(Storefronts))
	for code := range Storefronts {
		countries = append(countries, code)
	}
	sort.Strings(countries)
	return countries
}

// AllPublicCountries returns a sorted list of all public App Store country codes.
func AllPublicCountries() []string {
	return append([]string(nil), publicCountryCodes...)
}

// ListStorefronts returns storefront metadata in deterministic country order.
func ListStorefronts() []Storefront {
	countries := AllPublicCountries()
	storefronts := make([]Storefront, 0, len(countries))
	for _, country := range countries {
		storefronts = append(storefronts, Storefront{
			Country:      strings.ToUpper(country),
			CountryName:  publicCountryName(country),
			StorefrontID: Storefronts[country],
		})
	}
	return storefronts
}
