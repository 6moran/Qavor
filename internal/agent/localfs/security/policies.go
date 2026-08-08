package security

import "Qavor/pkg/config"

// Policies 聚合所有安全管控策略。
// 构建后只读，可跨 agent 共享，线程安全。
type Policies struct {
	enabled     bool
	credentials *Credentials
	command     *Command
	redaction   *Redaction
	syntax      *Syntax
	staleness   *FileState
	linePrefix  *LinePrefix
	exitCode    *ExitCode
	maxBytes    int
}

// NewPolicies 从配置构建安全策略。
// nil 配置按"全部机制默认开启"处理（与 ApplyDefaults 的语义一致）。
func NewPolicies(cfg *config.SecurityConfig) *Policies {
	if cfg == nil {
		cfg = &config.SecurityConfig{}
	}
	enabled := cfg.Enabled == nil || *cfg.Enabled
	p := &Policies{enabled: enabled}
	p.credentials = newCredentials(cfg.Credentials, enabled)
	p.command = newCommand(cfg.Command, enabled)
	p.redaction = newRedaction(cfg.Redaction, enabled)
	p.syntax = newSyntax(cfg.Syntax, enabled)
	p.staleness = newFileState(cfg.Staleness, enabled)
	p.linePrefix = newLinePrefix(cfg.LinePrefix, enabled)
	p.exitCode = newExitCode(cfg.ExitCode, enabled)
	p.maxBytes = cfg.Output.MaxBytes
	if p.maxBytes <= 0 {
		p.maxBytes = 51200
	}
	return p
}

// Enabled 返回总开关状态。
func (p *Policies) Enabled() bool { return p != nil && p.enabled }

// Credentials 返回凭据路径守卫。
func (p *Policies) Credentials() *Credentials {
	if p == nil {
		return nil
	}
	return p.credentials
}

// Command 返回高危命令黑名单。
func (p *Policies) Command() *Command {
	if p == nil {
		return nil
	}
	return p.command
}

// Redaction 返回输出脱敏。
func (p *Policies) Redaction() *Redaction {
	if p == nil {
		return nil
	}
	return p.redaction
}

// Syntax 返回写前语法预检。
func (p *Policies) Syntax() *Syntax {
	if p == nil {
		return nil
	}
	return p.syntax
}

// Staleness 返回陈旧警告状态跟踪。
func (p *Policies) Staleness() *FileState {
	if p == nil {
		return nil
	}
	return p.staleness
}

// LinePrefix 返回 read 行号前缀检测。
func (p *Policies) LinePrefix() *LinePrefix {
	if p == nil {
		return nil
	}
	return p.linePrefix
}

// ExitCode 返回退出码语义解释。
func (p *Policies) ExitCode() *ExitCode {
	if p == nil {
		return nil
	}
	return p.exitCode
}

// MaxBytes 返回 shell 输出截断阈值（字节）。
func (p *Policies) MaxBytes() int {
	if p == nil || p.maxBytes <= 0 {
		return 51200
	}
	return p.maxBytes
}
