# Telemetry records which API operation `asc api` calls

Changed in 7.0.0. The telemetry event of an `asc api` invocation now says
which App Store Connect API operation it called, without any identifier or
value. Events move to schema version 6 with three new fields:

| Field | Value |
| --- | --- |
| `api_method` | `GET`, `POST`, `PATCH`, or `DELETE` |
| `api_path_template` | The request path matched against the path templates of the API schema embedded in the CLI, with every path parameter written `{id}`, or `other` when no template matches |
| `api_query_params` | The names of the query parameters sent, each a name the schema defines, `cursor`, or `other`; sorted, at most 64 |

For example, `asc api GET /v1/apps/123456789/builds --query limit=5 --query
"filter[version]=1.2.3"` records `GET`, `/v1/apps/{id}/builds`, and
`["filter[version]", "limit"]`. The app ID, the version, the other query
values, headers, and request and response bodies are never sent.

Every other command sends the three fields as `null`, and an `asc api`
invocation that fails before its METHOD and PATH are read does too. Before
7.0, `asc api` events recorded only that the command ran and its outcome.

`asc telemetry disable`, `ASC_TELEMETRY_DISABLED`, and `DO_NOT_TRACK` work as
before: with telemetry off, nothing is computed or sent. Events queued by an
earlier version are sent as they were written.
