// Package scorer implements the deterministic, normalized scoring engine
// for ManulHeart candidate ranking.
//
// The scorer assigns a score in [0.0, 1.0] to each DOM candidate element
// relative to a target query. Multiple scoring channels are computed
// independently and combined with channel weights. The final score is
// normalized to [0.0, 1.0].
//
// Scoring channels (matching ManulEngine's architecture):
//
//	Text     (weight 0.45) — exact/normalized text, aria-label, placeholder, data-qa, name
//	ID       (weight 0.25) — html id, data-testid variants
//	Semantic (weight 0.60) — tag/role alignment with interaction mode, type-hint match
//	Penalty  (multiplier ) — disabled ×0.0, hidden ×0.1, normal ×1.0
//	Proximity(weight 0.10) — DOM depth / contextual proximity
package scorer

import (
	"math"
	"sort"
	"strings"

	"github.com/manulengineer/manulheart/pkg/dom"
	"github.com/manulengineer/manulheart/pkg/explain"
)

// Weights define the relative contribution of each scoring channel.
var Weights = struct {
	Text      float64
	ID        float64
	Semantic  float64
	Proximity float64
}{
	Text:      0.45,
	ID:        0.25,
	Semantic:  0.60,
	Proximity: 0.10,
}

// AnchorContext holds the fully-resolved context for a NEAR qualifier.
// It matches ManulEngine's approach: Euclidean pixel distance is blended
// with DOM ancestry affinity, and anchor word tokens enable entity-level
// dev-attribute matching (e.g. add-to-cart-sauce-labs-fleece-jacket).
type AnchorContext struct {
	// Rect is the bounding box of the resolved anchor element.
	Rect dom.Rect
	// XPath is the deterministic XPath of the anchor element.
	// Used to compute DOM ancestry affinity.
	XPath string
	// Words are lower-case ≥3-char tokens from the anchor text.
	// Used for entity-level dev-attribute affinity scoring.
	Words []string
}

// Score computes the normalized score breakdown for a single candidate element
// against a normalized target query string and mode.
//
//   - query   — lowercase target text extracted from the DSL command
//   - typeHint — optional element-type hint (button, link, field, …)
//   - mode    — interaction mode (clickable, input, checkbox, select)
//   - el      — the candidate element snapshot
//   - anchor  — optional resolved NEAR anchor context (nil = no NEAR)
func Score(query, typeHint, mode string, el *dom.ElementSnapshot, anchor *AnchorContext) explain.ScoreBreakdown {
	q := strings.ToLower(strings.TrimSpace(query))

	bd := explain.ScoreBreakdown{}

	// ── Text channel ──────────────────────────────────────────────────
	bd.ExactTextMatch = scoreExactText(q, el)
	bd.NormalizedTextMatch = scoreNormText(q, el)
	bd.LabelMatch = scoreLabelText(q, el)
	bd.PlaceholderMatch = scorePlaceholder(q, el)
	bd.AriaMatch = scoreAria(q, el)
	bd.DataQAMatch = scoreDataQA(q, el)

	textRaw := bd.ExactTextMatch + bd.NormalizedTextMatch + bd.LabelMatch +
		bd.PlaceholderMatch + bd.AriaMatch + bd.DataQAMatch

	// ── ID channel ────────────────────────────────────────────────────
	bd.IDMatch = scoreID(q, el)
	anchorAttr := 0.0
	if anchor != nil {
		anchorAttr = scoreAnchorAttrAffinity(anchor, el)
	}
	idRaw := bd.IDMatch + scoreClassName(q, el)

	// ── Semantic channel ──────────────────────────────────────────────
	bd.TagSemantics = scoreTagSemantics(mode, el)
	bd.TypeHintAlignment = scoreTypeHint(typeHint, el)
	semanticRaw := bd.TagSemantics + bd.TypeHintAlignment

	// ── Visibility & interactability ──────────────────────────────────
	if el.IsVisible {
		bd.VisibilityScore = 1.0
	} else {
		bd.VisibilityScore = 0.1
	}
	if !el.IsDisabled {
		bd.InteractabilityScore = 1.0
	} else {
		bd.InteractabilityScore = 0.0
	}

	// ── Proximity ─────────────────────────────────────────────────────
	// NEAR: linear decay to 500 px hard cutoff, blended with DOM ancestry
	// affinity (spatial*0.45 + domAffinity*0.55) — matching ManulEngine.
	// Anchor attr affinity is added into the proximity score (capped at 1.0)
	// so it competes directly with spatial distance with the full 1.5 weight
	// boost — this prevents physically-adjacent cards from beating the correct
	// in-card button when the card is tall.
	// Weight is boosted to 1.5 when contextual hint is active.
	proximityWeight := Weights.Proximity
	if anchor != nil {
		near := scoreNear(el, anchor)
		bd.ProximityScore = math.Min(1.0, near+anchorAttr)
		proximityWeight = 1.5
	} else {
		bd.ProximityScore = scoreDepth(el)
	}

	// ── Penalty multiplier ────────────────────────────────────────────
	penalty := 1.0
	if el.IsDisabled {
		penalty = 0.0
	} else if el.IsHidden {
		penalty = 0.1
	}

	// ── Combine channels ──────────────────────────────────────────────
	weighted := textRaw*Weights.Text +
		idRaw*Weights.ID +
		semanticRaw*Weights.Semantic +
		bd.ProximityScore*proximityWeight

	// Apply penalty: raw for sorting, clamped for display.
	raw := weighted * penalty
	bd.RawScore = raw
	bd.Total = clamp(raw, 0.0, 1.0)

	return bd
}

// RankedCandidate pairs a scored element with its explain.Candidate.
type RankedCandidate struct {
	Element  *dom.ElementSnapshot
	Explain  explain.Candidate
}

// Rank scores all elements against the query and returns them sorted by score descending.
// At most topN candidates are returned in the result (all are scored regardless).
// anchor is the fully-resolved NEAR anchor context (nil = no contextual proximity).
func Rank(query, typeHint, mode string, elements []dom.ElementSnapshot, topN int, anchor *AnchorContext) []RankedCandidate {
	type scored struct {
		el    *dom.ElementSnapshot
		score explain.ScoreBreakdown
	}

	all := make([]scored, len(elements))
	for i := range elements {
		el := &elements[i]
		el.Normalize()
		all[i] = scored{el: el, score: Score(query, typeHint, mode, el, anchor)}
	}

	// Sort by unclipped RawScore so that NEAR/attr differentiation survives
	// when all candidates saturate the [0,1] clamped Total.
	sort.Slice(all, func(i, j int) bool {
		return all[i].score.RawScore > all[j].score.RawScore
	})

	limit := len(all)
	if topN > 0 && topN < limit {
		limit = topN
	}

	result := make([]RankedCandidate, limit)
	for i := 0; i < limit; i++ {
		s := all[i]
		signals := buildSignals(s.score)
		result[i] = RankedCandidate{
			Element: s.el,
			Explain: explain.Candidate{
				Rank:        i + 1,
				XPath:       s.el.XPath,
				Tag:         s.el.Tag,
				Role:        s.el.Role,
				VisibleText: s.el.VisibleText,
				AriaLabel:   s.el.AriaLabel,
				Placeholder: s.el.Placeholder,
				DataQA:      s.el.DataQA,
				ID:          s.el.HTMLId,
				IsVisible:   s.el.IsVisible,
				IsEnabled:   !s.el.IsDisabled,
				IsEditable:  s.el.IsEditable,
				Score:       s.score,
				Signals:     signals,
				Chosen:      i == 0,
			},
		}
	}

	// Only the top candidate is "chosen"
	if len(result) > 1 {
		for i := 1; i < len(result); i++ {
			result[i].Explain.Chosen = false
		}
	}

	return result
}

// ── Individual scoring functions ──────────────────────────────────────────────

func scoreExactText(q string, el *dom.ElementSnapshot) float64 {
	for _, s := range el.AllTextSignals() {
		if s == q {
			return 1.0
		}
	}
	return 0.0
}

func scoreNormText(q string, el *dom.ElementSnapshot) float64 {
	best := 0.0
	for _, s := range el.AllTextSignals() {
		// Substring containment
		if strings.Contains(s, q) || strings.Contains(q, s) {
			sc := substringScore(q, s)
			if sc > best {
				best = sc
			}
		}
	}
	return best
}

func scoreLabelText(q string, el *dom.ElementSnapshot) float64 {
	if el.NormLabelText == "" {
		return 0.0
	}
	if el.NormLabelText == q {
		return 0.8
	}
	if strings.Contains(el.NormLabelText, q) {
		return 0.5
	}
	return 0.0
}

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

func scoreID(q string, el *dom.ElementSnapshot) float64 {
	if el.NormHTMLId == "" {
		return 0.0
	}
	normalized := el.NormHTMLId
	// Check exact, with-dashes, with-underscores variants
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
	case "input":
		if tag == "input" || tag == "textarea" {
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
		// Penalty for non-checkbox in checkbox mode
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

// scoreTypeHint returns a score for how well the element matches the explicit type hint.
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

// scoreDepth returns a small proximity score based on XPath depth.
// Shallower elements in the DOM get a slight bonus.
func scoreDepth(el *dom.ElementSnapshot) float64 {
	depth := strings.Count(el.XPath, "/")
	if depth <= 0 {
		return 0.5
	}
	// Score decays gently: depth 3 → 0.9, depth 10 → 0.5, depth 20+ → 0.1
	return clamp(1.0-0.04*float64(depth-3), 0.1, 1.0)
}

// scoreNear returns a [0.0, 1.0] proximity score for a NEAR qualifier,
// matching ManulEngine's _score_proximity implementation:
//   - Linear decay: max(0, 1 - dist/500) with hard cutoff at 500 px.
//   - Blended with DOM ancestry affinity when both XPaths are available:
//     score = spatial*0.45 + domAffinity*0.55
//   This helps card/list layouts prefer the button in the same product card
//   over a slightly closer button in an adjacent card.
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

	// Compute XPath common-prefix depth ratio (DOM ancestry affinity).
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
// element's CSS class names. Matches ManulEngine's context-word scoring on
// dev attributes (cls_n variable in _score_attributes), capped at 0.4.
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

// ── Utilities ─────────────────────────────────────────────────────────────────

func substringScore(query, candidate string) float64 {
	if query == candidate {
		return 0.9
	}
	ql, cl := float64(len(query)), float64(len(candidate))
	if ql == 0 || cl == 0 {
		return 0.0
	}
	ratio := math.Min(ql, cl) / math.Max(ql, cl)
	return ratio * 0.5
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

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
	return signals
}
