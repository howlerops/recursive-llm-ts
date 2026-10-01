package rlm

import (
	"fmt"
	"testing"
)

// Benchmarks for the CPU-bound context reduction paths (no LLM calls).
// Run: go test ./rlm -run '^$' -bench 'TextRank|TFIDF|Tokenizer|PageRank|Cosine' -benchmem

var compressionSizes = []int{2000, 8000, 32000}

func BenchmarkCompressTextRank(b *testing.B) {
	for _, size := range compressionSizes {
		text := generateDeterministicContext(size)
		b.Run(fmt.Sprintf("tokens=%d", size), func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				CompressContextTextRank(text, size/2)
			}
		})
	}
}

func BenchmarkCompressTFIDF(b *testing.B) {
	for _, size := range compressionSizes {
		text := generateDeterministicContext(size)
		b.Run(fmt.Sprintf("tokens=%d", size), func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				CompressContextTFIDF(text, size/2)
			}
		})
	}
}

func BenchmarkSimilarityGraph(b *testing.B) {
	for _, size := range compressionSizes {
		sentences := SplitSentences(generateDeterministicContext(size))
		b.Run(fmt.Sprintf("sentences=%d", len(sentences)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				BuildSimilarityGraph(sentences, DefaultTextRankConfig())
			}
		})
	}
}

func BenchmarkPageRank(b *testing.B) {
	for _, size := range compressionSizes {
		sentences := SplitSentences(generateDeterministicContext(size))
		graph := BuildSimilarityGraph(sentences, DefaultTextRankConfig())
		b.Run(fmt.Sprintf("sentences=%d", len(sentences)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				PageRank(graph, DefaultTextRankConfig())
			}
		})
	}
}

func BenchmarkTokenizerBPE(b *testing.B) {
	tok, err := NewTiktokenTokenizer("gpt-4o")
	if err != nil {
		b.Skipf("BPE encoding unavailable: %v", err)
	}
	for _, size := range compressionSizes {
		text := generateDeterministicContext(size)
		b.Run(fmt.Sprintf("tokens=%d", size), func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				tok.CountTokens(text)
			}
		})
	}
}
