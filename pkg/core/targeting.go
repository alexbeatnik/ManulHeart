// Package core implements the engine-core targeting pipeline for ManulHeart.
//
// This package owns:
//   - DOM interpretation and candidate extraction from JS probe results
//   - Text normalization
//   - Heuristic enrichment (via pkg/heuristics probes)
//   - Scoring (via pkg/scorer)
//   - Final target resolution
//   - Explainable result construction
//
// The critical architectural rule: no targeting logic lives in the browser
// backend. The Page interface provides raw page access; this package provides
// the intelligence that decides which element to act upon.
//
// Targeting pipeline:
//
//	1. Execute in-page JS heuristic probe (pkg/heuristics.SnapshotProbe)
//	   via Page.CallProbe — this is the FIRST and ONLY DOM query.
//	2. Deserialize the probe result into []dom.ElementSnapshot.
//	3. Normalize all text signals (el.Normalize()).
//	4. Score every candidate via pkg/scorer.Rank().
//	5. Select the top candidate if its score exceeds the threshold.
//	6. Build and return an explain.ExecutionResult with full reasoning chain.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dom"
	"github.com/manulengineer/manulheart/pkg/explain"
	"github.com/manulengineer/manulheart/pkg/heuristics"
	"github.com/manulengineer/manulheart/pkg/scorer"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// ResolvedTarget is the output of a successful targeting pipeline run.
type ResolvedTarget struct {
	// Element is the winning candidate element snapshot.
	Element *dom.ElementSnapshot
	// Score is the winning candidate's normalized score.
	Score float64
	// RankedCandidates holds the top-N candidates with full score breakdowns.
	RankedCandidates []scorer.RankedCandidate
	// TotalConsidered is the total number of candidates that were scored.
	TotalConsidered int
	// PageSnapshot is the full page snapshot from the probe.
	PageSnapshot *dom.PageSnapshot
}

// Targeting is the engine-core targeting pipeline.
type Targeting struct {
	cfg    config.Config
	logger *utils.Logger
}

// NewTargeting constructs a Targeting pipeline with the given config.
func NewTargeting(cfg config.Config, logger *utils.Logger) *Targeting {
	return &Targeting{cfg: cfg, logger: logger}
}

// Resolve runs the full targeting pipeline for a given page and query.
//
// This is the primary entry point for element resolution. It:
//  1. Invokes the in-page JS heuristic probe (FIRST page query)
//  2. Deserializes and normalizes candidates
//  3. Scores and ranks all candidates
//  4. Returns the best match with full explainability data
func (t *Targeting) Resolve(
	ctx context.Context,
	page browser.Page,
	query, typeHint, mode string,
) (*ResolvedTarget, error) {
	return t.ResolveWithContext(ctx, page, query, typeHint, mode, "")
}

// ResolveWithContext is like Resolve but accepts an optional nearAnchor for
// contextual NEAR-qualifier scoring.
func (t *Targeting) ResolveWithContext(
	ctx context.Context,
	page browser.Page,
	query, typeHint, mode, nearAnchor string,
) (*ResolvedTarget, error) {
	// Step 1: Execute in-page JS heuristic probe.
	// This is the first and primary DOM query — heuristics run here, not later.
	t.logger.Debug("targeting: probing page for %q (mode=%s, hint=%s, near=%q)", query, mode, typeHint, nearAnchor)

	probeArg := []any{mode, []string{strings.ToLower(query)}}

	// Retry up to 4 times (total ≤3 s) if the DOM probe returns 0 elements.
	// This handles post-navigation timing: the new page may not have finished
	// loading when a click that caused navigation returns immediately.
	const maxRetries = 4
	retryDelays := []time.Duration{300 * time.Millisecond, 600 * time.Millisecond, 1200 * time.Millisecond}
	var raw []byte
	var snapshot *dom.PageSnapshot
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			t.logger.Debug("targeting: 0 elements on attempt %d, waiting %v before retry", attempt, delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		raw, err = page.CallProbe(ctx, heuristics.SnapshotProbe(), probeArg)
		if err != nil {
			return nil, fmt.Errorf("core: page probe failed: %w", err)
		}
		snapshot, err = deserializeSnapshot(raw)
		if err != nil {
			return nil, fmt.Errorf("core: deserialize snapshot: %w", err)
		}
		if len(snapshot.Elements) > 0 {
			break
		}
		t.logger.Debug("targeting: probe attempt %d returned 0 elements (url=%s, mode=%s, query=%q)",
			attempt, snapshot.URL, mode, query)
	}

	if len(snapshot.Elements) == 0 {
		return nil, &utils.ResolutionError{
			Target:     query,
			Reason:     "no interactive elements found on page",
			Candidates: 0,
		}
	}

	t.logger.Debug("targeting: %d candidates found for %q", len(snapshot.Elements), query)

	// Step 3: Score and rank all candidates.
	// Normalization (el.Normalize()) is called inside scorer.Rank().
	// If a NEAR anchor is specified, resolve it first so the scorer can apply
	// proximity + DOM-ancestry + entity-word signals (matching ManulEngine).
	var nearCtx *scorer.AnchorContext
	if nearAnchor != "" {
		nearCtx = t.resolveNearAnchor(snapshot, nearAnchor)
		if nearCtx != nil {
			t.logger.Debug("targeting: NEAR anchor %q resolved at (%.0f,%.0f) xpath=%s",
				nearAnchor, nearCtx.Rect.Left, nearCtx.Rect.Top, nearCtx.XPath)
		} else {
			t.logger.Warn("targeting: NEAR anchor %q not resolved, ignoring", nearAnchor)
		}
	}
	ranked := scorer.Rank(query, typeHint, mode, snapshot.Elements, t.cfg.MaxCandidates, nearCtx)

	if len(ranked) == 0 {
		return nil, &utils.ResolutionError{
			Target:     query,
			Reason:     "no candidates survived scoring",
			Candidates: len(snapshot.Elements),
		}
	}

	best := ranked[0]
	t.logger.Debug("targeting: best candidate %q (xpath=%s, score=%.3f)",
		best.Element.VisibleText, best.Element.XPath, best.Explain.Score.Total)

	// Step 4: Apply score threshold.
	if best.Explain.Score.Total < t.cfg.ScoringThreshold {
		return nil, &utils.ResolutionError{
			Target:     query,
			Reason:     fmt.Sprintf("best score %.3f below threshold %.3f", best.Explain.Score.Total, t.cfg.ScoringThreshold),
			Candidates: len(snapshot.Elements),
			BestScore:  best.Explain.Score.Total,
		}
	}

	return &ResolvedTarget{
		Element:          best.Element,
		Score:            best.Explain.Score.Total,
		RankedCandidates: ranked,
		TotalConsidered:  len(snapshot.Elements),
		PageSnapshot:     snapshot,
	}, nil
}

// ProbeVisibleText runs the lightweight visible-text probe for VERIFY commands.
func (t *Targeting) ProbeVisibleText(ctx context.Context, page browser.Page) (string, string, error) {
	raw, err := page.CallProbe(ctx, heuristics.VisibleTextProbe(), nil)
	if err != nil {
		return "", "", fmt.Errorf("core: visible text probe: %w", err)
	}

	var result struct {
		URL  string `json:"url"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", fmt.Errorf("core: unmarshal text probe: %w", err)
	}
	return result.URL, result.Text, nil
}

// BuildCandidateExplain converts RankedCandidates to explain.Candidate slice.
func BuildCandidateExplain(ranked []scorer.RankedCandidate) []explain.Candidate {
	out := make([]explain.Candidate, len(ranked))
	for i, rc := range ranked {
		out[i] = rc.Explain
	}
	return out
}

// ── Deserialization ───────────────────────────────────────────────────────────

// probeElement is the raw JSON shape returned by the in-page probe.
type probeElement struct {
	ID          int           `json:"id"`
	XPath       string        `json:"xpath"`
	Tag         string        `json:"tag"`
	InputType   string        `json:"inputType"`
	VisibleText string        `json:"visibleText"`
	AriaLabel   string        `json:"ariaLabel"`
	Placeholder string        `json:"placeholder"`
	Title       string        `json:"title"`
	DataQA      string        `json:"dataQA"`
	DataTestID  string        `json:"dataTestId"`
	LabelText   string        `json:"labelText"`
	NameAttr    string        `json:"nameAttr"`
	HTMLId      string        `json:"htmlId"`
	ClassName   string        `json:"className"`
	Role        string        `json:"role"`
	Value       string        `json:"value"`
	IsVisible   bool          `json:"isVisible"`
	IsDisabled  bool          `json:"isDisabled"`
	IsHidden    bool          `json:"isHidden"`
	IsEditable  bool          `json:"isEditable"`
	IsInShadow  bool          `json:"isInShadow"`
	Rect        probeRect     `json:"rect"`
}

type probeRect struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Bottom float64 `json:"bottom"`
	Right  float64 `json:"right"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type probeResult struct {
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	VisibleText string         `json:"visibleText"`
	Elements    []probeElement `json:"elements"`
}

func deserializeSnapshot(raw []byte) (*dom.PageSnapshot, error) {
	var pr probeResult
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal probe result: %w", err)
	}

	elements := make([]dom.ElementSnapshot, len(pr.Elements))
	for i, pe := range pr.Elements {
		elements[i] = dom.ElementSnapshot{
			ID:          pe.ID,
			XPath:       pe.XPath,
			Tag:         strings.ToLower(pe.Tag),
			InputType:   strings.ToLower(pe.InputType),
			VisibleText: pe.VisibleText,
			AriaLabel:   pe.AriaLabel,
			Placeholder: pe.Placeholder,
			Title:       pe.Title,
			DataQA:      pe.DataQA,
			DataTestID:  pe.DataTestID,
			LabelText:   pe.LabelText,
			NameAttr:    pe.NameAttr,
			HTMLId:      pe.HTMLId,
			ClassName:   pe.ClassName,
			Role:        pe.Role,
			Value:       pe.Value,
			IsVisible:   pe.IsVisible,
			IsDisabled:  pe.IsDisabled,
			IsHidden:    pe.IsHidden,
			IsEditable:  pe.IsEditable,
			IsInShadow:  pe.IsInShadow,
			Rect: dom.Rect{
				Top:    pe.Rect.Top,
				Left:   pe.Rect.Left,
				Bottom: pe.Rect.Bottom,
				Right:  pe.Rect.Right,
				Width:  pe.Rect.Width,
				Height: pe.Rect.Height,
			},
		}
	}

	return &dom.PageSnapshot{
		URL:         pr.URL,
		Title:       pr.Title,
		VisibleText: pr.VisibleText,
		Elements:    elements,
	}, nil
}

// resolveNearAnchor finds the best anchor element for a NEAR qualifier and
// returns a scorer.AnchorContext with its rect, xpath, and tokenized words.
//
// Matches ManulEngine's _pick_near_anchor_candidate strategy:
//   - Score all elements against the anchor text.
//   - Among near-ties (within 15% of top score), prefer textual non-img
//     elements whose visible text actually contains the anchor string.
//   - Fall back to the raw top-scored candidate.
func (t *Targeting) resolveNearAnchor(snapshot *dom.PageSnapshot, nearAnchor string) *scorer.AnchorContext {
	needle := strings.ToLower(strings.TrimSpace(nearAnchor))

	type candidate struct {
		el    *dom.ElementSnapshot
		score float64
	}

	var candidates []candidate
	for i := range snapshot.Elements {
		el := &snapshot.Elements[i]
		el.Normalize()
		best := 0.0
		for _, sig := range el.AllTextSignals() {
			var sc float64
			if sig == needle {
				sc = 1.0
			} else if strings.Contains(sig, needle) {
				sc = float64(len(needle)) / float64(max(len(sig), 1))
			}
			if sc > best {
				best = sc
			}
		}
		if best >= 0.3 {
			candidates = append(candidates, candidate{el: el, score: best})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Build shortlist of near-ties (top 8 within 15% of best score).
	topScore := candidates[0].score
	var shortlist []candidate
	for _, c := range candidates {
		if len(shortlist) >= 8 {
			break
		}
		if c.score >= topScore-0.15 {
			shortlist = append(shortlist, c)
		}
	}

	// Among the shortlist: prefer non-img elements whose visible text contains
	// the anchor text (textual anchors are better geometric references).
	for _, c := range shortlist {
		if c.el.Tag != "img" && strings.Contains(c.el.NormText, needle) {
			return buildAnchorContext(c.el, nearAnchor)
		}
	}

	// Fallback: use the top candidate regardless.
	return buildAnchorContext(candidates[0].el, nearAnchor)
}

// buildAnchorContext constructs a scorer.AnchorContext from a resolved anchor element
// and the original anchor text. Words of ≥3 chars are extracted as entity tokens.
func buildAnchorContext(el *dom.ElementSnapshot, anchorText string) *scorer.AnchorContext {
	var words []string
	replacer := strings.NewReplacer(".", "", ",", "", ";", "", ":", "", "!", "", "?", "", "'", "", `"`, "")
	for _, w := range strings.Fields(strings.ToLower(anchorText)) {
		w = replacer.Replace(w)
		if len(w) >= 3 {
			words = append(words, w)
		}
	}
	return &scorer.AnchorContext{
		Rect:  el.Rect,
		XPath: el.XPath,
		Words: words,
	}
}
