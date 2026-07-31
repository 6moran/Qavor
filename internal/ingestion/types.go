package ingestion

import "context"

// ParserError is safe to return from the ingestion pipeline and API layers.
type ParserError struct {
	Code    string
	Message string
}

func (e *ParserError) Error() string { return e.Message }

// ParseInput describes a document supplied to an ingestion parser.
type ParseInput struct {
	Filename string
	Content  []byte
	Path     string
}

// ParsedPage preserves the source-page boundary when a parser can provide it.
type ParsedPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// ParseResult is the normalized representation consumed by the chunker.
type ParseResult struct {
	Markdown string         `json:"markdown"`
	Pages    []ParsedPage   `json:"pages,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// DocumentParser converts a source document into Markdown.
type DocumentParser interface {
	Parse(ctx context.Context, input ParseInput) (ParseResult, error)
}
