package ingestion

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseResultJSONRoundTrip(t *testing.T) {
	in := ParseResult{
		Markdown: "# hi",
		Pages:    []ParsedPage{{Number: 1, Text: "hello"}},
		Metadata: map[string]any{"file_type": ".docx"},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// 锁定与 Python 侧 dataclass 的 JSON 契约 key
	if !strings.Contains(string(data), `"pages":`) {
		t.Fatalf("pages key missing from JSON: %s", data)
	}
	var out ParseResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip mismatch:\n got: %+v\nwant: %+v", out, in)
	}
}
