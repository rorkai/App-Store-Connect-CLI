package schema

import "sync"

type operationNames struct {
	templates  []string
	queryNames map[string]struct{}
}

var loadOperationNames = sync.OnceValues(func() (operationNames, error) {
	endpoints, err := loadIndex()
	if err != nil {
		return operationNames{}, err
	}
	names := operationNames{queryNames: map[string]struct{}{}}
	seen := map[string]struct{}{}
	for _, endpoint := range endpoints {
		if _, ok := seen[endpoint.Path]; !ok {
			seen[endpoint.Path] = struct{}{}
			names.templates = append(names.templates, endpoint.Path)
		}
		for _, parameter := range endpoint.Parameters {
			if parameter.In == "query" {
				names.queryNames[parameter.Name] = struct{}{}
			}
		}
	}
	return names, nil
})

// PathTemplate returns the path template of the embedded schema index that a
// concrete request path matches, whatever the method, and whether one did.
// Template segments such as {id} match any single non-empty segment; a query
// string on path is ignored. The returned template is always a string from the
// index, never part of path.
func PathTemplate(path string) (string, bool, error) {
	names, err := loadOperationNames()
	if err != nil {
		return "", false, err
	}
	segments := requestPathSegments(path)
	if len(segments) == 0 {
		return "", false, nil
	}
	for _, template := range names.templates {
		if pathTemplateMatches(splitPath(template), segments) {
			return template, true, nil
		}
	}
	return "", false, nil
}

// IsQueryParameterName reports whether any operation in the embedded schema
// index defines a query parameter named name.
func IsQueryParameterName(name string) (bool, error) {
	names, err := loadOperationNames()
	if err != nil {
		return false, err
	}
	_, ok := names.queryNames[name]
	return ok, nil
}
