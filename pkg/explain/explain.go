// Package explain provides structured explainability output for ManulHeart execution.
//
// Every DSL command execution produces an ExecutionResult which records the full
// targeting decision chain: what was discovered, how it was scored, why the
// winning candidate was chosen, and what action was taken.
package explain

import "time"

// ScoreBreakdown records the contribution of each scoring signal to the final score.
type ScoreBreakdown struct {
	// ExactTextMatch is the score contribution from exact visible-text matching.
	ExactTextMatch float64 `json:"exact_text_match"`
	// NormalizedTextMatch is the score from normalized (lowercased, trimmed) text.
	NormalizedTextMatch float64 `json:"normalized_text_match"`
	// LabelMatch is the score from associated <label> element text.
	LabelMatch float64 `json:"label_match"`
	// PlaceholderMatch is the score from input placeholder attribute.
	PlaceholderMatch float64 `json:"placeholder_match"`
	// AriaMatch is the score from aria-label and accessible-name derivation.
	AriaMatch float64 `json:"aria_match"`
	// DataQAMatch is the score from data-qa / data-testid attributes.
	DataQAMatch float64 `json:"data_qa_match"`
	// IDMatch is the score from html id attribute matching.
	IDMatch float64 `json:"id_match"`
	// TagSemantics is the score from tag/role alignment with the action mode.
	TagSemantics float64 `json:"tag_semantics"`
	// TypeHintAlignment is the score from element-type hint (button, link, field…).
	TypeHintAlignment float64 `json:"type_hint_alignment"`
	// VisibilityScore reflects whether the element was visible (1.0) or hidden (0.1).
	VisibilityScore float64 `json:"visibility_score"`
	// InteractabilityScore reflects whether the element is enabled/clickable/editable.
	InteractabilityScore float64 `json:"interactability_score"`
	// ProximityScore is the contextual proximity bonus.
	ProximityScore float64 `json:"proximity_score"`
	// Total is the final normalized score in [0.0, 1.0].
	Total float64 `json:"total"`
	// RawScore is the unclipped weighted sum used for sorting.
	// Not included in JSON output — internal ranking only.
	RawScore float64 `json:"-"`
}

// CandidateSignal is a human-readable reason assigned to a scoring signal.
type CandidateSignal struct {
	Signal string  `json:"signal"`
	Value  string  `json:"value"`
	Score  float64 `json:"score"`
}

// Candidate represents a single DOM element considered during target resolution.
type Candidate struct {
	// Rank is the 1-based position in the scored list (1 = best match).
	Rank int `json:"rank"`
	// XPath is the deterministic XPath of this element.
	XPath string `json:"xpath"`
	// Tag is the HTML tag name (lowercase).
	Tag string `json:"tag"`
	// Role is the ARIA role or inferred semantic role.
	Role string `json:"role,omitempty"`
	// VisibleText is the innerText-derived text of the element.
	VisibleText string `json:"visible_text,omitempty"`
	// AriaLabel is the aria-label attribute value.
	AriaLabel string `json:"aria_label,omitempty"`
	// Placeholder is the input placeholder value.
	Placeholder string `json:"placeholder,omitempty"`
	// DataQA is the data-qa attribute value.
	DataQA string `json:"data_qa,omitempty"`
	// ID is the html id attribute value.
	ID string `json:"id,omitempty"`
	// IsVisible reports whether the element was visible during probing.
	IsVisible bool `json:"is_visible"`
	// IsEnabled reports whether the element was enabled (not disabled).
	IsEnabled bool `json:"is_enabled"`
	// IsEditable reports whether the element accepts text input.
	IsEditable bool `json:"is_editable"`
	// Score is the normalized score breakdown.
	Score ScoreBreakdown `json:"score"`
	// Signals lists the individual signals that contributed to this candidate's score.
	Signals []CandidateSignal `json:"signals,omitempty"`
	// Chosen reports whether this candidate was selected for the action.
	Chosen bool `json:"chosen"`
}

// ExecutionResult is the complete structured result of executing one DSL command.
type ExecutionResult struct {
	// Step is the original DSL command text, as written in the .hunt file.
	Step string `json:"step"`
	// StepIndex is the 0-based index of this command within its STEP block.
	StepIndex int `json:"step_index"`
	// StepBlock is the STEP N label this command belongs to (empty if ungrouped).
	StepBlock string `json:"step_block,omitempty"`
	// CommandType is the classified command kind (navigate, click, fill, verify, wait…).
	CommandType string `json:"command_type"`
	// PageURL is the URL of the page at the time of execution.
	PageURL string `json:"page_url"`
	// TargetRequired reports whether element resolution was needed for this command.
	TargetRequired bool `json:"target_required"`
	// TargetQuery is the plain-English target expression from the DSL command.
	TargetQuery string `json:"target_query,omitempty"`
	// TypeHint is the element type hint extracted from the DSL (button, link, field…).
	TypeHint string `json:"type_hint,omitempty"`
	// CandidatesConsidered is the total number of elements evaluated by the pipeline.
	CandidatesConsidered int `json:"candidates_considered"`
	// RankedCandidates holds the top-N candidates with full score breakdowns.
	RankedCandidates []Candidate `json:"ranked_candidates,omitempty"`
	// WinnerXPath is the XPath of the selected element.
	WinnerXPath string `json:"winner_xpath,omitempty"`
	// WinnerScore is the normalized score of the selected element.
	WinnerScore float64 `json:"winner_score,omitempty"`
	// ActionPerformed describes what action was executed (e.g. "click", "fill", "navigate").
	ActionPerformed string `json:"action_performed"`
	// ActionValue holds the value used for fill/type actions.
	ActionValue string `json:"action_value,omitempty"`
	// Success reports whether the command completed without error.
	Success bool `json:"success"`
	// Error is the error message if Success is false.
	Error string `json:"error,omitempty"`
	// DurationMS is the wall-clock time (milliseconds) taken to execute this command.
	DurationMS int64 `json:"duration_ms"`
	// Duration is the original time.Duration (not serialized).
	Duration time.Duration `json:"-"`
	// ProbeMetadata contains optional debug metadata from the in-page JS probe.
	ProbeMetadata map[string]any `json:"probe_metadata,omitempty"`
}

// HuntResult is the aggregate result of running an entire .hunt file.
type HuntResult struct {
	// HuntFile is the path to the .hunt file that was run.
	HuntFile string `json:"hunt_file"`
	// Title is the @title: header value, if present.
	Title string `json:"title,omitempty"`
	// Context is the @context: header value, if present.
	Context string `json:"context,omitempty"`
	// TotalSteps is the number of DSL commands executed.
	TotalSteps int `json:"total_steps"`
	// Passed is the number of commands that succeeded.
	Passed int `json:"passed"`
	// Failed is the number of commands that failed.
	Failed int `json:"failed"`
	// Results holds the per-command execution results in order.
	Results []ExecutionResult `json:"results"`
	// TotalDurationMS is the wall-clock time (milliseconds) for the entire run.
	TotalDurationMS int64 `json:"total_duration_ms"`
	// TotalDuration is the original time.Duration (not serialized).
	TotalDuration time.Duration `json:"-"`
	// Success reports true if all commands passed.
	Success bool `json:"success"`
}
