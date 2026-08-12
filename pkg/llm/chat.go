package llm

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ErrEmptyResponse 模型返回空响应的错误。
var ErrEmptyResponse = errors.New("模型返回空响应")

// Chat 执行一次独立的单轮 User 对话调用并返回文本。
// timeout 作为调用超时（<=0 时不额外限制，沿用调用方 ctx）。
// 错误统一经 ClassifyError 分类，可通过 errors.As(err, *ClassifiedError) 获取友好提示。
func Chat(ctx context.Context, model ChatModel, prompt string, timeout time.Duration) (string, error) {
	return ChatMessages(ctx, model, []*schema.Message{{Role: schema.User, Content: prompt}}, timeout)
}

// ChatMessages 执行一次独立的多消息对话调用并返回文本。
// timeout 语义与 Chat 一致；支持携带 system 提示词等自定义消息。
func ChatMessages(ctx context.Context, model ChatModel, messages []*schema.Message, timeout time.Duration) (string, error) {
	if model == nil {
		return "", &ClassifiedError{Category: CategoryUnknown, Friendly: "模型客户端未配置"}
	}
	callCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	resp, err := model.Generate(callCtx, messages)
	if err != nil {
		return "", ClassifyError(err)
	}
	if resp == nil {
		return "", &ClassifiedError{Category: CategoryUnknown, Friendly: ErrEmptyResponse.Error(), Err: ErrEmptyResponse}
	}
	return resp.Content, nil
}
