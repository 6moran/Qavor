package knowledge_file

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

type indexOneService struct {
	service.KnowledgeFileService
	req *request.IndexKnowledgeFilesRequest
}

func (s *indexOneService) IndexFiles(_ context.Context, _ string, req *request.IndexKnowledgeFilesRequest) (*response.ProcessingJobBatchResponse, error) {
	s.req = req
	return &response.ProcessingJobBatchResponse{}, nil
}

func TestIndexOneBindsFileIDFromPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &indexOneService{}
	router := gin.New()
	router.POST("/api/v1/knowledge/databases/:kb_id/documents/:doc_id/index", NewController(svc).IndexOne)
	body := []byte(`{"params":{"chunk_preset_id":"general","chunk_parser_config":{"chunk_token_num":500,"overlapped_percent":10}}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/databases/kb-1/documents/file-1/index", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if svc.req == nil || len(svc.req.FileIDs) != 1 || svc.req.FileIDs[0] != "file-1" {
		t.Fatalf("request=%+v", svc.req)
	}
}
