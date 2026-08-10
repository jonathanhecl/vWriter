package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	referenceTagPattern = regexp.MustCompile(`(?i)<\s*(Picture|Video|Audio)\s+(\d+)\s*>`)
	cameraMovement      = regexp.MustCompile(`(?i)\b(?:zoom(?:s|ed|ing)?|pan(?:s|ned|ning)?|doll(?:y|ies|ied|ying)|tracking shot|camera\s+(?:moves?|pulls?|pushes?|pans?|zooms?|tracks?|dollies?))\b`)
	noCutsBrief         = regexp.MustCompile(`(?i)\b(?:no cuts?|without cuts?|single continuous shot|one continuous shot)\b`)
	staticCameraBrief   = regexp.MustCompile(`(?i)\b(?:static|locked(?:-off)?|fixed)\s+camera\b|\bno camera movement\b`)
	motionOnlyClause    = regexp.MustCompile(`(?i)\b(?:only|solely)\b`)
	motionWords         = regexp.MustCompile(`(?i)\b(?:motion|movement|dance|choreograph\w*)\b`)
	videoNumber         = regexp.MustCompile(`(?i)\bVideo\s+([1-3])\b`)
	audioTaskLabel      = regexp.MustCompile(`(?i)\baudio\s+(?:reuse|reference)\b`)
)

// ReferenceTags extracts the normalized numbered media tags of a text,
// e.g. "<Picture 1>", "<Video 2>".
func ReferenceTags(text string) map[string]bool {
	tags := map[string]bool{}
	for _, groups := range referenceTagPattern.FindAllStringSubmatch(text, -1) {
		kind := strings.ToUpper(groups[1][:1]) + strings.ToLower(groups[1][1:])
		tags[fmt.Sprintf("<%s %s>", kind, groups[2])] = true
	}
	return tags
}

// UnexpectedAudioTask reports whether the summary task label declares audio
// reuse/reference without any uploaded audio asset.
func UnexpectedAudioTask(taskLabel string, expectedTags map[string]bool) bool {
	for tag := range expectedTags {
		if strings.HasPrefix(tag, "<Audio ") {
			return false
		}
	}
	return audioTaskLabel.MatchString(taskLabel)
}

// ExplicitConstraintViolations checks hard user constraints from the brief
// against the generated prompt: explicit no-cuts, explicit static camera, and
// motion-only video roles leaking excluded source traits.
func ExplicitConstraintViolations(creativeBrief, prompt string) []string {
	violations := []string{}
	if noCutsBrief.MatchString(creativeBrief) {
		if regexp.MustCompile(`(?i)\[Shot\s+[2-9]\d*\]|\bcut(?:s)?\s+to\b`).MatchString(prompt) {
			violations = append(violations, "the user explicitly requested one continuous shot without cuts")
		}
	}
	if staticCameraBrief.MatchString(creativeBrief) && cameraMovement.MatchString(prompt) {
		violations = append(violations, "the user explicitly requested a static camera")
	}

	motionOnlyVideos := map[int]bool{}
	for _, clause := range regexp.MustCompile(`[.\n;]+`).Split(creativeBrief, -1) {
		if motionOnlyClause.MatchString(clause) && motionWords.MatchString(clause) {
			for _, groups := range videoNumber.FindAllStringSubmatch(clause, -1) {
				if number, err := strconv.Atoi(groups[1]); err == nil {
					motionOnlyVideos[number] = true
				}
			}
		}
	}
	excludedTrait := `(?:environment|background|setting|location|lighting|performer|identity|clothing|wardrobe|outfit|audio|soundtrack)`
	numbers := make([]int, 0, len(motionOnlyVideos))
	for number := range motionOnlyVideos {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	for _, number := range numbers {
		tag := fmt.Sprintf(`<\s*Video\s+%d\s*>`, number)
		provenance := regexp.MustCompile(
			`(?i)\b` + excludedTrait + `\b[^.\n]{0,90}\b(?:from|of|in)\s*` + tag +
				`|` + tag + `[^.\n]{0,90}\b(?:provides?|defines?|supplies?|is used for)\s+(?:the\s+)?` + excludedTrait + `\b`,
		)
		if provenance.MatchString(prompt) {
			violations = append(violations,
				fmt.Sprintf("<Video %d> is assigned only to motion but supplies an excluded source trait", number))
		}
	}
	return violations
}

// AuditFailures summarizes the objective audit failures for the repair pass.
func AuditFailures(audit *Audit) []string {
	failures := []string{}
	if len(audit.MissingSections) > 0 {
		failures = append(failures, "missing required sections")
	}
	if !audit.SectionOrderValid {
		failures = append(failures, "incorrect section order")
	}
	if audit.MissingTaskLabel {
		failures = append(failures, "missing summary task label")
	}
	if audit.MissingShotMarker {
		failures = append(failures, "missing [Shot 1] marker")
	}
	if len(audit.InvalidTimestamps) > 0 {
		failures = append(failures, "invalid target timestamps")
	}
	if len(audit.InternalVideoTerms) > 0 {
		failures = append(failures, "internal contact-sheet language")
	}
	if audit.MissingDialogueSource {
		failures = append(failures, "dialogue without a stable speaker ID")
	}
	if len(audit.MissingReferenceTags) > 0 {
		failures = append(failures, "missing reference tags: "+strings.Join(audit.MissingReferenceTags, ", "))
	}
	if len(audit.UnexpectedReferenceTags) > 0 {
		failures = append(failures, "unexpected reference tags: "+strings.Join(audit.UnexpectedReferenceTags, ", "))
	}
	if audit.UnexpectedAudioTask {
		failures = append(failures, "audio reference/reuse declared without an uploaded audio asset")
	}
	return append(failures, audit.ExplicitConstraintViolations...)
}

// NarrowRepairMessages builds the single corrective pass: fix only the listed
// violations, preserve everything else, return the complete corrected prompt.
func NarrowRepairMessages(assembled *Assembled, draft string, violations []string, expectedTags map[string]bool, durationSeconds float64) []Message {
	var originalRequest string
	for _, message := range assembled.Messages {
		if message.Role == "user" {
			originalRequest = message.Content
		}
	}
	allowed := make([]string, 0, len(expectedTags))
	for tag := range expectedTags {
		allowed = append(allowed, tag)
	}
	sort.Strings(allowed)
	allowedText := strings.Join(allowed, ", ")
	if allowedText == "" {
		allowedText = "none"
	}
	system := "This is a narrow correction pass, not a new prompt-generation pass. Correct only the exact violations " +
		"listed below and preserve every other supported fact, reference role, action, dialogue line, shot, and " +
		"creative choice unchanged. Return the complete corrected prompt with no commentary. The exact allowed " +
		fmt.Sprintf("numbered media tags are: %s. Do not add any other media tag. Requested music without an ", allowedText) +
		"uploaded audio asset belongs only in non_diegetic_music and is not audio reference or reuse. Target " +
		fmt.Sprintf("timestamps must use MM:SS.mmm and remain within %g seconds. Violations: ", durationSeconds) +
		strings.Join(violations, "; ")
	user := fmt.Sprintf("ORIGINAL REQUEST:\n%s\n\nDRAFT TO CORRECT:\n%s", originalRequest, draft)
	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
}
