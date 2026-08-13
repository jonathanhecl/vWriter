package prompt

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	referenceTagPattern    = regexp.MustCompile(`(?i)<\s*(Picture|Video|Audio)\s+(\d+)\s*>`)
	subjectTagPattern      = regexp.MustCompile(`(?i)<\s*Subject\s+(\d+)\s*>`)
	malformedTagPattern    = regexp.MustCompile(`(?i)<\s*(Picture|Video|Audio|Subject)\s+([^0-9][^<>]*)\s*>`)
	cameraMovement         = regexp.MustCompile(`(?i)\b(?:zoom(?:s|ed|ing)?|pan(?:s|ned|ning)?|doll(?:y|ies|ied|ying)|tracking shot|camera\s+(?:moves?|pulls?|pushes?|pans?|zooms?|tracks?|dollies?))\b`)
	noCutsBrief            = regexp.MustCompile(`(?i)\b(?:no cuts?|without cuts?|single continuous shot|one continuous shot)\b`)
	staticCameraBrief      = regexp.MustCompile(`(?i)\b(?:static|locked(?:-off)?|fixed)\s+camera\b|\bno camera movement\b`)
	motionOnlyClause       = regexp.MustCompile(`(?i)\b(?:only|solely)\b`)
	motionWords            = regexp.MustCompile(`(?i)\b(?:motion|movement|dance|choreograph\w*)\b`)
	videoNumber            = regexp.MustCompile(`(?i)\bVideo\s+([1-3])\b`)
	audioTaskLabel         = regexp.MustCompile(`(?i)\baudio\s+(?:reuse|reference)\b`)
	undeclaredVideoMention = regexp.MustCompile(`(?i)\b(?:reference|source|input|original|uploaded|provided)\s+(?:video|footage|clip)s?\b|\bvideos?\s+\d+\b`)
	undeclaredAudioMention = regexp.MustCompile(`(?i)\b(?:reference|source|input|original|uploaded|provided)\s+(?:audio|track|song|music|voice)s?\b|\baudios?\s+\d+\b`)
	undeclaredImageMention = regexp.MustCompile(`(?i)\b(?:reference|source|input|original|uploaded|provided)\s+(?:image|picture|photo)s?\b|\bpictures?\s+\d+\b`)
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

// SubjectTags extracts the normalized numbered subject tags of a text,
// e.g. "<Subject 1>".
func SubjectTags(text string) map[string]bool {
	tags := map[string]bool{}
	for _, groups := range subjectTagPattern.FindAllStringSubmatch(text, -1) {
		tags["<Subject "+groups[1]+">"] = true
	}
	return tags
}

// MalformedTags lists malformed numbered-tag placeholders such as "<Video
// None>" or "<Audio N>", which never correspond to a real asset.
func MalformedTags(text string) []string {
	var tags []string
	seen := map[string]bool{}
	for _, groups := range malformedTagPattern.FindAllStringSubmatch(text, -1) {
		tag := strings.TrimSpace(groups[0])
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

// UndeclaredMediaMentions lists prose or bare-numbered mentions of media
// kinds the manifest does not declare: hallucinated inputs such as "the
// reference video" or "Video 1" when no video asset exists. Declared kinds
// are derived from the expected tags of the request.
func UndeclaredMediaMentions(text string, expectedTags map[string]bool) []string {
	declared := map[string]bool{}
	for tag := range expectedTags {
		switch {
		case strings.HasPrefix(tag, "<Picture "):
			declared["image"] = true
		case strings.HasPrefix(tag, "<Video "):
			declared["video"] = true
		case strings.HasPrefix(tag, "<Audio "):
			declared["audio"] = true
		}
	}
	var mentions []string
	if !declared["video"] {
		mentions = append(mentions, undeclaredVideoMention.FindAllString(text, -1)...)
	}
	if !declared["audio"] {
		mentions = append(mentions, undeclaredAudioMention.FindAllString(text, -1)...)
	}
	if !declared["image"] {
		mentions = append(mentions, undeclaredImageMention.FindAllString(text, -1)...)
	}
	return dedupe(mentions)
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

// SanitizePrompt removes every line that references a numbered media or
// subject tag not in the expected sets — an invented asset such as <Audio 1>
// or <Subject 3>, or a malformed placeholder such as <Video None>. It is a
// deterministic guard: media is always constrained to the expected set, while
// subjects are only constrained when a prior subject set exists (a fresh
// generation defines them freely).
func SanitizePrompt(text string, allowedMedia, allowedSubjects map[string]bool) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		if lineReferencesUndeclaredTag(line, allowedMedia, allowedSubjects) {
			changed = true
			continue
		}
		out = append(out, line)
	}
	if !changed {
		return text
	}
	return strings.Join(out, "\n")
}

// lineReferencesUndeclaredTag reports whether a line contains a malformed
// placeholder, a media tag not in the expected set, or — when a prior subject
// set exists — a subject tag outside it.
func lineReferencesUndeclaredTag(line string, allowedMedia, allowedSubjects map[string]bool) bool {
	if malformedTagPattern.MatchString(line) {
		return true
	}
	for _, groups := range referenceTagPattern.FindAllStringSubmatch(line, -1) {
		kind := strings.ToUpper(groups[1][:1]) + strings.ToLower(groups[1][1:])
		tag := fmt.Sprintf("<%s %s>", kind, groups[2])
		if !allowedMedia[tag] {
			return true
		}
	}
	if len(allowedSubjects) == 0 {
		return false
	}
	for _, groups := range subjectTagPattern.FindAllStringSubmatch(line, -1) {
		tag := "<Subject " + groups[1] + ">"
		if !allowedSubjects[tag] {
			return true
		}
	}
	return false
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
	if len(audit.UnexpectedSubjects) > 0 {
		failures = append(failures, "unexpected subject tags: "+strings.Join(audit.UnexpectedSubjects, ", "))
	}
	if audit.UnexpectedAudioTask {
		failures = append(failures, "audio reference/reuse declared without an uploaded audio asset")
	}
	if len(audit.UndeclaredMediaMentions) > 0 {
		failures = append(failures, "mentions media that was not provided: "+strings.Join(audit.UndeclaredMediaMentions, ", "))
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
	timestampRule := "Target timestamps must use MM:SS.mmm. "
	if durationSeconds > 0 {
		timestampRule = fmt.Sprintf("Target timestamps must use MM:SS.mmm and remain within %g seconds. ", durationSeconds)
	}
	system := "This is a narrow correction pass, not a new prompt-generation pass. Correct only the exact violations " +
		"listed below and preserve every other supported fact, reference role, action, dialogue line, shot, and " +
		"creative choice unchanged. Return the complete corrected prompt with no commentary. The exact allowed " +
		fmt.Sprintf("numbered media tags are: %s. Do not add any other media tag. Do not mention any reference, "+
			"source, or input media that is not listed in the original request manifest. Requested music without an ", allowedText) +
		"uploaded audio asset belongs only in non_diegetic_music and is not audio reference or reuse. " +
		timestampRule + "Violations: " + strings.Join(violations, "; ")
	user := fmt.Sprintf("ORIGINAL REQUEST:\n%s\n\nDRAFT TO CORRECT:\n%s", originalRequest, draft)
	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
}
