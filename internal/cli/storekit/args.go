package storekit

import (
	"flag"
	"strings"
)

func flagWasSet(fs *flag.FlagSet, names ...string) bool {
	set := map[string]struct{}{}
	fs.Visit(func(item *flag.Flag) { set[item.Name] = struct{}{} })
	for _, name := range names {
		if _, ok := set[name]; ok {
			return true
		}
	}
	return false
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
