package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nano-vllm/go-serving/internal/backend"
	"github.com/nano-vllm/go-serving/internal/balancer"
)

type Handler struct {
	pool     *backend.Pool
	balancer balancer.Balancer
}

func NewHandler(pool *backend.Pool, bal balancer.Balancer) *Handler {
	return &Handler{pool: pool, balancer: bal}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /v1/models", h.models)
	mux.HandleFunc("POST /v1/completions", h.completions)
	mux.HandleFunc("POST /v1/chat/completions", h.chatCompletions)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	healthy := h.pool.HealthyBackends()
	code := http.StatusOK
	status := "ok"
	if len(healthy) == 0 {
		code = http.StatusServiceUnavailable
		status = "no healthy backends"
	}
	writeJSON(w, code, map[string]any{
		"status":           status,
		"healthy_backends": len(healthy),
		"total_backends":   len(h.pool.Backends()),
	})
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": "nano-vllm", "object": "model", "created": time.Now().Unix(), "owned_by": "nano-vllm"},
		},
	})
}

func (h *Handler) completions(w http.ResponseWriter, r *http.Request) {
	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	prompt := extractPrompt(req.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	applyCompletionDefaults(&req)

	bresp, err := h.forward(r.Context(), BackendRequest{
		Prompt:      prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		slog.Error("backend error", "err", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, CompletionResponse{
		ID:      fmt.Sprintf("cmpl-%d", time.Now().UnixNano()),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []CompletionChoice{{Text: bresp.Text, Index: 0, FinishReason: "stop"}},
		Usage:   bresp.Usage,
	})
}

func (h *Handler) chatCompletions(w http.ResponseWriter, r *http.Request) {
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages is required")
		return
	}
	applyChatDefaults(&req)

	bresp, err := h.forward(r.Context(), BackendRequest{
		Prompt:      chatMLPrompt(req.Messages),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		slog.Error("backend error", "err", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatCompletionChoice{{
			Message:      ChatMessage{Role: "assistant", Content: bresp.Text},
			Index:        0,
			FinishReason: "stop",
		}},
		Usage: bresp.Usage,
	})
}

func (h *Handler) forward(ctx context.Context, req BackendRequest) (*BackendResponse, error) {
	backends := h.pool.HealthyBackends()
	b := h.balancer.Pick(backends)
	if b == nil {
		return nil, fmt.Errorf("no healthy backends available")
	}

	b.ActiveConns.Add(1)
	defer b.ActiveConns.Add(-1)

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.URL+"/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.pool.Client().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend %s returned HTTP %d", b.URL, resp.StatusCode)
	}

	var bresp BackendResponse
	if err := json.NewDecoder(resp.Body).Decode(&bresp); err != nil {
		return nil, err
	}
	return &bresp, nil
}

// chatMLPrompt formats messages in ChatML format, which Qwen3 uses.
func chatMLPrompt(messages []ChatMessage) string {
	var buf bytes.Buffer
	for _, m := range messages {
		fmt.Fprintf(&buf, "<|im_start|>%s\n%s<|im_end|>\n", m.Role, m.Content)
	}
	buf.WriteString("<|im_start|>assistant\n")
	return buf.String()
}

func extractPrompt(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []any:
		if len(s) > 0 {
			if str, ok := s[0].(string); ok {
				return str
			}
		}
	}
	return ""
}

func applyCompletionDefaults(r *CompletionRequest) {
	if r.MaxTokens == 0 {
		r.MaxTokens = 256
	}
	if r.Temperature == 0 {
		r.Temperature = 1.0
	}
	if r.N == 0 {
		r.N = 1
	}
}

func applyChatDefaults(r *ChatCompletionRequest) {
	if r.MaxTokens == 0 {
		r.MaxTokens = 256
	}
	if r.Temperature == 0 {
		r.Temperature = 1.0
	}
	if r.N == 0 {
		r.N = 1
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]any{"message": msg, "type": "error", "code": code},
	})
}
