package rlm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Agentic-Map Operator ───────────────────────────────────────────────────
// Implements Operator-Level Recursion from the LCM paper (Section 3.1).
// Similar to LLM-Map, but spawns a full sub-agent session for each item
// with access to tools (file I/O, code execution, multi-step reasoning).

// AgenticMapConfig configures an Agentic-Map operation.
type AgenticMapConfig struct {
	InputPath    string      `json:"input_path"`    // Path to JSONL input file
	OutputPath   string      `json:"output_path"`   // Path to JSONL output file
	Prompt       string      `json:"prompt"`        // Prompt template ({{item}} is replaced)
	OutputSchema *JSONSchema `json:"output_schema"` // Schema for output validation
	Concurrency  int         `json:"concurrency"`   // Worker pool size (default: 8, lower than LLM-Map)
	MaxRetries   int         `json:"max_retries"`   // Per-item retry limit (default: 2)
	Model        string      `json:"model"`         // Model for sub-agents (default: engine model)
	ReadOnly     bool        `json:"read_only"`     // If true, sub-agents cannot modify filesystem
	MaxDepth     int         `json:"max_depth"`     // Max recursion depth for sub-agents (default: 3)
	MaxIter      int         `json:"max_iterations"` // Max iterations per sub-agent (default: 15)
}

// AgenticMapResult contains results of an Agentic-Map operation.
type AgenticMapResult struct {
	TotalItems  int                 `json:"total_items"`
	Completed   int                 `json:"completed"`
	Failed      int                 `json:"failed"`
	OutputPath  string              `json:"output_path"`
	Duration    time.Duration       `json:"duration"`
	TokensUsed  int                 `json:"tokens_used"`
	ItemResults []AgenticItemResult `json:"item_results,omitempty"`
}

// AgenticItemResult tracks the status of a single agentic-map item.
type AgenticItemResult struct {
	Index      int             `json:"index"`
	Status     MapItemStatus   `json:"status"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	Retries    int             `json:"retries"`
	LLMCalls   int             `json:"llm_calls"`
	Iterations int             `json:"iterations"`
}

// AgenticMapper executes Agentic-Map operations using full sub-agent sessions.
type AgenticMapper struct {
	model       string
	apiBase     string
	apiKey      string
	timeout     int
	extraParams map[string]interface{}
	observer    *Observer
}

// NewAgenticMapper creates a new Agentic-Map executor.
func NewAgenticMapper(model, apiBase, apiKey string, timeout int, extraParams map[string]interface{}, observer *Observer) *AgenticMapper {
	return &AgenticMapper{
		model:       model,
		apiBase:     apiBase,
		apiKey:      apiKey,
		timeout:     timeout,
		extraParams: extraParams,
		observer:    observer,
	}
}

// Execute runs an Agentic-Map operation: parallel sub-agent sessions over JSONL input.
func (am *AgenticMapper) Execute(config AgenticMapConfig) (*AgenticMapResult, error) {
	start := time.Now()

	// Apply defaults
	if config.Concurrency <= 0 {
		config.Concurrency = 8 // Lower default than LLM-Map due to heavier per-item cost
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 2
	}
	if config.MaxDepth <= 0 {
		config.MaxDepth = 3
	}
	if config.MaxIter <= 0 {
		config.MaxIter = 15
	}
	model := config.Model
	if model == "" {
		model = am.model
	}

	am.observer.Debug("lcm.agentic_map", "Starting Agentic-Map: input=%s, concurrency=%d, model=%s, read_only=%v",
		config.InputPath, config.Concurrency, model, config.ReadOnly)

	// Read input items
	items, err := readJSONLFile(config.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	am.observer.Debug("lcm.agentic_map", "Read %d items from %s", len(items), config.InputPath)

	// Initialize results
	results := make([]AgenticItemResult, len(items))
	for i := range results {
		results[i] = AgenticItemResult{
			Index:  i,
			Status: MapItemPending,
		}
	}

	// Worker pool
	var wg sync.WaitGroup
	itemChan := make(chan int, len(items))
	var totalTokens int64

	for i := range items {
		itemChan <- i
	}
	close(itemChan)

	var mu sync.Mutex
	for w := 0; w < config.Concurrency && w < len(items); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range itemChan {
				result := am.processAgenticItem(items[idx], config, model)
				atomic.AddInt64(&totalTokens, int64(result.tokensUsed))

				mu.Lock()
				results[idx] = AgenticItemResult{
					Index:      idx,
					Status:     result.status,
					Output:     result.output,
					Error:      result.errMsg,
					Retries:    result.retries,
					LLMCalls:   result.llmCalls,
					Iterations: result.iterations,
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Write output file
	if config.OutputPath != "" {
		if err := writeAgenticOutput(config.OutputPath, results); err != nil {
			return nil, fmt.Errorf("failed to write output: %w", err)
		}
	}

	completed, failed := 0, 0
	for _, r := range results {
		switch r.Status {
		case MapItemCompleted:
			completed++
		case MapItemFailed:
			failed++
		}
	}

	duration := time.Since(start)
	am.observer.Debug("lcm.agentic_map", "Agentic-Map complete: %d/%d succeeded, %d failed in %s",
		completed, len(items), failed, duration)
	am.observer.Event("lcm.agentic_map_complete", map[string]string{
		"total_items": fmt.Sprintf("%d", len(items)),
		"completed":   fmt.Sprintf("%d", completed),
		"failed":      fmt.Sprintf("%d", failed),
		"duration_ms": fmt.Sprintf("%d", duration.Milliseconds()),
		"tokens_used": fmt.Sprintf("%d", totalTokens),
	})

	return &AgenticMapResult{
		TotalItems:  len(items),
		Completed:   completed,
		Failed:      failed,
		OutputPath:  config.OutputPath,
		Duration:    duration,
		TokensUsed:  int(totalTokens),
		ItemResults: results,
	}, nil
}

// ─── Per-Item Sub-Agent Processing ──────────────────────────────────────────

type agenticItemResult struct {
	status     MapItemStatus
	output     json.RawMessage
	errMsg     string
	retries    int
	tokensUsed int
	llmCalls   int
	iterations int
}

func (am *AgenticMapper) processAgenticItem(item json.RawMessage, config AgenticMapConfig, model string) agenticItemResult {
	prompt := strings.ReplaceAll(config.Prompt, "{{item}}", string(item))

	var lastErr string
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		currentPrompt := prompt
		if attempt > 0 && lastErr != "" {
			currentPrompt = fmt.Sprintf("%s\n\nPrevious attempt failed: %s\nPlease fix the output.", prompt, lastErr)
		}

		// Spawn a full sub-agent (RLM instance) for this item
		subConfig := Config{
			RecursiveModel: model,
			APIBase:        am.apiBase,
			APIKey:         am.apiKey,
			MaxDepth:       config.MaxDepth,
			MaxIterations:  config.MaxIter,
			TimeoutSeconds: am.timeout,
			ExtraParams:    am.extraParams,
		}

		subRLM := New(model, subConfig)
		subRLM.currentDepth = 1 // Sub-agents start at depth 1
		subRLM.useSharedObserver(am.observer)

		// Build context with schema instructions if provided
		context := ""
		if config.OutputSchema != nil {
			schemaJSON, _ := json.MarshalIndent(config.OutputSchema, "", "  ")
			context = fmt.Sprintf("You must output valid JSON matching this schema:\n%s\n\nRespond with ONLY the JSON output.", string(schemaJSON))
		}

		result, stats, err := subRLM.Completion(currentPrompt, context)
		subRLM.Shutdown()

		tokensUsed := stats.TotalTokens

		if err != nil {
			lastErr = err.Error()
			continue
		}

		// Extract JSON from the sub-agent's output
		output := extractJSON(result)
		if output == nil {
			// Try wrapping the raw result as a string value
			wrapped, _ := json.Marshal(result)
			output = wrapped
		}

		// Validate against schema if provided
		if config.OutputSchema != nil && output != nil {
			if validationErr := validateMapOutput(output, config.OutputSchema); validationErr != "" {
				lastErr = validationErr
				continue
			}
		}

		return agenticItemResult{
			status:     MapItemCompleted,
			output:     output,
			retries:    attempt,
			tokensUsed: tokensUsed,
			llmCalls:   stats.LlmCalls,
			iterations: stats.Iterations,
		}
	}

	return agenticItemResult{
		status:  MapItemFailed,
		errMsg:  lastErr,
		retries: config.MaxRetries,
	}
}

// ─── Output Writing ─────────────────────────────────────────────────────────

func writeAgenticOutput(path string, results []AgenticItemResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, r := range results {
		if r.Status == MapItemCompleted && r.Output != nil {
			if _, writeErr := w.Write(r.Output); writeErr != nil {
				return writeErr
			}
			if _, writeErr := w.WriteString("\n"); writeErr != nil {
				return writeErr
			}
		} else {
			errRecord := map[string]interface{}{
				"_error":      r.Error,
				"_index":      r.Index,
				"_llm_calls":  r.LLMCalls,
				"_iterations": r.Iterations,
			}
			data, _ := json.Marshal(errRecord)
			if _, writeErr := w.Write(data); writeErr != nil {
				return writeErr
			}
			if _, writeErr := w.WriteString("\n"); writeErr != nil {
				return writeErr
			}
		}
	}

	return w.Flush()
}
