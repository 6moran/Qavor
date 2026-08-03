package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
)

var ErrPythonParserFailed = errors.New("document parser failed")

// PythonParser 执行配置的 Python 解析器，无需通过 shell 调用。
type PythonParser struct {
	pythonPath string
	scriptPath string
	extraEnv   []string
	args       []string
}

func NewPythonParser(pythonPath, scriptPath string) *PythonParser {
	return &PythonParser{pythonPath: pythonPath, scriptPath: scriptPath}
}

func (p *PythonParser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	args := p.args
	if len(args) == 0 {
		args = []string{p.scriptPath, "--input", input.Path}
	}
	cmd := exec.CommandContext(ctx, p.pythonPath, args...)
	cmd.Env = append(os.Environ(), p.extraEnv...)
	stdout, err := cmd.Output()
	if err != nil {
		var failure struct {
			Code    string `json:"error_code"`
			Message string `json:"error_message"`
		}
		if json.Unmarshal(stdout, &failure) == nil && failure.Code != "" && failure.Message != "" {
			return ParseResult{}, &ParserError{Code: failure.Code, Message: failure.Message}
		}
		return ParseResult{}, ErrPythonParserFailed
	}
	var result ParseResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return ParseResult{}, ErrPythonParserFailed
	}
	return result, nil
}
