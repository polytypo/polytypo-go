package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
)

// Structural primitives shared by `dashes` (spec/rules/dashes.md, order 30) and `ranges`
// (spec/rules/ranges.md, order 25). Both rules scan the same DASH-token shape and share the same
// symmetry/isolation/cluster/joiner guards (dashes.md 3.2, 3.2a, 3.2b) -- this file is the single
// source of truth for that shared machinery, mirroring the JS reference implementation's
// dash-shared.ts and the Python port's _dash_shared.py. Each rule adds only its own
// branch-specific guards (P1/P4/P5 for `dashes`; G1-G5 for `ranges`) and its own locale style
// (`dash.parenthetical` vs `dash.range`) on top of what dshFindTokens returns.
//
// Not a registered rule itself -- an internal helper file for the two rules that are, exactly
// like dash-shared.ts is to dashes.ts/ranges.ts in the JS reference implementation.
//
// All identifiers here carry a `dsh` prefix because they live in the same Go package (`rules`)
// as every other rule's own scan function and helpers (spaces.go, ellipsis.go, and the sibling
// rule files other agents are writing concurrently) -- unlike JS/Python, where this module has
// its own import namespace, Go package-level names must not collide across files.
const (
	dshHyphenMinus          = rune(0x2D)
	dshHyphen               = rune(0x2010)
	dshFigureDash           = rune(0x2012)
	dshEnDash               = rune(0x2013)
	dshEmDash               = rune(0x2014)
	dshHorizontalBar        = rune(0x2015)
	dshMinusSign            = rune(0x2212)
	dshSoftHyphen           = rune(0x00AD)
	dshNonBreakingHyphen    = rune(0x2011)
	dshSmallEmDash          = rune(0xFE58)
	dshSmallHyphenMinus     = rune(0xFE63)
	dshFullwidthHyphenMinus = rune(0xFF0D)

	dshSpace              = rune(0x20)
	dshNoBreakSpace       = rune(0x00A0)
	dshNarrowNoBreakSpace = rune(0x202F)

	dshDigitZero = rune(0x30)
	dshDigitNine = rune(0x39)

	dshLF  = rune(0x0A)
	dshCR  = rune(0x0D)
	dshVT  = rune(0x0B)
	dshFF  = rune(0x0C)
	dshNEL = rune(0x85)
	dshLS  = rune(0x2028)
	dshPS  = rune(0x2029)

	// dashes.md 3.1 JOINER: U+2060, emitted only around a tight range dash (ranges.md 3.3.1).
	dshWordJoiner = rune(0x2060)

	dshComma        = rune(0x2C)
	dshFullStop     = rune(0x2E)
	dshSemicolon    = rune(0x3B)
	dshColon        = rune(0x3A)
	dshExclamation  = rune(0x21)
	dshQuestion     = rune(0x3F)
	dshEllipsisChar = rune(0x2026)

	dshParenOpen   = rune(0x28)
	dshParenClose  = rune(0x29)
	dshSquareOpen  = rune(0x5B)
	dshSquareClose = rune(0x5D)
	dshCurlyOpen   = rune(0x7B)
	dshCurlyClose  = rune(0x7D)
)

// dshIsDash is dashes.md 3.1 DASH.
func dshIsDash(cp rune) bool {
	switch cp {
	case dshHyphenMinus, dshHyphen, dshEnDash, dshEmDash, dshMinusSign:
		return true
	default:
		return false
	}
}

// dshIsInertDash is dashes.md 3.1 INERT-DASH: never a candidate, never produced, by either rule.
func dshIsInertDash(cp rune) bool {
	switch cp {
	case dshSoftHyphen, dshFigureDash, dshNonBreakingHyphen, dshHorizontalBar,
		dshSmallEmDash, dshSmallHyphenMinus, dshFullwidthHyphenMinus:
		return true
	default:
		return false
	}
}

// dshIsDashUnion is DASH union INERT-DASH -- the alphabet G2 and T1 read as "a dash".
func dshIsDashUnion(cp rune) bool {
	return dshIsDash(cp) || dshIsInertDash(cp)
}

// dshIsDigit is dashes.md 3.1 DIGIT: ASCII only, deliberately -- see ranges.md 7.1.
func dshIsDigit(cp rune) bool {
	return cp >= dshDigitZero && cp <= dshDigitNine
}

// dshIsBreak is BREAK, including engine.LineMarker: a member of BREAK for every rule everywhere
// (modes.md 3.2).
func dshIsBreak(cp rune) bool {
	switch cp {
	case dshLF, dshCR, dshVT, dshFF, dshNEL, dshLS, dshPS, engine.LineMarker:
		return true
	default:
		return false
	}
}

func dshIsNoBreakSpace(cp rune) bool {
	return cp == dshNoBreakSpace || cp == dshNarrowNoBreakSpace
}

// dshIsSpacedStyle reports whether a dash.parenthetical/dash.range value is one of the two
// "-spaced" forms.
func dshIsSpacedStyle(style string) bool {
	return style == "em-spaced" || style == "en-spaced"
}

// dshDashCodePoint is the target dash glyph for a style value.
func dshDashCodePoint(style string) rune {
	if style == "em-tight" || style == "em-spaced" {
		return dshEmDash
	}
	return dshEnDash
}

// dshSameContent reports whether cp[start:end] is exactly next, code point for code point.
func dshSameContent(cp []rune, start, end int, next []rune) bool {
	if end-start != len(next) {
		return false
	}
	for i, want := range next {
		if cp[start+i] != want {
			return false
		}
	}
	return true
}

// dshBuildReplacement is dashes.md 3.6: every replacement is built from U+0020 alone plus the
// target dash glyph. `bind` is meaningful only for `ranges` (ranges.md 3.3.1) -- `dashes`'
// parenthetical branch always calls this with bind=false, since a parenthetical dash never
// binds (an interrupting dash is exactly where a line may break).
func dshBuildReplacement(style string, bind bool) []rune {
	if !dshIsSpacedStyle(style) {
		if bind {
			return []rune{dshWordJoiner, dshDashCodePoint(style), dshWordJoiner}
		}
		return []rune{dshDashCodePoint(style)}
	}
	return []rune{dshSpace, dshDashCodePoint(style), dshSpace}
}

// dshIsStripBeforeOrCloseBracket is dashes.md 3.2 step 9 (T2)'s own set union CLOSE-BRACKET --
// the positions from which `spaces` (order 10) deletes a U+0020, plus U+2026 (T2's set is a
// strict superset of `spaces`' STRIP-BEFORE by exactly that one code point -- dashes.md 3.2
// step 9's own note).
func dshIsStripBeforeOrCloseBracket(cp rune) bool {
	switch cp {
	case dshComma, dshFullStop, dshSemicolon, dshColon, dshExclamation, dshQuestion, dshEllipsisChar,
		dshParenClose, dshSquareClose, dshCurlyClose:
		return true
	default:
		return false
	}
}

// dshIsOpenBracket is T2's OPEN-BRACKET set.
func dshIsOpenBracket(cp rune) bool {
	switch cp {
	case dshParenOpen, dshSquareOpen, dshCurlyOpen:
		return true
	default:
		return false
	}
}

// dshIsClusterMember is dashes.md 3.2 step 7's cluster alphabet: DASH union INERT-DASH union
// DIGIT union JOINER (a joiner `ranges` emitted on an earlier pass must not split a cluster it
// sits inside -- 3.2b).
func dshIsClusterMember(cp rune) bool {
	return dshIsDash(cp) || dshIsInertDash(cp) || dshIsDigit(cp) || cp == dshWordJoiner
}

// dshIsClusterInert is dashes.md 3.2 step 7 -- the cluster guard. A maximal span of cluster
// members containing this run is inert (declines every token in it) if it holds two or more
// maximal DASH-union-INERT-DASH runs.
func dshIsClusterInert(cp []rune, s, e int) bool {
	n := len(cp)
	start := s
	for start > 0 && dshIsClusterMember(cp[start-1]) {
		start--
	}
	end := e
	for end < n && dshIsClusterMember(cp[end]) {
		end++
	}

	runs := 0
	i := start
	for i < end {
		if !dshIsDashUnion(cp[i]) {
			i++
			continue
		}
		runs++
		if runs >= 2 {
			return true
		}
		for i < end && dshIsDashUnion(cp[i]) {
			i++
		}
	}
	return false
}

// dshEffectiveIndex is dashes.md 3.2b's "effective neighbour" walk: step from `from` in `step`
// direction (+1/-1) across a maximal run of JOINER, returning the resulting index, or -1 if the
// walk leaves the array.
func dshEffectiveIndex(cp []rune, from, step int) int {
	i := from
	for i >= 0 && i < len(cp) && cp[i] == dshWordJoiner {
		i += step
	}
	if i < 0 || i >= len(cp) {
		return -1
	}
	return i
}

// dshEffectiveNeighbour is dshEffectiveIndex's code point, or engine.None if the walk leaves the
// array.
func dshEffectiveNeighbour(cp []rune, from, step int) rune {
	i := dshEffectiveIndex(cp, from, step)
	if i < 0 {
		return engine.None
	}
	return cp[i]
}

// dshIsSpacingTransitionBlocked is dashes.md 3.2 step 8 (T1) -- the spacing-transition guard.
// A tight token must not become spaced when doing so would insert a U+0020 between itself and a
// digit run that has another dash on its far side, read through effective neighbours (3.2b).
// Shared because a `dashes` token becoming spaced can insert a space next to a `ranges` token's
// digit run, and vice versa.
func dshIsSpacingTransitionBlocked(cp []rune, left, right int) bool {
	n := len(cp)

	if left >= 0 && left < n && dshIsDigit(cp[left]) {
		d := left
		for d > 0 && dshIsDigit(cp[d-1]) {
			d--
		}
		i1 := dshEffectiveIndex(cp, d-1, -1)
		one := engine.None
		if i1 >= 0 {
			one = cp[i1]
		}
		two := engine.None
		if i1 >= 0 {
			two = dshEffectiveNeighbour(cp, i1-1, -1)
		}
		if dshIsDashUnion(one) {
			return true
		}
		if (one == dshSpace || dshIsNoBreakSpace(one)) && dshIsDashUnion(two) {
			return true
		}
	}

	if right >= 0 && right < n && dshIsDigit(cp[right]) {
		d := right
		for d+1 < n && dshIsDigit(cp[d+1]) {
			d++
		}
		i1 := dshEffectiveIndex(cp, d+1, 1)
		one := engine.None
		if i1 >= 0 {
			one = cp[i1]
		}
		two := engine.None
		if i1 >= 0 {
			two = dshEffectiveNeighbour(cp, i1+1, 1)
		}
		if dshIsDashUnion(one) {
			return true
		}
		if (one == dshSpace || dshIsNoBreakSpace(one)) && dshIsDashUnion(two) {
			return true
		}
	}

	return false
}

// dashToken is the shared dash-token shape returned by dshFindTokens (dashes.md 3.2, 3.2a).
type dashToken struct {
	// s, e is the DASH run itself, in input-array indices ([s, e)).
	s, e int
	// lsp, rsp are the token's outer spacing (0 or 1 U+0020 on each side).
	lsp, rsp int
	// left, right are the indices of the code points immediately outside the token's content,
	// after walking across any adjacent JOINER run (dashes.md 3.2a) -- L*/R* in the spec.
	left, right int
	// leftCp, rightCp are cp[left]/cp[right].
	leftCp, rightCp rune
	// spanStart, spanEnd are the full edit span, including outer spacing and any joiner this
	// token is re-entering across -- meaningful beyond plain s-lsp/e+rsp only when
	// crossedJoiner is true, which for a non-digit-flanked token always means "declined" (see
	// dshFindTokens's own doc: such a token is never returned).
	spanStart, spanEnd int
	crossedJoiner      bool
}

// dshFindTokens is dashes.md 3.2 steps 1-7 and 3.2a, exactly as they read before the `ranges`
// split -- the common prefix every DASH-run token must pass before either rule's own
// branch-specific guards run. Returns every token that survives symmetry, content,
// joiner-crossing, isolation and cluster guards; each rule then filters to the tokens it owns:
//
//   - `ranges` only ever processes a token whose leftCp/rightCp are both DIGIT.
//   - `dashes` must decline every such token unconditionally (operator decision, spec 0.5.0) --
//     never reinterpreting a digit-flanked stroke as a parenthetical dash, regardless of
//     whether `ranges` is enabled.
//
// A token with crossedJoiner=true and non-digit flanks is NOT returned at all (declined inline,
// exactly as dashes.md 3.2a specifies: "if a joiner was crossed in any other configuration, emit
// nothing") -- a joiner is `ranges`' own emission alphabet, and an author who types one next to a
// dash meant it, exactly as with INERT-DASH.
//
// Sequencing is load-bearing and must not be reordered: the joiner walk (3.2a) runs
// immediately after the basic array-bounds check on the token's raw L/R, and only THEN do the
// BREAK check, the isolation guard (step 6) and the cluster guard (step 7) read the POST-walk
// left/right -- never the pre-walk ones. Evaluating BREAK/isolation/cluster before the joiner
// walk is a known bug class (it diverged from the JS reference implementation during an earlier
// port and had to be fixed): a joiner-adjacent BREAK or space-like character must be read past
// the joiner, not at it.
func dshFindTokens(cp []rune) []dashToken {
	n := len(cp)
	var tokens []dashToken
	i := 0

	for i < n {
		if !dshIsDash(cp[i]) {
			i++
			continue
		}

		s := i
		e := s
		for e < n && dshIsDash(cp[e]) {
			e++
		}
		i = e

		// dashes.md 3.2 step 2 -- a run longer than three is decoration, not a dash.
		if e-s > 3 {
			continue
		}

		lsp := 0
		if s > 0 && cp[s-1] == dshSpace {
			lsp = 1
		}
		rsp := 0
		if e < n && cp[e] == dshSpace {
			rsp = 1
		}

		// dashes.md 3.2 step 4 -- symmetry guard.
		if lsp != rsp {
			continue
		}

		// dashes.md 3.2 step 5 -- content on both sides (array-bounds half; BREAK is checked
		// below, after the joiner walk, against the post-walk neighbour).
		left := s - 1 - lsp
		right := e + rsp
		if left < 0 || right >= n {
			continue
		}

		// dashes.md 3.2a -- joiner neighbours. This walk, and everything that reads its
		// result, must run before the BREAK/isolation/cluster guards below: those guards read
		// the effective (post-walk) neighbour, not the raw one.
		joinStart := left + 1
		joinEnd := right
		for left >= 0 && cp[left] == dshWordJoiner {
			left--
		}
		for right < n && cp[right] == dshWordJoiner {
			right++
		}
		if left < 0 || right >= n {
			continue
		}
		crossedJoiner := left+1 != joinStart || right != joinEnd
		joinStart = left + 1
		joinEnd = right

		leftCp := cp[left]
		rightCp := cp[right]

		if crossedJoiner && !(dshIsDigit(leftCp) && dshIsDigit(rightCp)) {
			continue
		}
		if dshIsBreak(leftCp) || dshIsBreak(rightCp) {
			continue
		}

		// dashes.md 3.2 step 6 -- isolation guard.
		if dshIsInertDash(leftCp) || dshIsInertDash(rightCp) {
			continue
		}
		if dshIsDash(leftCp) || dshIsDash(rightCp) {
			continue
		}
		if leftCp == dshSpace || dshIsNoBreakSpace(leftCp) {
			continue
		}
		if rightCp == dshSpace || dshIsNoBreakSpace(rightCp) {
			continue
		}

		// dashes.md 3.2 step 7 -- cluster guard.
		if dshIsClusterInert(cp, s, e) {
			continue
		}

		spanStart := s - lsp
		if joinStart < spanStart {
			spanStart = joinStart
		}
		spanEnd := e + rsp
		if joinEnd > spanEnd {
			spanEnd = joinEnd
		}

		tokens = append(tokens, dashToken{
			s:             s,
			e:             e,
			lsp:           lsp,
			rsp:           rsp,
			left:          left,
			right:         right,
			leftCp:        leftCp,
			rightCp:       rightCp,
			spanStart:     spanStart,
			spanEnd:       spanEnd,
			crossedJoiner: crossedJoiner,
		})
	}

	return tokens
}
