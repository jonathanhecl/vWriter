package ollama

import (
	"context"
	"net/http"
	"strings"
)

// ShowResult is the subset of POST /api/show used for capability detection.
type ShowResult struct {
	Capabilities  []string       `json:"capabilities"`
	ModelInfo     map[string]any `json:"model_info"`
	ProjectorInfo map[string]any `json:"projector_info"`
}

// HasVision reports whether the model can accept image inputs. The result is
// confirmed when the server lists the "vision" capability and inferred from
// projector metadata for servers that do not report capabilities yet.
func (s *ShowResult) HasVision() bool {
	for _, capability := range s.Capabilities {
		if strings.EqualFold(capability, "vision") {
			return true
		}
	}
	if enabled, ok := s.ProjectorInfo["clip.has_vision_encoder"].(bool); ok && enabled {
		return true
	}
	for key := range s.ModelInfo {
		if strings.Contains(strings.ToLower(key), "vision") {
			return true
		}
	}
	return false
}

// Show fetches model metadata and capabilities.
func (c *Client) Show(ctx context.Context, model string) (*ShowResult, error) {
	ctx, cancel := probeCtx(ctx)
	defer cancel()
	var out ShowResult
	payload := map[string]any{"model": model}
	if err := c.do(ctx, http.MethodPost, "/api/show", payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RequireVision returns the model metadata or a VISION_UNAVAILABLE error when
// the model cannot analyze images.
func (c *Client) RequireVision(ctx context.Context, model string) (*ShowResult, error) {
	info, err := c.Show(ctx, model)
	if err != nil {
		return nil, err
	}
	if !info.HasVision() {
		return nil, &Error{
			Code:    "VISION_UNAVAILABLE",
			Message: "The selected model does not support image inputs. Choose a vision-capable model.",
			Details: map[string]any{"model": model},
		}
	}
	return info, nil
}
