package agent

import "path/filepath"

// buildExecuteToolDesc 生成 execute 工具给模型看的自定义描述。
// 描述必须如实说明"不是隔离沙箱"的真实边界，避免模型误以为命令被关在沙箱里。
// workDir 为 agent 工作区绝对路径（data/workspaces/<slug>），用于告知模型真实工作区位置。
// 统一转正斜杠，避免 Windows 反斜杠被模型误作转义符。
// 安全边界提示不应被随意改动（改动需走代码评审）。
func buildExecuteToolDesc(workDir string) string {
	dir := filepath.ToSlash(workDir)
	return `
在 agent 的工作目录 ` + dir + ` 下执行 shell 命令，返回合并的 stdout/stderr 和退出码。

安全边界（重要，必须遵守）：
- 这不是隔离沙箱。命令以服务器进程的权限直接执行，可以 cd 到任意目录、读取任何该账号可访问的路径。只有毁灭性命令（rm -rf /、dd 刷盘、shutdown/reboot/halt/poweroff）会被拦截。
- 只在你的工作目录内操作，不要读取、修改或外传工作目录以外的文件。
- 输出中形如 KEY=值 的敏感内容会被替换为 [REDACTED]，不要在回复中复述密钥。
- 输出超过约 50KB 会被截断。
- 如果命令执行被安全策略阻止（结果含"该命令不在允许的执行范围内"），不要尝试用其他命令或方式绕过以获取相同信息。直接向用户说明该命令被安全策略阻止即可，本次操作即告终止。

用法：
- command 为必填。
- 长时间运行的命令（服务器、监听进程等无需等待的）设置 run_in_background=true。完成后你会收到通知，用 task_output 查询状态、task_stop 取消。
- timeout（毫秒）设置最大等待时间，省略则用默认值。
- 依赖命令用 && 连接，不依赖的用 ; 连接；不要在引号外使用换行。
- 尽量使用绝对路径，避免 cd。
- 不要用 cat/head/tail 读文件、find/grep 搜索，改用 read_file、glob、grep 工具。

示例：
好的：execute(command="pytest /foo/bar/tests")；execute(command="npm run dev", run_in_background=true)
不好的：execute(command="cd /foo/bar && pytest tests")；execute(command="cat file.txt")
`
}

// buildFsToolDesc 生成文件操作工具（ls/read_file/write_file/edit_file/glob/grep）给模型看的自定义描述。
// 这些工具共享同一根隔离语义：所有路径相对于 agent 工作区根 workDir。
// 覆盖 eino filesystem 默认描述（默认说"能读机器上所有文件"，会误导模型尝试越界）。
// 统一转正斜杠，避免 Windows 反斜杠被模型误作转义符。
func buildFsToolDesc(workDir string) string {
	dir := filepath.ToSlash(workDir)
	return `在 agent 的工作目录 ` + dir + ` 内操作文件。
所有路径都相对于该工作区根目录；工作区根目录之外的文件不可访问（访问会被拒绝）。
- 工作区根目录: ` + dir + `
- 只在这个工作区内读写文件，不要尝试读取、修改或引用工作区外的路径。
- 路径用绝对路径或相对工作区根的相对路径皆可。
`
}
