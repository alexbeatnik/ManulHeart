package explain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---- ScoreBreakdown: RawScore excluded from JSON ----------------------------

func TestScoreBreakdown_RawScoreExcludedFromJSON(t *testing.T) {
	sb := ScoreBreakdown{
		ExactTextMatch: 0.9,
		Total:          0.85,
		RawScore:       42.0,
	}
	data, err := json.Marshal(sb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "raw_score") {
		t.Errorf("RawScore should not appear in JSON: %s", s)
	}
	if !strings.Contains(s, `"total":0.85`) {
		t.Errorf("Total missing from JSON: %s", s)
	}
}

func TestScoreBreakdown_Roundtrip(t *testing.T) {
	orig := ScoreBreakdown{
		ExactTextMatch:      0.9,
		NormalizedTextMatch: 0.8,
		LabelMatch:          0.7,
		PlaceholderMatch:    0.6,
		AriaMatch:           0.5,
		DataQAMatch:         0.4,
		IDMatch:             0.3,
		TagSemantics:        0.85,
		TypeHintAlignment:   0.75,
		VisibilityScore:     1.0,
		InteractabilityScore: 1.0,
		ProximityScore:      0.0,
		Total:               0.88,
		RawScore:            99.9, // not serialized
	}
	data, _ := json.Marshal(orig)
	var got ScoreBreakdown
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Total != orig.Total {
		t.Errorf("Total=%v want %v", got.Total, orig.Total)
	}
	if got.RawScore != 0 {
		t.Errorf("RawScore should be zero after roundtrip, got %v", got.RawScore)
	}
}

// ---- ExecutionResult: Duration excluded from JSON ---------------------------

func TestExecutionResult_DurationExcludedFromJSON(t *testing.T) {
	er := ExecutionResult{
		Step:       "Click Login",
		DurationMS: 123,
		Duration:   500 * time.Millisecond,
		Success:    true,
	}
	data, err := json.Marshal(er)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, `"duration":`) && !strings.Contains(s, `"duration_ms"`) {
		// If only "duration" appears without "_ms" it's the unexported field leaking.
		t.Errorf("Go Duration field should not appear in JSON: %s", s)
	}
	if strings.Contains(s, "500000000") {
		t.Errorf("Duration nanoseconds leaked into JSON: %s", s)
	}
	if !strings.Contains(s, `"duration_ms":123`) {
		t.Errorf("DurationMS missing: %s", s)
	}
}

func TestExecutionResult_Roundtrip(t *testing.T) {
	orig := ExecutionResult{
		Step:        "Fill username",
		StepIndex:   2,
		StepBlock:   "Login",
		CommandType: "fill",
		PageURL:     "https://example.com",
		Success:     true,
		DurationMS:  42,
		Duration:    42 * time.Millisecond,
		ActionValue: "admin",
	}
	data, _ := json.Marshal(orig)
	var got ExecutionResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Step != orig.Step {
		t.Errorf("Step=%q want %q", got.Step, orig.Step)
	}
	if got.Duration != 0 {
		t.Errorf("Duration should be zero after roundtrip: %v", got.Duration)
	}
	if got.DurationMS != orig.DurationMS {
		t.Errorf("DurationMS=%d want %d", got.DurationMS, orig.DurationMS)
	}
}

// ---- HuntResult: TotalDuration excluded from JSON ---------------------------

func TestHuntResult_TotalDurationExcludedFromJSON(t *testing.T) {
	hr := HuntResult{
		HuntFile:        "login.hunt",
		TotalSteps:      3,
		Passed:          3,
		TotalDurationMS: 999,
		TotalDuration:   999 * time.Millisecond,
		Success:         true,
	}
	data, _ := json.Marshal(hr)
	s := string(data)
	if strings.Contains(s, "999000000") {
		t.Errorf("TotalDuration nanoseconds leaked into JSON: %s", s)
	}
	if !strings.Contains(s, `"total_duration_ms":999`) {
		t.Errorf("TotalDurationMS missing: %s", s)
	}
}

func TestHuntResult_Roundtrip(t *testing.T) {
	orig := HuntResult{
		HuntFile:        "smoke.hunt",
		Title:           "Smoke Test",
		TotalSteps:      2,
		Passed:          1,
		Failed:          1,
		TotalDurationMS: 500,
		TotalDuration:   500 * time.Millisecond,
		Success:         false,
		SoftErrors:      []string{"warn1", "warn2"},
	}
	data, _ := json.Marshal(orig)
	var got HuntResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TotalDuration != 0 {
		t.Errorf("TotalDuration should be zero after roundtrip: %v", got.TotalDuration)
	}
	if got.Title != orig.Title {
		t.Errorf("Title=%q want %q", got.Title, orig.Title)
	}
	if len(got.SoftErrors) != 2 {
		t.Errorf("SoftErrors=%v want 2 items", got.SoftErrors)
	}
}

// ---- Candidate / CandidateSignal roundtrip ----------------------------------

func TestCandidate_Roundtrip(t *testing.T) {
	c := Candidate{
		Rank:      1,
		XPath:     "//button[@id='login']",
		Tag:       "button",
		IsVisible: true,
		IsEnabled: true,
		Score: ScoreBreakdown{
			Total:    0.9,
			RawScore: 10.0,
		},
		Signals: []CandidateSignal{
			{Signal: "exact_text", Value: "Login", Score: 0.9},
		},
		Chosen: true,
	}
	data, _ := json.Marshal(c)
	var got Candidate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Score.RawScore != 0 {
		t.Errorf("nested RawScore should be zero after roundtrip: %v", got.Score.RawScore)
	}
	if got.Score.Total != 0.9 {
		t.Errorf("Total=%v want 0.9", got.Score.Total)
	}
	if !got.Chosen {
		t.Error("Chosen should be true")
	}
}

// ---- SignalBreakdown (map) --------------------------------------------------

func TestSignalBreakdown_Roundtrip(t *testing.T) {
	sb := SignalBreakdown{"text": 0.8, "id": 0.5}
	data, _ := json.Marshal(sb)
	var got SignalBreakdown
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["text"] != 0.8 {
		t.Errorf("text=%v want 0.8", got["text"])
	}
}
