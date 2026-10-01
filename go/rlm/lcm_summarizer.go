package rlm

import (
	"fmt"
	"strings"
)

// ─── Five-Level Summarization Escalation ────────────────────────────────────
// Implements the guaranteed-convergence summarization protocol from the LCM paper.
// Level 1 (Normal): LLM-Summarize with preserve_details mode
// Level 2 (Aggressive): LLM-Summarize with bullet_points mode, half target tokens
// Level 3 (TF-IDF): Extractive compression, no LLM, preserves actual sentences
// Level 4 (TextRank): Graph-based extractive compression, no LLM, better coherence
// Level 5 (Deterministic): DeterministicTruncate, no LLM, guaranteed reduction

// LCMSummarizer handles the five-level escalation for context compaction.
type LCMSummarizer struct {
	model       string
	apiBase     string
	apiKey      string
	timeout     int
	extraParams map[string]interface{}
	observer    *Observer
}

// NewLCMSummarizer creates a summarizer with the given LLM configuration.
func NewLCMSummarizer(model, apiBase, apiKey string, timeout int, extraParams map[string]interface{}, observer *Observer) *LCMSummarizer {
	return &LCMSummarizer{
		model:       model,
		apiBase:     apiBase,
		apiKey:      apiKey,
		timeout:     timeout,
		extraParams: extraParams,
		observer:    observer,
	}
}

// SummarizeResult contains the output of a summarization attempt.
type SummarizeResult struct {
	Content string // The summary text
	Tokens  int    // Token count of the summary
	Level   int    // Escalation level used (1-5)
}

// Summarize applies the five-level escalation to compress text to targetTokens.
// Guaranteed to converge: Level 5 is deterministic truncation.
func (ls *LCMSummarizer) Summarize(input string, targetTokens int) (*SummarizeResult, error) {
	inputTokens := EstimateTokens(input)

	// If already within budget, no summarization needed
	if inputTokens <= targetTokens {
		return &SummarizeResult{
			Content: input,
			Tokens:  inputTokens,
			Level:   0,
		}, nil
	}

	ls.observer.Debug("lcm.summarizer", "Starting escalation: %d tokens → target %d", inputTokens, targetTokens)

	// Level 1: Normal - preserve details
	result, err := ls.summarizeLevel1(input, targetTokens)
	if err == nil && result.Tokens < inputTokens {
		ls.observer.Debug("lcm.summarizer", "Level 1 succeeded: %d → %d tokens", inputTokens, result.Tokens)
		return result, nil
	}
	if err != nil {
		ls.observer.Debug("lcm.summarizer", "Level 1 failed: %v, escalating", err)
	} else {
		ls.observer.Debug("lcm.summarizer", "Level 1 did not reduce (%d >= %d), escalating", result.Tokens, inputTokens)
	}

	// Level 2: Aggressive - bullet points at half target
	result, err = ls.summarizeLevel2(input, targetTokens/2)
	if err == nil && result.Tokens < inputTokens {
		ls.observer.Debug("lcm.summarizer", "Level 2 succeeded: %d → %d tokens", inputTokens, result.Tokens)
		return result, nil
	}
	if err != nil {
		ls.observer.Debug("lcm.summarizer", "Level 2 failed: %v, escalating", err)
	} else {
		ls.observer.Debug("lcm.summarizer", "Level 2 did not reduce (%d >= %d), escalating", result.Tokens, inputTokens)
	}

	// Level 3: TF-IDF extractive compression
	result = ls.summarizeLevel3TFIDF(input, targetTokens)
	if result.Tokens < inputTokens {
		ls.observer.Debug("lcm.summarizer", "Level 3 (TF-IDF) succeeded: %d → %d tokens", inputTokens, result.Tokens)
		return result, nil
	}
	ls.observer.Debug("lcm.summarizer", "Level 3 (TF-IDF) did not reduce enough (%d >= %d), escalating", result.Tokens, inputTokens)

	// Level 4: TextRank graph-based compression
	result = ls.summarizeLevel4TextRank(input, targetTokens)
	if result.Tokens < inputTokens {
		ls.observer.Debug("lcm.summarizer", "Level 4 (TextRank) succeeded: %d → %d tokens", inputTokens, result.Tokens)
		return result, nil
	}
	ls.observer.Debug("lcm.summarizer", "Level 4 (TextRank) did not reduce enough (%d >= %d), escalating to deterministic", result.Tokens, inputTokens)

	// Level 5: Deterministic truncation - guaranteed convergence
	result = ls.deterministicTruncate(input, targetTokens)
	ls.observer.Debug("lcm.summarizer", "Level 5 (deterministic): %d → %d tokens", inputTokens, result.Tokens)
	return result, nil
}

// SummarizeMessages applies three-level escalation to a slice of messages.
func (ls *LCMSummarizer) SummarizeMessages(messages []*StoreMessage, targetTokens int) (*SummarizeResult, error) {
	// Build a formatted input from messages
	var sb strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&sb, "[%s] %s\n", msg.Role, msg.Content)
	}
	return ls.Summarize(sb.String(), targetTokens)
}

// ─── Level 1: Normal LLM Summarization ──────────────────────────────────────

func (ls *LCMSummarizer) summarizeLevel1(input string, targetTokens int) (*SummarizeResult, error) {
	prompt := fmt.Sprintf(`Summarize the following content in approximately %d tokens.
Preserve all key details, decisions, and specific information.
Maintain the logical flow and any action items or conclusions.

Content:
%s

Provide a comprehensive summary that retains the most important information:`, targetTokens, input)

	content, err := ls.callLLM(prompt, targetTokens)
	if err != nil {
		return nil, err
	}

	return &SummarizeResult{
		Content: content,
		Tokens:  EstimateTokens(content),
		Level:   1,
	}, nil
}

// ─── Level 2: Aggressive LLM Summarization ──────────────────────────────────

func (ls *LCMSummarizer) summarizeLevel2(input string, targetTokens int) (*SummarizeResult, error) {
	prompt := fmt.Sprintf(`Aggressively summarize the following content as bullet points.
Target: approximately %d tokens. Be extremely concise.
Keep only: key decisions, critical facts, action items, and conclusions.
Drop: examples, explanations, context, and elaboration.

Content:
%s

Bullet-point summary:`, targetTokens, input)

	content, err := ls.callLLM(prompt, targetTokens)
	if err != nil {
		return nil, err
	}

	return &SummarizeResult{
		Content: content,
		Tokens:  EstimateTokens(content),
		Level:   2,
	}, nil
}

// ─── Level 3: TF-IDF Extractive Compression ─────────────────────────────────

func (ls *LCMSummarizer) summarizeLevel3TFIDF(input string, targetTokens int) *SummarizeResult {
	compressed := CompressContextTFIDF(input, targetTokens)
	return &SummarizeResult{
		Content: compressed,
		Tokens:  EstimateTokens(compressed),
		Level:   3,
	}
}

// ─── Level 4: TextRank Graph-Based Compression ─────────────────────────────

func (ls *LCMSummarizer) summarizeLevel4TextRank(input string, targetTokens int) *SummarizeResult {
	compressed := CompressContextTextRank(input, targetTokens)
	return &SummarizeResult{
		Content: compressed,
		Tokens:  EstimateTokens(compressed),
		Level:   4,
	}
}

// ─── Level 5: Deterministic Truncation ──────────────────────────────────────

// deterministicTruncate is the guaranteed-convergence fallback.
// No LLM call involved — uses the shared TruncateText utility (compression.go).
func (ls *LCMSummarizer) deterministicTruncate(input string, maxTokens int) *SummarizeResult {
	truncated := TruncateText(input, TruncateTextParams{
		MaxTokens:  maxTokens,
		MarkerText: "\n[... content truncated for context management ...]\n",
	})

	return &SummarizeResult{
		Content: truncated,
		Tokens:  EstimateTokens(truncated),
		Level:   5,
	}
}

// ─── LLM Helper ─────────────────────────────────────────────────────────────

func (ls *LCMSummarizer) callLLM(prompt string, maxTokens int) (string, error) {
	// Cap max_tokens for summarization
	if maxTokens > 4096 {
		maxTokens = 4096
	}
	if maxTokens < 256 {
		maxTokens = 256
	}

	params := make(map[string]interface{})
	for k, v := range ls.extraParams {
		params[k] = v
	}
	params["max_tokens"] = maxTokens

	request := ChatRequest{
		Model: ls.model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		APIBase:     ls.apiBase,
		APIKey:      ls.apiKey,
		Timeout:     ls.timeout,
		ExtraParams: params,
	}

	result, err := CallChatCompletion(request)
	if err != nil {
		return "", fmt.Errorf("summarization LLM call failed: %w", err)
	}

	return result.Content, nil
}
