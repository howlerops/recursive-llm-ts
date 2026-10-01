package rlm

import (
	"fmt"
	"time"
)

type RLM struct {
	model            string
	recursiveModel   string
	apiBase          string
	apiKey           string
	maxDepth         int
	maxIterations    int
	timeoutSeconds   int
	useMetacognitive bool
	extraParams      map[string]interface{}
	currentDepth     int
	repl             *REPLExecutor
	stats            RLMStats
	observer         *Observer
	sharedObserver   bool // observer belongs to a parent RLM; don't shut it down
	metaAgent        *MetaAgent
	contextOverflow  *ContextOverflowConfig
	lcmEngine        *LCMEngine // Lossless Context Management engine (optional)
}

func New(model string, config Config) *RLM {
	recursiveModel := config.RecursiveModel
	if recursiveModel == "" {
		recursiveModel = model
	}

	// Setup observer
	var obs *Observer
	if config.Observability != nil {
		obs = NewObserver(*config.Observability)
	} else {
		obs = NewNoopObserver()
	}

	// Configure tokenizer for accurate token counting with this model
	SetDefaultTokenizer(model)

	r := &RLM{
		model:            model,
		recursiveModel:   recursiveModel,
		apiBase:          config.APIBase,
		apiKey:           config.APIKey,
		maxDepth:         config.MaxDepth,
		maxIterations:    config.MaxIterations,
		timeoutSeconds:   config.TimeoutSeconds,
		useMetacognitive: config.UseMetacognitive,
		extraParams:      config.ExtraParams,
		currentDepth:     0,
		repl:             NewREPLExecutor(),
		stats:            RLMStats{},
		observer:         obs,
	}
	// Setup meta-agent if enabled
	if config.MetaAgent != nil && config.MetaAgent.Enabled {
		r.metaAgent = NewMetaAgent(r, *config.MetaAgent, obs)
	}

	// Setup context overflow handling
	if config.ContextOverflow != nil {
		r.contextOverflow = config.ContextOverflow
	} else {
		// Enable by default with sensible defaults
		defaultConfig := DefaultContextOverflowConfig()
		r.contextOverflow = &defaultConfig
	}

	// Setup LCM engine if enabled
	if config.LCM != nil && config.LCM.Enabled {
		store := NewLCMStore(fmt.Sprintf("session_%d", time.Now().UnixNano()))
		summarizer := NewLCMSummarizer(model, config.APIBase, config.APIKey, config.TimeoutSeconds, config.ExtraParams, obs)
		modelLimit := 0
		if config.ContextOverflow != nil && config.ContextOverflow.MaxModelTokens > 0 {
			modelLimit = config.ContextOverflow.MaxModelTokens
		} else {
			modelLimit = LookupModelTokenLimit(model)
		}
		r.lcmEngine = NewLCMEngine(*config.LCM, store, summarizer, obs, modelLimit)
	}

	return r
}

func (r *RLM) Completion(query string, context string) (string, RLMStats, error) {
	ctx := r.observer.StartTrace("rlm.completion", map[string]string{
		"model":          r.model,
		"query_length":   fmt.Sprintf("%d", len(query)),
		"context_length": fmt.Sprintf("%d", len(context)),
		"depth":          fmt.Sprintf("%d", r.currentDepth),
	})
	defer r.observer.EndTrace(ctx)

	if query != "" && context == "" {
		context = query
		query = ""
	}

	if r.currentDepth >= r.maxDepth {
		return "", r.stats, NewMaxDepthError(r.maxDepth)
	}

	// Apply meta-agent optimization if enabled
	if r.metaAgent != nil && r.currentDepth == 0 {
		optimized, err := r.metaAgent.OptimizeQuery(query, context)
		if err == nil && optimized != "" {
			r.observer.Debug("rlm", "Using meta-agent optimized query")
			query = optimized
		}
	}

	r.stats.Depth = r.currentDepth
	replEnv := r.buildREPLEnv(query, context)
	systemPrompt := BuildSystemPrompt(len(context), r.currentDepth, query, r.useMetacognitive)

	// ─── LCM-managed completion flow ────────────────────────────────────
	if r.lcmEngine != nil && r.lcmEngine.IsEnabled() {
		return r.completionWithLCM(query, systemPrompt, replEnv)
	}

	// ─── Legacy completion flow (no LCM) ────────────────────────────────
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		r.stats.Iterations = iteration + 1
		r.observer.Debug("rlm", "Iteration %d/%d at depth %d", iteration+1, r.maxIterations, r.currentDepth)

		// Pre-emptive message overflow check: prune older messages if history is growing too large.
		// Regular completion stores context in the REPL env (not messages), but the iterative
		// loop appends assistant+user messages each iteration which can accumulate.
		if modelLimit := r.getModelTokenLimit(); modelLimit > 0 && len(messages) > 4 {
			msgTokens := EstimateMessagesTokens(messages)
			responseTokens := r.getResponseTokenBudget()
			safetyMargin := 0.15
			if r.contextOverflow != nil && r.contextOverflow.SafetyMargin > 0 {
				safetyMargin = r.contextOverflow.SafetyMargin
			}
			available := modelLimit - responseTokens - int(float64(modelLimit)*safetyMargin)
			if msgTokens > available {
				r.observer.Debug("rlm", "Message history overflow: %d tokens > %d available, pruning middle messages", msgTokens, available)
				messages = pruneMessages(messages, available)
			}
		}

		response, err := r.callLLM(messages)
		if err != nil {
			// Check for context overflow and attempt recovery
			if r.contextOverflow != nil && r.contextOverflow.Enabled {
				if _, isOverflow := IsContextOverflow(err); isOverflow && len(messages) > 4 {
					r.observer.Debug("rlm", "Context overflow on iteration %d, pruning messages and retrying", iteration+1)
					modelLimit := r.getModelTokenLimit()
					if modelLimit == 0 {
						modelLimit = 32768 // Reasonable default
					}
					responseTokens := r.getResponseTokenBudget()
					available := modelLimit - responseTokens - int(float64(modelLimit)*0.15)
					messages = pruneMessages(messages, available)
					// Retry this iteration
					iteration--
					continue
				}
			}
			r.observer.Error("rlm", "LLM call failed on iteration %d: %v", iteration+1, err)
			return "", r.stats, err
		}

		if IsFinal(response) {
			answer, ok := ParseResponse(response, replEnv)
			if ok {
				r.observer.Debug("rlm", "FINAL answer found on iteration %d", iteration+1)
				r.observer.Event("rlm.completion_success", map[string]string{
					"iterations": fmt.Sprintf("%d", iteration+1),
					"llm_calls":  fmt.Sprintf("%d", r.stats.LlmCalls),
				})
				return answer, r.stats, nil
			}
		}

		execResult, err := r.repl.Execute(response, replEnv)
		if err != nil {
			r.observer.Debug("rlm", "REPL execution error: %v", err)
			execResult = fmt.Sprintf("Error: %s", err.Error())
		} else {
			r.observer.Debug("rlm", "REPL output: %s", truncateStr(execResult, 200))
		}

		messages = append(messages, Message{Role: "assistant", Content: response})
		messages = append(messages, Message{Role: "user", Content: execResult})
	}

	return "", r.stats, NewMaxIterationsError(r.maxIterations)
}

// completionWithLCM runs the completion loop using the LCM engine for context management.
// Messages flow through the LCM store: persisted verbatim in the immutable store,
// active context assembled from recent messages + summary nodes, and compaction
// triggered via the context control loop after each turn.
func (r *RLM) completionWithLCM(query string, systemPrompt string, replEnv map[string]interface{}) (string, RLMStats, error) {
	store := r.lcmEngine.GetStore()

	// Persist system prompt and initial query in the immutable store
	store.PersistMessage(RoleSystem, systemPrompt, nil)
	store.PersistMessage(RoleUser, query, nil)

	r.observer.Debug("rlm.lcm", "Starting LCM-managed completion, initial tokens: %d",
		store.ActiveContextTokens())

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		r.stats.Iterations = iteration + 1
		r.observer.Debug("rlm.lcm", "Iteration %d/%d at depth %d (active tokens: %d)",
			iteration+1, r.maxIterations, r.currentDepth, store.ActiveContextTokens())

		// Run the LCM context control loop (may trigger async or blocking compaction)
		if err := r.lcmEngine.OnNewItem(); err != nil {
			r.observer.Error("rlm.lcm", "Context control loop error: %v", err)
			// Non-fatal: continue with current context
		}

		// Build messages from the active context (includes summaries with IDs)
		messages := store.BuildMessages()

		response, err := r.callLLM(messages)
		if err != nil {
			// Check for context overflow — LCM should handle this via compaction,
			// but fall back to blocking compaction if the API still rejects
			if r.contextOverflow != nil && r.contextOverflow.Enabled {
				if _, isOverflow := IsContextOverflow(err); isOverflow {
					r.observer.Debug("rlm.lcm", "Context overflow despite LCM, forcing blocking compaction")
					if compactErr := r.lcmEngine.blockingCompaction(); compactErr != nil {
						r.observer.Error("rlm.lcm", "Emergency compaction failed: %v", compactErr)
						return "", r.stats, err
					}
					// Also try condensing old summaries to free more space
					_ = r.lcmEngine.CondenseOldSummaries()
					iteration-- // Retry
					continue
				}
			}
			r.observer.Error("rlm.lcm", "LLM call failed on iteration %d: %v", iteration+1, err)
			return "", r.stats, err
		}

		// Persist assistant response in the immutable store
		store.PersistMessage(RoleAssistant, response, nil)

		if IsFinal(response) {
			answer, ok := ParseResponse(response, replEnv)
			if ok {
				r.observer.Debug("rlm.lcm", "FINAL answer on iteration %d (store: %d msgs, %d summaries)",
					iteration+1, store.MessageCount(), store.Stats().TotalSummaries)
				r.observer.Event("rlm.lcm.completion_success", map[string]string{
					"iterations":        fmt.Sprintf("%d", iteration+1),
					"llm_calls":         fmt.Sprintf("%d", r.stats.LlmCalls),
					"total_messages":    fmt.Sprintf("%d", store.MessageCount()),
					"total_summaries":   fmt.Sprintf("%d", store.Stats().TotalSummaries),
					"compression_ratio": fmt.Sprintf("%.2f", store.Stats().CompressionRatio),
				})
				return answer, r.stats, nil
			}
		}

		execResult, err := r.repl.Execute(response, replEnv)
		if err != nil {
			r.observer.Debug("rlm.lcm", "REPL execution error: %v", err)
			execResult = fmt.Sprintf("Error: %s", err.Error())
		} else {
			r.observer.Debug("rlm.lcm", "REPL output: %s", truncateStr(execResult, 200))
		}

		// Persist REPL result as user message in the immutable store
		store.PersistMessage(RoleUser, execResult, nil)
	}

	return "", r.stats, NewMaxIterationsError(r.maxIterations)
}

func (r *RLM) callLLM(messages []Message) (string, error) {
	r.stats.LlmCalls++
	defaultModel := r.model
	if r.currentDepth > 0 {
		defaultModel = r.recursiveModel
	}

	r.observer.Debug("llm", "Calling %s with %d messages", defaultModel, len(messages))

	start := time.Now()

	request := ChatRequest{
		Model:       defaultModel,
		Messages:    messages,
		APIBase:     r.apiBase,
		APIKey:      r.apiKey,
		Timeout:     r.timeoutSeconds,
		ExtraParams: r.extraParams,
	}

	result, err := CallChatCompletion(request)
	duration := time.Since(start)

	tokensUsed := 0
	if result.Usage != nil {
		r.stats.PromptTokens += result.Usage.PromptTokens
		r.stats.CompletionTokens += result.Usage.CompletionTokens
		r.stats.TotalTokens += result.Usage.TotalTokens
		tokensUsed = result.Usage.TotalTokens
	}

	r.observer.LLMCallWithUsage(defaultModel, len(messages), result.Usage, duration, err)

	if err != nil {
		return "", err
	}

	r.observer.Debug("llm", "Response received (%d chars, %d tokens) in %s", len(result.Content), tokensUsed, duration)
	return result.Content, nil
}

func (r *RLM) buildREPLEnv(query string, context string) map[string]interface{} {
	env := map[string]interface{}{
		"context": context,
		"query":   query,
	}

	env["re"] = NewRegexHelper()
	env["recursive_llm"] = func(subQuery string, subContext string) string {
		if r.currentDepth+1 >= r.maxDepth {
			return fmt.Sprintf("Max recursion depth (%d) reached", r.maxDepth)
		}

		r.observer.Debug("rlm", "Recursive call at depth %d: %s", r.currentDepth+1, truncateStr(subQuery, 100))

		subConfig := Config{
			RecursiveModel:   r.recursiveModel,
			APIBase:          r.apiBase,
			APIKey:           r.apiKey,
			MaxDepth:         r.maxDepth,
			MaxIterations:    r.maxIterations,
			TimeoutSeconds:   r.timeoutSeconds,
			UseMetacognitive: r.useMetacognitive,
			ExtraParams:      r.extraParams,
		}

		subRLM := New(r.recursiveModel, subConfig)
		subRLM.currentDepth = r.currentDepth + 1
		subRLM.useSharedObserver(r.observer) // Share observer for trace continuity

		answer, _, err := subRLM.Completion(subQuery, subContext)
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error())
		}
		return answer
	}

	return env
}

// pruneMessages removes older middle messages to fit within a token budget.
// Preserves the first message (system prompt) and the last 2 messages (most recent exchange).
func pruneMessages(messages []Message, targetTokens int) []Message {
	if len(messages) <= 3 {
		return messages
	}

	// Always keep: system prompt (first), last 2 messages (most recent exchange)
	system := messages[0]
	lastN := messages[len(messages)-2:]

	// Start with the preserved messages
	result := []Message{system}
	currentTokens := EstimateMessagesTokens(append(result, lastN...))

	if currentTokens >= targetTokens {
		// Even the minimum set exceeds the budget; return it anyway
		return append(result, lastN...)
	}

	// Add middle messages from most recent to oldest until budget is exceeded
	middle := messages[1 : len(messages)-2]
	for i := len(middle) - 1; i >= 0; i-- {
		msgTokens := 4 + EstimateTokens(middle[i].Content)
		if currentTokens+msgTokens > targetTokens {
			break
		}
		result = append(result, middle[i])
		currentTokens += msgTokens
	}

	// Reverse the added middle messages (they were added newest-first)
	if len(result) > 1 {
		added := result[1:]
		for i, j := 0, len(added)-1; i < j; i, j = i+1, j-1 {
			added[i], added[j] = added[j], added[i]
		}
	}

	return append(result, lastN...)
}

// GetObserver returns the observer for external access to events/traces.
func (r *RLM) GetObserver() *Observer {
	return r.observer
}

// GetLCMEngine returns the LCM engine if enabled, nil otherwise.
func (r *RLM) GetLCMEngine() *LCMEngine {
	return r.lcmEngine
}

// LLMMap executes an LLM-Map operation for parallel batch processing.
func (r *RLM) LLMMap(config LLMMapConfig) (*LLMMapResult, error) {
	mapper := NewLLMMapper(r.model, r.apiBase, r.apiKey, r.timeoutSeconds, r.extraParams, r.observer)
	return mapper.Execute(config)
}

// AgenticMap executes an Agentic-Map operation with full sub-agent sessions per item.
func (r *RLM) AgenticMap(config AgenticMapConfig) (*AgenticMapResult, error) {
	mapper := NewAgenticMapper(r.model, r.apiBase, r.apiKey, r.timeoutSeconds, r.extraParams, r.observer)
	return mapper.Execute(config)
}

// Shutdown gracefully shuts down the RLM engine and its observer. An observer
// shared from a parent RLM is left running; the parent shuts it down.
func (r *RLM) Shutdown() {
	if r.observer != nil && !r.sharedObserver {
		r.observer.Shutdown()
	}
}

// useSharedObserver makes a sub-agent report to its parent's observer for
// trace continuity, without taking over its lifecycle.
func (r *RLM) useSharedObserver(obs *Observer) {
	r.observer = obs
	r.sharedObserver = true
}
