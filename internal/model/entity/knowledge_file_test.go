package entity

import "testing"

func TestCanTransitionKnowledgeFile(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{FileUploaded, FileParseQueued, true},
		{FileParseQueued, FileParsing, true},
		{FileParseQueued, FileParseFailed, true},
		{FileParsing, FileParsed, true},
		{FileParsing, FileParseFailed, true},
		{FileParseFailed, FileParseQueued, true},
		{FileParsed, FileIndexQueued, true},
		{FileIndexQueued, FileIndexing, true},
		{FileIndexQueued, FileIndexFailed, true},
		{FileIndexing, FileIndexed, true},
		{FileIndexing, FileIndexFailed, true},
		{FileIndexFailed, FileIndexQueued, true},
		{FileIndexed, FileIndexQueued, true},
		{FileUploaded, FileIndexed, false},
	}
	for _, tt := range tests {
		if got := CanTransitionKnowledgeFile(tt.from, tt.to); got != tt.want {
			t.Fatalf("%s -> %s: got %v want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
