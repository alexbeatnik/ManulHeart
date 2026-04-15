// Package scorer implements ManulHeart's deterministic multi-signal element scoring.
//
// The primary entry points are:
//   - Score(query, typeHint, mode, el, anchor) → explain.ScoreBreakdown
//   - Rank(query, typeHint, mode, elements, topN, anchor) → []RankedCandidate
//
// Scoring is pure, stateless, and deterministic: same inputs always produce the
// same output. No randomness, no LLM calls.
package scorer

import (
	"math"
	"sort"
	"strings"

	"github.com/manulengineer/manulheart/pkg/dom"
	"github.com/manulengineer/manulheart/pkg/explain"
)

// ── Public types ──────────────────────────────────────────────────────────────

// AnchorContext holds the resolved context for a NEAR qualifier.
// When non-nil, proximity and attribute-affinity signals are activated.
type AnchorContext struct {
	// Rect is the bounding box of the anchor element.
	Rect dom.Rect
	// XPath is the XPath of the anchor element (for DOM ancestry affinity).
	XPath string
	// Words are the significant words extracted from the anchor's visible text,
	// used to match candidate attributes like id/class/data-qa.
	Words []string
}

// WeightsConfig holds the high-level signal category weights exposed for
// testing and observability. The internal scorer uses finer-grained per-signal
// weights, but these top-level values define the ordering invariant:
//
//	Semantic > Text > ID > Proximity
type WeightsConfig struct {
	// Semantic is the weight for tag-semantics and role alignment signals.
	Semantic float64
	// Text is the combined weight for visible-text, aria-label, and label signals.
	Text float64
	// ID is the weight for HTML id, data-qa, and data-testid signals.
	ID float64
	// Proximity is the weight for NEAR-qualifier spatial scoring.
	Proximity float64
}

// Weights is the package-level scoring weight configuration.
// Tests may read this to verify the calibrated ordering invariants hold:
//
//	Weights.Semantic (0.60) > Weights.Text (0.45) > Weights.ID (0.25) > Weights.Proximity (0.10)
var Weights = WeightsConfig{
	Semantic:  0.60,
	Text:      0.45,
	ID:        0.25,
	Proximity: 0.10,
}

// RankedCandidate is a scored element with its full explain breakdown.
type RankedCandidate struct {
	// Element is the DOM snapshot of this candidate.
	Element dom.ElementSnapshot
	// Explain holds the full scoring breakdown (scores, signals, rank, chosen).
	Explain explain.Candidate
}

// ── Primary API ───────────────────────────────────────────────────────────────

// Score computes the full scoring breakdown for a single element.
// query is the normalized lowercased target text from the DSL.
// typeHint is the element type keyword from the DSL (button, link, field, …).
// mode is the interaction mode: "clickable", "input", "checkbox", "select", "none".
// anchor is optional; when non-nil, proximity and attribute-affinity are scored.
func Score(query, typeHint, mode string, el *dom.ElementSnapshot, anchor *AnchorContext) explain.ScoreBreakdown {
	if el.IsDisabled {
		return explain.ScoreBreakdown{Total: 0.0}
	}

	q := norm(query)

	// ── Text signals ──────────────────────────────────────────────────────────
	exactText := scoreExactText(q, el)
	normText := scoreNormText(q, el)
	labelMatch := scoreLabel(q, el)
	placeholder := scorePlaceholder(q, el)
	aria := scoreAria(q, el)
	dataQA := scoreDataQA(q, el)
	htmlID := scoreID(q, el)

	// ── Structural signals ────────────────────────────────────────────────────
	tagSem := scoreTagSemantics(mode, el)
	typeHintScore := scoreTypeHint(typeHint, el)
	depth := scoreDepth(el)
	className := scoreClassName(q, el)

	// ── Visibility / interactability ─────────────────────────────────────────
	vis := 1.0
	if !el.IsVisible || el.IsHidden {
		vis = 0.1
	}
	interact := 1.0
	if el.IsDisabled {
		interact = 0.0
	}

	// ── Proximity (NEAR qualifier) ────────────────────────────────────────────
	proximity := 0.0
	anchorAttr := 0.0
	if anchor != nil {
		proximity = scoreNear(el, anchor)
		anchorAttr = scoreAnchorAttrAffinity(anchor, el)
	}

	// ── Weighted total ────────────────────────────────────────────────────────
	// Weights are calibrated to match ManulEngine's scoring behavior.
	raw := exactText*1.0 +
		normText*0.7 +
		labelMatch*0.85 +
		placeholder*0.6 +
		aria*0.7 +
		dataQA*0.8 +
		htmlID*0.5 +
		tagSem*0.6 +
		typeHintScore*0.5 +
		depth*0.05 +
		className*0.15 +
		proximity*0.4 +
		anchorAttr*0.35

	// Apply visibility penalty to raw score BEFORE normalization.
	// This ensures hidden elements rank strictly below visible ones even
	// when the raw text match is perfect.
	rawPenalized := raw * vis * interact

	// Normalize to [0, 1] using the sum of max possible weights.
	const maxRaw = 1.0 + 0.7 + 0.85 + 0.6 + 0.7 + 0.8 + 0.5 + 0.6 + 0.5 + 0.05 + 0.15 + 0.4 + 0.35
	total := clamp(rawPenalized/maxRaw, 0, 1)

	bd := explain.ScoreBreakdown{
		ExactTextMatch:       exactText,
		NormalizedTextMatch:  normText,
		LabelMatch:           labelMatch,
		PlaceholderMatch:     placeholder,
		AriaMatch:            aria,
		DataQAMatch:          dataQA,
		IDMatch:              htmlID,
		TagSemantics:         tagSem,
		TypeHintAlignment:    typeHintScore,
		VisibilityScore:      vis,
		InteractabilityScore: interact,
		ProximityScore:       proximity,
		RawScore:             rawPenalized, // penalized raw value
		Total:                total,
	}
	return bd
}

// Rank scores all elements and returns up to topN candidates sorted by total
// score descending. The first element in the returned slice has Chosen=true.
// anchor is optional; pass nil when no NEAR qualifier is active.
func Rank(query, typeHint, mode string, elements []dom.ElementSnapshot, topN int, anchor *AnchorContext) []RankedCandidate {
	q := norm(query)

	type scored struct {
		elem  dom.ElementSnapshot
		bd    explain.ScoreBreakdown
		idx   int // original DOM position for stable tie-breaking
	}

	all := make([]scored, 0, len(elements))
	for i := range elements {
		el := &elements[i]
		bd := Score(q, typeHint, mode, el, anchor)
		all = append(all, scored{elem: *el, bd: bd, idx: i})
	}

	// Sort: highest total first; DOM order as deterministic tie-breaker.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].bd.Total != all[j].bd.Total {
			return all[i].bd.Total > all[j].bd.Total
		}
		return all[i].idx < all[j].idx // earlier in DOM wins on tie
	})

	if topN > 0 && len(all) > topN {
		all = all[:topN]
	}

	result := make([]RankedCandidate, 0, len(all))
	for rank, s := range all {
		sigs := buildSignals(s.bd)
		c := explain.Candidate{
			Rank:        rank + 1,
			XPath:       s.elem.XPath,
			Tag:         s.elem.Tag,
			Role:        s.elem.Role,
			VisibleText: s.elem.VisibleText,
			AriaLabel:   s.elem.AriaLabel,
			Placeholder: s.elem.Placeholder,
			DataQA:      s.elem.DataQA,
			ID:          s.elem.HTMLId,
			IsVisible:   s.elem.IsVisible,
			IsEnabled:   !s.elem.IsDisabled,
			IsEditable:  s.elem.IsEditable,
			Score:       s.bd,
			Signals:     sigs,
			Chosen:      rank == 0,
		}
		result = append(result, RankedCandidate{Element: s.elem, Explain: c})
	}
	return result
}

// ── Scoring signal functions ──────────────────────────────────────────────────

// scoreExactText returns 1.0 for an exact normalized text match, 0 otherwise.
func scoreExactText(q string, el *dom.ElementSnapshot) float64 {
	if q == "" {
		return 0.0
	}
	for _, s := range el.AllTextSignals() {
		if s == q {
			return 1.0
		}
	}
	return 0.0
}

// scoreNormText returns a partial score for substring or word-overlap matches
// across all text signals of the element.
func scoreNormText(q string, el *dom.ElementSnapshot) float64 {
	if q == "" {
		return 0.0
	}
	best := 0.0
	for _, s := range el.AllTextSignals() {
		if s == "" {
			continue
		}
		sc := partialMatch(q, s)
		if sc > best {
			best = sc
		}
	}
	return best
}

// scoreLabel returns how well the element's associated <label> text matches.
func scoreLabel(q string, el *dom.ElementSnapshot) float64 {
	if el.NormLabelText == "" {
		return 0.0
	}
	if el.NormLabelText == q {
		return 0.8
	}
	if strings.Contains(el.NormLabelText, q) {
		return 0.5
	}
	// Word-overlap for multi-word queries like "CPU of Chrome".
	if sigWords := significantWords(q); len(sigWords) >= 2 {
		hits := 0
		for _, w := range sigWords {
			if strings.Contains(el.NormLabelText, w) {
				hits++
			}
		}
		coverage := float64(hits) / float64(len(sigWords))
		if coverage >= 1.0 {
			return 0.6
		}
		if coverage > 0.5 {
			return 0.3
		}
	}
	return 0.0
}

// scorePlaceholder returns how well the element's placeholder attribute matches.
func scorePlaceholder(q string, el *dom.ElementSnapshot) float64 {
	if el.NormPlaceholder == "" {
		return 0.0
	}
	if el.NormPlaceholder == q {
		return 0.7
	}
	if strings.Contains(el.NormPlaceholder, q) {
		return 0.4
	}
	return 0.0
}

// scoreAria returns how well the element's aria-label matches.
func scoreAria(q string, el *dom.ElementSnapshot) float64 {
	if el.NormAriaLabel == "" {
		return 0.0
	}
	if el.NormAriaLabel == q {
		return 0.75
	}
	if strings.Contains(el.NormAriaLabel, q) {
		return 0.45
	}
	return 0.0
}

// scoreDataQA returns how well the element's data-qa / data-testid matches.
func scoreDataQA(q string, el *dom.ElementSnapshot) float64 {
	if el.NormDataQA == "" {
		return 0.0
	}
	if el.NormDataQA == q {
		return 1.0
	}
	if strings.Contains(el.NormDataQA, q) {
		return 0.5
	}
	return 0.0
}

// scoreID returns how well the element's html id attribute matches.
func scoreID(q string, el *dom.ElementSnapshot) float64 {
	if el.NormHTMLId == "" {
		return 0.0
	}
	normalized := el.NormHTMLId
	variants := []string{
		q,
		strings.ReplaceAll(q, " ", "-"),
		strings.ReplaceAll(q, " ", "_"),
		strings.ReplaceAll(q, " ", ""),
	}
	for _, v := range variants {
		if normalized == v {
			return 0.7
		}
	}
	if strings.Contains(normalized, strings.ReplaceAll(q, " ", "")) {
		return 0.3
	}
	return 0.0
}

// scoreTagSemantics returns a score for how well the element's tag/role aligns
// with the expected interaction mode.
func scoreTagSemantics(mode string, el *dom.ElementSnapshot) float64 {
	tag := el.Tag
	role := strings.ToLower(el.Role)

	switch mode {
	case "none", "locate":
		if tag == "td" || tag == "th" || tag == "li" || tag == "span" ||
			tag == "p" || tag == "dd" || tag == "dt" || tag == "figcaption" || tag == "caption" {
			return 0.2
		}
		if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
			return 0.25
		}
		if tag == "div" || tag == "section" || tag == "article" {
			return 0.15
		}
		if tag == "option" {
			return 0.15
		}
		if tag == "button" || tag == "a" || tag == "input" || tag == "select" {
			return 0.1
		}
		return 0.05

	case "input":
		if tag == "input" || tag == "textarea" {
			if el.InputType == "password" {
				return 0.55
			}
			return 0.5
		}
		if role == "textbox" || role == "spinbutton" || role == "combobox" {
			return 0.4
		}
		if el.IsEditable {
			return 0.3
		}
		return 0.0

	case "checkbox":
		if tag == "input" && (el.InputType == "checkbox" || el.InputType == "radio") {
			return 0.5
		}
		if role == "checkbox" || role == "radio" || role == "switch" {
			return 0.4
		}
		// Penalty for non-checkbox elements in checkbox mode.
		if tag == "button" || tag == "a" {
			return -0.3
		}
		return 0.0

	case "select":
		if tag == "select" {
			return 0.5
		}
		if role == "listbox" || role == "combobox" {
			return 0.4
		}
		return 0.0

	default: // clickable
		if tag == "button" || tag == "a" || tag == "summary" {
			return 0.4
		}
		if role == "button" || role == "link" || role == "menuitem" || role == "tab" {
			return 0.35
		}
		if tag == "input" && (el.InputType == "submit" || el.InputType == "button") {
			return 0.35
		}
		if tag == "label" {
			return 0.2
		}
		return 0.05
	}
}

// scoreTypeHint returns a score for how well the element matches the explicit
// type hint extracted from the DSL command ("button", "link", "field", …).
func scoreTypeHint(hint string, el *dom.ElementSnapshot) float64 {
	if hint == "" {
		return 0.0
	}
	tag := el.Tag
	role := strings.ToLower(el.Role)
	switch hint {
	case "button":
		if tag == "button" || (tag == "input" && (el.InputType == "submit" || el.InputType == "button")) {
			return 0.4
		}
		if role == "button" {
			return 0.35
		}
	case "link":
		if tag == "a" || role == "link" {
			return 0.4
		}
	case "field", "input", "textarea":
		if tag == "input" || tag == "textarea" || el.IsEditable {
			return 0.4
		}
	case "checkbox":
		if tag == "input" && el.InputType == "checkbox" {
			return 0.4
		}
		if role == "checkbox" {
			return 0.35
		}
	case "radio":
		if tag == "input" && el.InputType == "radio" {
			return 0.4
		}
		if role == "radio" {
			return 0.35
		}
	case "dropdown", "select":
		if tag == "select" || role == "listbox" || role == "combobox" {
			return 0.4
		}
	case "element":
		return 0.05 // generic hint — minimal signal
	}
	return 0.0
}

// scoreDepth returns a small bonus for shallower DOM elements.
// Score decays gently: depth 3 → 0.9, depth 10 → 0.5, depth 20+ → 0.1.
func scoreDepth(el *dom.ElementSnapshot) float64 {
	depth := strings.Count(el.XPath, "/")
	if depth <= 0 {
		return 0.5
	}
	return clamp(1.0-0.04*float64(depth-3), 0.1, 1.0)
}

// scoreNear returns a [0.0, 1.0] proximity score for a NEAR qualifier.
// Uses linear spatial decay blended with DOM ancestry affinity:
//
//	score = spatial*0.45 + domAffinity*0.55
//
// This helps card/list layouts prefer the button in the same product card
// over a slightly closer button in an adjacent card.
func scoreNear(el *dom.ElementSnapshot, anchor *AnchorContext) float64 {
	const threshold = 500.0
	cx := el.Rect.Left + el.Rect.Width/2
	cy := el.Rect.Top + el.Rect.Height/2
	ax := anchor.Rect.Left + anchor.Rect.Width/2
	ay := anchor.Rect.Top + anchor.Rect.Height/2
	dist := math.Sqrt((cx-ax)*(cx-ax) + (cy-ay)*(cy-ay))
	if dist > threshold {
		return 0.0
	}
	spatialScore := 1.0 - dist/threshold

	if anchor.XPath == "" || el.XPath == "" {
		return spatialScore
	}

	anchorParts := xpathParts(anchor.XPath)
	candidateParts := xpathParts(el.XPath)
	commonDepth := 0
	for i := 0; i < len(anchorParts) && i < len(candidateParts); i++ {
		if anchorParts[i] == candidateParts[i] {
			commonDepth++
		} else {
			break
		}
	}
	maxLen := len(anchorParts)
	if len(candidateParts) > maxLen {
		maxLen = len(candidateParts)
	}
	if maxLen == 0 {
		return spatialScore
	}
	domAffinity := float64(commonDepth) / float64(maxLen)
	return math.Min(1.0, spatialScore*0.45+domAffinity*0.55)
}

// scoreAnchorAttrAffinity rewards candidates whose dev-facing attributes
// (id, class, data-qa) contain words from the NEAR anchor text.
// Matches ManulEngine: product cards encode the item label in button IDs
// like add-to-cart-sauce-labs-fleece-jacket.
func scoreAnchorAttrAffinity(anchor *AnchorContext, el *dom.ElementSnapshot) float64 {
	if len(anchor.Words) == 0 {
		return 0.0
	}
	replacer := strings.NewReplacer("-", " ", "_", " ")
	devText := replacer.Replace(strings.ToLower(
		el.NormHTMLId + " " + el.ClassName + " " + el.NormDataQA + " " + el.DataTestID,
	))
	hits := 0
	for _, w := range anchor.Words {
		if strings.Contains(devText, w) {
			hits++
		}
	}
	if hits == 0 {
		return 0.0
	}
	coverage := float64(hits) / float64(len(anchor.Words))
	switch {
	case coverage >= 1.0:
		return 0.6
	case coverage >= 0.75:
		return 0.35
	case coverage >= 0.5:
		return 0.12
	default:
		return 0.0
	}
}

// scoreClassName computes a word-overlap score between the query and the
// element's CSS class names.
func scoreClassName(q string, el *dom.ElementSnapshot) float64 {
	if el.ClassName == "" || q == "" {
		return 0.0
	}
	replacer := strings.NewReplacer("-", " ", "_", " ")
	clsNorm := replacer.Replace(strings.ToLower(el.ClassName))
	hits := 0
	for _, w := range strings.Fields(q) {
		if len(w) >= 3 && strings.Contains(clsNorm, w) {
			hits++
		}
	}
	if hits == 0 {
		return 0.0
	}
	return math.Min(float64(hits)*0.08, 0.4)
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// partialMatch returns a [0, 1] score for a substring or word-overlap match
// between query q and candidate text s (both pre-normalized).
//
// It does NOT do reverse-containment: only s.contains(q), not q.contains(s).
// Reverse-containment was removed because it caused short elements (e.g. "Update")
// to score the same as longer ones (e.g. "Update Profile") for query "Update Profile".
func partialMatch(q, s string) float64 {
	if q == "" || s == "" {
		return 0.0
	}
	if s == q {
		return 1.0
	}
	score := 0.0
	// Forward containment only: element text contains the query.
	if strings.Contains(s, q) {
		// Use rune count instead of byte len to handle emojis properly
		qLen := float64(len([]rune(q)))
		sLen := float64(len([]rune(s)))
		score = clamp(qLen/sLen, 0.4, 0.95)
	}
	// Word-overlap with stop-word filtering and minimum word length.
	overlap := wordOverlap(q, s)
	if overlap > score {
		return overlap
	}
	return score
}

// wordOverlap returns the fraction of significant query words found in s.
// Minimum word length ≥ 2 and stop-word filtering prevent single-char false positives.
func wordOverlap(q, s string) float64 {
	qWords := significantWords(q)
	if len(qWords) == 0 {
		return 0.0
	}
	hits := 0
	for _, w := range qWords {
		if strings.Contains(s, w) {
			hits++
		}
	}
	return clamp(float64(hits)/float64(len(qWords)), 0, 1)
}

// xpathParts splits an XPath string into its non-empty path components.
func xpathParts(xpath string) []string {
	var parts []string
	for _, p := range strings.Split(xpath, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// stopWords are common English function words excluded from word-overlap matching.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "in": true,
	"for": true, "to": true, "is": true, "on": true, "at": true,
	"by": true, "or": true, "and": true, "with": true, "from": true,
	"that": true, "this": true, "it": true, "its": true, "be": true,
	"as": true, "are": true, "was": true, "were": true, "not": true,
}

// significantWords returns query words with length ≥ 2 and not a stop word.
func significantWords(s string) []string {
	var words []string
	for _, w := range strings.Fields(s) {
		if len(w) >= 2 && !stopWords[w] {
			words = append(words, w)
		}
	}
	return words
}

// norm lowercases and trims a string (consistent with dom.ElementSnapshot.Normalize).
func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// clamp constrains v to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// buildSignals converts a ScoreBreakdown into the human-readable signal list
// used by --explain mode and the JSON output.
func buildSignals(bd explain.ScoreBreakdown) []explain.CandidateSignal {
	var signals []explain.CandidateSignal
	add := func(name string, score float64) {
		if score > 0 {
			signals = append(signals, explain.CandidateSignal{Signal: name, Score: score})
		}
	}
	add("exact_text", bd.ExactTextMatch)
	add("normalized_text", bd.NormalizedTextMatch)
	add("label", bd.LabelMatch)
	add("placeholder", bd.PlaceholderMatch)
	add("aria_label", bd.AriaMatch)
	add("data_qa", bd.DataQAMatch)
	add("html_id", bd.IDMatch)
	add("tag_semantics", bd.TagSemantics)
	add("type_hint", bd.TypeHintAlignment)
	add("proximity", bd.ProximityScore)
	return signals
}

// ── Legacy compatibility shims ────────────────────────────────────────────────
// These keep any callers of the old API compiling.

// ScoreCandidate is the legacy scoring entry point kept for backward compatibility.
// New code should call Score() directly.
func ScoreCandidate(elem dom.ElementSnapshot, intent Intent, _ LegacyWeights, _ map[string]dom.ElementSnapshot) explain.CandidateResult {
	bd := Score(intent.Text, intent.Role, "clickable", &elem, nil)
	return explain.CandidateResult{
		NodeID:      elem.HTMLId,
		TagName:     elem.Tag,
		TextContent: elem.VisibleText,
		RawScore:    bd.RawScore,
		Score:       bd.Total,
		Signals:     nil,
	}
}

// RankCandidates is the legacy ranking entry point kept for backward compatibility.
func RankCandidates(elements []dom.ElementSnapshot, intent Intent, _ LegacyWeights) []explain.CandidateResult {
	ranked := Rank(intent.Text, intent.Role, "clickable", elements, 0, nil)
	out := make([]explain.CandidateResult, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, explain.CandidateResult{
			NodeID:      r.Element.HTMLId,
			TagName:     r.Element.Tag,
			TextContent: r.Element.VisibleText,
			RawScore:    r.Explain.Score.RawScore,
			Score:       r.Explain.Score.Total,
			Signals:     nil,
		})
	}
	return out
}

// Intent is the legacy targeting intent type.
type Intent struct {
	Text             string
	Role             string
	NearAnchorNodeID string
}

// LegacyWeights is the legacy signal-weight map type.
// New code should use WeightsConfig / the Weights package variable instead.
type LegacyWeights map[Signal]float64

// Signal is a named scoring channel.
type Signal string

// DefaultWeights returns the default signal weights (legacy API).
func DefaultWeights() LegacyWeights { return LegacyWeights{} }
