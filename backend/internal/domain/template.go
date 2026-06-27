package domain

const (
	ScopePage     = "page"
	ScopeDocument = "document"
)

const (
	FormatMarkdown = "markdown"
	FormatJSON     = "json"
	FormatCSV      = "csv"
	FormatText     = "text"
)

// Fixed IDs for the two built-in templates seeded by migration 000002.
const (
	TemplateIDCleanMarkdown  = "00000000-0000-0000-0000-000000000001"
	TemplateIDStructuredJSON = "00000000-0000-0000-0000-000000000002"
)

type Template struct {
	ID           string
	OwnerUserID  *string
	Name         string
	DocTypeHint  string
	Scope        string
	Prompt       string
	OutputFormat string
	JSONSchema   *string
	IsBuiltin    bool
}
