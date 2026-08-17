package response

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qerrors "Qavor/pkg/errors"

	"github.com/gin-gonic/gin"
)

func TestBizErrorSerializesDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		BizError(c, qerrors.NewWithDetail(qerrors.CodeLLMRequestFailed, "账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额", "openai: error, status code: 402"))
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	router.ServeHTTP(w, req)

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if body.Code != qerrors.CodeLLMRequestFailed || body.Message == "" || body.Detail == "" {
		t.Fatalf("响应=%+v，期望输出 message 与 detail", body)
	}
	if body.Detail != "openai: error, status code: 402" {
		t.Fatalf("detail=%q", body.Detail)
	}
}

func TestBizErrorOmitsEmptyDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		BizError(c, qerrors.New(qerrors.CodeInvalidParam, "参数错误"))
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	router.ServeHTTP(w, req)

	if bytes.Contains(w.Body.Bytes(), []byte("detail")) {
		t.Fatalf("空 Detail 不应序列化: %s", w.Body.String())
	}
}
