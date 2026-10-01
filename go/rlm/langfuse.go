package rlm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultLangfuseHost      = "https://cloud.langfuse.com"
	langfuseIngestionPath    = "/api/public/ingestion"
	langfuseMaxBatchSize     = 100
	langfuseRequestTimeout   = 10 * time.Second
	langfuseTimestampLayout  = "2006-01-02T15:04:05.000Z07:00"
	langfuseTraceEventType   = "trace-create"
	langfuseGenerationType   = "generation-create"
	langfuseEventType        = "event-create"
	langfuseLevelError       = "ERROR"
	langfuseLevelDefault     = "DEFAULT"
	langfuseSDKName          = "recursive-llm-ts"
	langfuseDefaultTraceName = "rlm"
)

// langfuseIngestionEvent is one entry in a Langfuse ingestion batch.
type langfuseIngestionEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	Body      map[string]interface{} `json:"body"`
}

// langfuseExporter buffers observer events as Langfuse ingestion events and
// sends them in batches on Flush. One exporter produces one Langfuse trace:
// the first StartTrace/StartSpan names it, and LLM calls become generations.
type langfuseExporter struct {
	host      string
	publicKey string
	secretKey string
	client    *http.Client

	mu           sync.Mutex
	traceID      string
	traceCreated bool
	pending      []langfuseIngestionEvent
}

// newLangfuseExporter returns nil if Langfuse is disabled or keys are missing.
func newLangfuseExporter(config ObservabilityConfig) *langfuseExporter {
	if !config.LangfuseEnabled || config.LangfusePublicKey == "" || config.LangfuseSecretKey == "" {
		return nil
	}
	host := strings.TrimRight(config.LangfuseHost, "/")
	if host == "" {
		host = defaultLangfuseHost
	}
	return &langfuseExporter{
		host:      host,
		publicKey: config.LangfusePublicKey,
		secretKey: config.LangfuseSecretKey,
		client:    &http.Client{Timeout: langfuseRequestTimeout},
		traceID:   uuid.NewString(),
	}
}

func formatLangfuseTime(t time.Time) string {
	return t.UTC().Format(langfuseTimestampLayout)
}

func stringMetadata(attrs map[string]string) map[string]interface{} {
	redacted := RedactSensitive(attrs)
	metadata := make(map[string]interface{}, len(redacted))
	for k, v := range redacted {
		metadata[k] = v
	}
	return metadata
}

func (l *langfuseExporter) enqueueLocked(eventType string, ts time.Time, body map[string]interface{}) {
	l.pending = append(l.pending, langfuseIngestionEvent{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: formatLangfuseTime(ts),
		Body:      body,
	})
}

// ensureTraceLocked emits the trace-create event the first time it is called.
func (l *langfuseExporter) ensureTraceLocked(name string, ts time.Time, attrs map[string]string) {
	if l.traceCreated {
		return
	}
	l.traceCreated = true
	if name == "" {
		name = langfuseDefaultTraceName
	}
	l.enqueueLocked(langfuseTraceEventType, ts, map[string]interface{}{
		"id":        l.traceID,
		"name":      name,
		"timestamp": formatLangfuseTime(ts),
		"metadata":  stringMetadata(attrs),
	})
}

// observeStart handles StartTrace/StartSpan. The first call creates the trace;
// later calls are recorded as point-in-time events on it.
func (l *langfuseExporter) observeStart(name string, attrs map[string]string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.traceCreated {
		l.ensureTraceLocked(name, now, attrs)
		return
	}
	l.enqueueLocked(langfuseEventType, now, map[string]interface{}{
		"id":        uuid.NewString(),
		"traceId":   l.traceID,
		"name":      name,
		"startTime": formatLangfuseTime(now),
		"metadata":  stringMetadata(attrs),
	})
}

// observe converts a recorded ObservabilityEvent into Langfuse events.
func (l *langfuseExporter) observe(event ObservabilityEvent) {
	switch event.Type {
	case "llm_call", "error", "event":
	default:
		// trace_start/span_start are handled by observeStart
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensureTraceLocked("", event.Timestamp, nil)

	switch event.Type {
	case "llm_call":
		l.enqueueLocked(langfuseGenerationType, event.Timestamp, l.generationBody(event))
	case "error":
		l.enqueueLocked(langfuseEventType, event.Timestamp, map[string]interface{}{
			"id":            uuid.NewString(),
			"traceId":       l.traceID,
			"name":          event.Name,
			"startTime":     formatLangfuseTime(event.Timestamp),
			"level":         langfuseLevelError,
			"statusMessage": event.Attributes["message"],
			"metadata":      stringMetadata(event.Attributes),
		})
	case "event":
		l.enqueueLocked(langfuseEventType, event.Timestamp, map[string]interface{}{
			"id":        uuid.NewString(),
			"traceId":   l.traceID,
			"name":      event.Name,
			"startTime": formatLangfuseTime(event.Timestamp),
			"metadata":  stringMetadata(event.Attributes),
		})
	}
}

func (l *langfuseExporter) generationBody(event ObservabilityEvent) map[string]interface{} {
	attrs := event.Attributes
	start := event.Timestamp.Add(-event.Duration)
	body := map[string]interface{}{
		"id":        uuid.NewString(),
		"traceId":   l.traceID,
		"name":      "llm.call",
		"model":     attrs["model"],
		"startTime": formatLangfuseTime(start),
		"endTime":   formatLangfuseTime(event.Timestamp),
		"level":     langfuseLevelDefault,
		"metadata":  stringMetadata(attrs),
	}

	usage := map[string]interface{}{"unit": "TOKENS"}
	for attr, key := range map[string]string{
		"prompt_tokens":     "input",
		"completion_tokens": "output",
		"tokens_used":       "total",
	} {
		if n, err := strconv.Atoi(attrs[attr]); err == nil && n > 0 {
			usage[key] = n
		}
	}
	if len(usage) > 1 {
		body["usage"] = usage
	}

	if msg, ok := attrs["error"]; ok && msg != "" {
		body["level"] = langfuseLevelError
		body["statusMessage"] = msg
	}
	return body
}

// Flush sends all pending events to Langfuse in batches. Pending events are
// drained even on failure so a later Flush does not resend them.
func (l *langfuseExporter) Flush(ctx context.Context) error {
	l.mu.Lock()
	events := l.pending
	l.pending = nil
	l.mu.Unlock()

	var errs []string
	for start := 0; start < len(events); start += langfuseMaxBatchSize {
		end := start + langfuseMaxBatchSize
		if end > len(events) {
			end = len(events)
		}
		if err := l.send(ctx, events[start:end]); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("langfuse ingestion failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (l *langfuseExporter) send(ctx context.Context, batch []langfuseIngestionEvent) error {
	payload, err := json.Marshal(map[string]interface{}{
		"batch":    batch,
		"metadata": map[string]string{"sdk_name": langfuseSDKName},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.host+langfuseIngestionPath, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(l.publicKey, l.secretKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Ingestion returns 207 with per-event errors when some events are rejected.
	var result struct {
		Errors []struct {
			ID      string `json:"id"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &result) == nil && len(result.Errors) > 0 {
		first := result.Errors[0]
		return fmt.Errorf("%d of %d events rejected (first: status %d %s)", len(result.Errors), len(batch), first.Status, first.Message)
	}
	return nil
}
