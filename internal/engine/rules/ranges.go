package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// `ranges` -- spec/rules/ranges.md, order 25. Explicit opt-in: off by default
// (spec/rules/order.json's "default": "off" for this rule id; the engine's registry, not this
// file, is what enforces that). Split out of `dashes` (spec 0.5.0) -- see ranges.md 1 and
// dashes.md 7.11 for why this is opt-in rather than a bounded structural fix: separating a
// genuine numeric range (`5-10`) from a compound label sharing the identical shape
// (`Figure 5-10`) needs the preceding word, which is exactly the open-ended, per-locale context
// this project's rules are built never to consult.
//
// This function's own behaviour does not depend on whether the rule is enabled -- enable/disable
// is the caller's concern (the registry only invokes a rule's scan function when it is active).
//
// Explicit index-based scanning only: no regex anywhere, and every index addresses the
// code-point array, never a native string (ARCHITECTURE.md 4.1, 4.2).

const rngSolidus = rune(0x2F)

func init() {
	engine.RegisterRule("ranges", scanRanges)
}

// rngIsNonDecreasing is ranges.md 3.3 G5: equal-length ASCII digit runs compare
// lexicographically, so no integer arithmetic (and no locale-dependent parsing) is needed.
func rngIsNonDecreasing(cp []rune, leftStart, rightStart, length int) bool {
	for i := 0; i < length; i++ {
		l := cp[leftStart+i]
		r := cp[rightStart+i]
		if l < r {
			return true
		}
		if l > r {
			return false
		}
	}
	return true
}

// rngGuardsPass is ranges.md 3.2, G1-G5, over the flanks and digit runs 3.2a's walk produced.
// before/after read past a matched outer closed-up symbol, so G1-G3 judge the text in front of
// the whole member rather than the symbol itself -- which is what declines `US$15-$20` on G1.
func rngGuardsPass(cp []rune, f rangeFlanks) bool {
	left, right, a, b := f.left, f.right, f.a, f.b

	beforeFrom := a
	if f.outerLeft >= 0 {
		beforeFrom = f.outerLeft
	}
	afterFrom := b
	if f.outerRight >= 0 {
		afterFrom = f.outerRight
	}
	before := dshEffectiveNeighbour(cp, beforeFrom-1, -1)
	after := dshEffectiveNeighbour(cp, afterFrom+1, 1)

	// G1 -- no letter adjacency.
	if engine.IsLetter(before) {
		return false
	}
	// G2 -- no chain: an ISO date, an ISBN or a phone number always trips this. This guard
	// reads the input as it stood before this rule (or `dashes`) made any edit in this
	// pipeline pass -- ranges.md 4's own reasoning for why `ranges` must run before `dashes`.
	if dshIsDashUnion(before) {
		return false
	}
	if dshIsDashUnion(after) {
		return false
	}
	// G3 -- not part of a decimal or a path.
	if before == dshFullStop || before == dshComma || before == rngSolidus {
		return false
	}
	if after == rngSolidus {
		return false
	}
	// G4 -- run lengths: equal, or the directional (1,2) branch with no leading zero on Rrun.
	leftLength := left - a + 1
	rightLength := b - right + 1
	if leftLength == 1 && rightLength == 2 && cp[right] != dshDigitZero {
		return true
	}
	if leftLength != rightLength {
		return false
	}
	// G5 -- non-decreasing (sound only because G4 guarantees equal length in this branch).
	return rngIsNonDecreasing(cp, a, right, leftLength)
}

func scanRanges(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	var edits []engine.Edit
	style := locale.Dash.Range

	for _, token := range dshFindTokens(cp) {
		// ranges.md 3.2, 3.2a -- a candidate iff both flanks are DIGIT once a matched closed-up
		// symbol has been walked over. `ranges` never processes any other token shape; that is
		// `dashes`' territory, and `dashes` declines a candidate unconditionally too (operator
		// decision, spec 0.5.0) -- neither rule reinterprets the other's shape, whether or not
		// `ranges` is enabled.
		flanks, isCandidate := rngFlanks(cp, token.left, token.right)
		if !isCandidate {
			continue
		}

		if !rngGuardsPass(cp, flanks) {
			continue
		}

		// "none": the locale has no verified range convention, so nothing is substituted --
		// not a fallback to dash.parenthetical, nothing (ranges.md 2).
		if style == "none" {
			continue
		}

		if dshIsSpacedStyle(style) {
			// T1: a tight token may not become spaced across a digit run that has a far dash.
			// T1/T2 read the walked flanks: ranges.md 3.2a makes cp[L']/cp[R'] what every
			// shared guard sees once a closed-up symbol has been consumed.
			if token.lsp == 0 && token.rsp == 0 && dshIsSpacingTransitionBlocked(cp, flanks.left, flanks.right) {
				continue
			}
			// T2: the emitted U+0020 must not land where `spaces` (order 10) would delete it.
			if dshIsStripBeforeOrCloseBracket(cp[flanks.right]) {
				continue
			}
			if dshIsOpenBracket(cp[flanks.left]) {
				continue
			}
		}

		// ranges.md 3.3.1: never make an edit whose entire content is invisible. Try the
		// unbound replacement first; only add the joiner pair if the dash itself is genuinely
		// changing.
		unbound := dshBuildReplacement(style, false)
		onlyBindingWouldChange := !dshIsSpacedStyle(style) &&
			dshSameContent(cp, token.spanStart, token.spanEnd, unbound)
		bind := !dshIsSpacedStyle(style) && !onlyBindingWouldChange
		replacement := unbound
		if bind {
			replacement = dshBuildReplacement(style, true)
		}
		if dshSameContent(cp, token.spanStart, token.spanEnd, replacement) {
			continue
		}

		edits = append(edits, engine.Edit{
			Start:       token.spanStart,
			End:         token.spanEnd,
			Replacement: replacement,
			RuleID:      "ranges",
		})
	}

	return edits
}
