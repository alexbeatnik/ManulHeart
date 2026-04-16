package runtime

import (
	"context"
	"testing"

	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dom"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/utils"
)

func TestRuntime_ExplainMetadataForClick(t *testing.T) {
	mock := &MockPage{
		URL: "https://example.com/start",
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/button[1]", Tag: "button", VisibleText: "Save", IsVisible: true, Rect: dom.Rect{Top: 10, Left: 20, Width: 100, Height: 30}},
		},
	}
	mock.Elements[0].Normalize()

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	result, err := rt.RunHunt(context.Background(), &dsl.Hunt{
		Commands: []dsl.Command{{Type: dsl.CmdClick, Raw: "CLICK the 'Save' button", Target: "Save", TypeHint: "button"}},
	})
	if err != nil {
		t.Fatalf("RunHunt failed: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	step := result.Results[0]
	if !step.TargetRequired {
		t.Fatal("expected click to require target resolution")
	}
	if step.TargetQuery != "Save" {
		t.Fatalf("TargetQuery = %q, want Save", step.TargetQuery)
	}
	if step.CandidatesConsidered != 1 {
		t.Fatalf("CandidatesConsidered = %d, want 1", step.CandidatesConsidered)
	}
	if step.WinnerXPath != "/button[1]" {
		t.Fatalf("WinnerXPath = %q, want /button[1]", step.WinnerXPath)
	}
	if step.WinnerScore <= 0 {
		t.Fatalf("WinnerScore = %.3f, want > 0", step.WinnerScore)
	}
	if step.ActionPerformed != "click" {
		t.Fatalf("ActionPerformed = %q, want click", step.ActionPerformed)
	}
	if len(step.RankedCandidates) == 0 {
		t.Fatal("expected ranked candidates to be captured")
	}
	if step.ProbeMetadata == nil || step.ProbeMetadata["resolution_strategy"] == nil {
		t.Fatal("expected resolution strategy metadata")
	}
}

func TestRuntime_UploadFile(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 7, XPath: "/input[1]", Tag: "input", InputType: "file", AriaLabel: "Profile Picture", IsVisible: true, Rect: dom.Rect{Top: 10, Left: 20, Width: 120, Height: 30}},
		},
	}
	mock.Elements[0].Normalize()

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	res, err := rt.executeCommand(context.Background(), dsl.Command{
		Type:           dsl.CmdUploadFile,
		Raw:            "UPLOAD 'avatar.png' to 'Profile Picture'",
		Target:         "Profile Picture",
		UploadFilePath: "avatar.png",
	})
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}
	files := mock.FileInputs["/input[1]"]
	if len(files) != 1 || files[0] != "avatar.png" {
		t.Fatalf("uploaded files = %v, want [avatar.png]", files)
	}
	if res.ActionValue != "avatar.png" {
		t.Fatalf("ActionValue = %q, want avatar.png", res.ActionValue)
	}
	if res.WinnerXPath != "/input[1]" {
		t.Fatalf("WinnerXPath = %q, want /input[1]", res.WinnerXPath)
	}
	if !res.TargetRequired {
		t.Fatal("expected upload to require target resolution")
	}
}

func TestRuntime_SnapshotCacheUsedForRepeatedReadOnlyLookups(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/button[1]", Tag: "button", VisibleText: "Save", IsVisible: true},
		},
	}
	mock.Elements[0].Normalize()

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	ctx := context.Background()

	matched, err := rt.evaluateCondition(ctx, "button 'Save' exists")
	if err != nil {
		t.Fatalf("first evaluateCondition failed: %v", err)
	}
	if !matched {
		t.Fatal("expected first condition to match")
	}
	matched, err = rt.evaluateCondition(ctx, "text 'Save' is present")
	if err != nil {
		t.Fatalf("second evaluateCondition failed: %v", err)
	}
	if !matched {
		t.Fatal("expected second condition to match")
	}
	if mock.ProbeCalls != 1 {
		t.Fatalf("ProbeCalls = %d, want 1 with cache enabled", mock.ProbeCalls)
	}
}
