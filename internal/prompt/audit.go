package prompt

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ReferenceSections are the six required sections of a full-reference prompt,
// in their official order.
var ReferenceSections = []string{
	"subject_definitions",
	"summary",
	"retention_analysis",
	"detailed_description",
	"overall_soundscape",
	"non_diegetic_music",
}

var (
	timestampCandidate = regexp.MustCompile(`\d{2}:\d{2,3}(?:\.\d{1,3})?`)
	validTimestamp     = regexp.MustCompile(`^(\d{2}):(\d{2})\.(\d{3})$`)
	timestampToken     = regexp.MustCompile(`\d{2}:\d{2}\.\d{3}`)
	cameraDirection    = regexp.MustCompile(`(?i)\b(?:cut(?:s)?\s+to|zoom(?:s|ed|ing)?(?:-in|-out|\s+in|\s+out)?|pan(?:s|ned|ning)?(?:\s+(?:up|down|left|right|across))?|doll(?:y|ies|ied|ying)|tracking shot|camera\s+(?:moves?|pulls?|pushes?|pans?|zooms?|tracks?|dollies?))\b`)
	internalVideoTerms = regexp.MustCompile(`(?i)\b(?:contact sheet|sheet cell(?:s)?|sampled frame(?:s)?|sample frame(?:s)?|\d+(?:\.\d+)?s\s+mark)\b`)
	wordPattern        = regexp.MustCompile(`[A-Za-z]+(?:[-'][A-Za-z]+)*`)
	shotMarkerPattern  = regexp.MustCompile(`(?im)^\s*\[Shot\s+1\]`)
	shotTagStrip       = regexp.MustCompile(`(?i)\[Shot\s+\d+\]`)
	dialoguePattern    = regexp.MustCompile(`(?is)<d>.*?</d>`)
	speakerIDPattern   = regexp.MustCompile(`\(S\d+(?:\s*,\s*S\d+)*\)`)
	audioTagPattern    = regexp.MustCompile(`(?i)<Audio\s+\d+>`)
	cameraIntent       = regexp.MustCompile(`(?i)\b(?:camera|framing|shot|cut|zoom|pan|dolly|tracking|handheld|pov|temporal structure|whole video|entire video)\b`)
	speechVerbPattern  = regexp.MustCompile(`(?i)\b(?:says?|said|talk(?:s|ed|ing)?|speak(?:s|ing)?|spoke|exclaim(?:s|ed|ing)?|mutter(?:s|ed|ing)?|murmur(?:s|ed|ing)?|whisper(?:s|ed|ing)?|shout(?:s|ed|ing)?|reply(?:s|ied|ing)?|answer(?:s|ed|ing)?)\b`)
	speechNoLine       = regexp.MustCompile(`(?i)\b(?:nothing|no words|closed|silent|without speaking|does not speak|no dialogue)\b`)
)

// Audit is the structural and quality report of one generated prompt.
type Audit struct {
	RequiredSections            []string `json:"required_sections"`
	MissingSections             []string `json:"missing_sections"`
	SectionOrderValid           bool     `json:"section_order_valid"`
	TaskLabel                   string   `json:"task_label"`
	MissingTaskLabel            bool     `json:"missing_task_label"`
	MissingShotMarker           bool     `json:"missing_shot_marker"`
	DetailedDescriptionWords    int      `json:"detailed_description_words"`
	GenerationWordTargetApplies bool     `json:"generation_word_target_applies"`
	GenerationWordTargetMet     *bool    `json:"generation_word_target_met"`
	DetailedDescriptionLength   string   `json:"detailed_description_length_status"`
	StructurePass               bool     `json:"structure_pass"`
	InvalidTimestamps           []string `json:"invalid_timestamps"`
	UnsupportedCameraDirections []string `json:"unsupported_camera_directions"`
	QualityWarnings             []string `json:"quality_warnings"`
	InternalVideoTerms          []string `json:"internal_video_representation_terms"`
	MissingDialogueSource       bool     `json:"missing_dialogue_source"`
	RepairRequired              bool     `json:"repair_required"`
	OfficialFormatPass          bool     `json:"official_format_pass"`
	QualityTargetPass           bool     `json:"quality_target_pass"`

	// Filled by the caller (engine) after tag and constraint checks.
	MissingReferenceTags         []string `json:"missing_reference_tags,omitempty"`
	UnexpectedReferenceTags      []string `json:"unexpected_reference_tags,omitempty"`
	UnexpectedSubjects           []string `json:"unexpected_subjects,omitempty"`
	UnexpectedAudioTask          bool     `json:"unexpected_audio_task,omitempty"`
	ExplicitConstraintViolations []string `json:"explicit_constraint_violations,omitempty"`
	UndeclaredMediaMentions      []string `json:"undeclared_media_mentions,omitempty"`
}

// InvalidTimestamps extracts timestamp-looking tokens that are malformed or
// exceed the target duration. Neighbor checks replace Python's lookarounds,
// which RE2 does not support.
func InvalidTimestamps(prompt string, durationSeconds float64) []string {
	invalid := []string{}
	seen := map[string]bool{}
	for _, loc := range timestampCandidate.FindAllStringIndex(prompt, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isDigit(prompt[start-1]) {
			continue
		}
		if end < len(prompt) && isDigit(prompt[end]) {
			continue
		}
		value := prompt[start:end]
		groups := validTimestamp.FindStringSubmatch(value)
		bad := groups == nil
		if !bad {
			minutes, _ := strconv.Atoi(groups[1])
			seconds, _ := strconv.Atoi(groups[2])
			millis, _ := strconv.Atoi(groups[3])
			total := float64(minutes*60+seconds) + float64(millis)/1000
			bad = seconds >= 60 || (durationSeconds > 0 && total > durationSeconds-0.5)
		}
		if bad && !seen[value] {
			seen[value] = true
			invalid = append(invalid, value)
		}
	}
	return invalid
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// CameraStructureRequested reports whether the user's intent text asks for
// camera or temporal structure.
func CameraStructureRequested(intentText string) bool {
	return cameraIntent.MatchString(intentText)
}

// UnsupportedCameraDirections lists camera directions present in the prompt
// when none were allowed.
func UnsupportedCameraDirections(prompt string, allowed bool) []string {
	if allowed {
		return nil
	}
	return dedupe(cameraDirection.FindAllString(prompt, -1))
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// DuplicateSubjectDefinitions lists <Subject N> tags defined more than once in
// subject_definitions, a sign the model re-defined a character it already
// introduced.
func DuplicateSubjectDefinitions(promptText string) []string {
	var dupes []string
	seen := map[string]bool{}
	for _, line := range strings.Split(sectionContent(promptText, "subject_definitions"), "\n") {
		groups := subjectTagPattern.FindStringSubmatch(line)
		if groups == nil {
			continue
		}
		tag := "<Subject " + groups[1] + ">"
		if seen[tag] {
			dupes = append(dupes, tag)
		}
		seen[tag] = true
	}
	return dedupe(dupes)
}

// LooseSpeechLines lists detailed_description lines that describe speech with
// no <d> block and no speaker ID, which can make the video model invent lines
// that were not in the script.
func LooseSpeechLines(promptText string) []string {
	var lines []string
	for _, line := range strings.Split(sectionContent(promptText, "detailed_description"), "\n") {
		if !speechVerbPattern.MatchString(line) {
			continue
		}
		if strings.Contains(line, "<d>") || speakerIDPattern.MatchString(line) || speechNoLine.MatchString(line) {
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return dedupe(lines)
}

// formatSeconds renders a float time as MM:SS.mmm.
func formatSeconds(seconds float64) string {
	return fmt.Sprintf("%02d:%02d.%03d",
		int(seconds)/60,
		int(seconds)%60,
		int(math.Round((seconds-math.Floor(seconds))*1000)))
}

// ClampTimestamps rewrites every MM:SS.mmm timestamp that lands within the
// final half second of the target duration to the last safe moment (duration
// minus 0.5s), so events always keep room to finish — a 10-second part can
// never contain "00:09.999" or "00:10.000".
func ClampTimestamps(text string, durationSeconds float64) string {
	if durationSeconds <= 0 {
		return text
	}
	lastSafe := durationSeconds - 0.5
	clamped := formatSeconds(lastSafe)
	changed := false
	out := timestampToken.ReplaceAllStringFunc(text, func(match string) string {
		groups := validTimestamp.FindStringSubmatch(match)
		if groups == nil {
			return match
		}
		minutes, _ := strconv.Atoi(groups[1])
		seconds, _ := strconv.Atoi(groups[2])
		millis, _ := strconv.Atoi(groups[3])
		total := float64(minutes*60+seconds) + float64(millis)/1000
		if total > lastSafe {
			changed = true
			return clamped
		}
		return match
	})
	if !changed {
		return text
	}
	return out
}

// AuditPrompt checks a generated prompt against the official full-reference
// output format and vWriter quality rules.
func AuditPrompt(prompt string, durationSeconds float64, cameraStructureAllowed bool) *Audit {
	audit := &Audit{RequiredSections: append([]string(nil), ReferenceSections...)}

	type sectionMatch struct {
		name       string
		start, end int
	}
	var matches []sectionMatch
	for _, section := range ReferenceSections {
		pattern := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(section) + `\s*:\s*`)
		loc := pattern.FindStringIndex(prompt)
		if loc != nil {
			matches = append(matches, sectionMatch{section, loc[0], loc[1]})
		} else {
			audit.MissingSections = append(audit.MissingSections, section)
		}
	}
	// Sections found in ascending document order must match the official order.
	audit.SectionOrderValid = true
	lastStart := -1
	for _, section := range ReferenceSections {
		for _, match := range matches {
			if match.name == section {
				if match.start < lastStart {
					audit.SectionOrderValid = false
				}
				lastStart = match.start
			}
		}
	}

	var detailed string
	detailedFound := false
	for _, match := range matches {
		if match.name != "detailed_description" {
			continue
		}
		detailedFound = true
		end := len(prompt)
		for _, other := range matches {
			if other.start > match.start && other.start < end {
				end = other.start
			}
		}
		detailed = prompt[match.end:end]
	}
	detailedForCount := shotTagStrip.ReplaceAllString(detailed, "")
	audit.DetailedDescriptionWords = len(wordPattern.FindAllString(detailedForCount, -1))
	audit.MissingShotMarker = detailedFound && !shotMarkerPattern.MatchString(detailed)

	labelPattern := regexp.MustCompile(`^\s*\[([^\]]+)\]`)
	for _, match := range matches {
		if match.name != "summary" {
			continue
		}
		if groups := labelPattern.FindStringSubmatch(prompt[match.end:]); groups != nil {
			audit.TaskLabel = strings.TrimSpace(groups[1])
		}
	}
	summaryFound := false
	for _, match := range matches {
		if match.name == "summary" {
			summaryFound = true
		}
	}
	audit.MissingTaskLabel = summaryFound && audit.TaskLabel == ""

	audit.GenerationWordTargetApplies = strings.Contains(strings.ToLower(audit.TaskLabel), "generation")
	if audit.GenerationWordTargetApplies {
		met := audit.DetailedDescriptionWords >= 350 && audit.DetailedDescriptionWords <= 500
		audit.GenerationWordTargetMet = &met
		switch {
		case audit.DetailedDescriptionWords < 250:
			audit.DetailedDescriptionLength = "severely_short_internal_warning"
		case audit.DetailedDescriptionWords < 300:
			audit.DetailedDescriptionLength = "short_internal_warning"
		case audit.DetailedDescriptionWords < 350:
			audit.DetailedDescriptionLength = "acceptable_below_target"
		case audit.DetailedDescriptionWords <= 500:
			audit.DetailedDescriptionLength = "official_target"
		default:
			audit.DetailedDescriptionLength = "above_target"
		}
	} else {
		audit.DetailedDescriptionLength = "not_applicable"
	}

	audit.StructurePass = len(audit.MissingSections) == 0 && audit.SectionOrderValid
	audit.InvalidTimestamps = InvalidTimestamps(prompt, durationSeconds)
	audit.UnsupportedCameraDirections = UnsupportedCameraDirections(prompt, cameraStructureAllowed)
	audit.InternalVideoTerms = dedupe(internalVideoTerms.FindAllString(prompt, -1))

	hasDialogue := dialoguePattern.MatchString(prompt)
	dialogueSourceValid := speakerIDPattern.MatchString(prompt) || audioTagPattern.MatchString(prompt)
	audit.MissingDialogueSource = hasDialogue && !dialogueSourceValid

	switch audit.DetailedDescriptionLength {
	case "severely_short_internal_warning":
		audit.QualityWarnings = []string{"severely short detailed_description"}
	case "short_internal_warning":
		audit.QualityWarnings = []string{"short detailed_description"}
	}
	if dupes := DuplicateSubjectDefinitions(prompt); len(dupes) > 0 {
		audit.QualityWarnings = append(audit.QualityWarnings, "duplicate subject definitions: "+strings.Join(dupes, ", "))
	}
	if loose := LooseSpeechLines(prompt); len(loose) > 0 {
		audit.QualityWarnings = append(audit.QualityWarnings, "speech described without a <d> line: "+strings.Join(loose, " | "))
	}
	audit.QualityTargetPass = len(audit.QualityWarnings) == 0

	audit.OfficialFormatPass = audit.StructurePass &&
		len(audit.InvalidTimestamps) == 0 &&
		len(audit.InternalVideoTerms) == 0 &&
		!audit.MissingDialogueSource &&
		!audit.MissingTaskLabel &&
		!audit.MissingShotMarker
	audit.RepairRequired = !audit.OfficialFormatPass
	return audit
}
