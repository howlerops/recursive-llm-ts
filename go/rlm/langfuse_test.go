package rlm

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type langfuseRequest struct {
	Path     string
	User     string
	Password string
	Batch    []langfuseIngestionEvent
}

type fakeLangfuse struct {
	mu       sync.Mutex
	requests []langfuseRequest
	status   int
	response string
}

func (f *fakeLangfuse) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Batch []langfuseIngestionEvent `json:"batch"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid ingestion payload: %v", err)
		}
		user, pass, _ := r.BasicAuth()
		f.mu.Lock()
		f.requests = append(f.requests, langfuseRequest{Path: r.URL.Path, User: user, Password: pass, Batch: payload.Batch})
		status, response := f.status, f.response
		f.mu.Unlock()
		if status == 0 {
			status = http.StatusMultiStatus
		}
		if response == "" {
			response = `{"successes":[],"errors":[]}`
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}
}

func newLangfuseTestObserver(t *testing.T, fake *fakeLangfuse) *Observer {
	t.Helper()
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	return NewObserver(ObservabilityConfig{
		LangfuseEnabled:   true,
		LangfusePublicKey: "pk-test",
		LangfuseSecretKey: "sk-test",
		LangfuseHost:      server.URL + "/",
	})
}

func TestLangfuse_SendsTraceAndGenerationOnShutdown(t *testing.T) {
	fake := &fakeLangfuse{}
	obs := newLangfuseTestObserver(t, fake)

	obs.StartTrace("rlm.completion", map[string]string{"model": "gpt-4o-mini", "api_key": "sk-live"})
	obs.StartSpan("rlm.structured_completion", map[string]string{"schema_type": "object"})
	obs.LLMCallWithUsage("gpt-4o-mini", 2, &TokenUsage{PromptTokens: 120, CompletionTokens: 30, TotalTokens: 150}, 250*time.Millisecond, nil)
	obs.LLMCall("gpt-4o-mini", 3, 0, 10*time.Millisecond, errors.New("rate limited"))
	obs.Error("structured", "validation failed: %s", "missing summaryType")
	obs.Event("lcm.compaction", map[string]string{"messages": "4"})

	if len(fake.requests) != 0 {
		t.Fatalf("expected no requests before shutdown, got %d", len(fake.requests))
	}
	obs.Shutdown()

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 ingestion request, got %d", len(fake.requests))
	}
	req := fake.requests[0]
	if req.Path != "/api/public/ingestion" {
		t.Errorf("path = %q", req.Path)
	}
	if req.User != "pk-test" || req.Password != "sk-test" {
		t.Errorf("basic auth = %q/%q", req.User, req.Password)
	}

	types := make([]string, len(req.Batch))
	for i, e := range req.Batch {
		types[i] = e.Type
		if e.ID == "" || e.Timestamp == "" {
			t.Errorf("event %d missing id/timestamp: %+v", i, e)
		}
	}
	want := []string{"trace-create", "event-create", "generation-create", "generation-create", "event-create", "event-create"}
	if len(types) != len(want) {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("event types = %v, want %v", types, want)
		}
	}

	trace := req.Batch[0].Body
	traceID, _ := trace["id"].(string)
	if trace["name"] != "rlm.completion" || traceID == "" {
		t.Errorf("trace body = %v", trace)
	}
	if md, _ := trace["metadata"].(map[string]interface{}); md["api_key"] != "[REDACTED]" {
		t.Errorf("trace metadata not redacted: %v", trace["metadata"])
	}
	for i, e := range req.Batch[1:] {
		if e.Body["traceId"] != traceID {
			t.Errorf("event %d traceId = %v, want %s", i+1, e.Body["traceId"], traceID)
		}
	}

	gen := req.Batch[2].Body
	if gen["model"] != "gpt-4o-mini" || gen["level"] != "DEFAULT" {
		t.Errorf("generation body = %v", gen)
	}
	usage, _ := gen["usage"].(map[string]interface{})
	if usage["input"] != float64(120) || usage["output"] != float64(30) || usage["total"] != float64(150) || usage["unit"] != "TOKENS" {
		t.Errorf("generation usage = %v", usage)
	}
	start, _ := time.Parse(time.RFC3339Nano, gen["startTime"].(string))
	end, _ := time.Parse(time.RFC3339Nano, gen["endTime"].(string))
	if d := end.Sub(start); d < 249*time.Millisecond || d > 251*time.Millisecond {
		t.Errorf("generation duration = %s, want 250ms", d)
	}

	failed := req.Batch[3].Body
	if failed["level"] != "ERROR" || failed["statusMessage"] != "rate limited" {
		t.Errorf("failed generation body = %v", failed)
	}
	if _, ok := failed["usage"]; ok {
		t.Errorf("failed generation with no tokens should omit usage: %v", failed["usage"])
	}

	errEvent := req.Batch[4].Body
	if errEvent["level"] != "ERROR" || errEvent["statusMessage"] != "validation failed: missing summaryType" {
		t.Errorf("error event body = %v", errEvent)
	}
}

func TestLangfuse_FlushDrainsAndBatches(t *testing.T) {
	fake := &fakeLangfuse{}
	obs := newLangfuseTestObserver(t, fake)

	for i := 0; i < 150; i++ {
		obs.Event("tick", nil)
	}
	if err := obs.FlushLangfuse(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("expected 2 batches for 151 events, got %d", len(fake.requests))
	}
	if n := len(fake.requests[0].Batch) + len(fake.requests[1].Batch); n != 151 {
		t.Errorf("sent %d events, want 151 (trace + 150)", n)
	}

	// Already-sent events are not resent, and the trace is created only once.
	obs.Event("after", nil)
	if err := obs.FlushLangfuse(); err != nil {
		t.Fatalf("second flush: %v", err)
	}
	last := fake.requests[len(fake.requests)-1].Batch
	if len(last) != 1 || last[0].Type != "event-create" {
		t.Errorf("second flush batch = %+v", last)
	}
}

func TestLangfuse_ReportsRejectedEvents(t *testing.T) {
	fake := &fakeLangfuse{response: `{"successes":[],"errors":[{"id":"x","status":400,"message":"invalid body"}]}`}
	obs := newLangfuseTestObserver(t, fake)
	obs.Event("tick", nil)
	if err := obs.FlushLangfuse(); err == nil {
		t.Fatal("expected an error for rejected events")
	}

	fake.status = http.StatusUnauthorized
	fake.response = `{"message":"bad key"}`
	obs.Event("tick", nil)
	if err := obs.FlushLangfuse(); err == nil {
		t.Fatal("expected an error for HTTP 401")
	}
}

func TestLangfuse_DisabledWithoutKeys(t *testing.T) {
	if newLangfuseExporter(ObservabilityConfig{LangfuseEnabled: true, LangfusePublicKey: "pk"}) != nil {
		t.Error("exporter should be nil without a secret key")
	}
	if newLangfuseExporter(ObservabilityConfig{LangfusePublicKey: "pk", LangfuseSecretKey: "sk"}) != nil {
		t.Error("exporter should be nil when langfuse is not enabled")
	}
	exp := newLangfuseExporter(ObservabilityConfig{LangfuseEnabled: true, LangfusePublicKey: "pk", LangfuseSecretKey: "sk"})
	if exp == nil || exp.host != defaultLangfuseHost {
		t.Errorf("default host = %v", exp)
	}
	if err := NewNoopObserver().FlushLangfuse(); err != nil {
		t.Errorf("noop flush: %v", err)
	}
}
