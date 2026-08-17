package service

import (
	"reflect"
	"testing"
)

func TestParseMindmapJSONAcceptsMarkdownFence(t *testing.T) {
	content := "```json\n{\"content\":\"产品\",\"children\":[{\"content\":\"检索\"}]}\n```"

	got, err := parseMindmapJSON(content)
	if err != nil {
		t.Fatalf("parseMindmapJSON() error = %v", err)
	}
	if got.Content != "产品" || len(got.Children) != 1 || got.Children[0].Content != "检索" {
		t.Fatalf("parsed mindmap = %+v", got)
	}
}

func TestDiffMindmapFileIDs(t *testing.T) {
	got := diffMindmapFileIDs([]string{"file-1", "file-2"}, []string{"file-2", "file-3"})
	want := mindmapFileDiff{
		AddedFiles:   []string{"file-3"},
		RemovedFiles: []string{"file-1"},
		NeedsUpdate:  true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffMindmapFileIDs() = %+v, want %+v", got, want)
	}
}
