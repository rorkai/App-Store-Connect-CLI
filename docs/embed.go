package docsembed

import _ "embed"

//go:embed API_NOTES.md
var APINotesGuide string

//go:embed WORKFLOWS.md
var WorkflowsGuide string

//go:embed THIRD_PARTY_NOTICES.md
var ThirdPartyNoticesGuide string
