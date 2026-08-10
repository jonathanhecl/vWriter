// Package ollama implements a minimal client for the Ollama REST API.
package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultURL is the Ollama endpoint used when the user has not configured one.
const DefaultURL = "http://127.0.0.1:11434"

// probeTimeout bounds discovery calls (version, tags, show, ps). Chat
// streaming is not time-bounded; it is controlled by context cancellation.
const probeTimeout = 5 * time.Second

// Error is a stable, user-facing failure with a machine-readable code.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Client talks to a single Ollama server, local or remote.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient validates and normalizes an Ollama base URL. Any reachable
// http(s) host is accepted; there is no localhost restriction.
func NewClient(rawURL string) (*Client, error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		raw = DefaultURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, &Error{
			Code:    "INVALID_OLLAMA_URL",
			Message: "Enter a valid Ollama URL such as http://127.0.0.1:11434.",
		}
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, &Error{
			Code:    "INVALID_OLLAMA_URL",
			Message: "The Ollama URL cannot contain credentials, a query, or a fragment.",
		}
	}
	base := strings.TrimRight(parsed.String(), "/")
	return &Client{baseURL: base, http: &http.Client{}}, nil
}

// BaseURL returns the normalized server root, for example http://127.0.0.1:11434.
func (c *Client) BaseURL() string { return c.baseURL }

// do performs one JSON request and decodes the response body into out.
// out may be nil for endpoints whose body is irrelevant.
func (c *Client) do(ctx context.Context, method, path string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return c.transportError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return c.transportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.statusError(resp.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &Error{
			Code:    "OLLAMA_INVALID_RESPONSE",
			Message: "The Ollama server returned an unexpected response.",
			Details: map[string]any{"url": c.baseURL, "status": resp.StatusCode},
		}
	}
	return nil
}

// transportError maps connection failures and cancellation to stable codes.
func (c *Client) transportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return &Error{Code: "GENERATION_CANCELLED", Message: "The request was cancelled."}
	}
	return &Error{
		Code:    "OLLAMA_UNAVAILABLE",
		Message: "Could not reach the Ollama server.",
		Details: map[string]any{"url": c.baseURL, "reason": err.Error()},
	}
}

// statusError maps a non-2xx Ollama response to a stable code.
func (c *Client) statusError(status int, raw []byte) error {
	message := fmt.Sprintf("The Ollama server returned HTTP %d.", status)
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
	code := "OLLAMA_ERROR"
	if status == http.StatusNotFound {
		code = "MODEL_NOT_FOUND"
	}
	return &Error{
		Code:    code,
		Message: message,
		Details: map[string]any{"url": c.baseURL, "status": status},
	}
}

// probeCtx applies the short discovery timeout to a caller context.
func probeCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, probeTimeout)
}

// Version reports the Ollama runtime version, e.g. "0.32.6".
func (c *Client) Version(ctx context.Context) (string, error) {
	ctx, cancel := probeCtx(ctx)
	defer cancel()
	var out struct {
		Version string `json:"version"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/version", nil, &out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// Model is one installed model entry from GET /api/tags.
type Model struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ModifiedAt string `json:"modified_at"`
	Details    struct {
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

// Tags lists the models installed on the server.
func (c *Client) Tags(ctx context.Context) ([]Model, error) {
	ctx, cancel := probeCtx(ctx)
	defer cancel()
	var out struct {
		Models []Model `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// RunningModel is one loaded model entry from GET /api/ps.
type RunningModel struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Size      int64  `json:"size"`
	SizeVRAM  int64  `json:"size_vram"`
	ExpiresAt string `json:"expires_at"`
}

// Ps lists the models currently loaded in memory.
func (c *Client) Ps(ctx context.Context) ([]RunningModel, error) {
	ctx, cancel := probeCtx(ctx)
	defer cancel()
	var out struct {
		Models []RunningModel `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/ps", nil, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// Unload asks the server to evict a model from memory immediately.
func (c *Client) Unload(ctx context.Context, model string) error {
	ctx, cancel := probeCtx(ctx)
	defer cancel()
	payload := map[string]any{"model": model, "keep_alive": "0"}
	return c.do(ctx, http.MethodPost, "/api/generate", payload, nil)
}
