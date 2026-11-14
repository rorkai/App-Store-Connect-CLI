package ascterritory

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type nameLookup struct {
	id        string
	ambiguous bool
}

var (
	nameMapOnce sync.Once
	nameMap     map[string]nameLookup
)

func Normalize(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("territory is required")
	}

	upper := strings.ToUpper(trimmed)
	if _, ok := supportedIDs[upper]; ok {
		return upper, nil
	}

	if len(upper) == 2 {
		if region, err := language.ParseRegion(upper); err == nil {
			if iso3 := strings.ToUpper(strings.TrimSpace(region.ISO3())); iso3 != "" {
				if _, ok := supportedIDs[iso3]; ok {
					return iso3, nil
				}
			}
		}
	}

	lookup, ok := territoryNameMap()[normalizeName(trimmed)]
	if !ok {
		return "", fmt.Errorf("territory %q could not be mapped to an App Store Connect territory ID", trimmed)
	}
	if lookup.ambiguous || lookup.id == "" {
		return "", fmt.Errorf("territory %q is ambiguous; use a 3-letter territory ID like USA", trimmed)
	}
	return lookup.id, nil
}

func territoryNameMap() map[string]nameLookup {
	nameMapOnce.Do(func() {
		names := make(map[string]nameLookup)
		regionNamer := display.English.Regions()

		for code := range supportedIDs {
			region, err := language.ParseRegion(code)
			if err != nil {
				continue
			}

			name := strings.TrimSpace(regionNamer.Name(region))
			if name == "" || strings.EqualFold(name, code) || strings.EqualFold(name, "Unknown Region") {
				continue
			}

			key := normalizeName(name)
			if key == "" {
				continue
			}

			existing, exists := names[key]
			switch {
			case !exists:
				names[key] = nameLookup{id: code}
			case existing.id != code:
				names[key] = nameLookup{ambiguous: true}
			}
		}

		for alias, code := range territoryAliases {
			key := normalizeName(alias)
			if key == "" {
				continue
			}
			if _, ok := supportedIDs[code]; !ok {
				continue
			}
			names[key] = nameLookup{id: code}
		}

		for alias := range territoryAmbiguousAliases {
			key := normalizeName(alias)
			if key == "" {
				continue
			}
			names[key] = nameLookup{ambiguous: true}
		}

		nameMap = names
	})

	return nameMap
}
