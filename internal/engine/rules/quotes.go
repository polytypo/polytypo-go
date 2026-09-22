package rules

import (
	"sort"

	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/quotes.md (spec 0.5.0), order 40.
//
// Mandate 1 (every existing quote glyph is a re-typesetting candidate) and mandate 2 (a space
// touching a quote mark is sloppiness, not evidence) are this rule's whole architecture. Five
// passes plus an emit; no backtracking inside a pass, no regular expression, no native-string
// indexing (ARCHITECTURE.md 4.1, 4.2).

// qtWide is quotes.md 3.1 WIDE. NARROW is qaNarrow (quote_ambiguity.go), shared with apostrophe
// so the two rules cannot define two slightly different NARROW sets. WIDE and NARROW are
// disjoint and their union is QUOTEMARK.
var qtWide = map[rune]bool{
	0x22:   true,
	0xAB:   true,
	0xBB:   true,
	0x201C: true,
	0x201D: true,
	0x201E: true,
	0x201F: true,
	0x301D: true,
	0x301E: true,
	0x301F: true,
}

func qtIsQuoteMark(cp rune) bool {
	return qtWide[cp] || qaNarrow[cp]
}

func qtIsDigit(cp rune) bool {
	return cp >= 0x30 && cp <= 0x39
}

func qtIsAlnum(cp rune) bool {
	return qtIsDigit(cp) || engine.IsLetter(cp)
}

// qtIsBreak is quotes.md 3.1 BREAK, including engine.LineMarker: a member of BREAK for every
// rule, everywhere (modes.md 3.2).
func qtIsBreak(cp rune) bool {
	switch cp {
	case 0x0A, 0x0D, 0x0B, 0x0C, 0x85, 0x2028, 0x2029, engine.LineMarker:
		return true
	default:
		return false
	}
}

// qtIsSpacelike is quotes.md 3.1 SPACELIKE = INLINE-SPACE ∪ BREAK.
func qtIsSpacelike(cp rune) bool {
	return qaInlineSpace[cp] || qtIsBreak(cp)
}

// qtIsOpenish is quotes.md 3.1 OPENISH. QUOTEMARK is a member of both OPENISH and CLOSEISH, and
// is exempt from canOpen's closeish rejection — Lemma A's entire mechanism (quotes.md 5) and the
// reason a candidate's verdict never depends on which quote glyph its neighbour is. Do not
// "simplify" this back to per-glyph lists. engine.Marker is a member too (modes.md 3.3).
func qtIsOpenish(cp rune) bool {
	if cp == engine.Marker {
		return true
	}
	switch cp {
	case 0x28, 0x5B, 0x7B:
		return true
	}
	return qtIsQuoteMark(cp)
}

// qtIsCloseish is quotes.md 3.1 CLOSEISH — see qtIsOpenish's dual-membership note.
func qtIsCloseish(cp rune) bool {
	if cp == engine.Marker {
		return true
	}
	switch cp {
	case 0x29, 0x5D, 0x7D, 0x2C, 0x2E, 0x3B, 0x3A, 0x21, 0x3F, 0x2026, 0x2013, 0x2014:
		return true
	}
	return qtIsQuoteMark(cp)
}

func qtIsDashish(cp rune) bool {
	switch cp {
	case 0x2D, 0x2011, 0x2013, 0x2014:
		return true
	default:
		return false
	}
}

// qtIsDeleteLanding is quotes.md 3.1 DELETE-LANDING — the largest landing class for which every
// earlier-ordered rule's own classes are unaffected by a quote glyph or a U+0020 (quotes.md 3.7,
// composition obligation).
func qtIsDeleteLanding(cp rune) bool {
	return qtIsAlnum(cp) || qtIsQuoteMark(cp)
}

// qtV1Identity is quotes.md 3.2 V1ID (spec 0.4.1) — a conservative over-approximation, not a
// claim that every U+0027 becomes U+2019. apostrophe only ever emits U+2019 for a U+0027, but its
// own case ladder leaves some U+0027s unedited (the prime guard, and "nothing inferable"), and
// quotes cannot know which without re-deriving apostrophe's verdict against quotes' own
// not-yet-final output — circular. V1ID treats every U+0027 as possibly about to become U+2019,
// and every U+2019 as possibly a U+0027 that already did, so V1's comparison stays invariant
// across the two rules running in sequence on successive pipeline passes (quotes.md 5, Lemma A).
func qtV1Identity(cp rune) rune {
	if cp == 0x27 {
		return 0x2019
	}
	return cp
}

// qtGlyphCodePoint decodes a locale-declared quote glyph (guaranteed by schema to be exactly one
// code point) to its rune value.
func qtGlyphCodePoint(s string) rune {
	return []rune(s)[0]
}

type qtCandidate struct {
	index    int
	wide     bool
	canOpen  bool
	canClose bool
}

type qtPair struct {
	open  int
	close int
}

type qtSkipSets struct {
	spaceRight map[rune]bool
	spaceLeft  map[rune]bool
}

// qtComputeSkipSets is quotes.md 3.1a — locale-derived skip sets, computed once per call. These
// are exactly the positions at which nbsp's N8 can insert a space (nbsp.md 3.10), which is what
// makes Lemma B's coverage exact rather than a survey.
func qtComputeSkipSets(locale spec.LocaleData) qtSkipSets {
	spaceRight := map[rune]bool{}
	spaceLeft := map[rune]bool{}
	for _, pair := range []spec.QuotePair{locale.Quotes.Primary, locale.Quotes.Secondary} {
		if pair.InnerSpace == "none" {
			continue
		}
		open := qtGlyphCodePoint(pair.Open)
		closeCP := qtGlyphCodePoint(pair.Close)
		if open == closeCP {
			continue
		}
		spaceRight[open] = true
		spaceLeft[closeCP] = true
	}
	return qtSkipSets{spaceRight: spaceRight, spaceLeft: spaceLeft}
}

// qtSkipLeft is the straight-line walk of quotes.md 3.2: step outward across a maximal
// INLINE-SPACE run. MARKER and every BREAK stop it, because neither is in INLINE-SPACE.
func qtSkipLeft(cp []rune, i int) rune {
	j := i - 1
	for j >= 0 && qaInlineSpace[cp[j]] {
		j--
	}
	if j >= 0 {
		return cp[j]
	}
	return engine.None
}

func qtSkipRight(cp []rune, i int) rune {
	n := len(cp)
	j := i + 1
	for j < n && qaInlineSpace[cp[j]] {
		j++
	}
	if j < n {
		return cp[j]
	}
	return engine.None
}

// qtCollectCandidates is pass 1 (quotes.md 3.2) — collect and classify candidates. canOpen
// always skips right and canClose always skips left (mandate 2's inner-side skip); the outer
// side skips only when nbsp can reach it (the locale-derived spaceRight/spaceLeft sets), which
// is what keeps every verdict inert to nbsp (Lemma B).
func qtCollectCandidates(cp []rune, skip qtSkipSets, idioms []spec.ElisionIdiom, clitics spec.ElisionClitics) []qtCandidate {
	n := len(cp)
	var candidates []qtCandidate

	// spec 0.5.0: the veto set is the UNION of the cited-idiom match (unchanged since 0.4.0) and
	// the general ambiguous-medial-span shape (quotes.md 3.2a) — quotes must decline pairing for
	// both, so apostrophe's own case ladder never independently "fixes" a shape quotes left
	// alone.
	// spec 1.4.0 adds a third member to the same union: the span-boundary elision veto, which
	// fires only where one literal neighbour is the inline Marker (quotes.md 3.2).
	idiomMatched := qaComputeIdiomMatchedIndices(cp, idioms)
	ambiguousShape := qaComputeAmbiguousShapeIndices(cp)
	spanBoundary := qaComputeSpanBoundaryVetoIndices(cp, clitics)
	elisionVetoed := make(map[int]struct{}, len(idiomMatched)+len(ambiguousShape)+len(spanBoundary))
	for _, set := range []map[int]struct{}{idiomMatched, ambiguousShape, spanBoundary} {
		for idx := range set {
			elisionVetoed[idx] = struct{}{}
		}
	}

	for i := 0; i < n; i++ {
		g := cp[i]
		if !qtIsQuoteMark(g) {
			continue
		}

		lLit := qaAt(cp, i-1)
		rLit := qaAt(cp, i+1)
		lSkip := qtSkipLeft(cp, i)
		rSkip := qtSkipRight(cp, i)

		openLeft := lLit
		if skip.spaceLeft[g] {
			openLeft = lSkip
		}
		closeRight := rLit
		if skip.spaceRight[g] {
			closeRight = rSkip
		}

		canOpen := (openLeft == engine.None || qtIsSpacelike(openLeft) || qtIsOpenish(openLeft) || qtIsDashish(openLeft)) &&
			rSkip != engine.None && !qtIsSpacelike(rSkip) &&
			(!qtIsCloseish(rSkip) || qtIsQuoteMark(rSkip) || rSkip == engine.Marker)

		canClose := lSkip != engine.None && !qtIsSpacelike(lSkip) &&
			(closeRight == engine.None || qtIsSpacelike(closeRight) || qtIsCloseish(closeRight) || qtIsDashish(closeRight))

		// Medial-elision veto (quotes.md 3.2), NARROW marks only, literal reads: don't, l'été,
		// O'Brien, 1990's — and, on a second pipeline pass, don't with U+2019, because apostrophe
		// has converted the mark and U+2019 is also NARROW.
		if qaNarrow[g] && lLit != engine.None && rLit != engine.None && qtIsAlnum(lLit) && qtIsAlnum(rLit) {
			canOpen = false
			canClose = false
		}

		// Listed + general ambiguous-shape veto (quotes.md 3.2, spec 0.4.0/0.5.0): both
		// capabilities forced false, overriding every other test in this loop.
		if _, ok := elisionVetoed[i]; ok {
			canOpen = false
			canClose = false
		}

		// V1 — same-V1-identity adjacency veto (quotes.md 3.2), both widths: "", '', ««, ””,
		// plus the same shape separated by exactly one INLINE-SPACE code point at a position
		// nbsp can insert or remove (gapInsertable, scoped to Lemma B's two insertion sites).
		gV1 := qtV1Identity(g)
		gapInsertable := skip.spaceRight[g] || skip.spaceLeft[g]
		leftVetoed := qtV1Identity(lLit) == gV1 ||
			(lLit != engine.None && qaInlineSpace[lLit] && qtV1Identity(lSkip) == gV1 && gapInsertable)
		rightVetoed := qtV1Identity(rLit) == gV1 ||
			(rLit != engine.None && qaInlineSpace[rLit] && qtV1Identity(rSkip) == gV1 && gapInsertable)
		if leftVetoed || rightVetoed {
			canOpen = false
			canClose = false
		}

		if canOpen || canClose {
			candidates = append(candidates, qtCandidate{index: i, wide: qtWide[g], canOpen: canOpen, canClose: canClose})
		}
	}

	return candidates
}

// qtIsVacuous is quotes.md 3.3 vacuous(a, b). Vacuously true when b = a + 1.
func qtIsVacuous(cp []rune, a, b int) bool {
	for k := a + 1; k < b; k++ {
		if !qaInlineSpace[cp[k]] {
			return false
		}
	}
	return true
}

// qtPairCandidates is pass 2 (quotes.md 3.3) — pair the candidates, one stack per width. Closing
// is tried before opening; a candidate reaches exactly one of three outcomes (paired, pushed,
// unmatched), and a closer that fails the vacuity condition falls through to step 2 and then
// step 3 rather than being discarded — the exhaustive three-outcome shape the certification gate
// depends on.
func qtPairCandidates(cp []rune, candidates []qtCandidate) []qtPair {
	var wideStack, narrowStack []qtCandidate
	var pairs []qtPair

	for _, c := range candidates {
		stack := &narrowStack
		if c.wide {
			stack = &wideStack
		}
		if c.canClose && len(*stack) > 0 {
			top := (*stack)[len(*stack)-1]
			if !qtIsVacuous(cp, top.index, c.index) {
				*stack = (*stack)[:len(*stack)-1]
				pairs = append(pairs, qtPair{open: top.index, close: c.index})
				continue
			}
		}
		if c.canOpen {
			*stack = append(*stack, c)
		}
	}

	return pairs
}

// qtDepthOf is pass 3's depth (quotes.md 3.4), computed over the accepted set A, never the raw
// pass-2 output: on a second run the accepted set is the raw set, so a depth taken over the raw
// set on run 1 and the accepted set on run 2 would disagree whenever the gate declined anything.
func qtDepthOf(pairs []qtPair, p qtPair) int {
	depth := 1
	for _, q := range pairs {
		if q.open < p.open && p.close < q.close {
			depth++
		}
	}
	return depth
}

func qtPairFor(locale spec.LocaleData, depth int) spec.QuotePair {
	if depth%2 == 1 {
		return locale.Quotes.Primary
	}
	return locale.Quotes.Secondary
}

type qtRenderPlan struct {
	replace map[int]rune
	del     map[int]bool
}

// qtComputeRenderPlan is quotes.md 3.5 render's glyph/deletion plan, shared by the certification
// gate's hypothetical and the real emit (pass 5) — the only difference between them is whether
// the plan is applied to a throwaway array or actually returned as edits.
func qtComputeRenderPlan(cp []rune, accepted []qtPair, locale spec.LocaleData) qtRenderPlan {
	replace := map[int]rune{}
	del := map[int]bool{}
	n := len(cp)

	for _, p := range accepted {
		glyphs := qtPairFor(locale, qtDepthOf(accepted, p))
		replace[p.open] = qtGlyphCodePoint(glyphs.Open)
		replace[p.close] = qtGlyphCodePoint(glyphs.Close)

		if glyphs.InnerSpace != "none" {
			continue
		}

		// Open-side run: the maximal INLINE-SPACE run starting at p.open + 1.
		openStart := p.open + 1
		openEnd := openStart
		for openEnd < n && qaInlineSpace[cp[openEnd]] {
			openEnd++
		}
		openEmpty := openEnd == openStart
		openLanding := engine.None
		if openEnd < n {
			openLanding = cp[openEnd]
		}

		// Close-side run: the maximal INLINE-SPACE run ending at p.close - 1.
		closeEnd := p.close
		closeStart := closeEnd - 1
		for closeStart >= 0 && qaInlineSpace[cp[closeStart]] {
			closeStart--
		}
		closeStart++
		closeEmpty := closeStart == closeEnd
		closeLanding := engine.None
		if closeStart-1 >= 0 {
			closeLanding = cp[closeStart-1]
		}

		// A run is deleted iff non-empty, its landing is in DELETE-LANDING, and it is not
		// simultaneously both of the pair's runs — a pair enclosing nothing but spaces deletes
		// neither (quotes.md 3.5). Unreachable for an accepted pair given pass 2's vacuity
		// condition, but the guard is cheap and the spec states it unconditionally.
		sameRun := !openEmpty && !closeEmpty && openStart == closeStart && openEnd == closeEnd

		if !openEmpty && qtIsDeleteLanding(openLanding) && !sameRun {
			for k := openStart; k < openEnd; k++ {
				del[k] = true
			}
		}
		if !closeEmpty && qtIsDeleteLanding(closeLanding) && !sameRun {
			for k := closeStart; k < closeEnd; k++ {
				del[k] = true
			}
		}
	}

	return qtRenderPlan{replace: replace, del: del}
}

// qtApplyRenderPlan applies a render plan, returning the rendered array and the order-preserving
// index map from surviving input indices to output indices.
func qtApplyRenderPlan(cp []rune, plan qtRenderPlan) ([]rune, []int) {
	y := make([]rune, 0, len(cp))
	m := make([]int, len(cp))
	for i := range m {
		m[i] = -1
	}
	for i := 0; i < len(cp); i++ {
		if plan.del[i] {
			continue
		}
		m[i] = len(y)
		if r, ok := plan.replace[i]; ok {
			y = append(y, r)
		} else {
			y = append(y, cp[i])
		}
	}
	return y, m
}

func qtPairSetsEqual(a, b map[qtPair]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// qtCertify is pass 4, the certification gate (quotes.md 3.5): the accepted pairing is checked,
// not proved. Render the hypothetical output, re-run passes 1-2 on it, and decline pairs until
// the re-run reproduces the accepted set exactly. Declination is simultaneous per round, and
// when the intersection fails to shrink A, the pair with the greatest open index is forced out —
// both clauses are normative, so two ports cannot disagree.
func qtCertify(cp []rune, initial []qtPair, locale spec.LocaleData, skip qtSkipSets) []qtPair {
	accepted := append([]qtPair(nil), initial...)
	// Each round accepts or strictly shrinks `accepted`; it is finite and ∅ accepts
	// unconditionally, so the loop runs at most |A0| + 1 times (quotes.md 3.5). The bound below
	// is a defensive safety net, not a normative one.
	maxRounds := len(initial) + 2

	for round := 0; round <= maxRounds; round++ {
		if len(accepted) == 0 {
			return accepted
		}

		plan := qtComputeRenderPlan(cp, accepted, locale)
		y, m := qtApplyRenderPlan(cp, plan)
		rederived := qtPairCandidates(y, qtCollectCandidates(y, skip, locale.Quotes.ElisionIdioms, locale.Quotes.ElisionClitics))

		bSet := make(map[qtPair]bool, len(rederived))
		for _, p := range rederived {
			bSet[p] = true
		}

		projected := make([]qtPair, len(accepted))
		projSet := make(map[qtPair]bool, len(accepted))
		for idx, p := range accepted {
			proj := qtPair{open: m[p.open], close: m[p.close]}
			projected[idx] = proj
			projSet[proj] = true
		}

		if qtPairSetsEqual(projSet, bSet) {
			return accepted
		}

		survivors := make([]qtPair, 0, len(accepted))
		for idx, p := range accepted {
			if bSet[projected[idx]] {
				survivors = append(survivors, p)
			}
		}

		if len(survivors) == len(accepted) {
			removeIdx := 0
			for i := 1; i < len(accepted); i++ {
				if accepted[i].open > accepted[removeIdx].open {
					removeIdx = i
				}
			}
			accepted = append(accepted[:removeIdx], accepted[removeIdx+1:]...)
		} else {
			accepted = survivors
		}
	}

	// Unreachable given the termination argument; declines everything rather than looping.
	return nil
}

// qtEmit is pass 5 (quotes.md 3.6). An edit whose replacement equals the span it replaces is
// never emitted — the invisible-edit principle, applied per mark, not per pair. Map iteration
// order is never relied on: both replacement and deletion indices are sorted before edits are
// built (ARCHITECTURE.md 4.5).
func qtEmit(cp []rune, accepted []qtPair, locale spec.LocaleData) []engine.Edit {
	plan := qtComputeRenderPlan(cp, accepted, locale)
	var edits []engine.Edit

	replaceIdx := make([]int, 0, len(plan.replace))
	for idx := range plan.replace {
		replaceIdx = append(replaceIdx, idx)
	}
	sort.Ints(replaceIdx)
	for _, idx := range replaceIdx {
		newCP := plan.replace[idx]
		if cp[idx] == newCP {
			continue
		}
		edits = append(edits, engine.Edit{Start: idx, End: idx + 1, Replacement: []rune{newCP}, RuleID: "quotes"})
	}

	delIdx := make([]int, 0, len(plan.del))
	for idx := range plan.del {
		delIdx = append(delIdx, idx)
	}
	sort.Ints(delIdx)
	i := 0
	for i < len(delIdx) {
		j := i
		for j+1 < len(delIdx) && delIdx[j+1] == delIdx[j]+1 {
			j++
		}
		edits = append(edits, engine.Edit{Start: delIdx[i], End: delIdx[j] + 1, Replacement: []rune{}, RuleID: "quotes"})
		i = j + 1
	}

	sort.Slice(edits, func(a, b int) bool { return edits[a].Start < edits[b].Start })
	return edits
}

func init() {
	engine.RegisterRule("quotes", scanQuotes)
}

func scanQuotes(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	skip := qtComputeSkipSets(locale)

	candidates := qtCollectCandidates(cp, skip, locale.Quotes.ElisionIdioms, locale.Quotes.ElisionClitics)
	if len(candidates) == 0 {
		return nil
	}

	initialPairs := qtPairCandidates(cp, candidates)
	if len(initialPairs) == 0 {
		return nil
	}

	accepted := qtCertify(cp, initialPairs, locale, skip)
	if len(accepted) == 0 {
		return nil
	}

	return qtEmit(cp, accepted, locale)
}
