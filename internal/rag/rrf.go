package rag

import (
	"sort"

	"github.com/cloudwego/eino/schema"
)

type rrfCandidate struct {
	document     *schema.Document
	score        float64
	hitCount     int
	bestRank     int
	vectorScore  *float64
	keywordScore *float64
	branches     map[string]struct{}
}

// FuseRRF 按 chunk_id 融合多个有序列表，并生成稳定的阶段分数元数据。
func FuseRRF(lists [][]*schema.Document, rrfK, limit int) []*schema.Document {
	if rrfK <= 0 {
		rrfK = 60
	}
	candidates := make(map[string]*rrfCandidate)
	activeListCount := 0
	for _, list := range lists {
		seen := make(map[string]struct{})
		listActive := false
		for index, document := range list {
			chunkID := metaDataString(document, MetaKeyChunkID)
			if document == nil || chunkID == "" {
				continue
			}
			if _, duplicate := seen[chunkID]; duplicate {
				continue
			}
			seen[chunkID] = struct{}{}
			listActive = true
			candidate := candidates[chunkID]
			if candidate == nil {
				candidate = &rrfCandidate{
					document: cloneDocument(document),
					bestRank: index + 1,
					branches: make(map[string]struct{}),
				}
				candidates[chunkID] = candidate
			}
			rank := index + 1
			candidate.score += 1.0 / float64(rrfK+rank)
			candidate.hitCount++
			if rank < candidate.bestRank {
				candidate.bestRank = rank
			}
			collectCandidateScores(candidate, document)
		}
		if listActive {
			activeListCount++
		}
	}
	if activeListCount == 0 {
		return nil
	}

	ordered := make([]*rrfCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		metadata := candidate.document.MetaData
		metadata[MetaKeyRRFScore] = candidate.score
		metadata[MetaKeyScore] = clampScore(candidate.score / (float64(activeListCount) / float64(rrfK+1)))
		if candidate.vectorScore != nil {
			metadata[MetaKeyVectorScore] = *candidate.vectorScore
		}
		if candidate.keywordScore != nil {
			metadata[MetaKeyKeywordScore] = *candidate.keywordScore
		}
		metadata[MetaKeyMatchedBy] = orderedBranches(candidate.branches)
		ordered = append(ordered, candidate)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].score != ordered[j].score {
			return ordered[i].score > ordered[j].score
		}
		if ordered[i].hitCount != ordered[j].hitCount {
			return ordered[i].hitCount > ordered[j].hitCount
		}
		if ordered[i].bestRank != ordered[j].bestRank {
			return ordered[i].bestRank < ordered[j].bestRank
		}
		return metaDataString(ordered[i].document, MetaKeyChunkID) < metaDataString(ordered[j].document, MetaKeyChunkID)
	})
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	result := make([]*schema.Document, len(ordered))
	for index, candidate := range ordered {
		result[index] = candidate.document
	}
	return result
}

func collectCandidateScores(candidate *rrfCandidate, document *schema.Document) {
	branches := metadataBranches(document.MetaData[MetaKeyMatchedBy])
	for _, branch := range branches {
		candidate.branches[branch] = struct{}{}
		score, found := metadataNumber(document.MetaData[MetaKeyScore])
		if !found {
			continue
		}
		switch branch {
		case "vector":
			candidate.vectorScore = largerScore(candidate.vectorScore, score)
		case "keyword":
			candidate.keywordScore = largerScore(candidate.keywordScore, score)
		}
	}
}

func cloneDocument(document *schema.Document) *schema.Document {
	metadata := make(map[string]any, len(document.MetaData)+5)
	for key, value := range document.MetaData {
		metadata[key] = value
	}
	return &schema.Document{ID: document.ID, Content: document.Content, MetaData: metadata}
}

func metadataBranches(value any) []string {
	switch branches := value.(type) {
	case string:
		return []string{branches}
	case []string:
		return branches
	case []any:
		result := make([]string, 0, len(branches))
		for _, branch := range branches {
			if text, ok := branch.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func metadataNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
}

func largerScore(current *float64, score float64) *float64 {
	if current == nil || score > *current {
		value := score
		return &value
	}
	return current
}

func orderedBranches(branches map[string]struct{}) []string {
	result := make([]string, 0, len(branches))
	for _, branch := range []string{"vector", "keyword"} {
		if _, found := branches[branch]; found {
			result = append(result, branch)
			delete(branches, branch)
		}
	}
	rest := make([]string, 0, len(branches))
	for branch := range branches {
		rest = append(rest, branch)
	}
	sort.Strings(rest)
	return append(result, rest...)
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
