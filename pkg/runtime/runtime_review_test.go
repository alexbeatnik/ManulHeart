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

func TestResolveRestrictiveCandidatesPrefersAnchorWithNearbyControl(t *testing.T) {
	elements := []dom.ElementSnapshot{
		{ID: 1, XPath: "/html/body/div[1]/table[1]/tbody[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 30, Height: 20}},
		{ID: 2, XPath: "/html/body/div[1]/table[2]/tbody[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 200, Left: 100, Width: 30, Height: 20}},
		{ID: 3, XPath: "/html/body/div[1]/table[2]/tbody[1]/tr[1]/td[3]/input[1]", Tag: "input", InputType: "checkbox", IsVisible: true, Rect: dom.Rect{Top: 200, Left: 180, Width: 20, Height: 20}},
	}
	for i := range elements {
		elements[i].Normalize()
	}

	ranked, strategy := resolveRestrictiveCandidates("7", "checkbox", dsl.ModeCheckbox, elements, nil, nil)
	if strategy != "restrictive-pass3" && strategy != "restrictive-pass3-row" {
		t.Fatalf("strategy = %q, want restrictive-pass3 or restrictive-pass3-row", strategy)
	}
	if len(ranked) == 0 || ranked[0].Element.ID != 3 {
		t.Fatalf("winner = %+v, want checkbox ID 3", ranked)
	}
}

func TestResolveRestrictiveCandidatesPrefersSameRowCheckbox(t *testing.T) {
	elements := []dom.ElementSnapshot{
		{ID: 1, XPath: "/html/body/table[1]/tbody[1]/tr[8]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 1200, Left: -9000, Width: 30, Height: 20}},
		{ID: 2, XPath: "/html/body/table[2]/tbody[1]/tr[2]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 200, Left: 100, Width: 30, Height: 20}},
		{ID: 3, XPath: "/html/body/table[2]/tbody[1]/tr[2]/td[4]/input[1]", Tag: "input", InputType: "checkbox", IsVisible: true, Rect: dom.Rect{Top: 200, Left: 180, Width: 20, Height: 20}},
		{ID: 4, XPath: "/html/body/div[1]/input[1]", Tag: "input", InputType: "checkbox", IsVisible: true, Rect: dom.Rect{Top: 1210, Left: -8960, Width: 20, Height: 20}},
	}
	for i := range elements {
		elements[i].Normalize()
	}

	ranked, _ := resolveRestrictiveCandidates("7", "checkbox", dsl.ModeCheckbox, elements, nil, nil)
	if len(ranked) == 0 || ranked[0].Element.ID != 3 {
		t.Fatalf("winner = %+v, want same-row checkbox ID 3", ranked)
	}
}

func TestIsGenericListContainer(t *testing.T) {
	for _, input := range []string{"list", "the list", "dropdown", "dropdown list", "listbox"} {
		if !isGenericListContainer(input) {
			t.Fatalf("expected %q to be treated as a generic list container", input)
		}
	}
	if isGenericListContainer("Country") {
		t.Fatal("did not expect Country to be treated as a generic list container")
	}
}

func TestRuntime_VerifyCheckedFailsForUncheckedControl(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/table[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 30, Height: 20}},
			{ID: 2, XPath: "/html/body/table[1]/tr[1]/td[4]/input[1]", Tag: "input", InputType: "checkbox", LabelText: "Select", IsVisible: true, IsChecked: false, Rect: dom.Rect{Top: 100, Left: 180, Width: 20, Height: 20}},
		},
	}
	for i := range mock.Elements {
		mock.Elements[i].Normalize()
	}

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	_, err := rt.executeCommand(context.Background(), dsl.Command{
		Type:        dsl.CmdVerifyField,
		Raw:         "VERIFY that '7' is checked",
		VerifyText:  "7",
		VerifyState: "checked",
	})
	if err == nil {
		t.Fatal("expected verify checked to fail for unchecked control")
	}
}

func TestRuntime_CheckThenVerifyCheckedPasses(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/table[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 30, Height: 20}},
			{ID: 2, XPath: "/html/body/table[1]/tr[1]/td[4]/input[1]", Tag: "input", InputType: "checkbox", LabelText: "Select", IsVisible: true, IsChecked: false, Rect: dom.Rect{Top: 100, Left: 180, Width: 20, Height: 20}},
		},
	}
	for i := range mock.Elements {
		mock.Elements[i].Normalize()
	}

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	_, err := rt.RunHunt(context.Background(), &dsl.Hunt{
		Commands: []dsl.Command{
			{Type: dsl.CmdCheck, Raw: "CHECK the checkbox for '7'", Target: "7", TypeHint: "checkbox"},
			{Type: dsl.CmdVerifyField, Raw: "VERIFY that '7' is checked", VerifyText: "7", VerifyState: "checked"},
		},
	})
	if err != nil {
		t.Fatalf("expected hunt to pass after checking control, got %v", err)
	}
	if !mock.Elements[1].IsChecked {
		t.Fatal("expected mock checkbox state to be updated by CHECK")
	}
}

func TestRuntime_ReconcileStickyCheckboxStateReappliesVisibleRow(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/table[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 30, Height: 20}},
			{ID: 2, XPath: "/html/body/table[1]/tr[1]/td[4]/input[1]", Tag: "input", InputType: "checkbox", LabelText: "Select", IsVisible: true, IsChecked: false, Rect: dom.Rect{Top: 100, Left: 180, Width: 20, Height: 20}},
		},
	}
	for i := range mock.Elements {
		mock.Elements[i].Normalize()
	}

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	rt.rememberStickyCheckboxState("7", true)
	if err := rt.reconcileStickyCheckboxStates(context.Background()); err != nil {
		t.Fatalf("reconcileStickyCheckboxStates failed: %v", err)
	}
	if !mock.Elements[1].IsChecked {
		t.Fatal("expected reconcile to reapply checked state for visible row")
	}
}

func TestRuntime_ClickPrefersExactAriaLabel(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/div[1]/button[1]", Tag: "button", AriaLabel: "Expand all", IsVisible: true, Rect: dom.Rect{Top: 20, Left: 20, Width: 24, Height: 24}},
			{ID: 2, XPath: "/html/body/div[1]/button[2]", Tag: "button", AriaLabel: "Collapse all", IsVisible: true, Rect: dom.Rect{Top: 20, Left: 70, Width: 24, Height: 24}},
		},
	}
	for i := range mock.Elements {
		mock.Elements[i].Normalize()
	}

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	_, err := rt.executeCommand(context.Background(), dsl.Command{
		Type:     dsl.CmdClick,
		Raw:      "CLICK the 'Expand all' button",
		Target:   "Expand all",
		TypeHint: "button",
	})
	if err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if len(mock.Clicks) != 1 {
		t.Fatalf("expected 1 click, got %d", len(mock.Clicks))
	}
	if mock.Clicks[0].X != 32 || mock.Clicks[0].Y != 32 {
		t.Fatalf("clicked (%v,%v), want center of Expand all button (32,32)", mock.Clicks[0].X, mock.Clicks[0].Y)
	}
}

func TestRuntime_ClickNearAnchorChoosesClosestCandidate(t *testing.T) {
	mock := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/div[1]/span[1]", Tag: "span", VisibleText: "Quantity", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 80, Height: 20}},
			{ID: 2, XPath: "/html/body/div[1]/button[1]", Tag: "button", VisibleText: "Increase", AriaLabel: "Increase Quantity", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 200, Width: 24, Height: 24}},
			{ID: 3, XPath: "/html/body/div[2]/button[1]", Tag: "button", VisibleText: "Increase", AriaLabel: "Increase Adults", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 500, Width: 24, Height: 24}},
		},
	}
	for i := range mock.Elements {
		mock.Elements[i].Normalize()
	}

	rt := New(config.Config{}, mock, utils.NewLogger(utils.LogLevelInfo, nil))
	_, err := rt.executeCommand(context.Background(), dsl.Command{
		Type:       dsl.CmdClick,
		Raw:        "CLICK the 'Increase' button NEAR 'Quantity'",
		Target:     "Increase",
		TypeHint:   "button",
		NearAnchor: "Quantity",
	})
	if err != nil {
		t.Fatalf("click with near anchor failed: %v", err)
	}
	if len(mock.Clicks) != 1 {
		t.Fatalf("expected 1 click, got %d", len(mock.Clicks))
	}
	if mock.Clicks[0].X != 212 || mock.Clicks[0].Y != 112 {
		t.Fatalf("clicked (%v,%v), want center of near candidate (212,112)", mock.Clicks[0].X, mock.Clicks[0].Y)
	}
}

type noOpCheckPage struct {
	*MockPage
}

func (p *noOpCheckPage) SetChecked(ctx context.Context, id int, xpath string, checked bool) error {
	return nil
}

func TestRuntime_CheckFailsWhenCheckboxStateDoesNotChange(t *testing.T) {
	base := &MockPage{
		Elements: []dom.ElementSnapshot{
			{ID: 1, XPath: "/html/body/table[1]/tr[1]/td[1]", Tag: "td", VisibleText: "7", IsVisible: true, Rect: dom.Rect{Top: 100, Left: 100, Width: 30, Height: 20}},
			{ID: 2, XPath: "/html/body/table[1]/tr[1]/td[4]/input[1]", Tag: "input", InputType: "checkbox", LabelText: "Select", IsVisible: true, IsChecked: false, Rect: dom.Rect{Top: 100, Left: 180, Width: 20, Height: 20}},
		},
	}
	for i := range base.Elements {
		base.Elements[i].Normalize()
	}

	rt := New(config.Config{}, &noOpCheckPage{MockPage: base}, utils.NewLogger(utils.LogLevelInfo, nil))
	_, err := rt.executeCommand(context.Background(), dsl.Command{
		Type:     dsl.CmdCheck,
		Raw:      "CHECK the checkbox for '7'",
		Target:   "7",
		TypeHint: "checkbox",
	})
	if err == nil {
		t.Fatal("expected CHECK to fail when checkbox state does not actually change")
	}
}
