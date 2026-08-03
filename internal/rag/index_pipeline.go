package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// indexState 索引链节点间共享的本地状态，用于在输出阶段重建 IndexOutput。
type indexState struct {
	chunks []*schema.Document
}

// DocumentIndexPipeline 通过已编译的 Eino compose.Chain 执行文档索引。
// 实现 DocumentIndexer 接口，保持 Worker 调用方式不变。
type DocumentIndexPipeline struct {
	runnable compose.Runnable[IndexInput, IndexOutput]
}

// NewDocumentIndexPipeline 构建 Document -> Transformer -> Indexer 的 Eino Chain 并编译。
// 应用启动时调用一次，之后复用 Runnable。
func NewDocumentIndexPipeline(transformer document.Transformer, idx indexer.Indexer) (*DocumentIndexPipeline, error) {
	if transformer == nil {
		return nil, errors.New("document transformer is required")
	}
	if idx == nil {
		return nil, errors.New("indexer is required")
	}

	chain := compose.NewChain[IndexInput, IndexOutput](
		compose.WithGenLocalState(func(_ context.Context) *indexState {
			return &indexState{}
		}),
	)

	// 输入适配：IndexInput -> []*schema.Document
	chain.AppendLambda(compose.InvokableLambda(func(_ context.Context, in IndexInput) ([]*schema.Document, error) {
		if strings.TrimSpace(in.Markdown) == "" {
			return nil, errors.New("empty markdown content")
		}
		return []*schema.Document{{
			Content: in.Markdown,
			MetaData: map[string]any{
				MetaKeyKBID:     in.KBID,
				MetaKeyFileID:   in.FileID,
				MetaKeyFilename: in.Filename,
			},
		}}, nil
	}))

	// 分块：[]*schema.Document -> []*schema.Document，同时把分块结果写入状态。
	chain.AppendDocumentTransformer(transformer, compose.WithStatePostHandler(func(_ context.Context, docs []*schema.Document, state *indexState) ([]*schema.Document, error) {
		state.chunks = docs
		return docs, nil
	}))

	// 索引：[]*schema.Document -> []string
	chain.AppendIndexer(idx)

	// 输出适配：[]string -> IndexOutput，从状态读取分块详情。
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, _ []string) (IndexOutput, error) {
		var out IndexOutput
		err := compose.ProcessState[*indexState](ctx, func(_ context.Context, s *indexState) error {
			out.Chunks = make([]IndexedChunk, 0, len(s.chunks))
			for i, doc := range s.chunks {
				out.Chunks = append(out.Chunks, IndexedChunk{
					ChunkID:    doc.ID,
					ChunkIndex: i,
					Content:    doc.Content,
					TokenCount: metaDataInt(doc, MetaKeyTokenCount, utils.CountTokens(doc.Content)),
				})
			}
			return nil
		})
		return out, err
	}))

	runnable, err := chain.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compile index chain: %w", err)
	}
	return &DocumentIndexPipeline{runnable: runnable}, nil
}

// Index 实现 DocumentIndexer 接口，内部调用已编译的 Eino Chain。
func (p *DocumentIndexPipeline) Index(ctx context.Context, in IndexInput) (*IndexOutput, error) {
	if p == nil || p.runnable == nil {
		return nil, errors.New("index pipeline not compiled")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err := p.runnable.Invoke(ctx, in)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
