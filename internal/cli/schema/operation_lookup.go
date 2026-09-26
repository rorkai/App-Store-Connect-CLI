package schema

import (
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

// MatchOperation reports the indexed endpoint whose method and templated path
// match a concrete request path. Template segments such as {id} match any
// single non-empty path segment. The path may carry a query string, which is
// ignored, and a trailing slash.
func MatchOperation(method, path string) (Endpoint, bool, error) {
	endpoints, err := loadIndex()
	if err != nil {
		return Endpoint{}, false, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	segments := requestPathSegments(path)
	if len(segments) == 0 {
		return Endpoint{}, false, nil
	}
	for _, endpoint := range endpoints {
		if endpoint.Method != method {
			continue
		}
		if pathTemplateMatches(splitPath(endpoint.Path), segments) {
			return endpoint, true, nil
		}
	}
	return Endpoint{}, false, nil
}

// NearestOperations ranks indexed endpoints by how closely they resemble the
// requested method and path and returns up to limit of them. Endpoints whose
// template matches the whole path under a different method rank first, then
// endpoints sharing the longest leading segment run. Ties prefer the requested
// method, then the smallest edit distance between the first differing
// segments, then the smallest length difference.
func NearestOperations(method, path string, limit int) ([]Endpoint, error) {
	endpoints, err := loadIndex()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, nil
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	segments := requestPathSegments(path)

	type candidate struct {
		endpoint  Endpoint
		fullMatch bool
		shared    int
		method    bool
		distance  int
		lengthGap int
	}
	candidates := make([]candidate, 0, len(endpoints))
	for _, endpoint := range endpoints {
		template := splitPath(endpoint.Path)
		shared := sharedLeadingSegments(template, segments)
		if shared == 0 {
			continue
		}
		gap := len(template) - len(segments)
		if gap < 0 {
			gap = -gap
		}
		candidates = append(candidates, candidate{
			endpoint:  endpoint,
			fullMatch: pathTemplateMatches(template, segments),
			shared:    shared,
			method:    endpoint.Method == method,
			distance:  firstDifferingSegmentDistance(template, segments, shared),
			lengthGap: gap,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.fullMatch != right.fullMatch {
			return left.fullMatch
		}
		if left.shared != right.shared {
			return left.shared > right.shared
		}
		if left.method != right.method {
			return left.method
		}
		if left.distance != right.distance {
			return left.distance < right.distance
		}
		if left.lengthGap != right.lengthGap {
			return left.lengthGap < right.lengthGap
		}
		if left.endpoint.Path != right.endpoint.Path {
			return left.endpoint.Path < right.endpoint.Path
		}
		return left.endpoint.Method < right.endpoint.Method
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	nearest := make([]Endpoint, 0, len(candidates))
	for _, item := range candidates {
		nearest = append(nearest, item.endpoint)
	}
	return nearest, nil
}

func requestPathSegments(path string) []string {
	path = strings.TrimSpace(path)
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	return splitPath(path)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func pathTemplateMatches(template, segments []string) bool {
	if len(template) != len(segments) {
		return false
	}
	for i := range template {
		if !segmentMatches(template[i], segments[i]) {
			return false
		}
	}
	return true
}

// firstDifferingSegmentDistance measures how far the first unmatched request
// segment is from the template segment at the same position. A missing segment
// on either side counts as a full insertion or deletion.
func firstDifferingSegmentDistance(template, segments []string, shared int) int {
	var templateSegment, requestSegment string
	if shared < len(template) {
		templateSegment = template[shared]
	}
	if shared < len(segments) {
		requestSegment = segments[shared]
	}
	return suggest.Distance(templateSegment, requestSegment)
}

func sharedLeadingSegments(template, segments []string) int {
	shared := 0
	for shared < len(template) && shared < len(segments) {
		if !segmentMatches(template[shared], segments[shared]) {
			break
		}
		shared++
	}
	return shared
}

func segmentMatches(template, segment string) bool {
	if segment == "" {
		return false
	}
	if strings.HasPrefix(template, "{") && strings.HasSuffix(template, "}") {
		return true
	}
	return template == segment
}
