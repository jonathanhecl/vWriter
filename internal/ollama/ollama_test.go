package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestNewClientValidation(t *testing.T) {
	t.Run("empty uses default", func(t *testing.T) {
		client, err := NewClient("  ")
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if client.BaseURL() != DefaultURL {
			t.Fatalf("BaseURL = %q, want %q", client.BaseURL(), DefaultURL)
		}
	})
	t.Run("trailing slash trimmed", func(t *testing.T) {
		client, err := NewClient("http://192.168.1.50:11434/")
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if client.BaseURL() != "http://192.168.1.50:11434" {
			t.Fatalf("BaseURL = %q", client.BaseURL())
		}
	})
	t.Run("remote host accepted", func(t *testing.T) {
		if _, err := NewClient("https://ollama.example.com:8443"); err != nil {
			t.Fatalf("NewClient: %v", err)
		}
	})
	for _, raw := range []string{
		"not-a-url", "ftp://localhost:11434", "http://user:pass@localhost:11434",
		"http://localhost:11434?x=1", "http://localhost:11434#frag",
	} {
		t.Run("rejected "+raw, func(t *testing.T) {
			_, err := NewClient(raw)
			var oerr *Error
			if !errors.As(err, &oerr) || oerr.Code != "INVALID_OLLAMA_URL" {
				t.Fatalf("err = %v, want INVALID_OLLAMA_URL", err)
			}
		})
	}
}

func TestVersionTagsPs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"version": "0.32.6"})
	})
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{
			{"name": "gemma3:12b", "size": 8_000_000_000, "details": map[string]string{"family": "gemma3"}},
			{"name": "qwen3:8b", "size": 5_000_000_000},
		}})
	})
	mux.HandleFunc("/api/ps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{
			{"name": "gemma3:12b", "size_vram": 9_000_000_000},
		}})
	})
	client := newTestClient(t, mux)
	ctx := context.Background()

	version, err := client.Version(ctx)
	if err != nil || version != "0.32.6" {
		t.Fatalf("Version = %q, %v", version, err)
	}
	models, err := client.Tags(ctx)
	if err != nil || len(models) != 2 || models[0].Name != "gemma3:12b" {
		t.Fatalf("Tags = %+v, %v", models, err)
	}
	running, err := client.Ps(ctx)
	if err != nil || len(running) != 1 || running[0].SizeVRAM != 9_000_000_000 {
		t.Fatalf("Ps = %+v, %v", running, err)
	}
}

func TestShowVisionDetection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		switch payload["model"] {
		case "confirmed":
			json.NewEncoder(w).Encode(map[string]any{"capabilities": []string{"completion", "vision"}})
		case "inferred":
			json.NewEncoder(w).Encode(map[string]any{
				"projector_info": map[string]any{"clip.has_vision_encoder": true},
			})
		case "inferred-info":
			json.NewEncoder(w).Encode(map[string]any{
				"model_info": map[string]any{"gemma3.vision.scale": 1},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{"capabilities": []string{"completion"}})
		}
	})
	client := newTestClient(t, mux)
	ctx := context.Background()

	for _, model := range []string{"confirmed", "inferred", "inferred-info"} {
		info, err := client.RequireVision(ctx, model)
		if err != nil {
			t.Fatalf("RequireVision(%q): %v", model, err)
		}
		if !info.HasVision() {
			t.Fatalf("HasVision(%q) = false", model)
		}
	}
	_, err := client.RequireVision(ctx, "text-only")
	var oerr *Error
	if !errors.As(err, &oerr) || oerr.Code != "VISION_UNAVAILABLE" {
		t.Fatalf("err = %v, want VISION_UNAVAILABLE", err)
	}
}

func TestStatusErrorMapping(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "model 'nope' not found"})
	})
	client := newTestClient(t, mux)

	_, err := client.Show(context.Background(), "nope")
	var oerr *Error
	if !errors.As(err, &oerr) || oerr.Code != "MODEL_NOT_FOUND" {
		t.Fatalf("err = %v, want MODEL_NOT_FOUND", err)
	}
	if !strings.Contains(oerr.Message, "not found") {
		t.Fatalf("Message = %q", oerr.Message)
	}
}

func TestUnavailableServer(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Tags(context.Background())
	var oerr *Error
	if !errors.As(err, &oerr) || oerr.Code != "OLLAMA_UNAVAILABLE" {
		t.Fatalf("err = %v, want OLLAMA_UNAVAILABLE", err)
	}
}

func TestChatStreaming(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		var payload ChatRequest
		json.NewDecoder(r.Body).Decode(&payload)
		if !payload.Stream {
			t.Error("Stream must be forced true")
		}
		if payload.Model != "gemma3:12b" || len(payload.Messages) != 2 {
			t.Errorf("unexpected payload: %+v", payload)
		}
		if payload.KeepAlive != "0" {
			t.Errorf("KeepAlive = %q, want 0", payload.KeepAlive)
		}
		chunks := []string{
			`{"message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"message":{"role":"assistant","content":" world"},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":42,"eval_count":2,"total_duration":1000000,"eval_duration":500000}`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
		}
	})
	client := newTestClient(t, mux)

	var seen int
	result, err := client.Chat(context.Background(), ChatRequest{
		Model: "gemma3:12b",
		Messages: []ChatMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi", Images: []string{"aGk="}},
		},
		KeepAlive: "0",
	}, func(chunk ChatChunk) error {
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result.Content != "Hello world" || result.DoneReason != "stop" {
		t.Fatalf("result = %+v", result)
	}
	if result.PromptEvalCount != 42 || result.EvalCount != 2 {
		t.Fatalf("metrics = %+v", result)
	}
	if seen != 3 {
		t.Fatalf("onChunk calls = %d, want 3", seen)
	}
}

func TestChatModelNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "model 'gone' not found"})
	})
	client := newTestClient(t, mux)
	_, err := client.Chat(context.Background(), ChatRequest{Model: "gone"}, nil)
	var oerr *Error
	if !errors.As(err, &oerr) || oerr.Code != "MODEL_NOT_FOUND" {
		t.Fatalf("err = %v, want MODEL_NOT_FOUND", err)
	}
}

func TestChatCancellation(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() {
		close(block)
		server.Close()
	})
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	_, err = client.Chat(ctx, ChatRequest{Model: "any"}, nil)
	var oerr *Error
	if !errors.As(err, &oerr) || oerr.Code != "GENERATION_CANCELLED" {
		t.Fatalf("err = %v, want GENERATION_CANCELLED", err)
	}
}

func TestUnload(t *testing.T) {
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	client := newTestClient(t, mux)
	if err := client.Unload(context.Background(), "gemma3:12b"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if got["model"] != "gemma3:12b" || got["keep_alive"] != "0" {
		t.Fatalf("payload = %+v", got)
	}
}
