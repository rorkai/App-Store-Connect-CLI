package asc

import "net/url"

// IncludedFields maps an included resource type to its fields[type] values.
type IncludedFields map[string][]string

// AddTo sets one fields[type] parameter per included resource type.
func (f IncludedFields) AddTo(values url.Values) {
	for resourceType, fields := range f {
		addCSV(values, "fields["+resourceType+"]", fields)
	}
}

type includedFieldsQuery struct {
	includedFields IncludedFields
}

func (q *includedFieldsQuery) setIncludedFields(fields IncludedFields) {
	q.includedFields = fields
}

type includedFieldsSetter interface {
	setIncludedFields(IncludedFields)
}

// WithIncludedFields sets sparse fields for included resources on any read
// whose query embeds includedFieldsQuery, for example
// WithIncludedFields[BuildsOption](fields).
func WithIncludedFields[Option ~func(Query), Query includedFieldsSetter](fields IncludedFields) Option {
	return func(query Query) { query.setIncludedFields(fields) }
}
