package prompt

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jonathanhecl/vWriter/guides"
	"github.com/jonathanhecl/vWriter/internal/media"
)

// AspectRatios are the accepted target aspect ratios.
var AspectRatios = map[string]bool{
	"1:1": true, "2:3": true, "3:2": true, "3:4": true,
	"4:3": true, "9:16": true, "16:9": true, "21:9": true,
}

const (
	maxBriefChars       = 2000
	maxPromptChars      = 20000
	maxInstructionChars = 2000
)

// Message is one chat message. Name labels system messages for debugging.
type Message struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// MediaInput is one visual input the model receives.
type MediaInput struct {
	AssetID   string        `json:"asset_id"`
	Reference string        `json:"reference"`
	Type      string        `json:"type"`               // "image" or "video"
	Boundary  string        `json:"boundary,omitempty"` // "" | "first_frame" | "last_frame" (derived from the asset role)
	Frames    []media.Frame `json:"frames,omitempty"`
	ImagePath string        `json:"image_path,omitempty"` // prepared image
	SheetPath string        `json:"sheet_path,omitempty"` // video contact sheet
}

// Assembled is a complete, validated generation request.
type Assembled struct {
	SchemaVersion int            `json:"schema_version"`
	Guide         guides.Guide   `json:"guide"`
	Supporting    *guides.Guide  `json:"supporting_guide,omitempty"`
	SystemPrompt  SystemPrompt   `json:"system_prompt"`
	Messages      []Message      `json:"messages"`
	MediaInputs   []MediaInput   `json:"media_inputs"`
	Input         AssembledInput `json:"input"`
}

// SystemPrompt records whether the user customized the system prompt.
type SystemPrompt struct {
	Custom  bool   `json:"custom"`
	Content string `json:"content"`
}

// AssembledInput echoes the validated request fields.
type AssembledInput struct {
	DurationSeconds float64        `json:"duration_seconds,omitempty"`
	AspectRatio     string         `json:"aspect_ratio,omitempty"`
	CreativeBrief   string         `json:"creative_brief,omitempty"`
	CurrentPrompt   string         `json:"current_prompt,omitempty"`
	Instruction     string         `json:"instruction,omitempty"`
	Manifest        media.Manifest `json:"media_manifest"`
}

// GenerateRequest is the input to AssembleRequest.
type GenerateRequest struct {
	Manifest             media.Manifest
	CreativeBrief        string
	DurationSeconds      float64
	AspectRatio          string
	SystemPromptOverride *string
}

// RefineRequest is the input to AssembleRefinement.
type RefineRequest struct {
	Manifest             media.Manifest
	CurrentPrompt        string
	Instruction          string
	CachedObservation    string
	SystemPromptOverride *string
}

var explicitEditPattern = regexp.MustCompile(`(?is)\b(?:edit(?:ing)?|continue|continuation|extend|remix|re-cut)\b.{0,40}\bvideo\b|\bvideo\s+editing\b`)

// finalContract is the closing instruction block of the user message. It
// pins the request classification and the hard output rules.
func finalContract(taskText string) string {
	taskClassification := "reference generation, not keyframe completion or source-video editing"
	if explicitEditPattern.MatchString(taskText) {
		taskClassification = "source-video editing or continuation; scale detailed_description with source complexity"
	}
	return fmt.Sprintf(
		"Final request classification: %s. "+
			"Treat every explicitly assigned reference role as exclusive unless the user asks that reference to contribute "+
			"additional traits; 'only' and 'solely' emphasize this rule but are not required. Unspecified target environment, lighting, "+
			"composition, camera treatment, and atmosphere may be designed as new target content, but never described as "+
			"facts derived from a reference. Do not add unsupported subject actions, dialogue, props, visible text, or an "+
			"invented ending. Music requested without an uploaded audio asset belongs only in non_diegetic_music and must "+
			"not create audio-reference or audio-reuse semantics. Prefer one continuous shot unless cuts are requested; "+
			"purposeful camera movement within that shot is allowed. Because H3 receives each source video itself, bind the "+
			"complete choreography, temporal order, pacing, and rhythmic character of a motion-only video without "+
			"reconstructing individual sampled gestures, named steps, poses, expressions, transitions, or a concluding move. "+
			"When a concrete visible object, character, scene, or effect from <Video N> is reused in the target, describe that "+
			"reused visual element through an appropriate <Subject N> while keeping <Video N> as its source provenance; do not "+
			"automatically create a separate subject for ordinary motion transfer. "+
			"If the brief does not explicitly request music, non_diegetic_music must be N/A. "+
			"Use the official detail budget for grounded target composition, placement, lighting, atmosphere, camera treatment, "+
			"supported action progression, and reference application; never pad solely to reach a word count. Return only the complete "+
			"prompt with all six required sections in the official order and no commentary outside the prompt.",
		taskClassification,
	)
}

// guideMessages builds the system messages: effective system prompt, official
// reference guide, and the shared base-guide rules excerpt. It also returns
// the metadata-only copies of both guides for the assembled record.
func guideMessages(systemPrompt string) (messages []Message, guideMeta, baseMeta *guides.Guide, err error) {
	guide, err := guides.Reference()
	if err != nil {
		return nil, nil, nil, &Error{Code: "GUIDE_LOAD_FAILED", Message: err.Error()}
	}
	base, err := guides.Base()
	if err != nil {
		return nil, nil, nil, &Error{Code: "GUIDE_LOAD_FAILED", Message: err.Error()}
	}
	excerpt, err := guides.ReferenceBaseExcerpt()
	if err != nil {
		return nil, nil, nil, &Error{Code: "GUIDE_LOAD_FAILED", Message: err.Error()}
	}
	messages = []Message{
		{Role: "system", Name: "prompt_writer_system_prompt", Content: systemPrompt},
		{Role: "system", Name: "official_minimax_h3_guide", Content: guide.Content},
		{Role: "system", Name: "official_minimax_h3_shared_base_rules", Content: excerpt},
	}
	guide.Content = ""
	base.Content = ""
	return messages, &guide, &base, nil
}

// declaredReferences splits the manifest into declared references (audio is
// always declared; visuals when analysis is requested) and eligible visual
// model inputs.
func declaredReferences(manifest media.Manifest) (declared []*media.Asset, inputs []MediaInput) {
	for _, asset := range manifest.Assets {
		if asset.Type != media.Audio && !asset.AnalysisRequested {
			continue
		}
		declared = append(declared, asset)
		if asset.Type == media.Audio {
			continue
		}
		input := MediaInput{
			AssetID:   asset.ID,
			Reference: asset.Reference,
			Type:      string(asset.Type),
			Frames:    asset.Frames,
		}
		if asset.Role == media.RoleFirstFrame || asset.Role == media.RoleLastFrame {
			input.Boundary = asset.Role
		}
		if asset.Type == media.Image {
			input.ImagePath = asset.PreparedPath
			if input.ImagePath == "" {
				input.ImagePath = asset.OriginalPath
			}
		} else {
			input.SheetPath = asset.ContactSheetPath
		}
		inputs = append(inputs, input)
	}
	return declared, inputs
}

func referenceManifestText(declared []*media.Asset) string {
	if len(declared) == 0 {
		return "None"
	}
	all := declared // declared is already the full relevant set
	lines := make([]string, len(declared))
	for index, asset := range declared {
		lines[index] = media.ReferenceLineWithLinked(asset, all)
	}
	return strings.Join(lines, "\n")
}

// AssembleRequest validates and assembles a fresh full-reference generation.
func AssembleRequest(req GenerateRequest) (*Assembled, error) {
	brief := trimSpace(req.CreativeBrief)
	if brief == "" {
		return nil, &Error{Code: "INVALID_REQUEST", Message: "Creative brief is required.", Details: map[string]string{"field": "creative_brief"}}
	}
	if len(brief) > maxBriefChars {
		return nil, &Error{Code: "BRIEF_TOO_LONG", Message: "Creative brief cannot exceed 2,000 characters."}
	}
	if !AspectRatios[req.AspectRatio] {
		return nil, &Error{Code: "INVALID_ASPECT_RATIO", Message: "The selected aspect ratio is not supported."}
	}
	if req.DurationSeconds <= 0 || req.DurationSeconds > 20 {
		return nil, &Error{Code: "INVALID_DURATION", Message: "Duration must be between 1 and 20 seconds."}
	}
	if !req.Manifest.Valid {
		return nil, &Error{Code: "INVALID_MEDIA_MANIFEST", Message: "The media manifest is not valid.", Details: req.Manifest.Violations}
	}
	systemPrompt, custom, err := ResolveSystemPrompt(req.SystemPromptOverride)
	if err != nil {
		return nil, err
	}

	declared, inputs := declaredReferences(req.Manifest)
	userContent := fmt.Sprintf(
		"Mode: Reference\nDuration: %g seconds\nAspect ratio: %s\n\n"+
			"Reference manifest (audio is not analyzed by the local model; derive its copy/reference role only from the user's words and do not invent its content):\n%s\n\n"+
			"Creative brief:\n%s\n\n%s",
		req.DurationSeconds, req.AspectRatio, referenceManifestText(declared), brief, finalContract(brief),
	)
	messages, guide, base, err := guideMessages(systemPrompt)
	if err != nil {
		return nil, err
	}
	return &Assembled{
		SchemaVersion: 1,
		Guide:         *guide,
		Supporting:    base,
		SystemPrompt:  SystemPrompt{Custom: custom, Content: systemPrompt},
		Messages:      append(messages, Message{Role: "user", Content: userContent}),
		MediaInputs:   inputs,
		Input: AssembledInput{
			DurationSeconds: req.DurationSeconds,
			AspectRatio:     req.AspectRatio,
			CreativeBrief:   brief,
			Manifest:        req.Manifest,
		},
	}, nil
}

// AssembleRefinement validates and assembles a text-only refinement pass.
// Media is intentionally not attached; the reference manifest stays textual.
func AssembleRefinement(req RefineRequest) (*Assembled, error) {
	current := trimSpace(req.CurrentPrompt)
	if current == "" {
		return nil, &Error{Code: "INVALID_REQUEST", Message: "Current prompt is required.", Details: map[string]string{"field": "current_prompt"}}
	}
	if len(current) > maxPromptChars {
		return nil, &Error{Code: "PROMPT_TOO_LONG", Message: "The current prompt cannot exceed 20,000 characters."}
	}
	instruction := trimSpace(req.Instruction)
	if instruction == "" {
		return nil, &Error{Code: "INVALID_REQUEST", Message: "Revision instruction is required.", Details: map[string]string{"field": "instruction"}}
	}
	if len(instruction) > maxInstructionChars {
		return nil, &Error{Code: "INSTRUCTION_TOO_LONG", Message: "The revision instruction cannot exceed 2,000 characters."}
	}
	systemPrompt, custom, err := ResolveSystemPrompt(req.SystemPromptOverride)
	if err != nil {
		return nil, err
	}

	declared := req.Manifest.Assets
	observation := trimSpace(req.CachedObservation)
	if observation == "" {
		observation = current
	}
	userContent := fmt.Sprintf(
		"Rewrite the current H3 prompt according to the revision instruction. "+
			"Return only the complete revised H3 prompt. Do not discuss the changes.\n\n"+
			"Reference manifest (text only; media is intentionally not attached):\n%s\n\n"+
			"Cached first-pass observation:\n%s\n\nCurrent prompt:\n%s\n\nRevision instruction:\n%s\n\n%s",
		referenceManifestText(declared), observation, current, instruction,
		finalContract(current+" "+instruction),
	)
	messages, guide, base, err := guideMessages(systemPrompt)
	if err != nil {
		return nil, err
	}
	return &Assembled{
		SchemaVersion: 1,
		Guide:         *guide,
		Supporting:    base,
		SystemPrompt:  SystemPrompt{Custom: custom, Content: systemPrompt},
		Messages:      append(messages, Message{Role: "user", Content: userContent}),
		MediaInputs:   nil,
		Input: AssembledInput{
			CurrentPrompt: current,
			Instruction:   instruction,
			Manifest:      req.Manifest,
		},
	}, nil
}
