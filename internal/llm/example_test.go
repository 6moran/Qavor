package llm

import (
	"context"
	"fmt"
	"io"
	"testing"

	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func TestGenerate(t *testing.T) {
	// 创建配置
	temperature := float32(0.7)
	maxTokens := 2048
	topP := float32(1.0)

	cfg := &Config{
		Model:               "gpt-4o",
		APIKey:              "your-api-key",
		Temperature:         &temperature,
		MaxCompletionTokens: &maxTokens,
		TopP:                &topP,
	}

	// 创建客户端
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 构建消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一个有帮助的助手。",
		},
		{
			Role:    schema.User,
			Content: "你好，请介绍一下你自己。",
		},
	}

	// 同步生成
	resp, err := client.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Content)
}

func TestStream(t *testing.T) {
	// 创建配置
	temperature := float32(0.7)
	maxTokens := 2048
	topP := float32(1.0)

	cfg := &Config{
		Model:               "gpt-4o",
		APIKey:              "your-api-key",
		Temperature:         &temperature,
		MaxCompletionTokens: &maxTokens,
		TopP:                &topP,
	}

	// 创建客户端
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 构建消息
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "写一首关于春天的短诗。",
		},
	}

	// 流式生成
	stream, err := client.Stream(context.Background(), messages)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}
	defer stream.Close()

	// 读取流式响应
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to recv: %v", err)
		}

		fmt.Print(chunk.Content)
	}
	fmt.Println()
}

func TestWithResponseFormat(t *testing.T) {
	// 创建配置
	temperature := float32(0.7)
	maxTokens := 2048
	topP := float32(1.0)

	cfg := &Config{
		Model:               "gpt-4o",
		APIKey:              "your-api-key",
		Temperature:         &temperature,
		MaxCompletionTokens: &maxTokens,
		TopP:                &topP,
		ResponseFormat: &einoOpenAI.ChatCompletionResponseFormat{
			Type: einoOpenAI.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	// 创建客户端
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 构建消息
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "返回一个 JSON 对象，包含 name 和 age 字段",
		},
	}

	// 同步生成
	resp, err := client.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	fmt.Printf("JSON Response: %s\n", resp.Content)
}
