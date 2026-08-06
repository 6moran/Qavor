package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var ErrPythonParserFailed = errors.New("document parser failed")

// PythonParser 执行配置的 Python 解析器，无需通过 shell 调用。
// 输入内容写入临时目录后以本地路径传给脚本，修复 MinIO 对象路径误用问题。
// 临时目录在 Parse 成功后保留，由调用方在图片回填完成后调用 Cleanup 清理。
type PythonParser struct {
	pythonPath string
	scriptPath string
	extraEnv   []string
	args       []string
	tmpDir     string
}

func NewPythonParser(pythonPath, scriptPath string) *PythonParser {
	return &PythonParser{pythonPath: pythonPath, scriptPath: scriptPath}
}

// Parse 将输入内容写入临时目录（文件名保留原扩展名）后执行 Python 脚本解析。
func (p *PythonParser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	p.Cleanup() // 兜底清理上一次残留，防止泄漏
	tmpDir, err := os.MkdirTemp("", "qavor-parse-*")
	if err != nil {
		return ParseResult{}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	p.tmpDir = tmpDir

	localPath := filepath.Join(tmpDir, filepath.Base(input.Filename))
	if err := os.WriteFile(localPath, input.Content, 0o600); err != nil {
		p.Cleanup()
		return ParseResult{}, fmt.Errorf("写入临时文件失败: %w", err)
	}

	args := p.args
	if len(args) == 0 {
		args = []string{p.scriptPath, "--input", localPath}
	}
	cmd := exec.CommandContext(ctx, p.pythonPath, args...)
	cmd.Env = append(os.Environ(), p.extraEnv...)
	stdout, err := cmd.Output()
	result, parseErr := parsePythonOutput(stdout, err)
	if parseErr != nil {
		p.Cleanup()
		return ParseResult{}, parseErr
	}
	return result, nil
}

// Cleanup 删除最近一次 Parse 创建的临时目录。可重复调用。
func (p *PythonParser) Cleanup() {
	if p.tmpDir != "" {
		_ = os.RemoveAll(p.tmpDir)
		p.tmpDir = ""
	}
}

// parsePythonOutput 将脚本 stdout 与执行错误映射为 ParseResult。
// 脚本以错误码退出且 stdout 为错误 JSON 时映射为 ParserError。
func parsePythonOutput(stdout []byte, runErr error) (ParseResult, error) {
	if runErr != nil {
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
