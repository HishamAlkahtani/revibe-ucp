package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type modelResponse struct {
	Output            []json.RawMessage `json:"output"`
	Error             *providerError    `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

type providerError struct {
	Code    string `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *providerError) Error() string {
	code := e.Code
	if code == "" {
		code = e.Type
	}
	if code == "" {
		code = "unknown_error"
	}
	return fmt.Sprintf("OpenAI %s: %s", code, e.Message)
}

type Emit func(event string, data any) error

// Read complete SSE events, not HTTP chunks. Scanner retains split UTF-8 bytes.
func readSSE(reader io.Reader, consume func([]byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var lines []string
	dispatch := func() error {
		if len(lines) == 0 {
			return nil
		}
		data := strings.Join(lines, "\n")
		lines = nil
		return consume([]byte(data))
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func (a *API) stream(ctx context.Context, endpoint, key string, body any, emit Emit) (result modelResponse, err error) {
	started := time.Now()
	logger := requestLog(ctx)
	status := 0
	defer func() {
		attrs := []any{"http_status", status, "duration", time.Since(started).Round(time.Millisecond)}
		if err != nil {
			var provider *providerError
			if errors.As(err, &provider) {
				attrs = append(attrs, "code", provider.Code, "type", provider.Type)
			} else {
				attrs = append(attrs, "error", err.Error())
			}
			logger.Error("OpenAI stream failed", attrs...)
		} else {
			logger.Info("OpenAI stream completed", attrs...)
		}
	}()
	var completed *modelResponse
	encoded, err := json.Marshal(body)
	if err != nil {
		return modelResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(encoded))
	if err != nil {
		return modelResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+key)
	response, err := a.client.Do(req)
	if err != nil {
		return modelResponse{}, err
	}
	defer response.Body.Close()
	status = response.StatusCode
	if response.StatusCode != 200 {
		var envelope struct {
			Error *providerError `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&envelope) == nil && envelope.Error != nil {
			envelope.Error.Message = strings.ReplaceAll(envelope.Error.Message, key, "[redacted]")
			return modelResponse{}, envelope.Error
		}
		return modelResponse{}, fmt.Errorf("AI returned HTTP %d; check credentials, model access and quota", response.StatusCode)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return modelResponse{}, errors.New("AI did not return an event stream")
	}
	err = readSSE(response.Body, func(data []byte) error {
		if string(data) == "[DONE]" {
			return nil
		}
		var event struct {
			Type     string         `json:"type"`
			Delta    string         `json:"delta"`
			Response modelResponse  `json:"response"`
			Error    *providerError `json:"error"`
			Code     string         `json:"code"`
			Message  string         `json:"message"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		switch event.Type {
		case "response.output_text.delta", "response.refusal.delta":
			return emit("text_delta", map[string]string{"text": event.Delta})
		case "response.completed":
			completed = &event.Response
		case "response.failed", "error":
			provider := event.Response.Error
			if provider == nil {
				provider = event.Error
			}
			if provider == nil {
				provider = &providerError{Code: event.Code, Message: event.Message}
			}
			if provider.Message == "" {
				provider.Message = "The provider failed to generate a response."
			}
			provider.Message = strings.ReplaceAll(provider.Message, key, "[redacted]")
			return provider
		case "response.incomplete":
			reason := "unknown"
			if event.Response.IncompleteDetails != nil {
				reason = event.Response.IncompleteDetails.Reason
			}
			if reason == "max_output_tokens" {
				return errors.New("OpenAI response incomplete: output token limit reached")
			}
			return fmt.Errorf("OpenAI response incomplete: %s", reason)
		}
		return nil
	})
	if err != nil {
		return modelResponse{}, err
	}
	if completed == nil {
		return modelResponse{}, errors.New("AI stream ended before completion")
	}
	return *completed, nil
}
