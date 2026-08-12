package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

const defaultRerankerTimeout = 60 * time.Second

// HTTPRerankerConfig 描述 HTTP 重排客户端配置。
type HTTPRerankerConfig struct {
	Model   string
	BaseURL string
	APIKey  string
	Headers map[string]string
	Timeout time.Duration
}

// HTTPReranker 实现 OpenAI/Cohere 风格的重排协议。
type HTTPReranker struct {
	model    string
	endpoint string
	apiKey   string
	headers  map[string]string
	client   *http.Client
}

// NewHTTPReranker 创建并校验 HTTP 重排客户端。
func NewHTTPReranker(cfg HTTPRerankerConfig) (*HTTPReranker, error) {
	endpoint, err := normalizeRerankEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultRerankerTimeout
	}
	headers := make(map[string]string, len(cfg.Headers))
	for key, value := range cfg.Headers {
		headers[key] = value
	}
	return &HTTPReranker{
		model:    cfg.Model,
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		headers:  headers,
		client:   &http.Client{Timeout: timeout},
	}, nil
}

type rerankHTTPRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n"`
	ReturnDocuments bool     `json:"return_documents"`
}

type rerankHTTPResponse struct {
	Results []struct {
		Index          int         `json:"index"`
		RelevanceScore json.Number `json:"relevance_score"`
	} `json:"results"`
}

// Rerank 调用外部重排服务并严格校验返回索引与分数。
func (r *HTTPReranker) Rerank(ctx context.Context, query string, documents []RerankDocument, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("重排候选不能为空")
	}
	if topN <= 0 {
		return nil, fmt.Errorf("重排 top_n 必须大于 0")
	}
	contents := make([]string, len(documents))
	for index, document := range documents {
		contents[index] = document.Content
	}
	payload, err := json.Marshal(rerankHTTPRequest{
		Model:           r.model,
		Query:           query,
		Documents:       contents,
		TopN:            topN,
		ReturnDocuments: false,
	})
	if err != nil {
		return nil, fmt.Errorf("编码 Rerank 请求: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("创建 Rerank 请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 Rerank 服务: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Rerank 服务返回 HTTP %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	decoder.UseNumber()
	var decoded rerankHTTPResponse
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("解析 Rerank 响应 JSON: %w", err)
	}
	if len(decoded.Results) == 0 {
		return nil, fmt.Errorf("Rerank 服务返回空结果")
	}

	seen := make(map[int]struct{}, len(decoded.Results))
	results := make([]RerankResult, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		if item.Index < 0 || item.Index >= len(documents) {
			return nil, fmt.Errorf("Rerank 结果索引 %d 越界", item.Index)
		}
		score, err := strconv.ParseFloat(item.RelevanceScore.String(), 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, fmt.Errorf("Rerank 结果分数无效")
		}
		if _, duplicate := seen[item.Index]; duplicate {
			if logger.Initialized() {
				logger.Warn("Rerank 返回重复候选索引，已忽略后续结果", zap.Int("index", item.Index))
			}
			continue
		}
		seen[item.Index] = struct{}{}
		results = append(results, RerankResult{Index: item.Index, Score: score})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Rerank 结果无法映射到候选文档")
	}
	return results, nil
}

func normalizeRerankEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("Rerank Base URL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Rerank Base URL 无效")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/rerank") {
		path += "/v1/rerank"
	}
	parsed.Path = path
	return parsed.String(), nil
}

var _ Reranker = (*HTTPReranker)(nil)
