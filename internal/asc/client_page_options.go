package asc

import "strings"

type pager interface{ page() *listQuery }

func (q *listQuery) page() *listQuery { return q }

// Limit returns an option O that sets the page size. Values below 1 keep the
// endpoint's default. Write it as Limit[DevicesOption](n); the query type is inferred.
func Limit[O ~func(Q), Q pager](limit int) O {
	return func(q Q) {
		if limit > 0 {
			q.page().limit = limit
		}
	}
}

// NextURL returns an option O that fetches the given links.next URL instead
// of the first page. The URL is trimmed; a blank URL is ignored.
func NextURL[O ~func(Q), Q pager](next string) O {
	next = strings.TrimSpace(next)
	return func(q Q) {
		if next != "" {
			q.page().nextURL = next
		}
	}
}
