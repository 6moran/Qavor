package ocr

import (
	"context"
	"net/http"

	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

// Engine 描述一个可用的 OCR 引擎。
// 字段命名与前端 OCRSelector / FileUploadModal 的期望保持一致：
// engine_id 用于选中与回传，display_name 用于界面展示。
type Engine struct {
	EngineID    string `json:"engine_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
}

// Health 描述某个 OCR 引擎的健康状态。
// status 取值需落在前端判定为可用的集合内（healthy / configured / ok），
// 否则会被归入"不可用"列表并禁用选择。
type Health struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// OCRConfigProvider 提供通用 OCR API 的配置（供健康检查判断可用性）。
type OCRConfigProvider interface {
	GetOCRAPIConfig(ctx context.Context) (service.OCRAPIConfig, error)
}

// Controller 提供 OCR 引擎列表与健康检查的 HTTP 接口。
// 路由挂载在 /api/v1/system/ocr 下。
type Controller struct {
	engines  []Engine
	ocrCfg   OCRConfigProvider
}

// NewController 创建 OCR 控制器并初始化引擎清单。
//
// 引擎清单与 docs/学习 中的 OCR 方案选型保持一致：
//   - rapid_ocr：本地 RapidOCR，已随服务部署，默认可用；
//   - api_ocr：通用 HTTP OCR API，需要在系统设置中配置服务地址与 API Key；
//   - mineru_host / mineru_official / pp_structure_v3 / paddleocr_api：
//     对应系统设置中的 OCR 服务配置项，需要先在系统设置中完成后端配置才可用。
//
// ocrCfg 可选；为 nil 时 api_ocr 恒判定为不可用。
func NewController(ocrCfg OCRConfigProvider) *Controller {
	return &Controller{
		engines: []Engine{
			{
				EngineID:    "rapid_ocr",
				DisplayName: "RapidOCR（本地）",
				Description: "本地 Python RapidOCR 引擎，已随服务部署，无需额外配置。",
			},
			{
				EngineID:    "api_ocr",
				DisplayName: "通用 OCR API",
				Description: "通过 HTTP 接口调用外部 OCR 服务，需要在系统设置中配置服务地址与 API Key。",
			},
			{
				EngineID:    "mineru_host",
				DisplayName: "MinerU 自托管服务",
				Description: "通过内网 MinerU 服务进行解析，需要在系统设置中配置服务地址。",
			},
			{
				EngineID:    "mineru_official",
				DisplayName: "MinerU 官方 API",
				Description: "MinerU 官方托管 API，需要在系统设置中配置 API Key。",
			},
			{
				EngineID:    "pp_structure_v3",
				DisplayName: "PaddleOCR PP-StructureV3",
				Description: "PaddleOCR PP-StructureV3 服务，需要在系统设置中配置服务地址。",
			},
			{
				EngineID:    "paddleocr_api",
				DisplayName: "PaddleOCR API",
				Description: "PaddleOCR 托管 API，需要在系统设置中配置访问凭证。",
			},
		},
		ocrCfg: ocrCfg,
	}
}

// RegisterRoutes 在 /api/v1/system/ocr 下注册路由。
func (c *Controller) RegisterRoutes(r *gin.RouterGroup) {
	grp := r.Group("/system/ocr")
	grp.GET("/options", c.GetOptions)
	grp.GET("/health", c.GetHealth)
}

// GetOptions 返回可用 OCR 引擎列表与默认引擎。
// 前端 OCRSelector 与 FileUploadModal 都依赖此接口的 engines 与 default_engine 字段。
func (c *Controller) GetOptions(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"engines":        c.engines,
		"default_engine": "rapid_ocr",
	})
}

// GetHealth 返回各 OCR 引擎的健康状态。
//
// 当前策略：本地 rapid_ocr 默认可用（其可用性在解析阶段再做真实兜底）；
// api_ocr 读取系统设置中的通用 OCR API 配置，配置了服务地址即视为可配置；
// 其余远程引擎需要先完成系统设置中的服务配置，因此在未接入配置子系统前统一标记为不可用，
// 前端会将其折叠进"不可用"分组，不会误导用户选择。
func (c *Controller) GetHealth(ctx *gin.Context) {
	health := make(map[string]Health, len(c.engines))
	for _, e := range c.engines {
		switch e.EngineID {
		case "rapid_ocr":
			health[e.EngineID] = Health{Status: "configured", Message: "本地 RapidOCR 引擎可用"}
		case "api_ocr":
			health[e.EngineID] = c.apiOCRHealth(ctx)
		default:
			health[e.EngineID] = Health{
				Status:  "unavailable",
				Message: "尚未配置该 OCR 服务（请在系统设置中配置后启用）",
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"health": health})
}

// apiOCRHealth 依据通用 OCR API 配置判断可用性。
func (c *Controller) apiOCRHealth(ctx *gin.Context) Health {
	if c.ocrCfg == nil {
		return Health{Status: "unavailable", Message: "未注入 OCR 配置读取器"}
	}
	cfg, err := c.ocrCfg.GetOCRAPIConfig(ctx.Request.Context())
	if err != nil {
		return Health{Status: "unavailable", Message: "读取 OCR 配置失败"}
	}
	if cfg.BaseURL == "" {
		return Health{Status: "unavailable", Message: "尚未配置通用 OCR API 服务地址"}
	}
	message := "通用 OCR API 已配置"
	if cfg.APIKey == "" {
		message = "已配置服务地址，未配置 API Key（可能仅适用于免鉴权服务）"
	}
	return Health{Status: "configured", Message: message}
}
