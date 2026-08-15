package repository

import (
	"testing"

	"Qavor/internal/model/entity"
)

func TestSearchableKnowledgeFileStatusIsIndexed(t *testing.T) {
	if searchableKnowledgeFileStatus != entity.FileIndexed {
		t.Fatalf("searchable status=%q want %q", searchableKnowledgeFileStatus, entity.FileIndexed)
	}
}
