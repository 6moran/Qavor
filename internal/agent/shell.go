package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

// DefaultStreamingShell 流式 Shell 实现，逐行读取命令输出并通过 StreamReader 流式返回。
// 跨平台支持：Windows 使用 cmd.exe，Unix 使用 sh。
type DefaultStreamingShell struct{}

// ExecuteStreaming 执行 Shell 命令并流式返回输出。
// 每一行输出作为一个 chunk 发送，命令结束后发送携带 exit code 的终止 chunk。
func (s *DefaultStreamingShell) ExecuteStreaming(ctx context.Context, req *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if req.Command == "" {
		return nil, fmt.Errorf("command is empty")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", req.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", req.Command)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	sr, sw := schema.Pipe[*filesystem.ExecuteResponse](16)
	go func() {
		defer sw.Close()

		// 并发读取 stdout 和 stderr，合并到一个流中
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.streamLines(ctx, stdout, sw)
		}()
		go func() {
			defer wg.Done()
			s.streamLines(ctx, stderr, sw)
		}()
		wg.Wait()

		// 命令结束，等待退出码
		waitErr := cmd.Wait()
		exitCode := 0
		if waitErr != nil {
			if ee, ok := waitErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else if ctx.Err() == nil {
				exitCode = -1
			}
		}
		sw.Send(&filesystem.ExecuteResponse{
			Output:   "",
			ExitCode: &exitCode,
		}, nil)
	}()

	return sr, nil
}

// streamLines 逐行读取 reader 内容并发送到流中。
func (s *DefaultStreamingShell) streamLines(ctx context.Context, r io.Reader, sw *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 支持最长 1MB 的行
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		if closed := sw.Send(&filesystem.ExecuteResponse{
			Output:   scanner.Text(),
			ExitCode: nil,
		}, nil); closed {
			return
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		sw.Send(&filesystem.ExecuteResponse{}, err)
	}
}
