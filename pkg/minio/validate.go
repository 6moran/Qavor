package minio

import (
	"fmt"
	"net/http"
	"strings"
)

// ValidateFileType 校验文件类型是否允许
// allowedTypes 为空时允许所有类型
// 支持通配符匹配，如 "image/*" 匹配所有图片类型
func ValidateFileType(contentType string, allowedTypes []string) error {
	if len(allowedTypes) == 0 {
		return nil
	}

	for _, allowed := range allowedTypes {
		// 精确匹配
		if allowed == contentType {
			return nil
		}
		// 通配符匹配: "image/*" 匹配 "image/png", "image/jpeg" 等
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(contentType, prefix+"/") {
				return nil
			}
		}
	}

	return fmt.Errorf("文件类型 %s 不允许，允许的类型: %v", contentType, allowedTypes)
}

// ValidateFileSize 校验文件大小
func ValidateFileSize(size, maxSize int64) error {
	if maxSize > 0 && size > maxSize {
		maxMB := float64(maxSize) / (1024 * 1024)
		sizeMB := float64(size) / (1024 * 1024)
		return fmt.Errorf("文件大小 %.2fMB 超出限制 %.2fMB", sizeMB, maxMB)
	}
	return nil
}

// DetectContentType 检测文件真实 MIME 类型（不信任客户端传值）
// 读取文件前 512 字节进行检测
func DetectContentType(data []byte) string {
	contentType := http.DetectContentType(data)
	// http.DetectContentType 对某些类型返回不够精确，如返回 "application/octet-stream"
	// 这里可以后续扩展更精确的检测逻辑
	return contentType
}

// SanitizeFileName 清理文件名，移除路径分隔符和特殊字符
func SanitizeFileName(name string) string {
	// 取最后一段路径
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "\\"); idx >= 0 {
		name = name[idx+1:]
	}
	// 替换危险字符
	replacer := strings.NewReplacer(
		"..", "_",
		";", "_",
		"|", "_",
		"&", "_",
		"?", "_",
		"=", "_",
		"#", "_",
		"%", "_",
		" ", "_",
	)
	name = replacer.Replace(name)
	if name == "" {
		name = "unnamed"
	}
	return name
}
