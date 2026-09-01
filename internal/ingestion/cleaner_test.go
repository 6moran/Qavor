package ingestion

import (
	"strings"
	"testing"
)

func TestDocumentCleanerGolden(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normalizes control characters trailing spaces and blank lines",
			input: "\u0000# 标题   \r\n\r\n\r\n正文\t \r\n",
			want:  "# 标题\n\n正文",
		},
		{
			name:  "removes only empty markdown artifacts",
			input: "#\n##   \n![]()\n![]\n![ ](   )\n<!-- image -->\n\n正文\n",
			want:  "正文",
		},
		{
			name:  "preserves code fences",
			input: "```go\n# not a heading\nfunc main() {}\n```",
			want:  "```go\n# not a heading\nfunc main() {}\n```",
		},
		{
			name:  "preserves fenced code whitespace while cleaning prose",
			input: "正文   \n\n\n```go\nfunc main() {  \n\n\n\treturn\n}\t\n```\n\n\n结尾   ",
			want:  "正文\n\n```go\nfunc main() {  \n\n\n\treturn\n}\t\n```\n\n结尾",
		},
		{
			name:  "preserves indented code whitespace while cleaning prose",
			input: "正文   \n\n\n    # code heading   \n    ![]()  \n\n\n    return value\t\n\n结尾   ",
			want:  "正文\n\n    # code heading   \n    ![]()  \n\n\n    return value\t\n\n结尾",
		},
		{
			name:  "preserves trailing blank line in indented code",
			input: "    value\n\n",
			want:  "    value\n\n",
		},
		{
			name:  "preserves footnotes",
			input: "正文[^1]\n\n[^1]: 保留脚注",
			want:  "正文[^1]\n\n[^1]: 保留脚注",
		},
		{
			name:  "preserves disclaimers",
			input: "> 保密声明：仅限内部使用",
			want:  "> 保密声明：仅限内部使用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (DocumentCleaner{}).Clean(tt.input)
			if got.Degraded {
				t.Fatal("cleaner unexpectedly degraded")
			}
			if got.Markdown != tt.want {
				t.Fatalf("Clean() = %q, want %q", got.Markdown, tt.want)
			}
		})
	}
}

func TestDocumentCleanerPreservesMeaningfulMarkdown(t *testing.T) {
	input := "公司内部资料 版本 2\r\n<!-- page:1 -->\r\n# 标题\r\n\r\n|列A|列B|\r\n|-|-|\r\n|1|2|\r\n\r\n![图表文字](https://objects.test/a.png)\r\n第 1 页 / 共 2 页"
	got := (DocumentCleaner{}).Clean(input)
	for _, want := range []string{"公司内部资料 版本 2", "<!-- page:1 -->", "|1|2|", "https://objects.test/a.png", "第 1 页 / 共 2 页"} {
		if !strings.Contains(got.Markdown, want) {
			t.Fatalf("missing preserved content %q in %q", want, got.Markdown)
		}
	}

}
