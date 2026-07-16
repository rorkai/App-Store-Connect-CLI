package asc

import "net/url"

type listQuery struct {
	limit   int
	nextURL string
}

func buildListQuery(query *listQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
