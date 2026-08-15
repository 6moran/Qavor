package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/pkg/config"
	bizerrors "Qavor/pkg/errors"

	einoEmbedding "github.com/cloudwego/eino/components/embedding"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ---------- mock: 知识库仓库 ----------

type querySvcKBRepo struct {
	bases   map[string]*entity.KnowledgeBase
	updated []*entity.KnowledgeBase
}

func (r *querySvcKBRepo) Create(*entity.KnowledgeBase) error { return nil }
func (r *querySvcKBRepo) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	return r.bases[kbID], nil
}
func (r *querySvcKBRepo) FindByKBIDs(kbIDs []string) ([]*entity.KnowledgeBase, error) {
	bases := make([]*entity.KnowledgeBase, 0, len(kbIDs))
	for _, kbID := range kbIDs {
		if base := r.bases[kbID]; base != nil {
			bases = append(bases, base)
		}
	}
	return bases, nil
}
func (r *querySvcKBRepo) List(int, int, string) ([]*entity.KnowledgeBase, int64, error) {
	return nil, 0, nil
}
func (r *querySvcKBRepo) Update(base *entity.KnowledgeBase) error {
	r.updated = append(r.updated, base)
	return nil
}
func (r *querySvcKBRepo) DeleteByKBID(string) error { return nil }
func (r *querySvcKBRepo) GetStatsByKBIDs([]string) (map[string]*repository.KnowledgeBaseStats, error) {
	return nil, nil
}

// ---------- mock: 文件仓库 ----------

type querySvcFileRepo struct {
	files []*entity.KnowledgeFile
}

func (r *querySvcFileRepo) Create(*entity.KnowledgeFile) error { return nil }
func (r *querySvcFileRepo) FindByKBIDAndFileID(string, string) (*entity.KnowledgeFile, error) {
	return nil, nil
}
func (r *querySvcFileRepo) ListByKBID(string, int, int, string, string, bool, string) ([]*entity.KnowledgeFile, int64, error) {
	return nil, 0, nil
}
func (r *querySvcFileRepo) ListAllByKBID(context.Context, string) ([]*entity.KnowledgeFile, error) {
	return r.files, nil
}
func (r *querySvcFileRepo) SearchByKBID(string, string, int, int) ([]*entity.KnowledgeFile, int64, error) {
	return nil, 0, nil
}
func (r *querySvcFileRepo) DeleteByKBIDAndFileID(string, string) error { return nil }
func (r *querySvcFileRepo) DeleteWithChunks(context.Context, string, string) error {
	return nil
}
func (r *querySvcFileRepo) UpdateProcessingResult(string, string, string, string, string) error {
	return nil
}
func (r *querySvcFileRepo) TransitionStatus(context.Context, string, string, []string, string, map[string]any) (bool, error) {
	return false, nil
}
func (r *querySvcFileRepo) ListByKBIDAndStatuses(context.Context, string, []string, int) ([]*entity.KnowledgeFile, error) {
	return nil, nil
}

// ---------- mock: RAG 服务 ----------

type querySvcRAGService struct {
	kbIDs  []string
	query  string
	cfg    *RetrievalTestConfig
	result *RAGRetrieveResult
	err    error
}

func (m *querySvcRAGService) Retrieve(context.Context, []string, string, int) (*RAGRetrieveResult, error) {
	return nil, errors.New("Retrieve must not be called by query service")
}
func (m *querySvcRAGService) Answer(context.Context, []string, string) (*RAGAnswerResult, error) {
	return nil, errors.New("Answer must not be called by query service")
}
func (m *querySvcRAGService) RetrieveTest(_ context.Context, kbIDs []string, query string, cfg *RetrievalTestConfig) (*RAGRetrieveResult, error) {
	m.kbIDs = kbIDs
	m.query = query
	m.cfg = cfg
	return m.result, m.err
}

// ---------- mock: 模型服务 ----------

type querySvcModelService struct {
	chatModel einoModel.ToolCallingChatModel
	err       error
	// 记录 ResolveChatModelWithTimeout 收到的超时下限，便于断言。
	minTimeout time.Duration
}

func (m *querySvcModelService) CreateModel(*request.CreateModelRequest) (*dto.ModelResponse, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) GetModel(uint) (*dto.ModelResponse, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) UpdateModel(uint, *request.UpdateModelRequest) (*dto.ModelResponse, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) DeleteModel(uint) error { return errors.New("not used") }
func (m *querySvcModelService) ListModels(*request.ModelListRequest) (*dto.ModelListResponse, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) GetModelWithDecryptedKey(uint) (*entity.Model, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) CreateLLMClient(context.Context, uint) (llm.Client, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) ResolveEmbedding(context.Context, uint) (einoEmbedding.Embedder, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) ResolveChatModel(context.Context, uint) (einoModel.ToolCallingChatModel, error) {
	return m.chatModel, m.err
}
func (m *querySvcModelService) ResolveChatModelWithTimeout(_ context.Context, _ uint, minTimeout time.Duration) (einoModel.ToolCallingChatModel, error) {
	m.minTimeout = minTimeout
	return m.chatModel, m.err
}
func (m *querySvcModelService) ResolveReranker(context.Context, uint) (rag.Reranker, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) TestConnection(context.Context, *request.ModelConnectionTestRequest) (*dto.ModelConnectionTestResponse, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) FetchRemoteModels(context.Context, *request.FetchRemoteModelsRequest) ([]string, error) {
	return nil, errors.New("not used")
}
func (m *querySvcModelService) SetModelConfigChangeCallback(func(modelID string)) {}
func (m *querySvcModelService) GetModelInfo(uint) (string, string, int, bool) {
	return "", "", 0, false
}

// ---------- mock: Eino ChatModel ----------

type fakeChatModel struct {
	content string
	err     error
}

func (m *fakeChatModel) Generate(context.Context, []*schema.Message, ...einoModel.Option) (*schema.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{Role: schema.Assistant, Content: m.content}, nil
}
func (m *fakeChatModel) Stream(context.Context, []*schema.Message, ...einoModel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented")
}
func (m *fakeChatModel) WithTools([]*schema.ToolInfo) (einoModel.ToolCallingChatModel, error) {
	return m, nil
}

// ---------- 工具函数 ----------

func newQueryService(kbRepo *querySvcKBRepo, fileRepo *querySvcFileRepo, ragSvc RAGService, modelSvc ModelService) KnowledgeQueryService {
	if fileRepo == nil {
		fileRepo = &querySvcFileRepo{}
	}
	if ragSvc == nil {
		ragSvc = &querySvcRAGService{}
	}
	if modelSvc == nil {
		modelSvc = &querySvcModelService{}
	}
	return NewKnowledgeQueryService(config.RAGConfig{TopK: 5, RerankTopK: 3, ScoreThreshold: 0.3}, kbRepo, fileRepo, ragSvc, modelSvc)
}

// ---------- QueryTest ----------

func TestQueryTestParsesWhitelistMeta(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{"kb-1": {KBID: "kb-1"}}}
	ragSvc := &querySvcRAGService{result: &RAGRetrieveResult{QueryText: "退款"}}
	svc := newQueryService(kbRepo, nil, ragSvc, nil)

	meta := map[string]any{"top_k": 8, "vector_top_k": 30, "keyword_top_k": "12", "include_distances": true}
	if _, err := svc.QueryTest(context.Background(), "kb-1", "退款", meta); err != nil {
		t.Fatalf("QueryTest() error = %v", err)
	}
	if !reflect.DeepEqual(ragSvc.kbIDs, []string{"kb-1"}) || ragSvc.query != "退款" {
		t.Fatalf("kbIDs=%v query=%q", ragSvc.kbIDs, ragSvc.query)
	}
	cfg := ragSvc.cfg
	if cfg == nil || cfg.TopK == nil || *cfg.TopK != 8 || cfg.VectorTopK == nil || *cfg.VectorTopK != 30 ||
		cfg.KeywordTopK == nil || *cfg.KeywordTopK != 12 || cfg.FusedTopK != nil {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestQueryTestIgnoresInvalidMeta(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{"kb-1": {KBID: "kb-1"}}}
	ragSvc := &querySvcRAGService{result: &RAGRetrieveResult{}}
	svc := newQueryService(kbRepo, nil, ragSvc, nil)

	meta := map[string]any{"top_k": 100, "vector_top_k": "abc", "score_threshold": -1, "rrf_k": 0}
	if _, err := svc.QueryTest(context.Background(), "kb-1", "q", meta); err != nil {
		t.Fatalf("QueryTest() error = %v", err)
	}
	if ragSvc.cfg != nil {
		t.Fatalf("cfg = %+v, want nil (all values invalid)", ragSvc.cfg)
	}
}

func TestQueryTestNilMetaPassesNilConfig(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{"kb-1": {KBID: "kb-1"}}}
	ragSvc := &querySvcRAGService{result: &RAGRetrieveResult{}}
	svc := newQueryService(kbRepo, nil, ragSvc, nil)

	if _, err := svc.QueryTest(context.Background(), "kb-1", "q", nil); err != nil {
		t.Fatalf("QueryTest() error = %v", err)
	}
	if ragSvc.cfg != nil {
		t.Fatalf("cfg = %+v, want nil", ragSvc.cfg)
	}
}

// ---------- GetQueryParams ----------

func TestGetQueryParamsMergesSavedValuesAndDefaults(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", QueryParams: entity.JSON{"top_k": 12.0, "rrf_k": 90.0, "unknown": 1.0}},
	}}
	svc := newQueryService(kbRepo, nil, nil, nil)

	payload, err := svc.GetQueryParams("kb-1")
	if err != nil {
		t.Fatalf("GetQueryParams() error = %v", err)
	}
	if len(payload.Params.Options) != 7 {
		t.Fatalf("options = %d, want 7", len(payload.Params.Options))
	}
	byKey := map[string]QueryParamOption{}
	for _, option := range payload.Params.Options {
		byKey[option.Key] = option
	}
	if got := byKey[metaKeyTopK].Default; got != 12.0 {
		t.Fatalf("top_k default = %v, want 12", got)
	}
	if got := byKey[metaKeyRRFK].Default; got != 90.0 {
		t.Fatalf("rrf_k default = %v, want 90", got)
	}
	if got := byKey[metaKeyScoreThreshold].Default; got != 0.3 {
		t.Fatalf("score_threshold default = %v, want 0.3", got)
	}
	if byKey[metaKeyTopK].Type != "number" || byKey[metaKeyTopK].Max == nil || *byKey[metaKeyTopK].Max != 20 {
		t.Fatalf("top_k option = %+v", byKey[metaKeyTopK])
	}
}

func TestGetQueryParamsKBNotFound(t *testing.T) {
	svc := newQueryService(&querySvcKBRepo{}, nil, nil, nil)
	_, err := svc.GetQueryParams("ghost")
	assertBizCode(t, err, bizerrors.CodeResourceNotFound)
}

// ---------- UpdateQueryParams ----------

func TestUpdateQueryParamsFiltersWhitelist(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", QueryParams: entity.JSON{"top_k": 3.0}},
	}}
	svc := newQueryService(kbRepo, nil, nil, nil)

	values := map[string]any{"top_k": 8, "vector_top_k": 25, "include_distances": true, "foo": 1}
	if err := svc.UpdateQueryParams("kb-1", values); err != nil {
		t.Fatalf("UpdateQueryParams() error = %v", err)
	}
	if len(kbRepo.updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(kbRepo.updated))
	}
	saved := kbRepo.updated[0].QueryParams
	if len(saved) != 2 || saved["top_k"] != 8 || saved["vector_top_k"] != 25 {
		t.Fatalf("saved params = %v", saved)
	}
}

// ---------- GetSampleQuestions ----------

func TestGetSampleQuestionsEmptyReturnsNotFound(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{"kb-1": {KBID: "kb-1"}}}
	svc := newQueryService(kbRepo, nil, nil, nil)

	_, err := svc.GetSampleQuestions("kb-1")
	assertBizCode(t, err, bizerrors.CodeResourceNotFound)
	if !strings.Contains(err.Error(), "还没有生成") {
		t.Fatalf("error message = %v, want contains 还没有生成", err)
	}
}

func TestGetSampleQuestionsFiltersNonStrings(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", SampleQuestions: entity.JSONArray{"问题1", 123, "问题2", ""}},
	}}
	svc := newQueryService(kbRepo, nil, nil, nil)

	payload, err := svc.GetSampleQuestions("kb-1")
	if err != nil {
		t.Fatalf("GetSampleQuestions() error = %v", err)
	}
	if !reflect.DeepEqual(payload.Questions, []string{"问题1", "问题2"}) {
		t.Fatalf("questions = %v", payload.Questions)
	}
}

// ---------- GenerateSampleQuestions ----------

func TestGenerateSampleQuestionsSuccess(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", Name: "产品知识库", ChatModelID: 7},
	}}
	fileRepo := &querySvcFileRepo{files: []*entity.KnowledgeFile{
		{OriginalFilename: "退款政策.md"},
		{Filename: "常见问题.md", IsFolder: false},
		{OriginalFilename: "内部目录", IsFolder: true},
	}}
	modelSvc := &querySvcModelService{chatModel: &fakeChatModel{content: `["如何退款？","退款要多久？","如何退款？",""]`}}
	svc := newQueryService(kbRepo, fileRepo, nil, modelSvc)

	payload, err := svc.GenerateSampleQuestions(context.Background(), "kb-1", 10)
	if err != nil {
		t.Fatalf("GenerateSampleQuestions() error = %v", err)
	}
	if !reflect.DeepEqual(payload.Questions, []string{"如何退款？", "退款要多久？"}) {
		t.Fatalf("questions = %v", payload.Questions)
	}
	if len(kbRepo.updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(kbRepo.updated))
	}
	saved := kbRepo.updated[0].SampleQuestions
	if len(saved) != 2 || saved[0] != "如何退款？" {
		t.Fatalf("saved questions = %v", saved)
	}
	// 生成走独立宽松超时路径。
	if modelSvc.minTimeout != sampleQuestionTimeout {
		t.Fatalf("minTimeout = %v, want %v", modelSvc.minTimeout, sampleQuestionTimeout)
	}
}

func TestGenerateSampleQuestionsNoChatModel(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{"kb-1": {KBID: "kb-1"}}}
	svc := newQueryService(kbRepo, nil, nil, nil)

	_, err := svc.GenerateSampleQuestions(context.Background(), "kb-1", 10)
	assertBizCode(t, err, bizerrors.CodeInvalidParam)
}

func TestGenerateSampleQuestionsInvalidLLMOutput(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", ChatModelID: 7},
	}}
	modelSvc := &querySvcModelService{chatModel: &fakeChatModel{content: "这不是JSON"}}
	svc := newQueryService(kbRepo, nil, nil, modelSvc)

	_, err := svc.GenerateSampleQuestions(context.Background(), "kb-1", 10)
	assertBizCode(t, err, bizerrors.CodeLLMResponseInvalid)
}

func TestGenerateSampleQuestionsParsesCodeBlock(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", ChatModelID: 7},
	}}
	modelSvc := &querySvcModelService{chatModel: &fakeChatModel{content: "```json\n[\"问题A\",\"问题B\"]\n```"}}
	svc := newQueryService(kbRepo, nil, nil, modelSvc)

	payload, err := svc.GenerateSampleQuestions(context.Background(), "kb-1", 10)
	if err != nil {
		t.Fatalf("GenerateSampleQuestions() error = %v", err)
	}
	if !reflect.DeepEqual(payload.Questions, []string{"问题A", "问题B"}) {
		t.Fatalf("questions = %v", payload.Questions)
	}
}

func TestGenerateSampleQuestionsCountLimit(t *testing.T) {
	kbRepo := &querySvcKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", ChatModelID: 7},
	}}
	modelSvc := &querySvcModelService{chatModel: &fakeChatModel{content: `["q1","q2","q3","q4","q5"]`}}
	svc := newQueryService(kbRepo, nil, nil, modelSvc)

	payload, err := svc.GenerateSampleQuestions(context.Background(), "kb-1", 2)
	if err != nil {
		t.Fatalf("GenerateSampleQuestions() error = %v", err)
	}
	if !reflect.DeepEqual(payload.Questions, []string{"q1", "q2"}) {
		t.Fatalf("questions = %v", payload.Questions)
	}
}

// assertBizCode 断言错误为指定业务错误码。
func assertBizCode(t *testing.T, err error, code int) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %d", code)
	}
	var biz *bizerrors.BizError
	if !errors.As(err, &biz) || biz.Code != code {
		t.Fatalf("error = %v, want code %d", err, code)
	}
}
