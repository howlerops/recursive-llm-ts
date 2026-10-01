package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/howlerops/recursive-llm-ts/go/rlm"
)

type requestPayload struct {
	Model      string                 `json:"model"`
	Query      string                 `json:"query"`
	Context    string                 `json:"context"`
	Config     map[string]interface{} `json:"config"`
	Structured  *structuredRequest      `json:"structured,omitempty"`
	LLMMap      *rlm.LLMMapConfig      `json:"llm_map,omitempty"`      // LCM LLM-Map operation
	AgenticMap  *rlm.AgenticMapConfig  `json:"agentic_map,omitempty"`  // LCM Agentic-Map operation
}

type structuredRequest struct {
	Schema            *rlm.JSONSchema `json:"schema"`
	ParallelExecution bool            `json:"parallelExecution"`
	MaxRetries        int             `json:"maxRetries"`
}

type responsePayload struct {
	Result           interface{}        `json:"result"`
	Stats            rlm.RLMStats       `json:"stats"`
	StructuredResult bool               `json:"structured_result,omitempty"`
	TraceEvents      interface{}        `json:"trace_events,omitempty"`
	LCMStats          *rlm.LCMStoreStats    `json:"lcm_stats,omitempty"`
	LLMMapResult      *rlm.LLMMapResult     `json:"llm_map_result,omitempty"`
	AgenticMapResult  *rlm.AgenticMapResult  `json:"agentic_map_result,omitempty"`
}

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read stdin:", err)
		os.Exit(1)
	}

	var req requestPayload
	if err := json.Unmarshal(input, &req); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse input JSON:", err)
		os.Exit(1)
	}

	if req.Model == "" {
		fmt.Fprintln(os.Stderr, "Missing model in request payload")
		os.Exit(1)
	}

	config := rlm.ConfigFromMap(req.Config)
	engine := rlm.New(req.Model, config)
	defer engine.Shutdown()

	// os.Exit skips deferred calls, so flush observability (e.g. Langfuse)
	// before exiting on an error.
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		engine.Shutdown()
		os.Exit(1)
	}

	var resp responsePayload

	// Handle LLM-Map operation if requested
	if req.LLMMap != nil {
		mapResult, err := engine.LLMMap(*req.LLMMap)
		if err != nil {
			fail(err)
		}
		resp = responsePayload{
			Result:       "llm_map_complete",
			LLMMapResult: mapResult,
		}
	} else if req.AgenticMap != nil {
		// Handle Agentic-Map operation
		agenticResult, err := engine.AgenticMap(*req.AgenticMap)
		if err != nil {
			fail(err)
		}
		resp = responsePayload{
			Result:           "agentic_map_complete",
			AgenticMapResult: agenticResult,
		}
	} else if req.Structured != nil {
	// Handle structured completion if requested
		structuredConfig := &rlm.StructuredConfig{
			Schema:            req.Structured.Schema,
			ParallelExecution: req.Structured.ParallelExecution,
			MaxRetries:        req.Structured.MaxRetries,
		}

		result, stats, err := engine.StructuredCompletion(req.Query, req.Context, structuredConfig)
		if err != nil {
			fail(err)
		}

		resp = responsePayload{
			Result:           result,
			Stats:            stats,
			StructuredResult: true,
		}
	} else {
		// Regular completion
		result, stats, err := engine.Completion(req.Query, req.Context)
		if err != nil {
			fail(err)
		}

		resp = responsePayload{
			Result: result,
			Stats:  stats,
		}
	}

	// Include trace events if observability is enabled
	obs := engine.GetObserver()
	if obs != nil {
		events := obs.GetEvents()
		if len(events) > 0 {
			resp.TraceEvents = events
		}
	}

	// Include LCM stats if enabled
	if lcmEngine := engine.GetLCMEngine(); lcmEngine != nil {
		stats := lcmEngine.GetStore().Stats()
		resp.LCMStats = &stats
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to encode response JSON:", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
