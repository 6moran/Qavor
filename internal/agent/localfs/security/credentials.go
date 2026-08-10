package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"Qavor/pkg/config"
)

// defaultCredentialPatterns 内置敏感文件模式（大小写不敏感，按文件名/路径匹配）。
// 宽泛模式（secret/token 等）可能误拦正常文件，这是有意的安全取舍，用户可追加自定义模式。
var defaultCredentialPatterns = []string{
	`\.env(\.\w+)?$`, // .env / .env.local / .env.production
	`\.pem$`,
	`\.key$`,
	`\.p12$`,
	`\.pfx$`,
	`secret`,
	`credential`,
	`token`,
	`password`,
	`api[_-]?key`,
}

// Credentials 凭据路径守卫：拦截工作区内可能含密钥的文件读写。
type Credentials struct {
	enabled  bool
	patterns []*regexp.Regexp
}

func newCredentials(cfg config.CredentialsConfig, base bool) *Credentials {
	c := &Credentials{enabled: base && (cfg.Enabled == nil || *cfg.Enabled)}
	if !c.enabled {
		return c
	}
	patterns := append([]string{}, defaultCredentialPatterns...)
	patterns = append(patterns, cfg.ExtraPatterns...)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		re, err := regexp.Compile("(?i)" + p)
		if err == nil {
			c.patterns = append(c.patterns, re)
		}
	}
	return c
}

// IsSensitive 判断路径是否命中敏感文件模式。
// path 应为解析后的绝对路径（含 filepath.Clean 与 EvalSymlinks 两种形态，由调用方各检查一次）。
func (c *Credentials) IsSensitive(path string) bool {
	if c == nil || !c.enabled || path == "" {
		return false
	}
	// /proc/*/environ（仅 Unix；进程环境镜像）
	if strings.HasPrefix(path, "/proc/") && strings.HasSuffix(path, "/environ") {
		return true
	}
	base := filepath.Base(path)
	for _, re := range c.patterns {
		if re.MatchString(path) || re.MatchString(base) {
			return true
		}
	}
	return false
}

// DenyMessage 返回统一拒绝消息。
func (c *Credentials) DenyMessage(path string) error {
	return fmt.Errorf("%w: 该文件为受限文件，无权访问", ErrDenied)
}
