package prompt

import (
	"regexp"
	"strings"
)

// extractShotPattern matches any [Shot N] marker. It is intentionally
// distinct from the audit's shotMarkerPattern (which anchors [Shot 1] to a
// line start).
var extractShotPattern = regexp.MustCompile(`(?i)\[Shot\s+\d+\]`)

// ExtractEndingState returns the final scene state of a generated prompt: the
// text after the last [Shot N] marker inside detailed_description, which
// describes the exact frame where the video ends. It falls back to the summary
// and then to the whole detailed_description when no shot markers are present.
func ExtractEndingState(promptText string) string {
	detailed := sectionContent(promptText, "detailed_description")
	if matches := extractShotPattern.FindAllStringIndex(detailed, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		if tail := cleanState(detailed[last[1]:]); tail != "" {
			return tail
		}
	}
	if summary := cleanState(sectionContent(promptText, "summary")); summary != "" {
		return summary
	}
	return cleanState(detailed)
}

// sectionContent returns the text of one section, from its "name:" header up
// to the next official section header (or the end of the prompt).
func sectionContent(promptText, name string) string {
	pattern := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(name) + `\s*:\s*`)
	loc := pattern.FindStringIndex(promptText)
	if loc == nil {
		return ""
	}
	start := loc[1]
	end := len(promptText)
	for _, other := range ReferenceSections {
		if other == name {
			continue
		}
		otherPattern := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(other) + `\s*:\s*`)
		if otherLoc := otherPattern.FindStringIndex(promptText); otherLoc != nil && otherLoc[0] > start && otherLoc[0] < end {
			end = otherLoc[0]
		}
	}
	return promptText[start:end]
}

// cleanState trims an extracted state and bounds its length so a long
// detailed_description cannot blow the continuation context budget.
func cleanState(text string) string {
	const maxEndingStateChars = 1200
	text = strings.TrimSpace(text)
	if len(text) > maxEndingStateChars {
		text = text[:maxEndingStateChars]
		if index := strings.LastIndex(text, " "); index > 0 {
			text = text[:index]
		}
	}
	return strings.TrimSpace(text)
}
