package suggest

import (
	"sort"
	"strings"
)

type candidate struct {
	name  string
	score int
	dist  int
}

// Commands returns up to a few likely command-name suggestions for the provided input.
// It is intentionally conservative: if we aren't reasonably confident, it returns nil.
func Commands(input string, candidates []string) []string {
	in := strings.ToLower(strings.TrimSpace(input))
	if in == "" {
		return nil
	}

	collected := make([]candidate, 0, len(candidates))
	for _, raw := range candidates {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || name == in {
			continue
		}

		// Strong signal: prefix relationship.
		if strings.HasPrefix(name, in) || strings.HasPrefix(in, name) {
			collected = append(collected, candidate{name: name, score: 0, dist: 0})
			continue
		}

		d := levenshtein(in, name)
		if !withinThreshold(in, d) && !isAdjacentTransposition(in, name) {
			continue
		}
		collected = append(collected, candidate{name: name, score: 1, dist: d})
	}

	if len(collected) == 0 {
		return nil
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].score != collected[j].score {
			return collected[i].score < collected[j].score
		}
		if collected[i].dist != collected[j].dist {
			return collected[i].dist < collected[j].dist
		}
		return collected[i].name < collected[j].name
	})

	const max = 3
	out := make([]string, 0, max)
	seen := make(map[string]struct{}, max)
	for _, c := range collected {
		if _, ok := seen[c.name]; ok {
			continue
		}
		seen[c.name] = struct{}{}
		out = append(out, c.name)
		if len(out) >= max {
			break
		}
	}
	return out
}

func isAdjacentTransposition(a, b string) bool {
	if len(a) != len(b) || len(a) < 2 {
		return false
	}
	first := -1
	for i := range len(a) {
		if a[i] == b[i] {
			continue
		}
		if first == -1 {
			first = i
			continue
		}
		return i == first+1 && a[first] == b[i] && a[i] == b[first] && a[i+1:] == b[i+1:]
	}
	return false
}

func withinThreshold(input string, dist int) bool {
	n := len(input)
	// Conservative default thresholds that work well for short command names.
	switch {
	case n <= 4:
		return dist <= 1
	case n <= 7:
		return dist <= 2
	default:
		return dist <= 3
	}
}

// levenshtein computes the Levenshtein distance between two strings.
// For our command names (ASCII, short), this is fast enough.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	// Ensure a is the shorter string to reduce memory.
	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(a)+1)
	cur := make([]int, len(a)+1)
	for i := 0; i <= len(a); i++ {
		prev[i] = i
	}

	for j := 1; j <= len(b); j++ {
		cur[0] = j
		bj := b[j-1]
		for i := 1; i <= len(a); i++ {
			cost := 0
			if a[i-1] != bj {
				cost = 1
			}
			del := prev[i] + 1
			ins := cur[i-1] + 1
			sub := prev[i-1] + cost
			cur[i] = min3(del, ins, sub)
		}
		prev, cur = cur, prev
	}

	return prev[len(a)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}
