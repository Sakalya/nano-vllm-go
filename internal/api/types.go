package api

// OpenAI-compatible types

type CompletionRequest struct {
	Model       string `json:"model"`
	Prompt      any    `json:"prompt"` // string or []string
	MaxTokens   int    `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	N           int    `json:"n"`
	Stream      bool   `json:"stream"`
}

type CompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	N           int           `json:"n"`
	Stream      bool          `json:"stream"`
}

type ChatCompletionChoice struct {
	Message      ChatMessage `json:"message"`
	Index        int         `json:"index"`
	FinishReason string      `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Internal wire format between Go proxy and Python backend
type BackendRequest struct {
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type BackendResponse struct {
	Text     string `json:"text"`
	TokenIDs []int  `json:"token_ids"`
	Usage    Usage  `json:"usage"`
}
