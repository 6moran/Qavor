package ingestion

import (
	"strings"
	"unicode"
)

// CleanResult is the stable Markdown emitted by DocumentCleaner. Degraded is
// reserved for a future rule failure; cleaning never discards the input when a
// single rule cannot be applied.
type CleanResult struct {
	Markdown string
	Degraded bool
}

type cleanRule func(string) (string, error)

// DocumentCleaner applies only deterministic, formatting-level cleanup. It
// deliberately does not identify headers, footers, page numbers, or duplicates.
type DocumentCleaner struct{}

func (DocumentCleaner) Clean(markdown string) CleanResult {
	result := CleanResult{Markdown: markdown}
	for _, rule := range []cleanRule{
		normalizeNewlines,
		removeUnsafeControls,
		trimLineEnds,
		removeEmptyArtifacts,
		collapseBlankLines,
	} {
		next, err := rule(result.Markdown)
		if err != nil {
			result.Degraded = true
			continue
		}
		result.Markdown = next
	}
	return result
}

func normalizeNewlines(markdown string) (string, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	return strings.ReplaceAll(markdown, "\r", "\n"), nil
}

func removeUnsafeControls(markdown string) (string, error) {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, markdown), nil
}

func trimLineEnds(markdown string) (string, error) {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n"), nil
}

func removeEmptyArtifacts(markdown string) (string, error) {
	lines := strings.Split(markdown, "\n")
	kept := make([]string, 0, len(lines))
	inCodeFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isFence(trimmed) {
			inCodeFence = !inCodeFence
			kept = append(kept, line)
			continue
		}
		if !inCodeFence && (isEmptyHeading(trimmed) || isEmptyImagePlaceholder(trimmed) || trimmed == "<!-- image -->") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), nil
}

func isFence(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func isEmptyHeading(line string) bool {
	if line == "" {
		return false
	}
	for _, r := range line {
		if r != '#' && !unicode.IsSpace(r) {
			return false
		}
	}
	return strings.Contains(line, "#")
}

func isEmptyImagePlaceholder(line string) bool {
	if line == "![]" || line == "![]()" {
		return true
	}
	if !strings.HasPrefix(line, "![") || !strings.HasSuffix(line, ")") {
		return false
	}
	closeAlt := strings.Index(line, "](")
	if closeAlt < 0 {
		return false
	}
	return strings.TrimSpace(line[2:closeAlt]) == "" && strings.TrimSpace(line[closeAlt+2:len(line)-1]) == ""
}

func collapseBlankLines(markdown string) (string, error) {
	lines := strings.Split(markdown, "\n")
	output := make([]string, 0, len(lines))
	previousBlank := true
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank {
			if previousBlank {
				continue
			}
			output = append(output, "")
			previousBlank = true
			continue
		}
		output = append(output, line)
		previousBlank = false
	}
	if len(output) > 0 && output[len(output)-1] == "" {
		output = output[:len(output)-1]
	}
	return strings.Join(output, "\n"), nil
}
