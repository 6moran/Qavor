package agent

import (
	"Qavor/internal/agent/localfs/security"

	"github.com/cloudwego/eino/adk/backgroundtask"
)

// AgentRuntime agent 运行时共享依赖（本地文件系统与后台任务）。
// 由 internal/app/app.go 装配一次，注入所有 deep agent。
type AgentRuntime struct {
	// Policies 共享安全策略（构建后只读，线程安全）。
	Policies *security.Policies
	// WorkspaceRoot agent 默认工作区根目录，每个 agent 在其下建 <slug> 子目录。
	WorkspaceRoot string
	// ShellTimeoutSeconds 单条 shell 命令超时（0=无显式超时，依赖后台任务管理器的前台超时兜底）。
	ShellTimeoutSeconds int
	// Background 全局后台任务管理器。
	Background *backgroundtask.Manager
}
