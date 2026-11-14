package shared

import (
	"flag"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// QuerySource is the query of a read whose schema parameters a list or view
// builder exposes as flags. NewQueryFlags makes one.
type QuerySource interface {
	querySchema() asc.QuerySchema
	setQueryValues(map[string]string)
}

// QueryFlags holds the values of the query flags of the read whose option
// type is O.
type QueryFlags[O ~func(Q), Q asc.SchemaQuery] struct {
	values map[string]string
}

// NewQueryFlags returns the query flags of the read whose option type is O.
func NewQueryFlags[O ~func(Q), Q asc.SchemaQuery]() *QueryFlags[O, Q] {
	return &QueryFlags[O, Q]{}
}

func (f *QueryFlags[O, Q]) querySchema() asc.QuerySchema { return asc.QuerySchemaFor[O]() }

func (f *QueryFlags[O, Q]) setQueryValues(values map[string]string) { f.values = values }

// Option sends the values given to the query flags.
func (f *QueryFlags[O, Q]) Option() O { return asc.WithQueryParams[O](f.values) }

var queryParamName = regexp.MustCompile(`^([A-Za-z]+)(?:\[([A-Za-z0-9.]+)\])?$`)

// QueryFlagName is the flag that exposes a query parameter of a read that
// returns resource:
//
//	filter[x]          --x, the last word singular (filter[roles]: --role)
//	fields[resource]   --fields
//	fields[t]          --<t, last word singular>-fields (fields[apps]: --app-fields)
//	limit[x]           --x-limit
//	exists[x]          --exists-x
//	include, sort, …   --include, --sort, …
//
// Names are kebab case; a dot becomes a dash.
func QueryFlagName(param, resource string) string {
	match := queryParamName.FindStringSubmatch(param)
	if match == nil {
		return kebabCase(param)
	}
	kind, inner := match[1], match[2]
	switch {
	case inner == "":
		return kebabCase(kind)
	case kind == "filter":
		return singularLastWord(kebabCase(inner))
	case kind == "fields" && inner == resource:
		return "fields"
	case kind == "fields":
		return singularLastWord(kebabCase(inner)) + "-fields"
	case kind == "limit":
		return kebabCase(inner) + "-limit"
	default:
		return kebabCase(kind) + "-" + kebabCase(inner)
	}
}

// kebabCase turns a camelCase name with dots into kebab case: "appStoreVersions.platform"
// becomes "app-store-versions-platform".
func kebabCase(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == '.':
			b.WriteByte('-')
			continue
		case r >= 'A' && r <= 'Z':
			prevLower := i > 0 && (runes[i-1] >= 'a' && runes[i-1] <= 'z' || runes[i-1] >= '0' && runes[i-1] <= '9')
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			prevUpper := i > 0 && runes[i-1] >= 'A' && runes[i-1] <= 'Z'
			if prevLower || (prevUpper && nextLower) {
				b.WriteByte('-')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// singularLastWord makes the last word of a kebab-case name singular:
// "visible-apps" becomes "visible-app", "territories" "territory". Words
// ending in "ss" or "us" (status) stay.
func singularLastWord(name string) string {
	cut := strings.LastIndexByte(name, '-') + 1
	word := name[cut:]
	switch {
	case strings.HasSuffix(word, "ies"):
		word = strings.TrimSuffix(word, "ies") + "y"
	case strings.HasSuffix(word, "ss"), strings.HasSuffix(word, "us"):
	case strings.HasSuffix(word, "s"):
		word = strings.TrimSuffix(word, "s")
	}
	return name[:cut] + word
}

// queryFlag is one registered query flag.
type queryFlag struct {
	param asc.QueryParam
	flag  *flag.Flag
}

// queryFlagHelp marks the flags that help lists under QUERY FLAGS.
var queryFlagHelp sync.Map

// bindQueryFlags registers a flag for each parameter of the query, after the
// command's own flags. names overrides the flag of a parameter; an empty name
// leaves the parameter to a flag the command declares. A generated name that
// is already a flag, or an override for a parameter the read does not
// define, panics: both are mistakes in the command's declaration.
func bindQueryFlags(fs *flag.FlagSet, query QuerySource, names map[string]string) []queryFlag {
	if query == nil {
		return nil
	}
	schema := query.querySchema()
	for param := range names {
		if !slices.ContainsFunc(schema.Params, func(p asc.QueryParam) bool { return p.Name == param }) {
			panic(fmt.Sprintf("%s: QueryFlagNames names %s, which the read does not define", fs.Name(), param))
		}
	}
	var flags []queryFlag
	for _, param := range schema.Params {
		name, overridden := names[param.Name]
		if !overridden {
			name = QueryFlagName(param.Name, schema.Resource)
		}
		if name == "" {
			continue
		}
		if fs.Lookup(name) != nil {
			panic(fmt.Sprintf("%s: flag --%s for query parameter %s is already defined; name it in QueryFlagNames", fs.Name(), name, param.Name))
		}
		usage := queryFlagUsage(param)
		switch param.Type {
		case "integer":
			fs.Int(name, 0, usage)
		case "boolean":
			fs.Bool(name, false, usage)
		default:
			fs.String(name, "", usage)
		}
		f := fs.Lookup(name)
		queryFlagHelp.Store(f, struct{}{})
		flags = append(flags, queryFlag{param: param, flag: f})
	}
	return flags
}

// queryFlagUsage is the help line of a query flag, built from the schema.
func queryFlagUsage(param asc.QueryParam) string {
	switch param.Type {
	case "array":
		if len(param.Values) > 0 {
			return param.Name + ", comma-separated: " + strings.Join(param.Values, ", ")
		}
		return param.Name + ", comma-separated"
	case "integer":
		if param.Maximum > 0 {
			return fmt.Sprintf("%s (1-%d)", param.Name, param.Maximum)
		}
		return param.Name + " (at least 1)"
	case "boolean":
		return param.Name + " (true or false)"
	default:
		if len(param.Values) > 0 {
			return param.Name + ": one of " + strings.Join(param.Values, ", ")
		}
		return param.Name
	}
}

// queryFlagNames returns the names of the query flags.
func queryFlagNames(flags []queryFlag) []string {
	names := make([]string, len(flags))
	for i, f := range flags {
		names[i] = f.flag.Name
	}
	return names
}

// resolveQueryFlags validates the query flags given on the command line and
// hands their values to the query.
func resolveQueryFlags(d *Deps, fs *flag.FlagSet, query QuerySource, flags []queryFlag, prefix string) error {
	if query == nil {
		return nil
	}
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	values := map[string]string{}
	var include []string
	for _, f := range flags {
		if !given[f.flag.Name] {
			continue
		}
		flagName := "--" + f.flag.Name
		value, err := queryFlagValue(f.param, flagName, f.flag.Value.String())
		if err != nil {
			return WithDiagnostic(d.UsageErrorf("%s: %v", prefix, err), DiagnosticInvalidInput, flagName)
		}
		values[f.param.Name] = value
		if f.param.Name == "include" {
			include = strings.Split(value, ",")
		}
	}
	for _, f := range flags {
		relationship, isLimit := strings.CutPrefix(f.param.Name, "limit[")
		relationship = strings.TrimSuffix(relationship, "]")
		if isLimit && given[f.flag.Name] && includeFlagOffers(flags, relationship) && !slices.Contains(include, relationship) {
			return WithDiagnostic(d.UsageErrorf("%s: --%s requires --include %s", prefix, f.flag.Name, relationship), DiagnosticInvalidInput, "--"+f.flag.Name)
		}
	}
	query.setQueryValues(values)
	return nil
}

// includeFlagOffers reports whether a generated include flag accepts the
// relationship, so that a limit[relationship] flag can require it.
func includeFlagOffers(flags []queryFlag, relationship string) bool {
	for _, f := range flags {
		if f.param.Name == "include" {
			return slices.Contains(f.param.Values, relationship)
		}
	}
	return false
}

// queryFlagValue validates a query flag's value and returns it as sent.
func queryFlagValue(param asc.QueryParam, flagName, raw string) (string, error) {
	switch param.Type {
	case "integer":
		n, _ := strconv.Atoi(raw)
		if param.Maximum > 0 && (n < 1 || n > param.Maximum) {
			return "", fmt.Errorf("%s must be between 1 and %d", flagName, param.Maximum)
		}
		if n < 1 {
			return "", fmt.Errorf("%s must be at least 1", flagName)
		}
		return strconv.Itoa(n), nil
	case "boolean":
		return raw, nil
	}
	items := []string{strings.TrimSpace(raw)}
	if param.Type == "array" {
		items = SplitCSV(raw)
	}
	if len(items) == 0 || items[0] == "" {
		return "", fmt.Errorf("%s must not be empty", flagName)
	}
	if len(param.Values) > 0 {
		for i, item := range items {
			value, ok := enumValue(param.Values, item)
			if !ok {
				return "", fmt.Errorf("%s must be one of: %s", flagName, strings.Join(param.Values, ", "))
			}
			items[i] = value
		}
	}
	return strings.Join(items, ","), nil
}

// enumValue returns the allowed value an item names. An item that differs
// from an allowed value only in case names it, unless two allowed values
// differ only in case.
func enumValue(allowed []string, item string) (string, bool) {
	if slices.Contains(allowed, item) {
		return item, true
	}
	var match string
	for _, value := range allowed {
		if strings.EqualFold(value, item) {
			if match != "" {
				return "", false
			}
			match = value
		}
	}
	return match, match != ""
}
