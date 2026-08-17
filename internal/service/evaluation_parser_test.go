package service

import (
	"strings"
	"testing"
)

func TestParseDatasetJSONL(t *testing.T) {
	t.Run("标准检索+问答条目", func(t *testing.T) {
		content := `{"query": "退款流程是什么？", "gold_chunk_ids": ["c1", "c2"], "gold_answer": "联系客服"}
{"query": "支持哪些支付方式？", "gold_answer": "微信和支付宝"}
{"query": "仅检索问题", "gold_chunk_ids": ["c3"]}`
		items, hasGoldChunks, hasGoldAnswers, err := parseDatasetJSONL([]byte(content))
		if err != nil {
			t.Fatalf("parseDatasetJSONL() error = %v", err)
		}
		if len(items) != 3 {
			t.Fatalf("items = %d, want 3", len(items))
		}
		if !hasGoldChunks || !hasGoldAnswers {
			t.Errorf("hasGoldChunks = %v, hasGoldAnswers = %v, want both true", hasGoldChunks, hasGoldAnswers)
		}
		if items[0].Query != "退款流程是什么？" {
			t.Errorf("items[0].Query = %q", items[0].Query)
		}
		if len(items[0].GoldChunkIDs) != 2 {
			t.Errorf("items[0].GoldChunkIDs = %v, want 2", items[0].GoldChunkIDs)
		}
		if items[2].GoldAnswer != "" {
			t.Errorf("items[2].GoldAnswer = %q, want empty", items[2].GoldAnswer)
		}
		// 排序序号按行递增
		if items[0].SortOrder != 0 || items[2].SortOrder != 2 {
			t.Errorf("sort_order 错误: %d, %d", items[0].SortOrder, items[2].SortOrder)
		}
	})

	t.Run("空行与空白被忽略", func(t *testing.T) {
		content := "\n{\"query\": \"q1\"}\n\n{\"query\": \"q2\"}\n"
		items, _, _, err := parseDatasetJSONL([]byte(content))
		if err != nil {
			t.Fatalf("parseDatasetJSONL() error = %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d, want 2", len(items))
		}
	})

	t.Run("JSON 格式错误报行号", func(t *testing.T) {
		content := "{\"query\": \"ok\"}\nnot-json\n"
		_, _, _, err := parseDatasetJSONL([]byte(content))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if !strings.Contains(err.Error(), "第 2 行") {
			t.Errorf("error = %q, want line number hint", err.Error())
		}
	})

	t.Run("缺少 query 报错", func(t *testing.T) {
		content := `{"gold_answer": "没有问题"}`
		_, _, _, err := parseDatasetJSONL([]byte(content))
		if err == nil {
			t.Fatal("expected error for missing query")
		}
		if !strings.Contains(err.Error(), "query") {
			t.Errorf("error = %q, want query hint", err.Error())
		}
	})

	t.Run("空文件报错", func(t *testing.T) {
		_, _, _, err := parseDatasetJSONL([]byte(""))
		if err == nil {
			t.Fatal("expected error for empty file")
		}
	})

	t.Run("gold_chunk_ids 空串被过滤", func(t *testing.T) {
		content := `{"query": "q", "gold_chunk_ids": ["", "c1", "  "]}`
		items, hasGoldChunks, _, err := parseDatasetJSONL([]byte(content))
		if err != nil {
			t.Fatalf("parseDatasetJSONL() error = %v", err)
		}
		if len(items[0].GoldChunkIDs) != 1 || items[0].GoldChunkIDs[0] != "c1" {
			t.Errorf("GoldChunkIDs = %v, want [c1]", items[0].GoldChunkIDs)
		}
		if !hasGoldChunks {
			t.Error("hasGoldChunks = false, want true")
		}
	})
}

func TestParseQAJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantQ   string
		wantA   string
		wantErr bool
	}{
		{
			name:    "标准 JSON",
			content: `{"query": "问题", "gold_answer": "答案"}`,
			wantQ:   "问题",
			wantA:   "答案",
		},
		{
			name:    "代码块包裹",
			content: "```json\n{\"query\": \"问题2\", \"gold_answer\": \"答案2\"}\n```",
			wantQ:   "问题2",
			wantA:   "答案2",
		},
		{
			name:    "前后夹杂说明文字",
			content: "好的，这是生成的问答对：{\"query\": \"q\", \"gold_answer\": \"a\"} 请查收",
			wantQ:   "q",
			wantA:   "a",
		},
		{
			name:    "无 JSON 报错",
			content: "抱歉，我无法生成",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQAJSON(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQAJSON() error = %v", err)
			}
			if got["query"] != tt.wantQ || got["gold_answer"] != tt.wantA {
				t.Errorf("parseQAJSON() = %v, want query=%q gold_answer=%q", got, tt.wantQ, tt.wantA)
			}
		})
	}
}

func TestParseScoreJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    float64
		wantErr bool
	}{
		{name: "标准 JSON", content: `{"score": 0.85}`, want: 0.85},
		{name: "代码块包裹", content: "```json\n{\"score\": 0.5}\n```", want: 0.5},
		{name: "裸数字", content: "0.92", want: 0.92},
		{name: "整数分数", content: `{"score": 1}`, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScoreJSON(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseScoreJSON() error = %v", err)
			}
			if got["score"] != tt.want {
				t.Errorf("parseScoreJSON() score = %v, want %v", got["score"], tt.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	if got := toInt(float64(10)); got != 10 {
		t.Errorf("toInt(float64) = %d, want 10", got)
	}
	if got := toInt(10); got != 10 {
		t.Errorf("toInt(int) = %d, want 10", got)
	}
	if got := toInt("10"); got != 0 {
		t.Errorf("toInt(string) = %d, want 0", got)
	}
}
