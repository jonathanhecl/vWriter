package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// ChatMessage is one message in a /api/chat request. Images holds raw
// base64-encoded pictures attached to the message, in display order.
type ChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// ChatOptions maps to Ollama's per-request inference options.
type ChatOptions struct {
	NumCtx      int      `json:"num_ctx,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	TopK        *int     `json:"top_k,omitempty"`
	Seed        *int     `json:"seed,omitempty"`
}

// ChatRequest is the payload for POST /api/chat. KeepAlive accepts Ollama
// durations ("10m"), "0" to unload immediately, or a negative value to keep
// the model loaded indefinitely. Think enables thinking on supporting models.
type ChatRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	Think     *bool         `json:"think,omitempty"`
	KeepAlive string        `json:"keep_alive,omitempty"`
	Options   *ChatOptions  `json:"options,omitempty"`
}

// ChatChunk is one NDJSON line of a streaming chat response. The final chunk
// has Done set and carries the usage metrics.
type ChatChunk struct {
	Message struct {
		Role     string `json:"role"`
		Content  string `json:"content"`
		Thinking string `json:"thinking"`
	} `json:"message"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	TotalDuration   int64  `json:"total_duration"`
	EvalDuration    int64  `json:"eval_duration"`
}

// ChatResult is the accumulated outcome of a completed chat request.
type ChatResult struct {
	Content         string
	Thinking        string
	DoneReason      string
	PromptEvalCount int
	EvalCount       int
	TotalDuration   int64
	EvalDuration    int64
}

// Chat streams a chat completion. onChunk is invoked for every streamed
// chunk (it may be nil); returning an error from onChunk aborts the request.
// Cancellation of ctx interrupts the stream immediately.
func (c *Client) Chat(ctx context.Context, request ChatRequest, onChunk func(ChatChunk) error) (*ChatResult, error) {
	request.Stream = true
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/x-ndjson")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.transportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, c.statusError(resp.StatusCode, raw)
	}
	return c.consumeStream(resp, onChunk)
}

func (c *Client) consumeStream(resp *http.Response, onChunk func(ChatChunk) error) (*ChatResult, error) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	result := &ChatResult{}
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var chunk ChatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		result.Content += chunk.Message.Content
		result.Thinking += chunk.Message.Thinking
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return nil, err
			}
		}
		if chunk.Done {
			result.DoneReason = chunk.DoneReason
			result.PromptEvalCount = chunk.PromptEvalCount
			result.EvalCount = chunk.EvalCount
			result.TotalDuration = chunk.TotalDuration
			result.EvalDuration = chunk.EvalDuration
			return result, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, c.transportError(err)
	}
	return nil, &Error{
		Code:    "OLLAMA_INVALID_RESPONSE",
		Message: "The Ollama stream ended before the final chunk.",
		Details: map[string]any{"url": c.baseURL},
	}
}
