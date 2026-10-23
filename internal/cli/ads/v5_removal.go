package ads

import (
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const adsV5RemovedCommand = "v5"

// adsV5Replacements maps each removed `asc ads v5` command path to its
// Platform API v1 migration guidance. Group paths cover the former
// list-alias groups, such as `asc ads v5 campaigns`.
var adsV5Replacements = map[string]string{
	"acls list":                              "Use `asc ads acls list`.",
	"me view":                                "Use `asc ads me view`.",
	"apps search":                            "Use `asc ads apps search`.",
	"apps view":                              "Use `asc ads apps view`.",
	"apps localized-details":                 "Use `asc ads apps locales find`.",
	"apps eligibility find":                  "Use `asc ads apps eligibility find`.",
	"apps assets find":                       "Use `asc ads assets find`.",
	"product-pages list":                     "Use `asc ads product-pages find`.",
	"product-pages view":                     "Use `asc ads product-pages view`.",
	"product-pages locales list":             "Use `asc ads product-pages locales find`.",
	"product-pages countries list":           "No one-command replacement exists. Maintain the supported country or region catalog used by your workflow.",
	"product-pages devices list":             "No one-command replacement exists. Use the Platform API creative requirements for the selected supply source.",
	"budget-orders list":                     "Use `asc ads budget-orders find`.",
	"budget-orders create":                   "Use `asc ads budget-orders create`.",
	"budget-orders view":                     "Use `asc ads budget-orders view`.",
	"budget-orders update":                   "Use `asc ads budget-orders update`.",
	"campaigns list":                         "Use `asc ads campaigns find`.",
	"campaigns create":                       "Use `asc ads campaigns create`.",
	"campaigns find":                         "Use `asc ads campaigns find`.",
	"campaigns delete":                       "Use `asc ads campaigns delete`.",
	"campaigns view":                         "Use `asc ads campaigns view`.",
	"campaigns update":                       "Use `asc ads campaigns update`.",
	"ad-groups list":                         "Use `asc ads ad-groups find`.",
	"ad-groups create":                       "Use `asc ads ad-groups create`.",
	"ad-groups find":                         "Use `asc ads ad-groups find`.",
	"ad-groups find-org":                     "Use `asc ads ad-groups find`.",
	"ad-groups delete":                       "Use `asc ads ad-groups delete`.",
	"ad-groups view":                         "Use `asc ads ad-groups view`.",
	"ad-groups update":                       "Use `asc ads ad-groups update`.",
	"ads list":                               "Use `asc ads ads find`.",
	"ads create":                             "Use `asc ads ads create`.",
	"ads delete":                             "Use `asc ads ads delete`.",
	"ads view":                               "Use `asc ads ads view`.",
	"ads update":                             "Use `asc ads ads update`.",
	"ads find":                               "Use `asc ads ads find`.",
	"ads find-org":                           "Use `asc ads ads find`.",
	"targeting-keywords list":                "Use `asc ads targeting-keywords find`.",
	"targeting-keywords find":                "Use `asc ads targeting-keywords find`.",
	"targeting-keywords view":                "Use `asc ads targeting-keywords view`.",
	"targeting-keywords create-bulk":         "Use `asc ads targeting-keywords create-bulk`.",
	"targeting-keywords update-bulk":         "Use `asc ads targeting-keywords update-bulk`.",
	"targeting-keywords delete":              "Use `asc ads targeting-keywords delete`.",
	"targeting-keywords delete-bulk":         "No one-command replacement exists. Query matching keywords with `asc ads targeting-keywords find`, then delete each ID with `asc ads targeting-keywords delete --keyword KEYWORD_ID --confirm`.",
	"campaign-negative-keywords list":        "Use `asc ads negative-keywords find`.",
	"campaign-negative-keywords find":        "Use `asc ads negative-keywords find`.",
	"campaign-negative-keywords view":        "Use `asc ads negative-keywords view`.",
	"campaign-negative-keywords create-bulk": "Use `asc ads negative-keywords create-bulk`.",
	"campaign-negative-keywords update-bulk": "Use `asc ads negative-keywords update-bulk`.",
	"campaign-negative-keywords delete-bulk": "No one-command replacement exists. Query matching negative keywords with `asc ads negative-keywords find`, then delete each ID with `asc ads negative-keywords delete --negative-keyword NEGATIVE_KEYWORD_ID --confirm`.",
	"ad-group-negative-keywords list":        "Use `asc ads negative-keywords find`.",
	"ad-group-negative-keywords find":        "Use `asc ads negative-keywords find`.",
	"ad-group-negative-keywords view":        "Use `asc ads negative-keywords view`.",
	"ad-group-negative-keywords create-bulk": "Use `asc ads negative-keywords create-bulk`.",
	"ad-group-negative-keywords update-bulk": "Use `asc ads negative-keywords update-bulk`.",
	"ad-group-negative-keywords delete-bulk": "No one-command replacement exists. Query matching negative keywords with `asc ads negative-keywords find`, then delete each ID with `asc ads negative-keywords delete --negative-keyword NEGATIVE_KEYWORD_ID --confirm`.",
	"geo search":                             "Use `asc ads geo search`.",
	"geo resolve":                            "Use `asc ads geo resolve`.",
	"creatives list":                         "Use `asc ads creatives find`.",
	"creatives create":                       "Use `asc ads creatives create`.",
	"creatives find":                         "Use `asc ads creatives find`.",
	"creatives view":                         "Use `asc ads creatives view`.",
	"rejection-reasons find":                 "Use `asc ads rejection-reasons apps find`.",
	"rejection-reasons view":                 "Use `asc ads rejection-reasons apps view`.",
	"reports campaigns":                      "Use `asc ads reports apps campaigns`.",
	"reports ad-groups":                      "Use `asc ads reports apps ad-groups`.",
	"reports keywords":                       "Use `asc ads reports apps keywords`.",
	"reports search-terms":                   "Use `asc ads reports apps search-terms`.",
	"reports ads":                            "Use `asc ads reports apps ads`.",
	"reports ad-group-keywords":              "Use `asc ads reports apps keywords`.",
	"reports ad-group-search-terms":          "Use `asc ads reports apps search-terms`.",
	"impression-share-reports list":          "No one-command replacement exists. Use `asc ads insights impression-share find` with an equivalent query payload.",
	"impression-share-reports create":        "Use `asc ads insights impression-share find`.",
	"impression-share-reports view":          "No one-command replacement exists. Use `asc ads insights impression-share find` with an equivalent query payload.",
	"me":                                     "Use `asc ads me view`.",
	"acls":                                   "Use `asc ads acls list`.",
	"product-pages":                          "Use `asc ads product-pages find`.",
	"budget-orders":                          "Use `asc ads budget-orders find`.",
	"campaigns":                              "Use `asc ads campaigns find`.",
	"ad-groups":                              "Use `asc ads ad-groups find`.",
	"ads":                                    "Use `asc ads ads find`.",
	"targeting-keywords":                     "Use `asc ads targeting-keywords find`.",
	"campaign-negative-keywords":             "Use `asc ads negative-keywords find`.",
	"ad-group-negative-keywords":             "Use `asc ads negative-keywords find`.",
	"creatives":                              "Use `asc ads creatives find`.",
	"impression-share-reports":               "No one-command replacement exists. Use `asc ads insights impression-share find` with an equivalent query payload.",
	"campaigns pause":                        "Use `asc ads campaigns pause`.",
	"campaigns resume":                       "Use `asc ads campaigns resume`.",
	"api request":                            "Use `asc ads api request` with a v1/ path.",
	"reports preset":                         "Use the matching `asc ads reports apps` command for the former `--level`; ad-group keyword and search-term levels map to the consolidated `keywords` and `search-terms` reports.",
}

// removedAdsV5UsageError explains that the Campaign Management API v5
// namespace was removed and names the Platform API v1 replacement for the
// requested command when one is known. Only paths from the replacement table
// are echoed, never raw arguments.
func removedAdsV5UsageError(args []string) error {
	command := "asc ads v5"
	guidance := "Use the Platform API v1 commands listed by `asc ads --help`."
	if path, replacement, ok := adsV5Replacement(args); ok {
		command += " " + path
		guidance = replacement
	}
	return shared.WithDiagnostic(shared.UsageErrorf(
		"`%s` was removed in 6.0.0 after Apple retired Campaign Management API v5 on January 26, 2027. %s Platform API v1 commands take --ad-account and v1 request bodies.",
		command, guidance,
	), shared.DiagnosticInvalidInput, "")
}

func adsV5Replacement(args []string) (string, string, bool) {
	path := make([]string, 0, 3)
	for _, arg := range args {
		if len(path) == 3 || strings.HasPrefix(arg, "-") {
			break
		}
		path = append(path, arg)
	}
	for length := len(path); length > 0; length-- {
		joined := strings.Join(path[:length], " ")
		if replacement, ok := adsV5Replacements[joined]; ok {
			return joined, replacement, true
		}
	}
	return "", "", false
}
