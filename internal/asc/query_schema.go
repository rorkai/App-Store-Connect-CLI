package asc

import "net/url"

// QueryParam is a query parameter that an operation defines in the API
// schema.
type QueryParam struct {
	// Name is the parameter as sent, such as "filter[roles]" or
	// "limit[visibleApps]".
	Name string
	// Type is the schema type: "array" (sent as one comma-separated value),
	// "integer", "boolean" or "string".
	Type string
	// Values are the allowed values, or nil when the schema lists none.
	Values []string
	// Maximum is the largest allowed integer, or 0 when the schema sets none.
	Maximum int
}

// QuerySchema is what the API schema defines for the query of one read.
type QuerySchema struct {
	// Resource is the type of the resources the read returns, such as
	// "userInvitations".
	Resource string
	// Params are the query parameters other than limit, in schema order.
	Params []QueryParam
}

// SchemaQuery is a generated query that can send every parameter its
// operation defines, by name.
type SchemaQuery interface {
	querySchema() QuerySchema
	setSchemaParam(name, value string)
}

// schemaQuery holds the parameters set through WithQueryParams.
type schemaQuery struct {
	params url.Values
}

func (q *schemaQuery) setSchemaParam(name, value string) {
	if q.params == nil {
		q.params = url.Values{}
	}
	q.params.Set(name, value)
}

// addSchemaParams adds the parameters set by name. A generated builder adds
// them before its typed options, so a typed option for the same parameter
// wins.
func addSchemaParams(values url.Values, query schemaQuery) {
	for name, value := range query.params {
		values[name] = value
	}
}

// QuerySchemaFor returns the query schema of the read whose option type is O.
func QuerySchemaFor[O ~func(Q), Q SchemaQuery]() QuerySchema {
	var query Q
	return query.querySchema()
}

// WithQueryParams returns an option O that sends each value under its
// parameter name as given: a list comma-separated, an integer in decimal, a
// boolean as true or false. Names come from QuerySchemaFor[O].
func WithQueryParams[O ~func(Q), Q SchemaQuery](values map[string]string) O {
	return func(query Q) {
		for name, value := range values {
			query.setSchemaParam(name, value)
		}
	}
}
