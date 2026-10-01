package rlm

import (
	"math"
	"testing"
)

// Reference implementations: the original pairwise similarity graph and
// column-scanning PageRank, kept to check the optimized versions.
func referenceSimilarityGraph(sentences []string, config TextRankConfig) [][]float64 {
	n := len(sentences)
	vectors := buildTFIDFVectors(sentences)
	graph := newSquareMatrix(n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if sim := cosineSimilarity(vectors[i], vectors[j]); sim >= config.MinSimilarity {
				graph[i][j] = sim
				graph[j][i] = sim
			}
		}
	}
	return graph
}

func referencePageRank(graph [][]float64, config TextRankConfig) []float64 {
	n := len(graph)
	d := config.DampingFactor
	scores := make([]float64, n)
	newScores := make([]float64, n)
	for i := range scores {
		scores[i] = 1.0 / float64(n)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			out[i] += graph[i][j]
		}
	}
	for iter := 0; iter < config.MaxIterations; iter++ {
		maxDelta := 0.0
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if graph[j][i] > 0 && out[j] > 0 {
					sum += graph[j][i] / out[j] * scores[j]
				}
			}
			newScores[i] = (1-d)/float64(n) + d*sum
			maxDelta = math.Max(maxDelta, math.Abs(newScores[i]-scores[i]))
		}
		scores, newScores = newScores, scores
		if maxDelta < config.ConvergenceThreshold {
			break
		}
	}
	return scores
}

func TestTextRankOptimizedMatchesReference(t *testing.T) {
	config := DefaultTextRankConfig()
	inputs := map[string]string{
		"deterministic": generateDeterministicContext(6000),
		"prose":         fixedEnglishProse500Words(),
	}
	for name, text := range inputs {
		sentences := SplitSentences(text)
		got := BuildSimilarityGraph(sentences, config)
		want := referenceSimilarityGraph(sentences, config)
		for i := range want {
			for j := range want[i] {
				if math.Abs(got[i][j]-want[i][j]) > 1e-9 {
					t.Fatalf("%s: graph[%d][%d] = %v, want %v", name, i, j, got[i][j], want[i][j])
				}
			}
		}

		gotScores := PageRank(want, config)
		wantScores := referencePageRank(want, config)
		for i := range wantScores {
			if math.Abs(gotScores[i]-wantScores[i]) > 1e-9 {
				t.Fatalf("%s: score[%d] = %v, want %v", name, i, gotScores[i], wantScores[i])
			}
		}
	}
}
