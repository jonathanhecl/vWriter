package app

import (
	"image/color"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/x/richtext"

	"github.com/jonathanhecl/vWriter/internal/media"
)

// Highlight colors — kept distinct and readable on dark background.
var (
	hlColorHeader    = color.NRGBA{R: 230, G: 175, B: 60, A: 255}  // amber — section labels
	hlColorPicRef    = color.NRGBA{R: 80, G: 160, B: 255, A: 255}  // blue — <Picture/Video/Audio N>
	hlColorSubject   = color.NRGBA{R: 175, G: 110, B: 240, A: 255} // violet — <Subject N>
	hlColorShot      = color.NRGBA{R: 80, G: 200, B: 140, A: 255}  // green — [Shot N]
	hlColorKeyword   = color.NRGBA{R: 100, G: 210, B: 215, A: 255} // teal — [reference generation]
	hlColorRetention = color.NRGBA{R: 225, G: 130, B: 80, A: 255}  // orange — attribute_transfer etc.
)

// combined regex — finds all special tokens in one pass (order matters: longest match first)
var hlPattern = regexp.MustCompile(
	`<(?:Picture|Video|Audio)\s+\d+>` + // <Picture N>, <Video N>, <Audio N>
		`|<Subject\s+\d+>` + // <Subject N>
		`|\[Shot\s+\d+\]` + // [Shot N]
		`|\[reference generation\]` + // [reference generation]
		`|\b(?:attribute_transfer|fully_preserved|partial_transfer|absent|not_included)\b`,
)

var reHeaderLine = regexp.MustCompile(`^[a-z][a-z_]+:$`)
var reShotLine = regexp.MustCompile(`^\[Shot\s+\d+\]`)

// highlightState holds the InteractiveText state for the richtext renderer.
type highlightState struct {
	it richtext.InteractiveText
}

// tokenizeOutput converts a full output string into []richtext.SpanStyle with
// correct colors for each highlight class.
func tokenizeOutput(text string, baseColor color.NRGBA, bodySize unit.Sp) []richtext.SpanStyle {
	if text == "" {
		return nil
	}

	headerFont := font.Font{Weight: font.SemiBold}
	plainFont := font.Font{}

	var spans []richtext.SpanStyle
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		// Append newline between lines (not after last).
		nl := "\n"
		if lineIdx == len(lines)-1 {
			nl = ""
		}

		trimmed := strings.TrimSpace(line)

		// Section header line: e.g. "subject_definitions:" or "summary:"
		if reHeaderLine.MatchString(trimmed) {
			spans = append(spans, richtext.SpanStyle{
				Font:    headerFont,
				Size:    bodySize,
				Color:   hlColorHeader,
				Content: line + nl,
			})
			continue
		}

		// Line with no special tokens — emit as plain text.
		locs := hlPattern.FindAllStringIndex(line, -1)
		if len(locs) == 0 {
			spans = append(spans, richtext.SpanStyle{
				Font:    plainFont,
				Size:    bodySize,
				Color:   baseColor,
				Content: line + nl,
			})
			continue
		}

		// Mixed line: interleave plain and highlighted spans.
		last := 0
		for i, loc := range locs {
			// Plain prefix before this match.
			if loc[0] > last {
				spans = append(spans, richtext.SpanStyle{
					Font:    plainFont,
					Size:    bodySize,
					Color:   baseColor,
					Content: line[last:loc[0]],
				})
			}
			// Highlighted token.
			token := line[loc[0]:loc[1]]
			col := tokenColor(token)
			spans = append(spans, richtext.SpanStyle{
				Font:    plainFont,
				Size:    bodySize,
				Color:   col,
				Content: token,
			})
			last = loc[1]
			// After last match: append trailing plain text + newline.
			if i == len(locs)-1 {
				if last < len(line) {
					spans = append(spans, richtext.SpanStyle{
						Font:    plainFont,
						Size:    bodySize,
						Color:   baseColor,
						Content: line[last:] + nl,
					})
				} else {
					spans = append(spans, richtext.SpanStyle{
						Font:    plainFont,
						Size:    bodySize,
						Color:   baseColor,
						Content: nl,
					})
				}
			}
		}
	}
	return spans
}

// tokenColor returns the highlight color for a matched token string.
func tokenColor(token string) color.NRGBA {
	switch {
	case strings.HasPrefix(token, "<Picture") ||
		strings.HasPrefix(token, "<Video") ||
		strings.HasPrefix(token, "<Audio"):
		return hlColorPicRef
	case strings.HasPrefix(token, "<Subject"):
		return hlColorSubject
	case strings.HasPrefix(token, "[Shot"):
		return hlColorShot
	case token == "[reference generation]":
		return hlColorKeyword
	default:
		return hlColorRetention
	}
}

// layoutHighlightedOutput renders the output text with syntax highlighting
// using gioui.org/x/richtext.
func (a *App) layoutHighlightedOutput(gtx layout.Context) layout.Dimensions {
	text := a.outputEditor.Text()
	if strings.TrimSpace(text) == "" {
		return layout.Dimensions{}
	}
	spans := tokenizeOutput(text, colorText, 13)
	ts := richtext.Text(&a.highlightState.it, a.theme.Shaper, spans...)
	ts.WrapPolicy = 0 // WrapHeuristically
	return ts.Layout(gtx)
}

// Ensure media import is used (for the unused import guard)
var _ = media.Image
