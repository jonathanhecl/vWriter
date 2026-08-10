// Package guides embeds the official MiniMax H3 prompt-writing guides and
// verifies their integrity at load time.
package guides

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

//go:embed VIDEO_PROMPT_WRITING_GUIDE_base_en.md
var baseRaw string

//go:embed VIDEO_PROMPT_WRITING_GUIDE_ref_en.md
var referenceRaw string

// sourceRevision pins the upstream MiniMax H3 docs snapshot the guides come from.
const sourceRevision = "bfc8ed0353f5a9733be73e6b2c98ec0948195b86"
const sourceRoot = "https://huggingface.co/MiniMaxAI/MiniMax-H3/blob/" + sourceRevision + "/docs"

// Guide is one loaded official guide with its provenance metadata.
type Guide struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Filename       string `json:"filename"`
	SourceURL      string `json:"source_url"`
	SourceRevision string `json:"source_revision"`
	ContentSHA256  string `json:"content_sha256"`
	Content        string `json:"content,omitempty"`
}

// specs are the known guides with their expected content digests.
var specs = map[string]struct {
	title    string
	filename string
	sha256   string
	raw      *string
}{
	"base": {
		title:    "MiniMax H3 Video Prompt Writing Guide",
		filename: "VIDEO_PROMPT_WRITING_GUIDE_base_en.md",
		sha256:   "2cfebc096a6e08370f288d468d90b60f7f9bcb938f94bf090816e910e48e75fc",
		raw:      &baseRaw,
	},
	"reference": {
		title:    "MiniMax H3 Reference Prompt Writing Guide",
		filename: "VIDEO_PROMPT_WRITING_GUIDE_ref_en.md",
		sha256:   "1e574f356716ad55612247ffb7bbccbcdb484ad96599d63c7dca1af186b1fab7",
		raw:      &referenceRaw,
	},
}

var (
	loadOnce sync.Once
	loadErr  error
	loaded   map[string]Guide
)

// normalize canonicalizes line endings and trailing whitespace before hashing.
func normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")
	return strings.TrimRight(text, " \t\n") + "\n"
}

// Load parses and integrity-checks both guides once.
func Load() error {
	loadOnce.Do(func() {
		loaded = map[string]Guide{}
		for id, spec := range specs {
			content := normalize(*spec.raw)
			digest := sha256.Sum256([]byte(content))
			hexDigest := hex.EncodeToString(digest[:])
			if hexDigest != spec.sha256 {
				loadErr = fmt.Errorf("official guide integrity check failed for %s", spec.filename)
				return
			}
			loaded[id] = Guide{
				ID:             id,
				Title:          spec.title,
				Filename:       spec.filename,
				SourceURL:      sourceRoot + "/" + spec.filename,
				SourceRevision: sourceRevision,
				ContentSHA256:  hexDigest,
				Content:        content,
			}
		}
	})
	return loadErr
}

// Reference returns the full-reference guide.
func Reference() (Guide, error) {
	if err := Load(); err != nil {
		return Guide{}, err
	}
	return loaded["reference"], nil
}

// Base returns the shared base guide.
func Base() (Guide, error) {
	if err := Load(); err != nil {
		return Guide{}, err
	}
	return loaded["base"], nil
}

// ReferenceBaseExcerpt extracts only the shared base-guide rules referenced by
// full-reference mode: the first prose paragraph(s) of the listed sections.
func ReferenceBaseExcerpt() (string, error) {
	base, err := Base()
	if err != nil {
		return "", err
	}
	paragraphLimits := []struct {
		title string
		limit int
	}{
		{"4.2 Shots and Cuts", 1},
		{"4.3 Camera Motion: Motion Type + Amplitude + Speed", 1},
		{"4.4 Speakers, Dialogue, and Singing", 2},
		{"4.5 On-Screen Text", 1},
		{"4.6 overall_soundscape", 1},
		{"4.7 non_diegetic_music", 1},
	}
	sections := strings.Split(base.Content, "\n### ")
	selected := make([]string, 0, len(paragraphLimits))
	for _, wanted := range paragraphLimits {
		found := false
		for _, section := range sections {
			title, body, _ := strings.Cut(section, "\n")
			if strings.TrimSpace(title) != wanted.title {
				continue
			}
			var prose []string
			for _, part := range strings.Split(body, "\n\n") {
				part = strings.TrimSpace(part)
				if part == "" || strings.HasPrefix(part, "```") || strings.HasPrefix(part, "|") || strings.HasPrefix(part, "## ") {
					continue
				}
				prose = append(prose, part)
			}
			if len(prose) > wanted.limit {
				prose = prose[:wanted.limit]
			}
			selected = append(selected, "### "+wanted.title+"\n\n"+strings.Join(prose, "\n\n"))
			found = true
			break
		}
		if !found {
			return "", fmt.Errorf("could not extract section %q from the official base guide", wanted.title)
		}
	}
	return "# Shared official base-guide rules used by full-reference mode\n\n" +
		strings.Join(selected, "\n\n") + "\n", nil
}
