package ingestion

import "context"

// ParserError 可以安全地从数据摄入管道和 API 层返回。
type ParserError struct {
	Code    string
	Message string
}

func (e *ParserError) Error() string { return e.Message }

// ParseInput 描述提供给数据摄入解析器的文档。
type ParseInput struct {
	Filename string
	Content  []byte
	Path     string
}

// ParsedPage 在解析器能够提供时保留源页面边界信息。
type ParsedPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// ParseResult 是分块器消费的标准化表示。
type ParseResult struct {
	Markdown     string         `json:"markdown"`
	PicturePaths []string       `json:"picture_paths,omitempty"` // 提取的图片临时文件路径(绝对路径,正斜杠)
	Pages        []ParsedPage   `json:"pages,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// OCRConfig is the runtime OCR selection passed to the persistent Python parser.
// Credentials stay on the local stdio connection and are never logged.
type OCRConfig struct {
	Engine   string
	APIURL   string
	APIKey   string
	APIModel string
}

// PythonWorkerOptions configures one long-lived local parser subprocess.
type PythonWorkerOptions struct {
	PythonPath string
	ScriptPath string
	MaxTasks   int
	OCR        OCRConfig
	ExtraEnv   []string
	Images     ImageUploader
}

// DocumentParser 将源文档转换为 Markdown 格式。
type DocumentParser interface {
	Parse(ctx context.Context, input ParseInput) (ParseResult, error)
}
