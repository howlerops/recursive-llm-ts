package rlm

import (
	"math"
	"sort"
	"strings"
)

// ─── TextRank Graph-Based Sentence Ranking ──────────────────────────────────
//
// Pure Go, zero external dependencies, zero API calls.
// Implements the TextRank algorithm (Mihalcea & Tarau, 2004):
// 1. Build TF-IDF vectors for each sentence
// 2. Compute cosine similarity between all sentence pairs
// 3. Run PageRank iteration on the similarity graph
// 4. Select top-ranked sentences that fit within token budget
// 5. Preserve original document order

// TextRankConfig controls the TextRank algorithm parameters.
type TextRankConfig struct {
	// DampingFactor is the PageRank damping factor (default: 0.85)
	DampingFactor float64
	// MaxIterations for PageRank convergence (default: 100)
	MaxIterations int
	// ConvergenceThreshold for PageRank (default: 0.0001)
	ConvergenceThreshold float64
	// MinSimilarity threshold to create an edge (default: 0.1)
	MinSimilarity float64
}

// DefaultTextRankConfig returns sensible defaults for TextRank.
func DefaultTextRankConfig() TextRankConfig {
	return TextRankConfig{
		DampingFactor:        0.85,
		MaxIterations:        100,
		ConvergenceThreshold: 0.0001,
		MinSimilarity:        0.1,
	}
}

// tfidfVector represents a sparse TF-IDF vector for a sentence.
type tfidfVector struct {
	terms map[string]float64
	norm  float64 // precomputed L2 norm
}

// buildTFIDFVectors computes TF-IDF vectors for each sentence.
func buildTFIDFVectors(sentences []string) []tfidfVector {
	n := len(sentences)
	if n == 0 {
		return nil
	}

	// Tokenize and filter
	docWords := make([][]string, n)
	for i, s := range sentences {
		docWords[i] = FilterStopWords(TokenizeWords(s))
	}

	// Compute document frequency
	df := make(map[string]int)
	for _, words := range docWords {
		seen := make(map[string]bool)
		for _, w := range words {
			if !seen[w] {
				df[w]++
				seen[w] = true
			}
		}
	}

	nf := float64(n)

	// Build vectors
	vectors := make([]tfidfVector, n)
	for i, words := range docWords {
		tf := make(map[string]int)
		for _, w := range words {
			tf[w]++
		}

		terms := make(map[string]float64)
		normSq := 0.0
		for word, freq := range tf {
			idf := math.Log(nf / float64(df[word]))
			val := float64(freq) * idf
			terms[word] = val
			normSq += val * val
		}

		vectors[i] = tfidfVector{
			terms: terms,
			norm:  math.Sqrt(normSq),
		}
	}

	return vectors
}

// cosineSimilarity computes the cosine similarity between two TF-IDF vectors.
func cosineSimilarity(a, b tfidfVector) float64 {
	if a.norm == 0 || b.norm == 0 {
		return 0
	}

	// Compute dot product using the smaller vector for efficiency
	dot := 0.0
	small, large := a.terms, b.terms
	if len(a.terms) > len(b.terms) {
		small, large = b.terms, a.terms
	}
	for term, val := range small {
		if otherVal, ok := large[term]; ok {
			dot += val * otherVal
		}
	}

	return dot / (a.norm * b.norm)
}

// BuildSimilarityGraph creates a weighted adjacency matrix of sentence similarities.
// Only edges above the MinSimilarity threshold are kept.
//
// Dot products are accumulated through an inverted index (term -> sentences
// containing it), so only sentence pairs that share a weighted term are
// touched. Terms present in every sentence have zero IDF weight and are skipped.
func BuildSimilarityGraph(sentences []string, config TextRankConfig) [][]float64 {
	n := len(sentences)
	graph := newSquareMatrix(n)
	if n == 0 {
		return graph
	}
	vectors := buildTFIDFVectors(sentences)

	type posting struct {
		doc    int
		weight float64
	}
	index := make(map[string][]posting)
	for i, v := range vectors {
		for term, weight := range v.terms {
			if weight != 0 {
				index[term] = append(index[term], posting{doc: i, weight: weight})
			}
		}
	}

	// Postings are in ascending doc order, so this fills the upper triangle.
	for _, postings := range index {
		for a, pa := range postings {
			row := graph[pa.doc]
			for _, pb := range postings[a+1:] {
				row[pb.doc] += pa.weight * pb.weight
			}
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := 0.0
			if dot := graph[i][j]; dot != 0 && vectors[i].norm != 0 && vectors[j].norm != 0 {
				sim = dot / (vectors[i].norm * vectors[j].norm)
			}
			if sim < config.MinSimilarity {
				sim = 0
			}
			graph[i][j] = sim
			graph[j][i] = sim
		}
	}

	return graph
}

// newSquareMatrix allocates an n x n matrix backed by one contiguous slice.
func newSquareMatrix(n int) [][]float64 {
	backing := make([]float64, n*n)
	m := make([][]float64, n)
	for i := range m {
		m[i] = backing[i*n : (i+1)*n : (i+1)*n]
	}
	return m
}

// PageRank runs the PageRank algorithm on a weighted graph.
// Returns a score for each node (sentence).
func PageRank(graph [][]float64, config TextRankConfig) []float64 {
	n := len(graph)
	if n == 0 {
		return nil
	}

	d := config.DampingFactor
	scores := make([]float64, n)
	newScores := make([]float64, n)

	// Initialize with uniform scores
	initial := 1.0 / float64(n)
	for i := range scores {
		scores[i] = initial
	}

	// Precompute outgoing weight sums for each node
	outWeights := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			outWeights[i] += graph[i][j]
		}
	}

	// Sparse transition matrix: for each node i, the incoming edges j with
	// weight graph[j][i] / outWeights[j]. Most pairs fall below MinSimilarity,
	// so iterating only non-zero edges is much cheaper than a dense scan.
	type edge struct {
		from   int
		weight float64
	}
	incoming := make([][]edge, n)
	for j := 0; j < n; j++ {
		if outWeights[j] <= 0 {
			continue
		}
		for i := 0; i < n; i++ {
			if w := graph[j][i]; w > 0 {
				incoming[i] = append(incoming[i], edge{from: j, weight: w / outWeights[j]})
			}
		}
	}

	// Iterate until convergence
	for iter := 0; iter < config.MaxIterations; iter++ {
		maxDelta := 0.0

		for i := 0; i < n; i++ {
			sum := 0.0
			for _, e := range incoming[i] {
				sum += e.weight * scores[e.from]
			}
			newScores[i] = (1-d)/float64(n) + d*sum

			delta := math.Abs(newScores[i] - scores[i])
			if delta > maxDelta {
				maxDelta = delta
			}
		}

		// Swap slices
		scores, newScores = newScores, scores

		// Check convergence
		if maxDelta < config.ConvergenceThreshold {
			break
		}
	}

	return scores
}

// CompressContextTextRank reduces context to fit within a token budget using
// TextRank graph-based sentence ranking.
// Preserves original sentence order in the output.
func CompressContextTextRank(text string, targetTokens int) string {
	return CompressContextTextRankWithConfig(text, targetTokens, DefaultTextRankConfig())
}

// CompressContextTextRankWithConfig is like CompressContextTextRank but with custom TextRank parameters.
func CompressContextTextRankWithConfig(text string, targetTokens int, config TextRankConfig) string {
	if EstimateTokens(text) <= targetTokens {
		return text
	}

	sentences := SplitSentences(text)
	if len(sentences) == 0 {
		return text
	}

	// Build similarity graph and run PageRank
	graph := BuildSimilarityGraph(sentences, config)
	scores := PageRank(graph, config)

	// Create scored sentences with PageRank scores
	ranked := make([]ScoredSentence, len(sentences))
	for i, s := range sentences {
		ranked[i] = ScoredSentence{
			Text:  s,
			Score: scores[i],
			Index: i,
		}
	}

	// Sort by score descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	// Greedily select top sentences until budget is reached
	var selected []ScoredSentence
	currentTokens := 0
	for _, s := range ranked {
		sentTokens := EstimateTokens(s.Text)
		if currentTokens+sentTokens > targetTokens {
			continue
		}
		selected = append(selected, s)
		currentTokens += sentTokens
	}

	if len(selected) == 0 {
		// Budget too small - truncate the top sentence
		if len(ranked) > 0 {
			maxChars := targetTokens * 3
			if maxChars > len(ranked[0].Text) {
				maxChars = len(ranked[0].Text)
			}
			return ranked[0].Text[:maxChars]
		}
		return text
	}

	// Re-sort by original index to preserve document order
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Index < selected[j].Index
	})

	parts := make([]string, len(selected))
	for i, s := range selected {
		parts[i] = s.Text
	}
	return strings.Join(parts, " ")
}
