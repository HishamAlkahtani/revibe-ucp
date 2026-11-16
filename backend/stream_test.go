package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelStreamReportsProviderErrors(t *testing.T) {
	cases := []struct{ name, event, want string }{
		{"quota in error event", `{"type":"error","error":{"type":"insufficient_quota","code":"credit_balance_exhausted","message":"You have no credits remaining."}}`, "credit_balance_exhausted"},
		{"quota in failed response", `{"type":"response.failed","response":{"error":{"code":"credit_balance_exhausted","message":"You have no credits remaining."}}}`, "credit_balance_exhausted"},
		{"flat error event", `{"type":"error","code":"rate_limit_exceeded","message":"Too many requests."}`, "rate_limit_exceeded"},
		{"token limit", `{"type":"response.incomplete","response":{"incomplete_details":{"reason":"max_output_tokens"}}}`, "output token limit reached"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", tt.event)
			}))
			defer upstream.Close()
			api := &API{client: upstream.Client()}
			_, err := api.stream(context.Background(), upstream.URL, "test-key", nil, func(string, any) error { return nil })
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("wanted %q, got %v", tt.want, err)
			}
		})
	}
}

func TestModelHTTPErrorPreservesCodeAndRedactsKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"code":"invalid_api_key","message":"Invalid key: test-secret"}}`)
	}))
	defer upstream.Close()
	api := &API{client: upstream.Client()}
	_, err := api.stream(context.Background(), upstream.URL, "test-secret", nil, func(string, any) error { return nil })
	var provider *providerError
	if !errors.As(err, &provider) || provider.Code != "invalid_api_key" || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadSSE(t *testing.T) {
	input := ": heartbeat\r\n\r\nevent: custom\r\ndata: {\r\ndata: \"text\":\"مرحبا 🌍\"}\r\n\r\n"
	count := 0
	err := readSSE(strings.NewReader(input), func(data []byte) error {
		count++
		var value map[string]string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value["text"] != "مرحبا 🌍" {
			t.Fatal("UTF-8 data corrupted")
		}
		return nil
	})
	if err != nil || count != 1 {
		t.Fatalf("events=%d error=%v", count, err)
	}
}

func TestModelStreamDeliversBeforeCompletion(t *testing.T) {
	delivered := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["stream"] != true {
			t.Error("streaming was not requested")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-delivered:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n")
	}))
	defer upstream.Close()
	api := &API{client: upstream.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := api.stream(ctx, upstream.URL, "test", map[string]any{"stream": true}, func(event string, data any) error {
		if event != "text_delta" || data.(map[string]string)["text"] != "Hello" {
			t.Fatal("unexpected delta")
		}
		close(delivered)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModelStreamFailureAndCancellation(t *testing.T) {
	for _, event := range []string{
		`{"type":"response.output_text.delta","delta":"partial"}`,
		`{"type":"response.failed"}`,
		`{"type":"response.incomplete"}`,
		`{bad json}`,
	} {
		t.Run(event, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", event)
			}))
			defer upstream.Close()
			api := &API{client: upstream.Client()}
			_, err := api.stream(context.Background(), upstream.URL, "test", nil, func(string, any) error { return nil })
			if err == nil {
				t.Fatal("failed/truncated stream treated as success")
			}
		})
	}
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api := &API{client: upstream.Client()}
	_, err := api.stream(ctx, upstream.URL, "test", nil, func(string, any) error { cancel(); return ctx.Err() })
	if err == nil {
		t.Fatal("cancellation ignored")
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream was not canceled")
	}
}
