package schema

import (
	"slices"
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

// NearOperation is an indexed operation that resembles a request.
// Correction reports that it has the request's method and the request's
// shape but names the API version or one literal segment differently: a
// misspelling, a plural, reordered or missing words, or an expanded
// abbreviation.
type NearOperation struct {
	Endpoint
	Correction bool
}

// NearestOperations ranks indexed endpoints by how closely they resemble the
// requested method and path and returns up to limit of them. Endpoints whose
// template matches the whole path under a different method rank first, then
// corrections (see NearOperation), then endpoints sharing the longest leading
// segment run. The API version segment is compared loosely, so /v1 and /v2
// templates compete on their resource segments. An endpoint must share at
// least the resource segment or be a correction, so a request naming a
// resource the index does not know returns nothing. Ties prefer the requested
// method, then the requested API version, then the smallest edit distance
// between the first differing segments, then the smallest length difference.
func NearestOperations(method, path string, limit int) ([]NearOperation, error) {
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
		endpoint    Endpoint
		fullMatch   bool
		correction  bool
		edits       int
		shared      int
		method      bool
		sameVersion bool
		distance    int
		lengthGap   int
	}
	candidates := make([]candidate, 0, len(endpoints))
	for _, endpoint := range endpoints {
		template := splitPath(endpoint.Path)
		shared := sharedLeadingSegments(template, segments)
		edits, correction := correctionEdits(template, segments)
		if shared < 2 && !correction {
			continue
		}
		gap := len(template) - len(segments)
		if gap < 0 {
			gap = -gap
		}
		full := pathTemplateMatches(template, segments)
		candidates = append(candidates, candidate{
			endpoint:    endpoint,
			fullMatch:   full,
			correction:  correction && !full && endpoint.Method == method,
			edits:       edits,
			shared:      shared,
			method:      endpoint.Method == method,
			sameVersion: len(template) > 0 && len(segments) > 0 && template[0] == segments[0],
			distance:    firstDifferingSegmentDistance(template, segments, shared),
			lengthGap:   gap,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.fullMatch != right.fullMatch {
			return left.fullMatch
		}
		if left.correction != right.correction {
			return left.correction
		}
		if left.correction && left.edits != right.edits {
			return left.edits < right.edits
		}
		if left.shared != right.shared {
			return left.shared > right.shared
		}
		if left.method != right.method {
			return left.method
		}
		if left.sameVersion != right.sameVersion {
			return left.sameVersion
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
	nearest := make([]NearOperation, 0, len(candidates))
	for _, item := range candidates {
		nearest = append(nearest, NearOperation{Endpoint: item.endpoint, Correction: item.correction})
	}
	return nearest, nil
}

// correctionEdits reports whether template has the request's shape with at
// most one literal segment named differently (see similarSegment), and counts
// the differences, a different API version included.
func correctionEdits(template, segments []string) (int, bool) {
	if len(template) != len(segments) || len(template) == 0 {
		return 0, false
	}
	edits, renamed := 0, 0
	for i := range template {
		switch {
		case segmentMatches(template[i], segments[i]):
		case i == 0 && isVersionSegment(template[i]) && isVersionSegment(segments[i]):
			edits++
		case i > 0 && !isTemplateParameter(template[i]) && similarSegment(template[i], segments[i]):
			edits++
			renamed++
		default:
			return 0, false
		}
	}
	return edits, renamed <= 1
}

// similarSegment reports whether a request segment names the same resource as
// a template segment in another spelling: a different case or plural, an edit
// distance of at most two, the same camel-case words in another order, one
// word more or less (see oneWordApart), or the template word spelled out
// (appInformations for appInfos).
func similarSegment(template, segment string) bool {
	if segment == "" {
		return false
	}
	left, right := singular(strings.ToLower(template)), singular(strings.ToLower(segment))
	if left == right {
		return true
	}
	if min(len(left), len(right)) >= 5 && suggest.Distance(left, right) <= 2 {
		return true
	}
	templateWords, segmentWords := camelWords(template), camelWords(segment)
	if sameWords(templateWords, segmentWords) || oneWordApart(templateWords, segmentWords) {
		return true
	}
	return abbreviates(template, segment) || abbreviates(segment, template)
}

// abbreviates reports whether short, without a plural s, starts long and
// long continues the same word there (appInfo in appInformation).
func abbreviates(short, long string) bool {
	stem := strings.TrimSuffix(short, "s")
	if len(stem) < 6 || len(long) <= len(stem) || !strings.HasPrefix(long, stem) {
		return false
	}
	next := long[len(stem)]
	return next >= 'a' && next <= 'z'
}

func camelWords(segment string) []string {
	var words []string
	start := 0
	for i := 1; i < len(segment); i++ {
		if segment[i] >= 'A' && segment[i] <= 'Z' {
			words = append(words, singular(strings.ToLower(segment[start:i])))
			start = i
		}
	}
	return append(words, singular(strings.ToLower(segment[start:])))
}

func sameWords(left, right []string) bool {
	if len(left) != len(right) || len(left) < 2 {
		return false
	}
	sortedLeft, sortedRight := slices.Clone(left), slices.Clone(right)
	slices.Sort(sortedLeft)
	slices.Sort(sortedRight)
	return slices.Equal(sortedLeft, sortedRight)
}

// oneWordApart reports whether one word list is the other without one middle
// word: appVersions for appStoreVersions. The last word names the resource
// type and the first its owner, so betaApps is not betaAppLocalizations and
// ageRating is not territoryAgeRatings.
func oneWordApart(left, right []string) bool {
	if len(left) < len(right) {
		left, right = right, left
	}
	if len(right) < 2 || len(left) != len(right)+1 {
		return false
	}
	for i := 1; i < len(left)-1; i++ {
		if slices.Equal(slices.Concat(left[:i], left[i+1:]), right) {
			return true
		}
	}
	return false
}

func singular(word string) string {
	if len(word) > 3 && strings.HasSuffix(word, "ies") {
		return word[:len(word)-3] + "y"
	}
	return strings.TrimSuffix(word, "s")
}

func isVersionSegment(segment string) bool {
	return len(segment) >= 2 && segment[0] == 'v' && strings.Trim(segment[1:], "0123456789") == ""
}

func isTemplateParameter(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
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

// sharedLeadingSegments counts the leading segments template and segments
// share, taking any two API version segments as equal.
func sharedLeadingSegments(template, segments []string) int {
	shared := 0
	for shared < len(template) && shared < len(segments) {
		looseVersion := shared == 0 && isVersionSegment(template[0]) && isVersionSegment(segments[0])
		if !looseVersion && !segmentMatches(template[shared], segments[shared]) {
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
	if isTemplateParameter(template) {
		return true
	}
	return template == segment
}
