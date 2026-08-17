package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// answerGraphState 问答 Graph 节点间共享的本地状态。
// input 保存原始输入（含 KB IDs 和 query），docs 保存检索结果用于构造 citations。
type answerGraphState struct {
	input AnswerInput
	docs  []*schema.Document
}

// 节点 key 常量。
const (
	nodeInputAdapter  = "input_adapter"
	nodeRetrieve      = "retrieve"
	nodeEmptyAnswer   = "empty_answer"
	nodeContextMapper = "context_mapper"
	nodeChatTemplate  = "chat_template"
	nodeChatModel     = "chat_model"
	nodeOutputMapper  = "output_mapper"
)

// 分支目标节点。
const (
	branchEmpty         = nodeEmptyAnswer
	branchContextMapper = nodeContextMapper
)

// AnswerEngine 通过已编译的 Eino compose.Graph 执行问答。
// 实现 AnswerChain 接口，保持 Service 调用方式不变。
type AnswerEngine struct {
	runnable compose.Runnable[AnswerInput, AnswerOutput]
}

// NewAnswerGraph 构建并编译问答 Graph。
// 应用启动时调用一次，之后复用 Runnable。
// ret 为 Eino retriever.Retriever；tpl 为 Eino prompt.ChatTemplate；chatModel 为 Eino model.BaseChatModel。
func NewAnswerGraph(ret retriever.Retriever, tpl prompt.ChatTemplate, chatModel model.BaseChatModel) (*AnswerEngine, error) {
	if ret == nil {
		return nil, errors.New("retriever is required")
	}
	if tpl == nil {
		return nil, errors.New("chat template is required")
	}
	if chatModel == nil {
		return nil, ErrLLMNotConfigured
	}

	graph := compose.NewGraph[AnswerInput, AnswerOutput](
		compose.WithGenLocalState(func(_ context.Context) *answerGraphState {
			return &answerGraphState{}
		}),
	)

	// 1. 输入适配：AnswerInput -> string (query)，同时把输入保存到状态。
	if err := graph.AddLambdaNode(nodeInputAdapter,
		compose.InvokableLambda(func(_ context.Context, in AnswerInput) (string, error) {
			if strings.TrimSpace(in.Query) == "" {
				return "", errors.New("empty query")
			}
			if len(in.KnowledgeBaseIDs) == 0 {
				return "", errors.New("no knowledge base ids")
			}
			return in.Query, nil
		}),
		compose.WithStatePreHandler(func(_ context.Context, in AnswerInput, state *answerGraphState) (AnswerInput, error) {
			state.input = in
			return in, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add input_adapter node: %w", err)
	}

	// 2. 检索：string -> []*schema.Document。
	// 从状态读取 KB IDs，通过类型化自定义 Option 传入 Retriever。
	// 检索结果同时写入状态，供输出映射构造 citations。
	if err := graph.AddLambdaNode(nodeRetrieve,
		compose.InvokableLambda(func(ctx context.Context, query string) ([]*schema.Document, error) {
			var kbs []string
			if err := compose.ProcessState[*answerGraphState](ctx, func(_ context.Context, s *answerGraphState) error {
				kbs = s.input.KnowledgeBaseIDs
				return nil
			}); err != nil {
				return nil, err
			}
			docs, err := ret.Retrieve(ctx, query, WithKnowledgeBaseIDs(kbs))
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrRetrievalUnavailable, err)
			}
			if docs == nil {
				docs = []*schema.Document{}
			}
			return docs, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, docs []*schema.Document, state *answerGraphState) ([]*schema.Document, error) {
			state.docs = docs
			return docs, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add retrieve node: %w", err)
	}

	// 3. 命中判断分支：无命中直接走 empty_answer，不调用 ChatModel。
	branch := compose.NewGraphBranch(func(_ context.Context, docs []*schema.Document) (string, error) {
		if len(docs) == 0 {
			return branchEmpty, nil
		}
		return branchContextMapper, nil
	}, map[string]bool{branchEmpty: true, branchContextMapper: true})

	// 4. 无命中分支：[]*schema.Document -> AnswerOutput，不调用 LLM。
	if err := graph.AddLambdaNode(nodeEmptyAnswer,
		compose.InvokableLambda(func(_ context.Context, _ []*schema.Document) (AnswerOutput, error) {
			return AnswerOutput{Answer: EmptyAnswer, Citations: []Citation{}}, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add empty_answer node: %w", err)
	}

	// 5. Context 映射：[]*schema.Document -> map[string]any (query + context)。
	// 从状态读取 query，将检索文档拼接为 context 字符串。
	if err := graph.AddLambdaNode(nodeContextMapper,
		compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) (map[string]any, error) {
			var query string
			if err := compose.ProcessState[*answerGraphState](ctx, func(_ context.Context, s *answerGraphState) error {
				query = s.input.Query
				return nil
			}); err != nil {
				return nil, err
			}
			return map[string]any{
				"query":   query,
				"context": BuildContextFromDocs(docs),
			}, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add context_mapper node: %w", err)
	}

	// 6. ChatTemplate：map[string]any -> []*schema.Message。
	if err := graph.AddChatTemplateNode(nodeChatTemplate, tpl); err != nil {
		return nil, fmt.Errorf("add chat_template node: %w", err)
	}

	// 7. ChatModel：[]*schema.Message -> *schema.Message。
	if err := graph.AddChatModelNode(nodeChatModel, chatModel); err != nil {
		return nil, fmt.Errorf("add chat_model node: %w", err)
	}

	// 8. 输出映射：*schema.Message -> AnswerOutput。
	// Citations 严格由检索文档生成，模型只提供答案文本。
	if err := graph.AddLambdaNode(nodeOutputMapper,
		compose.InvokableLambda(func(ctx context.Context, msg *schema.Message) (AnswerOutput, error) {
			var docs []*schema.Document
			if err := compose.ProcessState[*answerGraphState](ctx, func(_ context.Context, s *answerGraphState) error {
				docs = s.docs
				return nil
			}); err != nil {
				return AnswerOutput{}, err
			}
			answer := ""
			if msg != nil {
				answer = msg.Content
			}
			citations := buildCitations(docs)
			return AnswerOutput{Answer: answer, Citations: citations}, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add output_mapper node: %w", err)
	}

	// 连边。
	if err := graph.AddEdge(compose.START, nodeInputAdapter); err != nil {
		return nil, fmt.Errorf("add edge START->input_adapter: %w", err)
	}
	if err := graph.AddEdge(nodeInputAdapter, nodeRetrieve); err != nil {
		return nil, fmt.Errorf("add edge input_adapter->retrieve: %w", err)
	}
	if err := graph.AddBranch(nodeRetrieve, branch); err != nil {
		return nil, fmt.Errorf("add branch after retrieve: %w", err)
	}
	if err := graph.AddEdge(nodeContextMapper, nodeChatTemplate); err != nil {
		return nil, fmt.Errorf("add edge context_mapper->chat_template: %w", err)
	}
	if err := graph.AddEdge(nodeChatTemplate, nodeChatModel); err != nil {
		return nil, fmt.Errorf("add edge chat_template->chat_model: %w", err)
	}
	if err := graph.AddEdge(nodeChatModel, nodeOutputMapper); err != nil {
		return nil, fmt.Errorf("add edge chat_model->output_mapper: %w", err)
	}
	if err := graph.AddEdge(nodeOutputMapper, compose.END); err != nil {
		return nil, fmt.Errorf("add edge output_mapper->END: %w", err)
	}
	if err := graph.AddEdge(nodeEmptyAnswer, compose.END); err != nil {
		return nil, fmt.Errorf("add edge empty_answer->END: %w", err)
	}

	runnable, err := graph.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compile answer graph: %w", err)
	}
	return &AnswerEngine{runnable: runnable}, nil
}

// Answer 实现 AnswerChain 接口，内部调用已编译的 Eino Graph。
func (e *AnswerEngine) Answer(ctx context.Context, in AnswerInput) (*AnswerOutput, error) {
	if e == nil || e.runnable == nil {
		return nil, errors.New("answer graph not compiled")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err := e.runnable.Invoke(ctx, in)
	if err != nil {
		// 将 Eino 节点内部已包装的业务错误透传，其余错误统一为 LLM 调用失败。
		if !errors.Is(err, ErrEmbeddingUnavailable) && !errors.Is(err, ErrRetrievalUnavailable) && !errors.Is(err, ErrLLMNotConfigured) {
			return nil, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
		}
		return nil, err
	}
	return &out, nil
}

// buildCitations 严格依据检索文档生成引用，模型不能生成或修改结构化引用元数据。
func buildCitations(docs []*schema.Document) []Citation {
	citations := make([]Citation, 0, len(docs))
	for i, d := range docs {
		citations = append(citations, Citation{
			Index:        i + 1,
			ChunkID:      metaDataString(d, MetaKeyChunkID),
			FileID:       metaDataString(d, MetaKeyFileID),
			Filename:     metaDataString(d, MetaKeyFilename),
			Content:      d.Content,
			Score:        metaDataFloat64(d, MetaKeyScore, 0),
			VectorScore:  metaDataFloat64Pointer(d, MetaKeyVectorScore),
			KeywordScore: metaDataFloat64Pointer(d, MetaKeyKeywordScore),
			RRFScore:     metaDataFloat64Pointer(d, MetaKeyRRFScore),
			RerankScore:  metaDataFloat64Pointer(d, MetaKeyRerankScore),
			MatchedBy:    metadataBranches(d.MetaData[MetaKeyMatchedBy]),
		})
	}
	return citations
}

// 确保 AnswerEngine 实现 AnswerChain 接口。
var _ AnswerChain = (*AnswerEngine)(nil)
