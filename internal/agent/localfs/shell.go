package localfs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"Qavor/internal/agent/localfs/security"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

// LocalStreamingShell 流式 Shell 实现，带安全管控。
// 逐行读取命令输出并通过 StreamReader 流式返回。
type LocalStreamingShell struct {
	cwd     string // 默认工作目录（shell 子进程 cwd）
	sec     *security.Policies
	timeout time.Duration // 单条命令超时（0=无超时）
}

// NewLocalStreamingShell 创建流式 Shell。
func NewLocalStreamingShell(cwd string, sec *security.Policies, timeout time.Duration) *LocalStreamingShell {
	return &LocalStreamingShell{cwd: cwd, sec: sec, timeout: timeout}
}

// ExecuteStreaming 执行 Shell 命令并流式返回输出。
// 执行前经高危命令黑名单检查；执行中逐行脱敏并截断超限输出；结束时解释退出码。
func (s *LocalStreamingShell) ExecuteStreaming(ctx context.Context, req *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if req.Command == "" {
		return nil, fmt.Errorf("命令不能为空")
	}
	// 高危命令黑名单：命中立即拒绝，不启动进程。
	// 以「工具结果」形式返回拒绝说明，而非以错误中断整个 agent 运行，
	// 这样模型能拿到结果并向用户解释命令被拦截。
	if s.sec != nil && s.sec.Command() != nil {
		if err := s.sec.Command().Check(req.Command); err != nil {
			return s.denialReader(err), nil
		}
	}
	// 超时上下文
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	command := req.Command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// 强制 UTF-8 代码页，避免中文乱码
		command = "chcp 65001 >nul 2>&1 && " + command
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	if s.cwd != "" {
		cmd.Dir = s.cwd
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("命令启动失败: 无法获取输出管道")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("命令启动失败: 无法获取错误管道")
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("命令启动失败: %w", err)
	}

	sr, sw := schema.Pipe[*filesystem.ExecuteResponse](16)
	go func() {
		defer sw.Close()

		state := &shellOutputState{maxBytes: s.maxBytes()}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.streamLines(ctx, stdout, sw, state)
		}()
		go func() {
			defer wg.Done()
			s.streamLines(ctx, stderr, sw, state)
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

		resp := &filesystem.ExecuteResponse{ExitCode: &exitCode}
		// 软失败语义：退出码 1 且为信息性命令（grep 无匹配等）时不标失败
		if s.sec != nil && s.sec.ExitCode() != nil && exitCode != 0 {
			if fail, note := s.sec.ExitCode().Interpret(req.Command, exitCode); !fail {
				resp.ExitCode = nil
				resp.Output = fmt.Sprintf("[info] %s (exit code %d)", note, exitCode)
			}
		}
		sw.Send(resp, nil)
	}()

	return sr, nil
}

// denialReader 将安全策略拒绝转换为一条「拒绝说明」工具结果流，
// 让模型能正常接收并向用户解释命令被拦截，而不是让错误中断 agent 运行。
func (s *LocalStreamingShell) denialReader(err error) *schema.StreamReader[*filesystem.ExecuteResponse] {
	msg := err.Error()
	const prefix = "access denied by security policy: "
	if strings.HasPrefix(msg, prefix) {
		msg = msg[len(prefix):]
	}
	sr, sw := schema.Pipe[*filesystem.ExecuteResponse](1)
	_ = sw.Send(&filesystem.ExecuteResponse{Output: security.SecurityBlockMarker + " 命令执行被安全策略阻止: " + msg}, nil)
	sw.Close()
	return sr
}

func (s *LocalStreamingShell) maxBytes() int {
	if s.sec != nil {
		return s.sec.MaxBytes()
	}
	return 51200
}

// shellOutputState 跨 stdout/stderr 的共享输出截断状态。
type shellOutputState struct {
	mu        sync.Mutex
	total     int
	truncated bool
	maxBytes  int
}

// streamLines 逐行读取 reader 内容并发送到流中，逐行脱敏、累计截断。
func (s *LocalStreamingShell) streamLines(ctx context.Context, r io.Reader, sw *schema.StreamWriter[*filesystem.ExecuteResponse], state *shellOutputState) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 支持最长 1MB 的行
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		if s.sec != nil && s.sec.Redaction() != nil {
			line = s.sec.Redaction().Redact(line)
		}

		state.mu.Lock()
		if state.truncated {
			state.mu.Unlock()
			if closed := sw.Send(&filesystem.ExecuteResponse{Output: "", Truncated: true}, nil); closed {
				return
			}
			continue
		}
		state.total += len(line) + 1
		if state.total > state.maxBytes {
			state.truncated = true
			state.mu.Unlock()
			if closed := sw.Send(&filesystem.ExecuteResponse{Output: line, Truncated: true}, nil); closed {
				return
			}
			continue
		}
		state.mu.Unlock()

		if closed := sw.Send(&filesystem.ExecuteResponse{Output: line, ExitCode: nil}, nil); closed {
			return
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		sw.Send(&filesystem.ExecuteResponse{}, err)
	}
}
