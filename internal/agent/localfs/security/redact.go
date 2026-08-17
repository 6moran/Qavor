package security

import (
	"os"
	"regexp"
	"strings"

	"Qavor/pkg/config"
)

// defaultSensitiveEnvKeys 默认敏感环境变量名（大小写不敏感）。
var defaultSensitiveEnvKeys = []string{
	"API_KEY",
	"SECRET_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PASSWD",
	"PRIVATE_KEY",
	"ACCESS_KEY",
	"AUTH_TOKEN",
	"AUTHORIZATION",
}

// Redaction 输出脱敏：替换流式输出中的敏感 KEY=VALUE / KEY: VALUE 与敏感环境变量裸值。
type Redaction struct {
	enabled   bool
	envKeys   []string // 敏感环境变量名（大写）
	kvEquals  *regexp.Regexp
	keyColons []struct {
		key string
		re  *regexp.Regexp
	}
	secrets []string // 额外需要脱敏的裸值（敏感环境变量的实际值）
}

func newRedaction(cfg config.RedactionConfig, base bool) *Redaction {
	r := &Redaction{enabled: base && (cfg.Enabled == nil || *cfg.Enabled)}
	if !r.enabled {
		return r
	}
	seen := make(map[string]bool)
	for _, k := range append(defaultSensitiveEnvKeys, cfg.ExtraEnvKeys...) {
		u := strings.ToUpper(strings.TrimSpace(k))
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		r.envKeys = append(r.envKeys, u)
	}
	r.kvEquals = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)=([^\s,]+)`)
	for _, k := range r.envKeys {
		re, err := regexp.Compile(`(?i)(` + regexp.QuoteMeta(k) + `):\s*[^\r\n]*`)
		if err != nil {
			continue
		}
		r.keyColons = append(r.keyColons, struct {
			key string
			re  *regexp.Regexp
		}{key: k, re: re})
	}
	return r
}

// AddSecrets 注册需要脱敏的裸值（如敏感环境变量的实际值），任意出现形式都会被替换。
func (r *Redaction) AddSecrets(values ...string) {
	if r == nil || !r.enabled {
		return
	}
	for _, v := range values {
		if v != "" {
			r.secrets = append(r.secrets, v)
		}
	}
}

// Redact 对单行/单 chunk 做脱敏处理。
func (r *Redaction) Redact(s string) string {
	if r == nil || !r.enabled || s == "" {
		return s
	}
	// 1. 敏感 KEY=value 紧凑形式（保留原始 key 大小写）
	s = r.kvEquals.ReplaceAllStringFunc(s, func(m string) string {
		parts := r.kvEquals.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		for _, k := range r.envKeys {
			if strings.EqualFold(parts[1], k) {
				return parts[1] + "=[REDACTED]"
			}
		}
		return m
	})
	// 2. 敏感 KEY: value 整段（覆盖 Authorization: Bearer xxx 等）
	for _, kc := range r.keyColons {
		s = kc.re.ReplaceAllStringFunc(s, func(m string) string {
			parts := kc.re.FindStringSubmatch(m)
			if len(parts) < 2 {
				return m
			}
			return parts[1] + "=[REDACTED]"
		})
	}
	// 3. 裸值脱敏
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "[REDACTED]")
	}
	return s
}

// AddEnvSecrets 从 os.Environ 提取与敏感 key 匹配的环境变量值并注册。
func (r *Redaction) AddEnvSecrets() {
	if r == nil || !r.enabled {
		return
	}
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			continue
		}
		key, value := kv[:idx], kv[idx+1:]
		for _, k := range r.envKeys {
			if strings.EqualFold(key, k) && value != "" {
				r.AddSecrets(value)
			}
		}
	}
}
