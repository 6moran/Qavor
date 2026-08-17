package rag

import (
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestBuildCitationsPreservesHybridOrderAndFinalScores(t *testing.T) {
	documents := []*schema.Document{
		{ID: "second", Content: "模型排第一", MetaData: map[string]any{
			MetaKeyChunkID: "second", MetaKeyFileID: "file-2", MetaKeyFilename: "second.md",
			MetaKeyScore: 0.95, MetaKeyVectorScore: 0.7, MetaKeyKeywordScore: 0.6,
			MetaKeyRRFScore: 0.032, MetaKeyRerankScore: 0.95, MetaKeyMatchedBy: []string{"vector", "keyword"},
		}},
		{ID: "first", Content: "模型排第二", MetaData: map[string]any{
			MetaKeyChunkID: "first", MetaKeyFileID: "file-1", MetaKeyFilename: "first.md",
			MetaKeyScore: 0.82, MetaKeyVectorScore: 0.8, MetaKeyRRFScore: 0.016, MetaKeyMatchedBy: []string{"vector"},
		}},
	}
	citations := buildCitations(documents)
	if len(citations) != 2 || citations[0].ChunkID != "second" || citations[1].ChunkID != "first" {
		t.Fatalf("引用顺序=%+v", citations)
	}
	first := citations[0]
	if first.Index != 1 || first.Score != 0.95 || first.RerankScore == nil || *first.RerankScore != 0.95 || first.RRFScore == nil || *first.RRFScore != 0.032 {
		t.Fatalf("第一条引用=%+v", first)
	}
	if !reflect.DeepEqual(first.MatchedBy, []string{"vector", "keyword"}) {
		t.Fatalf("matched_by=%v", first.MatchedBy)
	}
	if citations[1].RerankScore != nil || citations[1].Score != 0.82 {
		t.Fatalf("第二条引用=%+v", citations[1])
	}
}
