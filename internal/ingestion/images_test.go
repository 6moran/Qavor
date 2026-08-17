package ingestion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingImageUploader struct{}

func (recordingImageUploader) UploadImage(_, filename string, _ []byte) (string, error) {
	return "https://objects.test/" + filename, nil
}

func TestReplaceImageLinksPreservesOCRAlt(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "chart.png")
	if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	markdownPath := filepath.ToSlash(path)
	input := "![一季度 120 万 二季度 165 万](" + markdownPath + ")"

	got := ReplaceImageLinks(input, []string{markdownPath}, "derived", recordingImageUploader{})
	want := "![一季度 120 万 二季度 165 万](https://objects.test/chart.png)"
	if strings.TrimSpace(got) != want {
		t.Fatalf("ReplaceImageLinks() = %q, want %q", got, want)
	}
}
