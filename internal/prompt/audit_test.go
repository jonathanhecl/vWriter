package prompt

import (
	"strings"
	"testing"
)

func TestDuplicateSubjectDefinitions(t *testing.T) {
	prompt := "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n<Subject 2> stands behind.\n<Subject 1> appears again redefined.\n\nsummary:\n[reference generation] A shot.\n"
	if got := DuplicateSubjectDefinitions(prompt); len(got) != 1 || got[0] != "<Subject 1>" {
		t.Fatalf("duplicate detection = %v, want [<Subject 1>]", got)
	}
	if got := DuplicateSubjectDefinitions("subject_definitions:\n<Subject 1> comes from <Picture 1>.\n<Subject 2> stands behind.\n"); len(got) != 0 {
		t.Fatalf("clean subject definitions must not warn, got %v", got)
	}
}

// referencePrompt builds a structurally valid full-reference prompt whose
// detailed_description has exactly wordCount filler words.
func referencePrompt(wordCount int, includeSoundscape bool) string {
	detailed := strings.TrimRight(strings.Repeat("visible ", wordCount), " ")
	soundscape := ""
	if includeSoundscape {
		soundscape = "overall_soundscape:\nN/A\n\n"
	}
	return "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n\n" +
		"summary:\n[reference generation] A restrained shot.\n\n" +
		"retention_analysis:\n<Subject 1>: fully_preserved.\n\n" +
		"detailed_description:\n[Shot 1] " + detailed + "\n\n" +
		soundscape +
		"non_diegetic_music:\nN/A"
}

func Test340WordsAcceptedWithoutRepair(t *testing.T) {
	audit := AuditPrompt(referencePrompt(340, true), 10, true)
	if !audit.OfficialFormatPass || audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
	if audit.GenerationWordTargetMet == nil || *audit.GenerationWordTargetMet {
		t.Fatal("340 words must not meet the 350+ target")
	}
	if audit.DetailedDescriptionLength != "acceptable_below_target" {
		t.Fatalf("length = %q", audit.DetailedDescriptionLength)
	}
}

func Test250to299WordsIsInternalWarningOnly(t *testing.T) {
	audit := AuditPrompt(referencePrompt(270, true), 10, true)
	if audit.DetailedDescriptionLength != "short_internal_warning" || audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestUnder250WordsIsQualityWarningNotRepair(t *testing.T) {
	audit := AuditPrompt(referencePrompt(249, true), 10, true)
	if audit.DetailedDescriptionLength != "severely_short_internal_warning" {
		t.Fatalf("length = %q", audit.DetailedDescriptionLength)
	}
	if len(audit.QualityWarnings) != 1 || audit.QualityWarnings[0] != "severely short detailed_description" {
		t.Fatalf("warnings = %v", audit.QualityWarnings)
	}
	if audit.RepairRequired {
		t.Fatal("quality warnings must not require repair")
	}
}

func TestMissingSectionRequiresRepairRegardlessOfLength(t *testing.T) {
	audit := AuditPrompt(referencePrompt(360, false), 10, true)
	if len(audit.MissingSections) != 1 || audit.MissingSections[0] != "overall_soundscape" {
		t.Fatalf("missing = %v", audit.MissingSections)
	}
	if audit.StructurePass || !audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestMalformedTimestampRequiresRepair(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "At 00:153, a restrained shot.", 1)
	audit := AuditPrompt(prompt, 10, true)
	if len(audit.InvalidTimestamps) != 1 || audit.InvalidTimestamps[0] != "00:153" {
		t.Fatalf("invalid = %v", audit.InvalidTimestamps)
	}
	if !audit.RepairRequired {
		t.Fatal("repair required")
	}
}

func TestTimestampBeyondDurationRequiresRepair(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "At 00:12.000, a restrained shot.", 1)
	audit := AuditPrompt(prompt, 10, true)
	if len(audit.InvalidTimestamps) != 1 || audit.InvalidTimestamps[0] != "00:12.000" {
		t.Fatalf("invalid = %v", audit.InvalidTimestamps)
	}
}

func TestValidTimestampAccepted(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "At 00:03.500, a restrained shot.", 1)
	audit := AuditPrompt(prompt, 10, true)
	if len(audit.InvalidTimestamps) != 0 || audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestUnrequestedCameraDirectionIsNotHardError(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "The shot cuts to a close-up and slowly zooms in.", 1)
	audit := AuditPrompt(prompt, 10, false)
	got := strings.Join(audit.UnsupportedCameraDirections, "|")
	if got != "cuts to|zooms in" {
		t.Fatalf("unsupported = %q", got)
	}
	if audit.RepairRequired {
		t.Fatal("unsupported camera direction is a warning, not a repair")
	}
}

func TestRequestedCameraDirectionAccepted(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "The shot cuts to a close-up and slowly zooms in.", 1)
	audit := AuditPrompt(prompt, 10, true)
	if len(audit.UnsupportedCameraDirections) != 0 || audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestInternalVideoSheetLanguageRequiresRepair(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.",
		"Follow the sampled frames and the gesture at the 5.507s mark.", 1)
	audit := AuditPrompt(prompt, 10, true)
	got := strings.Join(audit.InternalVideoTerms, "|")
	if got != "sampled frames|5.507s mark" {
		t.Fatalf("internal terms = %q", got)
	}
	if !audit.RepairRequired {
		t.Fatal("repair required")
	}
}

func TestDialogueWithoutSpeakerIDRequiresRepair(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "She says <d>Hello.</d>", 1)
	audit := AuditPrompt(prompt, 10, true)
	if !audit.MissingDialogueSource || !audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestDialogueWithSpeakerIDAccepted(t *testing.T) {
	prompt := strings.Replace(referencePrompt(340, true), "A restrained shot.", "(S1) says <d>Hello.</d>", 1)
	audit := AuditPrompt(prompt, 10, true)
	if audit.MissingDialogueSource || audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestMissingTaskLabelRequiresRepair(t *testing.T) {
	audit := AuditPrompt(strings.Replace(referencePrompt(340, true), "[reference generation] ", "", 1), 10, true)
	if !audit.MissingTaskLabel || !audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestMissingShotMarkerRequiresRepair(t *testing.T) {
	audit := AuditPrompt(strings.Replace(referencePrompt(340, true), "[Shot 1] ", "", 1), 10, true)
	if !audit.MissingShotMarker || !audit.RepairRequired {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestCameraStructureIntentDetection(t *testing.T) {
	if !CameraStructureRequested("One continuous camera move around her.") {
		t.Fatal("camera intent expected")
	}
	if CameraStructureRequested("A quiet morning in the kitchen.") {
		t.Fatal("no camera intent expected")
	}
}
