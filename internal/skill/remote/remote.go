package remote

import "fmt"

// RemoteSkillMeta 与前端 SkillCardList 搜索/列表项字段对齐
type RemoteSkillMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`             // 仓库地址 owner/repo，搜索结果按此分组
	Installs    int64  `json:"installs,omitempty"` // skills.sh 安装量
	Slug        string `json:"slug,omitempty"`     // skill 目录名
}

// SourceProvider 拉取源：本迭代仅 GitHub 一个实现
type SourceProvider interface {
	Name() string
	Recognize(source string) bool // 是否处理该来源
	List(source string) ([]RemoteSkillMeta, error)
	Fetch(source, slug string) ([]byte, error) // 返回单个 skill 的 zip 字节
}

var (
	providers []SourceProvider
)

// RegisterSource 注册拉取源
func RegisterSource(p SourceProvider) { providers = append(providers, p) }

// Resolve 按 source 找到第一个可识别的拉取源
func Resolve(source string) (SourceProvider, error) {
	for _, p := range providers {
		if p.Recognize(source) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("不支持的来源: %s", source)
}
