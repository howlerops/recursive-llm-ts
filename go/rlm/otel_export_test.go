package rlm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestOtlpTracesURL(t *testing.T) {
	cases := []struct {
		in, want string
		warn     bool
	}{
		{"http://localhost:4318", "http://localhost:4318/v1/traces", false},
		{"localhost:4318", "http://localhost:4318/v1/traces", false},
		{"https://collector.example.com/", "https://collector.example.com/v1/traces", false},
		{"https://cloud.langfuse.com/api/public/otel", "https://cloud.langfuse.com/api/public/otel/v1/traces", false},
		{"http://localhost:4318/v1/traces", "http://localhost:4318/v1/traces", false},
		{"localhost:4317", "http://localhost:4317/v1/traces", true},
	}
	for _, c := range cases {
		got, warning := otlpTracesURL(c.in)
		if got != c.want || (warning != "") != c.warn {
			t.Errorf("otlpTracesURL(%q) = %q, %q; want %q, warn=%v", c.in, got, warning, c.want, c.warn)
		}
	}
}

// captureStdout runs fn and returns everything written to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stdout = orig }()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}

func TestTracing_NeverWritesSpansToStdout(t *testing.T) {
	out := captureStdout(t, func() {
		obs := NewObserver(ObservabilityConfig{TraceEnabled: true, Debug: true})
		ctx := obs.StartTrace("rlm.completion", map[string]string{"model": "m"})
		obs.LLMCall("m", 1, 10, time.Millisecond, nil)
		obs.EndTrace(ctx)
		obs.Shutdown()
	})
	if out != "" {
		t.Fatalf("expected nothing on stdout (it carries the CLI response), got %d bytes:\n%s", len(out), out)
	}
}

func TestTracing_ExportsToOTLPEndpoint(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/traces" && r.Method == http.MethodPost {
			atomic.AddInt32(&hits, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	obs := NewObserver(ObservabilityConfig{TraceEnabled: true, TraceEndpoint: server.URL, ServiceName: "rlm-test"})
	ctx := obs.StartTrace("rlm.completion", nil)
	obs.LLMCall("m", 1, 10, time.Millisecond, nil)
	obs.EndTrace(ctx)
	obs.Shutdown()

	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("expected spans to be posted to /v1/traces")
	}
}

func TestSubAgentShutdownLeavesSharedObserverRunning(t *testing.T) {
	fake := &fakeLangfuse{}
	obs := newLangfuseTestObserver(t, fake)

	parent := New("m", Config{})
	parent.observer = obs
	child := New("m", Config{})
	child.useSharedObserver(obs)

	obs.Event("before", nil)
	child.Shutdown()
	if len(fake.requests) != 0 {
		t.Fatalf("sub-agent shutdown flushed the parent's observer (%d requests)", len(fake.requests))
	}

	parent.Shutdown()
	if len(fake.requests) != 1 {
		t.Fatalf("parent shutdown should flush once, got %d requests", len(fake.requests))
	}
}
