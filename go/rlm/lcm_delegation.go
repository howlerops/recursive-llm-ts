package rlm

import (
	"fmt"
	"strings"
)

// ─── Infinite Delegation Guard ──────────────────────────────────────────────
// Implements the scope-reduction invariant from the LCM paper (Section 3.2).
//
// When a sub-agent spawns a further sub-agent, it must declare:
//   - delegated_scope: the specific slice of work being handed off
//   - kept_work: the work the caller will still perform itself
//
// If the caller cannot articulate what it's retaining (i.e., it would delegate
// its entire responsibility), the call is rejected. This forces each level of
// delegation to represent a strict reduction in responsibility.
//
// Exemptions:
//   - Root agent (depth 0): no parent to recurse with
//   - Read-only agents: cannot spawn further sub-agents
//   - Parallel decomposition (sibling tasks): not nested delegation

// DelegationRequest represents a request to delegate work to a sub-agent.
type DelegationRequest struct {
	// Prompt is the task description for the sub-agent.
	Prompt string `json:"prompt"`

	// DelegatedScope describes the specific slice of work being handed off.
	// Required for non-root agents.
	DelegatedScope string `json:"delegated_scope"`

	// KeptWork describes the work the caller retains for itself.
	// Required for non-root agents. Must be non-empty and distinct from DelegatedScope.
	KeptWork string `json:"kept_work"`

	// ReadOnly indicates this is a read-only exploration agent (exempt from guard).
	ReadOnly bool `json:"read_only"`

	// Parallel indicates this is parallel decomposition (exempt from guard).
	Parallel bool `json:"parallel"`
}

// DelegationGuard enforces the scope-reduction invariant.
type DelegationGuard struct {
	observer *Observer
}

// NewDelegationGuard creates a new delegation guard.
func NewDelegationGuard(observer *Observer) *DelegationGuard {
	return &DelegationGuard{observer: observer}
}

// DelegationError is returned when a delegation request violates the scope-reduction invariant.
type DelegationError struct {
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
}

func (e *DelegationError) Error() string {
	return fmt.Sprintf("delegation rejected: %s. %s", e.Reason, e.Suggestion)
}

// ValidateDelegation checks if a delegation request is allowed at the given depth.
// Returns nil if allowed, or a DelegationError explaining why it was rejected.
func (g *DelegationGuard) ValidateDelegation(depth int, req DelegationRequest) error {
	// Root agent (depth 0) is always allowed to delegate
	if depth == 0 {
		g.observer.Debug("lcm.delegation", "Root agent delegation allowed (depth 0)")
		return nil
	}

	// Read-only agents are exempt (they can't spawn further sub-agents)
	if req.ReadOnly {
		g.observer.Debug("lcm.delegation", "Read-only agent delegation allowed")
		return nil
	}

	// Parallel decomposition is exempt (sibling, not nested)
	if req.Parallel {
		g.observer.Debug("lcm.delegation", "Parallel decomposition delegation allowed")
		return nil
	}

	// Non-root agents must declare scope reduction
	if strings.TrimSpace(req.DelegatedScope) == "" {
		g.observer.Debug("lcm.delegation", "Delegation rejected: no delegated_scope at depth %d", depth)
		return &DelegationError{
			Reason:     "sub-agent must declare delegated_scope",
			Suggestion: "Describe the specific slice of work being handed off, or perform the work directly.",
		}
	}

	if strings.TrimSpace(req.KeptWork) == "" {
		g.observer.Debug("lcm.delegation", "Delegation rejected: no kept_work at depth %d", depth)
		return &DelegationError{
			Reason:     "sub-agent must declare kept_work (what the caller retains)",
			Suggestion: "If you cannot articulate what you're retaining, perform the work directly instead of delegating.",
		}
	}

	// Check for full delegation (delegated_scope ≈ entire task)
	if isTotalDelegation(req.DelegatedScope, req.KeptWork) {
		g.observer.Debug("lcm.delegation", "Delegation rejected: total delegation detected at depth %d", depth)
		return &DelegationError{
			Reason:     "delegated_scope appears to encompass the entire task; kept_work is trivial",
			Suggestion: "Break the task into meaningful subtasks where you retain substantial work, or perform it directly.",
		}
	}

	g.observer.Debug("lcm.delegation", "Delegation allowed at depth %d: scope=%q, kept=%q",
		depth, truncateStr(req.DelegatedScope, 80), truncateStr(req.KeptWork, 80))
	return nil
}

// isTotalDelegation detects when an agent is trying to delegate its entire responsibility.
// This is a heuristic check — it catches obvious cases of trivial kept_work.
func isTotalDelegation(delegatedScope, keptWork string) bool {
	kept := strings.TrimSpace(strings.ToLower(keptWork))

	// Trivial kept_work patterns that indicate full delegation
	trivialPatterns := []string{
		"none",
		"nothing",
		"n/a",
		"na",
		"",
		"will wait",
		"waiting",
		"just wait",
		"aggregate",
		"collect results",
		"return results",
		"pass through",
		"forward",
	}

	for _, pattern := range trivialPatterns {
		if kept == pattern {
			return true
		}
	}

	// Check if kept_work is suspiciously short compared to delegated_scope
	// (less than 10% of the delegated scope's length and under 20 chars)
	if len(kept) < 20 && len(kept) < len(delegatedScope)/10 {
		return true
	}

	return false
}

// ─── Integration with RLM Engine ────────────────────────────────────────────

// DelegateTask validates and executes a delegation request through the RLM engine.
// This is the main entry point for task delegation with the infinite recursion guard.
func (r *RLM) DelegateTask(req DelegationRequest) (string, RLMStats, error) {
	// Create or use existing delegation guard
	guard := NewDelegationGuard(r.observer)

	// Validate the delegation
	if err := guard.ValidateDelegation(r.currentDepth, req); err != nil {
		return "", RLMStats{}, err
	}

	// Create sub-agent
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
	subRLM.useSharedObserver(r.observer)
	defer subRLM.Shutdown()

	r.observer.Debug("lcm.delegation", "Spawning sub-agent at depth %d for: %s",
		r.currentDepth+1, truncateStr(req.Prompt, 100))

	result, stats, err := subRLM.Completion(req.Prompt, "")
	return result, stats, err
}

// DelegateTasks validates and executes multiple parallel delegation requests.
// This implements the Tasks() tool from the LCM paper (Appendix C.3).
// Parallel decomposition is exempt from the recursion guard.
func (r *RLM) DelegateTasks(tasks []DelegationRequest) ([]string, []RLMStats, error) {
	if len(tasks) < 2 {
		return nil, nil, fmt.Errorf("DelegateTasks requires at least 2 tasks for parallel decomposition")
	}

	guard := NewDelegationGuard(r.observer)

	// Mark all as parallel (exempt from guard) but still validate basic structure
	for i := range tasks {
		tasks[i].Parallel = true
		if err := guard.ValidateDelegation(r.currentDepth, tasks[i]); err != nil {
			return nil, nil, fmt.Errorf("task %d validation failed: %w", i, err)
		}
	}

	r.observer.Debug("lcm.delegation", "Spawning %d parallel sub-agents at depth %d",
		len(tasks), r.currentDepth+1)

	type taskResult struct {
		index  int
		result string
		stats  RLMStats
		err    error
	}

	results := make(chan taskResult, len(tasks))

	for i, task := range tasks {
		go func(idx int, t DelegationRequest) {
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
			subRLM.useSharedObserver(r.observer)
			defer subRLM.Shutdown()

			result, stats, err := subRLM.Completion(t.Prompt, "")
			results <- taskResult{index: idx, result: result, stats: stats, err: err}
		}(i, task)
	}

	// Collect results in order
	resultSlice := make([]string, len(tasks))
	statsSlice := make([]RLMStats, len(tasks))

	for range tasks {
		tr := <-results
		if tr.err != nil {
			return nil, nil, fmt.Errorf("parallel task %d failed: %w", tr.index, tr.err)
		}
		resultSlice[tr.index] = tr.result
		statsSlice[tr.index] = tr.stats
	}

	return resultSlice, statsSlice, nil
}
