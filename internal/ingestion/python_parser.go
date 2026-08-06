package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"
)

var ErrPythonParserFailed = errors.New("document parser failed")

// PythonParser 执行配置的 Python 解析器，无需通过 shell 调用。
// 输入内容写入临时目录后以本地路径传给脚本，修复 MinIO 对象路径误用问题。
// 临时目录为 Parse 局部状态，解析结束后（无论成败）立即清理；
// 成功时若配置了 ImageUploader，会先将产物图片上传回填 Markdown 再清理。
// 实例无跨调用共享状态，可并发使用。
type PythonParser struct {
	pythonPath string
	scriptPath string
	extraEnv   []string
	args       []string
	images     ImageUploader
}

// NewPythonParser 创建解析器。images 可选，提供后解析产出的图片会上传并回填 Markdown 链接，仅首个生效。
func NewPythonParser(pythonPath, scriptPath string, images ...ImageUploader) *PythonParser {
	var img ImageUploader
	if len(images) > 0 {
		img = images[0]
	}
	return &PythonParser{pythonPath: pythonPath, scriptPath: scriptPath, images: img}
}

// Parse 将输入内容写入临时目录（文件名保留原扩展名）后执行 Python 脚本解析。
func (p *PythonParser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	tmpDir, err := os.MkdirTemp("", "qavor-parse-*")
	if err != nil {
		return ParseResult{}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			logWarn("清理解析临时目录失败", zap.String("dir", tmpDir), zap.Error(rmErr))
		}
	}()

	localPath := filepath.Join(tmpDir, filepath.Base(input.Filename))
	if err := os.WriteFile(localPath, input.Content, 0o600); err != nil {
		return ParseResult{}, fmt.Errorf("写入临时文件失败: %w", err)
	}

	args := p.args
	if len(args) == 0 {
		args = []string{p.scriptPath, "--input", localPath}
	}
	cmd := exec.CommandContext(ctx, p.pythonPath, args...)
	cmd.Env = append(os.Environ(), p.extraEnv...)
	stdout, runErr := cmd.Output()
	result, parseErr := parsePythonOutput(stdout, runErr)
	if parseErr != nil {
		return ParseResult{}, parseErr
	}
	// 图片回填必须在临时目录清理前完成（defer 随后执行）。
	result.Markdown = ReplaceImageLinks(result.Markdown, result.PicturePaths, DeriveImageFolder(input.Path), p.images)
	return result, nil
}

// parsePythonOutput 将脚本 stdout 与执行错误映射为 ParseResult。
// 脚本以错误码退出且 stdout 为错误 JSON 时映射为 ParserError；
// 退出失败时脚本 stderr 透传到日志，便于排障（依赖缺失、模型下载失败等）。
func parsePythonOutput(stdout []byte, runErr error) (ParseResult, error) {
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			logWarn("Python 解析脚本退出失败", zap.ByteString("stderr", exitErr.Stderr))
		}
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
