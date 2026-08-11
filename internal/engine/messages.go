package engine

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jonathanhecl/vWriter/internal/media"
	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

const (
	samplingTemperature = 1.0
	samplingTopP        = 0.95
	samplingTopK        = 64
)

// buildChatRequest converts an assembled request into the Ollama chat call:
// one system message, one user message whose content starts with the media
// binding texts, and the base64 images in the same order.
func buildChatRequest(model string, assembled *prompt.Assembled, plan *prompt.Plan, thinking bool, seed *int) (ollama.ChatRequest, error) {
	var bindings []string
	var images []string
	inputs := append([]prompt.MediaInput(nil), assembled.MediaInputs...)
	sort.SliceStable(inputs, func(i, j int) bool {
		return inputs[i].Type == "image" && inputs[j].Type != "image"
	})
	for _, input := range inputs {
		path := input.ImagePath
		binding := fmt.Sprintf("%s: image reference.", input.Reference)
		switch input.Boundary {
		case media.RoleFirstFrame:
			binding = fmt.Sprintf(
				"%s MANDATORY: the output sequence MUST start with this exact image as its first frame; "+
					"everything in the story develops forward from it.",
				binding)
		case media.RoleLastFrame:
			binding = fmt.Sprintf(
				"%s MANDATORY: the output sequence MUST end with this exact image as its last frame; "+
					"everything in the story converges to it.",
				binding)
		}
		if input.Type == "video" {
			path = input.SheetPath
			binding = fmt.Sprintf(
				"%[1]s: one ordered contact sheet sampled from this same video. "+
					"Read frames left-to-right, then top-to-bottom, using the displayed order and the accompanying manifest timestamps to infer motion. "+
					"This sheet is only the internal visual representation of %[1]s; it is not a <Picture N> "+
					"and must never change or renumber the external reference labels.",
				input.Reference)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return ollama.ChatRequest{}, &Error{
				Code:    "MEDIA_PREPARATION_FAILED",
				Message: fmt.Sprintf("The prepared visual for %s is missing.", input.Reference),
				Details: err.Error(),
			}
		}
		bindings = append(bindings, binding)
		images = append(images, base64.StdEncoding.EncodeToString(raw))
	}

	var systemParts []string
	for _, message := range assembled.Messages {
		if message.Role == "system" {
			systemParts = append(systemParts, message.Content)
		}
	}
	userText := userMessageContent(assembled)
	if len(bindings) > 0 {
		userText = strings.Join(bindings, "\n") + "\n\n" + userText
	}
	return ollama.ChatRequest{
		Model: model,
		Messages: []ollama.ChatMessage{
			{Role: "system", Content: strings.Join(systemParts, "\n\n")},
			{Role: "user", Content: userText, Images: images},
		},
		Think:   &thinking,
		Options: chatOptions(plan, seed),
	}, nil
}

// chatOptions maps the context plan and sampling defaults to Ollama options.
func chatOptions(plan *prompt.Plan, seed *int) *ollama.ChatOptions {
	temperature := samplingTemperature
	topP := samplingTopP
	topK := samplingTopK
	return &ollama.ChatOptions{
		NumCtx:      plan.ContextTokens,
		NumPredict:  plan.MaxOutputTokens,
		Temperature: &temperature,
		TopP:        &topP,
		TopK:        &topK,
		Seed:        seed,
	}
}

// toOllamaMessages converts prompt messages, optionally attaching images to
// the last user message.
func toOllamaMessages(messages []prompt.Message, images []string) []ollama.ChatMessage {
	out := make([]ollama.ChatMessage, len(messages))
	for index, message := range messages {
		out[index] = ollama.ChatMessage{Role: message.Role, Content: message.Content}
		if message.Role == "user" && len(images) > 0 {
			out[index].Images = images
		}
	}
	return out
}
