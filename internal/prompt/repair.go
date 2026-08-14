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
	malformedMediaPattern  = regexp.MustCompile(`(?i)<\s*(Picture|Video|Audio)\s+([^0-9][^<>]*)\s*>`)
	malformedTagPattern    = regexp.MustCompile(`(?i)<\s*(Picture|Video|Audio|Subject)\s+([^0-9][^<>]*)\s*>`)
	videoPlaceholder       = regexp.MustCompile(`(?i)<\s*Video\s+N\s*>`)
	subjectPlaceholder     = regexp.MustCompile(`(?i)<\s*Subject\s+N\s*>`)
	mediaPlaceholder       = regexp.MustCompile(`(?i)<\s*(?:Picture|Audio)\s+N\s*>`)
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

// SanitizeMediaPrompt removes short lines that reference media the user did
// not provide (an invented <Audio 1> or a malformed <Video None>). Only
// lines that are predominantly such a reference — retention/definition lines
// and other short lines — are dropped; long prose is never touched, so valid
// content is never destroyed. Subjects are never removed.
func SanitizeMediaPrompt(text string, allowedMedia map[string]bool) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		if lineReferencesInventedMedia(line, allowedMedia) && len([]rune(strings.TrimSpace(line))) <= 150 {
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

// FixPlaceholderLabels removes generic placeholder labels such as "<Video N>"
// or "<Subject N>" that the model copies verbatim from the instructions. A
// <Video N> placeholder is replaced with the actual source video label when
// one is provided, so continuations keep the correct reference number.
func FixPlaceholderLabels(text, sourceVideo string) string {
	text = videoPlaceholder.ReplaceAllStringFunc(text, func(string) string {
		if sourceVideo != "" {
			return sourceVideo
		}
		return ""
	})
	text = subjectPlaceholder.ReplaceAllString(text, "")
	text = mediaPlaceholder.ReplaceAllString(text, "")
	return text
}

var (
	summaryHeadingPattern     = regexp.MustCompile(`(?im)^\s*summary\s*:\s*`)
	subjectDefsHeadingPattern = regexp.MustCompile(`(?im)^\s*subject_definitions\s*:\s*`)
	retentionHeadingPattern   = regexp.MustCompile(`(?im)^\s*retention_analysis\s*:\s*`)
	summaryTaskLabelPattern   = regexp.MustCompile(`^\s*\[([^\]]+)\]`)
)

// EnforceContinuationContract makes a continuation's output name the
// continuation and cite its source video when the model omitted either. It
// only adds the missing required elements — a subject_definitions line for the
// source video and a "[video continuation]" summary prefix — and preserves
// every other line verbatim, so the model's real response stays intact.
func EnforceContinuationContract(text, sourceVideo string) string {
	sourceVideo = strings.TrimSpace(sourceVideo)
	if sourceVideo == "" {
		return text
	}
	out := text
	if !regexp.MustCompile(`(?i)` + regexp.QuoteMeta(sourceVideo)).MatchString(out) {
		line := fmt.Sprintf("%s is the video generated from the previous part of this story. Follow its style, color palette, character appearance, clothing, and scene, but generate NEW content. Never extend, reuse, or re-render its frames or shots.", sourceVideo)
		out = insertAfterHeading(out, line, subjectDefsHeadingPattern, retentionHeadingPattern)
	}
	return ensureContinuationTaskLabel(out)
}

// insertAfterHeading inserts line on its own line right after the first
// heading matched by primary (falling back to fallback), declaring the
// reference inside the section the guide assigns to it.
func insertAfterHeading(text, line string, primary, fallback *regexp.Regexp) string {
	pattern := primary
	if pattern.FindStringIndex(text) == nil {
		pattern = fallback
	}
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return text
	}
	return text[:loc[1]] + "\n" + line + text[loc[1]:]
}

// ensureContinuationTaskLabel adds "[video continuation]" to the summary's
// task-type prefix when it is missing, following the guide's combination rule
// ("video continuation + ...") when another type was already declared.
func ensureContinuationTaskLabel(text string) string {
	loc := summaryHeadingPattern.FindStringIndex(text)
	if loc == nil {
		return text
	}
	rest := text[loc[1]:]
	m := summaryTaskLabelPattern.FindStringSubmatchIndex(rest)
	if m == nil {
		return text[:loc[1]] + "\n[video continuation] " + rest
	}
	label := strings.TrimSpace(rest[m[2]:m[3]])
	if strings.Contains(strings.ToLower(label), "continuation") {
		return text
	}
	replacement := "[video continuation + " + label + "]"
	return text[:loc[1]] + rest[:m[0]] + replacement + rest[m[1]:]
}

// lineReferencesInventedMedia reports whether a line references media that is
// not in the allowed set. A line is only "invented" when it references media
// and none of those references are allowed, so a line that also cites an
// allowed reference (e.g. the source video) is always preserved.
func lineReferencesInventedMedia(line string, allowedMedia map[string]bool) bool {
	if malformedMediaPattern.MatchString(line) {
		return true
	}
	hasMedia := false
	hasAllowed := false
	for _, groups := range referenceTagPattern.FindAllStringSubmatch(line, -1) {
		hasMedia = true
		kind := strings.ToUpper(groups[1][:1]) + strings.ToLower(groups[1][1:])
		tag := fmt.Sprintf("<%s %s>", kind, groups[2])
		if allowedMedia[tag] {
			hasAllowed = true
		}
	}
	return hasMedia && !hasAllowed
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
