package ingestion

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// ImageUploader 上传解析产出的图片到对象存储。
type ImageUploader interface {
	// UploadImage 上传图片字节，folder 为目标目录（不含文件名），返回可公开访问的 URL。
	UploadImage(folder, filename string, data []byte) (url string, err error)
}

// imageLinkPattern 匹配 Markdown 图片语法 ![alt](src)。
var imageLinkPattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

// dataURIPattern 匹配内嵌 base64 图片 data:image/xxx;base64,....
var dataURIPattern = regexp.MustCompile(`data:image/([a-zA-Z0-9.+-]+);base64,([A-Za-z0-9+/=]+)`)

// DeriveImageFolder 从 MinIO 对象路径 knowledge/{kbID}/{fileID}.{ext} 推导
// 解析产物的图片目录 knowledge-internal/{kbID}/{fileID}/derived/images。
// 无法解析时回退到通用目录 knowledge-internal/derived-images。
func DeriveImageFolder(objectPath string) string {
	parts := strings.Split(strings.Trim(objectPath, "/"), "/")
	if len(parts) >= 3 && parts[0] == "knowledge" {
		fileBase := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
		return fmt.Sprintf("knowledge-internal/%s/%s/derived/images", parts[1], fileBase)
	}
	return "knowledge-internal/derived-images"
}

// logWarn 在日志器已初始化时输出告警（未初始化时忽略，避免 nil 指针）。
func logWarn(msg string, fields ...zap.Field) {
	if logger.Initialized() {
		logger.Warn(msg, fields...)
	}
}

// ReplaceImageLinks 将 Markdown 中对本地临时图片路径的引用替换为上传后的 URL。
// 单张图片读取或上传失败时保留原引用并跳过，不使整体失败；uploader 为 nil 时原样返回。
func ReplaceImageLinks(markdown string, paths []string, folder string, uploader ImageUploader) string {
	if uploader == nil || len(paths) == 0 {
		return markdown
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			logWarn("读取解析产物图片失败", zap.String("path", p), zap.Error(err))
			continue
		}
		url, err := uploader.UploadImage(folder, filepath.Base(p), data)
		if err != nil {
			logWarn("上传解析产物图片失败", zap.String("path", p), zap.Error(err))
			continue
		}
		markdown = strings.ReplaceAll(markdown, p, url)
	}
	return markdown
}

// ReplaceDataURILinks 将 Markdown 中内嵌的 data URI 图片解码上传并替换为 URL。
// 解析或上传失败时保留原引用；uploader 为 nil 时原样返回。
func ReplaceDataURILinks(markdown string, folder string, uploader ImageUploader) string {
	if uploader == nil {
		return markdown
	}
	return imageLinkPattern.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := imageLinkPattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		alt, src := sub[1], sub[2]
		data, ext, ok := parseDataURI(src)
		if !ok {
			return match
		}
		url, err := uploader.UploadImage(folder, fmt.Sprintf("embedded_%d.%s", time.Now().UnixNano(), ext), data)
		if err != nil {
			logWarn("上传 data URI 图片失败", zap.Error(err))
			return match
		}
		return fmt.Sprintf("![%s](%s)", alt, url)
	})
}

// parseDataURI 解析 data:image/xxx;base64,.... 返回 (图片字节, 扩展名, 是否成功)。
func parseDataURI(src string) ([]byte, string, bool) {
	sub := dataURIPattern.FindStringSubmatch(src)
	if len(sub) != 3 {
		return nil, "", false
	}
	ext := sub[1]
	if ext == "jpeg" {
		ext = "jpg"
	}
	data, err := base64.StdEncoding.DecodeString(sub[2])
	if err != nil {
		return nil, "", false
	}
	return data, ext, true
}
