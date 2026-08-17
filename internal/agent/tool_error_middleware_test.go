package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestToolErrorRecoveryMiddlewareReturnsRecoverableResult(t *testing.T) {
	mw := newToolErrorRecoveryMiddleware()
	endpoint := mw.Invokable(func(context.Context, *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, errors.New("stat: file does not exist")
	})
	out, err := endpoint(context.Background(), &compose.ToolInput{Name: "read_file", Arguments: `{"file_path":"kb.md"}`})
	if err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	if out == nil || !strings.Contains(out.Result, "WORKSPACE_FILE_NOT_FOUND") || !strings.Contains(out.Result, "query_kb") {
		t.Fatalf("unexpected recoverable result: %#v", out)
	}
}
