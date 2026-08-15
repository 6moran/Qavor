package rag

import (
	"math"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func rrfDocument(id, content, branch string, score float64) *schema.Document {
	return &schema.Document{ID: id, Content: content, MetaData: map[string]any{
		MetaKeyChunkID: id, MetaKeyScore: score, MetaKeyMatchedBy: []string{branch},
	}}
}

func TestFuseRRF_DualHitOutranksSingleHitsAndPreservesScores(t *testing.T) {
	vector := []*schema.Document{
		rrfDocument("vector-only", "仅向量", "vector", 0.91),
		rrfDocument("dual", "双路", "vector", 0.72),
	}
	keyword := []*schema.Document{
		rrfDocument("keyword-only", "仅关键词", "keyword", 0.88),
		rrfDocument("dual", "双路", "keyword", 0.65),
	}

	fused := FuseRRF([][]*schema.Document{vector, keyword}, 60, 20)
	if len(fused) != 3 || fused[0].ID != "dual" {
		t.Fatalf("融合顺序=%v", rankedDocumentIDs(fused))
	}
	dual := fused[0]
	wantRRF := 2.0 / 62.0
	if math.Abs(metaDataFloat64(dual, MetaKeyRRFScore, 0)-wantRRF) > 1e-12 {
		t.Fatalf("RRF=%v，期望 %v", dual.MetaData[MetaKeyRRFScore], wantRRF)
	}
	if metaDataFloat64(dual, MetaKeyVectorScore, 0) != 0.72 || metaDataFloat64(dual, MetaKeyKeywordScore, 0) != 0.65 {
		t.Fatalf("原始分数=%v", dual.MetaData)
	}
	wantNormalized := wantRRF / (2.0 / 61.0)
	if math.Abs(metaDataFloat64(dual, MetaKeyScore, 0)-wantNormalized) > 1e-12 {
		t.Fatalf("最终分数=%v，期望 %v", dual.MetaData[MetaKeyScore], wantNormalized)
	}
	if !reflect.DeepEqual(dual.MetaData[MetaKeyMatchedBy], []string{"vector", "keyword"}) {
		t.Fatalf("matched_by=%v", dual.MetaData[MetaKeyMatchedBy])
	}
}

func TestFuseRRF_UsesChunkIDAndCountsDuplicateOncePerList(t *testing.T) {
	list := []*schema.Document{
		rrfDocument("chunk-b", "相同正文", "vector", 0.9),
		rrfDocument("chunk-a", "相同正文", "vector", 0.8),
		rrfDocument("chunk-a", "重复条目", "vector", 0.1),
	}
	fused := FuseRRF([][]*schema.Document{list}, 60, 20)
	if len(fused) != 2 || fused[0].ID != "chunk-b" || fused[1].ID != "chunk-a" {
		t.Fatalf("融合结果=%v", rankedDocumentIDs(fused))
	}
	wantChunkA := 1.0 / 62.0
	if math.Abs(metaDataFloat64(fused[1], MetaKeyRRFScore, 0)-wantChunkA) > 1e-12 {
		t.Fatalf("重复 chunk 的 RRF=%v，期望 %v", fused[1].MetaData[MetaKeyRRFScore], wantChunkA)
	}
}

func TestFuseRRF_StableTieBreakersAndLimit(t *testing.T) {
	listOne := []*schema.Document{
		rrfDocument("single", "single", "vector", 0.8),
		rrfDocument("filler-a", "a", "vector", 0.7),
		rrfDocument("dual", "dual", "vector", 0.6),
		rrfDocument("z-id", "z", "vector", 0.5),
	}
	listTwo := []*schema.Document{
		rrfDocument("filler-b", "b", "keyword", 0.8),
		rrfDocument("filler-c", "c", "keyword", 0.7),
		rrfDocument("dual", "dual", "keyword", 0.6),
		rrfDocument("a-id", "a", "keyword", 0.5),
	}
	fused := FuseRRF([][]*schema.Document{listOne, listTwo}, 1, 3)
	if len(fused) != 3 {
		t.Fatalf("结果数量=%d，期望 3", len(fused))
	}
	if fused[0].ID != "dual" {
		t.Fatalf("相同 RRF 时双路命中未优先: %v", rankedDocumentIDs(fused))
	}

	idTie := FuseRRF([][]*schema.Document{
		{rrfDocument("z-id", "z", "vector", 0.5)},
		{rrfDocument("a-id", "a", "keyword", 0.5)},
	}, 60, 20)
	if len(idTie) != 2 || idTie[0].ID != "a-id" {
		t.Fatalf("chunk ID 平局顺序=%v", rankedDocumentIDs(idTie))
	}
}

func TestFuseRRF_IgnoresInvalidDocuments(t *testing.T) {
	missingID := &schema.Document{Content: "无 ID", MetaData: map[string]any{MetaKeyScore: 1.0}}
	fused := FuseRRF([][]*schema.Document{{nil, missingID, rrfDocument("valid", "有效", "vector", 0.4)}, nil}, 60, 20)
	if len(fused) != 1 || fused[0].ID != "valid" {
		t.Fatalf("结果=%v", rankedDocumentIDs(fused))
	}
}
