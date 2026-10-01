package rlm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityConfig configures tracing, logging, and observability.
type ObservabilityConfig struct {
	// Debug enables verbose debug logging of all internal operations
	Debug bool `json:"debug"`

	// TraceEnabled enables OpenTelemetry tracing
	TraceEnabled bool `json:"trace_enabled"`

	// TraceEndpoint is the OTLP/HTTP endpoint for trace export (e.g., "http://localhost:4318").
	// "/v1/traces" is appended. Without an endpoint, spans are pretty-printed to the log output.
	TraceEndpoint string `json:"trace_endpoint,omitempty"`

	// ServiceName is the service name for traces (default: "rlm")
	ServiceName string `json:"service_name,omitempty"`

	// LogOutput controls where debug logs are written ("stderr", "stdout", or a file path)
	LogOutput string `json:"log_output,omitempty"`

	// LangfuseEnabled sends traces to Langfuse (requires public and secret keys).
	// Events are batched and posted to {LangfuseHost}/api/public/ingestion when
	// the observer shuts down.
	LangfuseEnabled bool `json:"langfuse_enabled"`

	// LangfusePublicKey is the Langfuse public key
	LangfusePublicKey string `json:"langfuse_public_key,omitempty"`

	// LangfuseSecretKey is the Langfuse secret key
	LangfuseSecretKey string `json:"langfuse_secret_key,omitempty"`

	// LangfuseHost is the Langfuse API host (default: "https://cloud.langfuse.com")
	LangfuseHost string `json:"langfuse_host,omitempty"`

	// OnEvent is a callback for observability events (for custom integrations)
	OnEvent func(event ObservabilityEvent) `json:"-"`
}

// ObservabilityEvent represents a single observability event.
type ObservabilityEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Type       string            `json:"type"`       // "span_start", "span_end", "llm_call", "log", "error", "event"
	Name       string            `json:"name"`       // Span or event name
	Attributes map[string]string `json:"attributes"` // Key-value attributes
	Duration   time.Duration     `json:"duration,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
	ParentID   string            `json:"parent_id,omitempty"`
}

// Observer manages observability for an RLM instance.
type Observer struct {
	config   ObservabilityConfig
	tracer   trace.Tracer
	logger   *log.Logger
	events   []ObservabilityEvent
	mu       sync.Mutex
	provider *sdktrace.TracerProvider
	rootCtx  context.Context
	rootSpan trace.Span
	langfuse *langfuseExporter
}

// NewObserver creates a new Observer with the given configuration.
func NewObserver(config ObservabilityConfig) *Observer {
	obs := &Observer{
		config: config,
		events: make([]ObservabilityEvent, 0),
	}

	// Setup logger
	obs.setupLogger()

	// Setup OTEL tracer if enabled
	if config.TraceEnabled {
		obs.setupTracer()
	}

	// Setup Langfuse export if enabled
	if config.LangfuseEnabled {
		obs.langfuse = newLangfuseExporter(config)
		if obs.langfuse == nil {
			fmt.Fprintln(os.Stderr, "[RLM] langfuse_enabled is set but the public or secret key is missing; Langfuse export is disabled")
		}
	}

	return obs
}

// NewNoopObserver creates an observer that does nothing (for when observability is disabled).
func NewNoopObserver() *Observer {
	return &Observer{
		config: ObservabilityConfig{},
		events: make([]ObservabilityEvent, 0),
		logger: log.New(io.Discard, "", 0),
	}
}

func (o *Observer) setupLogger() {
	if !o.config.Debug {
		o.logger = log.New(io.Discard, "", 0)
		return
	}

	var output io.Writer
	switch o.config.LogOutput {
	case "stdout":
		output = os.Stdout
	case "", "stderr":
		output = os.Stderr
	default:
		f, err := os.OpenFile(o.config.LogOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			output = os.Stderr
		} else {
			output = f
		}
	}

	o.logger = log.New(output, "[RLM] ", log.LstdFlags|log.Lmicroseconds)
}

func (o *Observer) setupTracer() {
	exporter, err := o.newSpanExporter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[RLM] failed to create trace exporter: %v\n", err)
		return
	}

	serviceName := o.config.ServiceName
	if serviceName == "" {
		serviceName = "rlm"
	}

	o.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", serviceName))),
	)

	otel.SetTracerProvider(o.provider)
	o.tracer = o.provider.Tracer(serviceName)
}

// newSpanExporter returns an OTLP/HTTP exporter when an endpoint is configured,
// otherwise a pretty-printing exporter on the log output. Spans are never
// written to stdout, which carries the CLI's JSON response.
func (o *Observer) newSpanExporter() (sdktrace.SpanExporter, error) {
	if o.config.TraceEndpoint != "" {
		endpointURL, warning := otlpTracesURL(o.config.TraceEndpoint)
		if warning != "" {
			fmt.Fprintf(os.Stderr, "[RLM] %s\n", warning)
		}
		// Headers (e.g. auth) come from OTEL_EXPORTER_OTLP_HEADERS / OTEL_EXPORTER_OTLP_TRACES_HEADERS.
		return otlptracehttp.New(context.Background(), otlptracehttp.WithEndpointURL(endpointURL))
	}
	return stdouttrace.New(stdouttrace.WithPrettyPrint(), stdouttrace.WithWriter(o.traceWriter()))
}

// traceWriter is where the fallback span exporter writes: the debug log file
// if one is configured, otherwise stderr.
func (o *Observer) traceWriter() io.Writer {
	switch o.config.LogOutput {
	case "", "stderr", "stdout":
		return os.Stderr
	default:
		if f, err := os.OpenFile(o.config.LogOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			return f
		}
		return os.Stderr
	}
}

// otlpTracesURL normalizes a trace endpoint into a full OTLP/HTTP traces URL.
// Following OTEL_EXPORTER_OTLP_ENDPOINT semantics, "/v1/traces" is appended to
// the base URL; "host:port" without a scheme is treated as http://host:port.
func otlpTracesURL(endpoint string) (string, string) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint, ""
	}
	if !strings.HasSuffix(u.Path, "/v1/traces") {
		u.Path = strings.TrimRight(u.Path, "/") + "/v1/traces"
	}
	warning := ""
	if u.Port() == "4317" {
		warning = "trace_endpoint uses port 4317, which is usually OTLP/gRPC; RLM exports OTLP/HTTP (usually port 4318)"
	}
	return u.String(), warning
}

// StartTrace begins a new root trace for an RLM operation.
func (o *Observer) StartTrace(name string, attrs map[string]string) context.Context {
	if o.langfuse != nil {
		o.langfuse.observeStart(name, attrs)
	}
	if o.tracer == nil {
		o.rootCtx = context.Background()
		return o.rootCtx
	}

	otelAttrs := mapToAttributes(attrs)
	ctx, span := o.tracer.Start(context.Background(), name,
		trace.WithAttributes(otelAttrs...),
	)
	o.rootCtx = ctx
	o.rootSpan = span

	o.recordEvent(ObservabilityEvent{
		Timestamp:  time.Now(),
		Type:       "trace_start",
		Name:       name,
		Attributes: attrs,
		TraceID:    span.SpanContext().TraceID().String(),
		SpanID:     span.SpanContext().SpanID().String(),
	})

	return ctx
}

// EndTrace ends the root trace.
func (o *Observer) EndTrace(ctx context.Context) {
	if o.rootSpan != nil {
		o.rootSpan.End()
	}
	if o.provider != nil {
		_ = o.provider.ForceFlush(context.Background())
	}
}

// StartSpan begins a new child span.
func (o *Observer) StartSpan(name string, attrs map[string]string) context.Context {
	if o.langfuse != nil {
		o.langfuse.observeStart(name, attrs)
	}
	if o.tracer == nil {
		if o.rootCtx == nil {
			o.rootCtx = context.Background()
		}
		return o.rootCtx
	}

	parentCtx := o.rootCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	otelAttrs := mapToAttributes(attrs)
	ctx, span := o.tracer.Start(parentCtx, name,
		trace.WithAttributes(otelAttrs...),
	)

	o.recordEvent(ObservabilityEvent{
		Timestamp:  time.Now(),
		Type:       "span_start",
		Name:       name,
		Attributes: attrs,
		TraceID:    span.SpanContext().TraceID().String(),
		SpanID:     span.SpanContext().SpanID().String(),
	})

	return ctx
}

// EndSpan ends a child span.
func (o *Observer) EndSpan(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.End()
	}
}

// LLMCall records an LLM API call event.
func (o *Observer) LLMCall(model string, messageCount int, tokensUsed int, duration time.Duration, err error) {
	o.LLMCallWithUsage(model, messageCount, &TokenUsage{TotalTokens: tokensUsed}, duration, err)
}

// LLMCallWithUsage records an LLM API call event with a prompt/completion token breakdown.
func (o *Observer) LLMCallWithUsage(model string, messageCount int, usage *TokenUsage, duration time.Duration, err error) {
	if usage == nil {
		usage = &TokenUsage{}
	}
	attrs := map[string]string{
		"model":         model,
		"message_count": fmt.Sprintf("%d", messageCount),
		"tokens_used":   fmt.Sprintf("%d", usage.TotalTokens),
		"duration_ms":   fmt.Sprintf("%d", duration.Milliseconds()),
	}
	if usage.PromptTokens > 0 {
		attrs["prompt_tokens"] = fmt.Sprintf("%d", usage.PromptTokens)
	}
	if usage.CompletionTokens > 0 {
		attrs["completion_tokens"] = fmt.Sprintf("%d", usage.CompletionTokens)
	}
	if err != nil {
		attrs["error"] = err.Error()
	}

	o.Debug("llm_call", "model=%s messages=%d duration=%s", model, messageCount, duration)

	if o.tracer != nil && o.rootCtx != nil {
		_, span := o.tracer.Start(o.rootCtx, "llm.call",
			trace.WithAttributes(mapToAttributes(attrs)...),
		)
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}

	o.recordEvent(ObservabilityEvent{
		Timestamp:  time.Now(),
		Type:       "llm_call",
		Name:       fmt.Sprintf("llm.%s", model),
		Attributes: attrs,
		Duration:   duration,
	})
}

// Debug logs a debug message if debug mode is enabled.
func (o *Observer) Debug(component string, format string, args ...interface{}) {
	if !o.config.Debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	o.logger.Printf("[%s] %s", component, msg)
}

// Error logs an error message (always logged regardless of debug mode).
func (o *Observer) Error(component string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if o.config.Debug {
		o.logger.Printf("[ERROR][%s] %s", component, msg)
	}

	o.recordEvent(ObservabilityEvent{
		Timestamp: time.Now(),
		Type:      "error",
		Name:      component,
		Attributes: map[string]string{
			"message": msg,
		},
	})
}

// Event records a named event with attributes.
func (o *Observer) Event(name string, attrs map[string]string) {
	o.Debug("event", "%s: %v", name, attrs)

	if o.tracer != nil && o.rootCtx != nil {
		span := trace.SpanFromContext(o.rootCtx)
		if span != nil {
			span.AddEvent(name, trace.WithAttributes(mapToAttributes(attrs)...))
		}
	}

	o.recordEvent(ObservabilityEvent{
		Timestamp:  time.Now(),
		Type:       "event",
		Name:       name,
		Attributes: attrs,
	})
}

// GetEvents returns all recorded observability events.
func (o *Observer) GetEvents() []ObservabilityEvent {
	o.mu.Lock()
	defer o.mu.Unlock()
	events := make([]ObservabilityEvent, len(o.events))
	copy(events, o.events)
	return events
}

// GetEventsJSON returns all events as a JSON string.
func (o *Observer) GetEventsJSON() (string, error) {
	events := o.GetEvents()
	data, err := json.Marshal(events)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Shutdown gracefully shuts down the observer and flushes any pending data.
func (o *Observer) Shutdown() {
	if o.provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = o.provider.Shutdown(ctx)
	}
	if err := o.FlushLangfuse(); err != nil {
		// Export failures must not fail the completion; report them on stderr.
		fmt.Fprintf(os.Stderr, "[RLM] %v\n", err)
	}
}

// FlushLangfuse sends any buffered Langfuse events. It is a no-op when
// Langfuse export is disabled.
func (o *Observer) FlushLangfuse() error {
	if o.langfuse == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*langfuseRequestTimeout)
	defer cancel()
	return o.langfuse.Flush(ctx)
}

func (o *Observer) recordEvent(event ObservabilityEvent) {
	o.mu.Lock()
	o.events = append(o.events, event)
	o.mu.Unlock()

	// Call custom event handler if configured
	if o.config.OnEvent != nil {
		o.config.OnEvent(event)
	}

	// Buffer for Langfuse if enabled (sent on Shutdown)
	if o.langfuse != nil {
		o.langfuse.observe(event)
	}
}

// mapToAttributes converts a map to OTEL attributes.
func mapToAttributes(attrs map[string]string) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		result = append(result, attribute.String(k, v))
	}
	return result
}

// ObservabilityConfigFromMap parses observability config from a map.
func ObservabilityConfigFromMap(config map[string]interface{}) ObservabilityConfig {
	obs := ObservabilityConfig{}
	if config == nil {
		return obs
	}

	if v, ok := config["debug"].(bool); ok {
		obs.Debug = v
	}
	if v, ok := config["trace_enabled"].(bool); ok {
		obs.TraceEnabled = v
	}
	if v, ok := config["trace_endpoint"].(string); ok {
		obs.TraceEndpoint = v
	}
	if v, ok := config["service_name"].(string); ok {
		obs.ServiceName = v
	}
	if v, ok := config["log_output"].(string); ok {
		obs.LogOutput = v
	}
	if v, ok := config["langfuse_enabled"].(bool); ok {
		obs.LangfuseEnabled = v
	}
	if v, ok := config["langfuse_public_key"].(string); ok {
		obs.LangfusePublicKey = v
	}
	if v, ok := config["langfuse_secret_key"].(string); ok {
		obs.LangfuseSecretKey = v
	}
	if v, ok := config["langfuse_host"].(string); ok {
		obs.LangfuseHost = v
	}

	// Also check environment variables
	if os.Getenv("RLM_DEBUG") == "1" || os.Getenv("RLM_DEBUG") == "true" {
		obs.Debug = true
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		obs.TraceEnabled = true
		obs.TraceEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if os.Getenv("LANGFUSE_PUBLIC_KEY") != "" {
		obs.LangfuseEnabled = true
		obs.LangfusePublicKey = os.Getenv("LANGFUSE_PUBLIC_KEY")
	}
	if os.Getenv("LANGFUSE_SECRET_KEY") != "" {
		obs.LangfuseSecretKey = os.Getenv("LANGFUSE_SECRET_KEY")
	}
	if v := os.Getenv("LANGFUSE_HOST"); v != "" {
		obs.LangfuseHost = v
	}

	return obs
}

// FormatStatsWithObservability enriches RLMStats with observability data.
func FormatStatsWithObservability(stats RLMStats, obs *Observer) map[string]interface{} {
	result := map[string]interface{}{
		"llm_calls":  stats.LlmCalls,
		"iterations": stats.Iterations,
		"depth":      stats.Depth,
	}

	if stats.ParsingRetries > 0 {
		result["parsing_retries"] = stats.ParsingRetries
	}

	if stats.TotalTokens > 0 {
		result["total_tokens"] = stats.TotalTokens
		result["prompt_tokens"] = stats.PromptTokens
		result["completion_tokens"] = stats.CompletionTokens
	}

	if obs != nil && obs.config.Debug {
		events := obs.GetEvents()
		if len(events) > 0 {
			result["trace_events"] = events
		}
	}

	return result
}

// ExtractObservabilityConfig extracts observability config from the general config map.
func ExtractObservabilityConfig(config map[string]interface{}) map[string]interface{} {
	obsConfig := make(map[string]interface{})

	obsKeys := []string{
		"debug", "trace_enabled", "trace_endpoint", "service_name",
		"log_output", "langfuse_enabled", "langfuse_public_key",
		"langfuse_secret_key", "langfuse_host",
	}

	for _, key := range obsKeys {
		if v, ok := config[key]; ok {
			obsConfig[key] = v
		}
	}

	// Also check nested "observability" key
	if obsMap, ok := config["observability"].(map[string]interface{}); ok {
		for k, v := range obsMap {
			obsConfig[k] = v
		}
	}

	return obsConfig
}

// RedactSensitive removes sensitive data from attributes for logging.
func RedactSensitive(attrs map[string]string) map[string]string {
	redacted := make(map[string]string, len(attrs))
	sensitiveKeys := []string{"api_key", "secret", "password", "token", "authorization"}

	for k, v := range attrs {
		isRedacted := false
		keyLower := strings.ToLower(k)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(keyLower, sensitive) {
				redacted[k] = "[REDACTED]"
				isRedacted = true
				break
			}
		}
		if !isRedacted {
			redacted[k] = v
		}
	}
	return redacted
}
