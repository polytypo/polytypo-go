package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// `dashes` -- spec/rules/dashes.md, order 30. Parenthetical-dash processing only, as of spec
// 0.5.0: numeric/date-range recognition moved to the `ranges` rule (order 25, off by default),
// which owns `dash.range` and shares this rule's token-scanning and guard machinery via
// dash_shared.go. See dashes.md 1 and 7.11, and ranges.md 1, for why the split happened and why
// range detection is opt-in rather than fixed structurally.
//
// A digit-flanked dash token is declined here unconditionally -- never reinterpreted as a
// parenthetical dash -- regardless of whether `ranges` is enabled (operator decision, spec
// 0.5.0). That was already true of every prior spec version: the range/parenthetical branches
// have always been mutually exclusive per token, on the same "both flanks DIGIT" test that now
// decides which rule a token belongs to rather than which branch of one rule it takes.
//
// Explicit index-based scanning only: no regex anywhere, and every index addresses the
// code-point array, never a native string (ARCHITECTURE.md 4.1, 4.2).

const (
	dashesRomanI = rune(0x49)
	dashesRomanV = rune(0x56)
	dashesRomanX = rune(0x58)
	dashesRomanL = rune(0x4C)
	dashesRomanC = rune(0x43)
	dashesRomanD = rune(0x44)
	dashesRomanM = rune(0x4D)
)

func init() {
	engine.RegisterRule("dashes", scanDashes)
}

// dashesIsRoman is dashes.md 3.1 ROMAN: the seven uppercase Roman-numeral letters only.
// Lower-case forms are not members -- see dashes.md 3.4 P4.
func dashesIsRoman(cp rune) bool {
	switch cp {
	case dashesRomanI, dashesRomanV, dashesRomanX, dashesRomanL, dashesRomanC, dashesRomanD, dashesRomanM:
		return true
	default:
		return false
	}
}

// dashesIsRomanFlanked is dashes.md 3.4 P4 -- the Roman-numeral veto. A tight dash between two
// word-bounded ROMAN runs is a range already in its correct Russian form (`в XV—XVII веках`);
// `ranges` cannot see it, because ranges.md 3.2 needs a DIGIT on each side, so without this the
// parenthetical branch would space out input that was already right.
//
// A veto only: it never converts. Admitting ROMAN runs as range candidates would also fix
// `XV-XVII`, but it fires on all-caps words built from the same letters (`MIX`, `CIVIL`), and
// converting is the direction that damages -- see dashes.md 7.10.
func dashesIsRomanFlanked(cp []rune, left, right int) bool {
	n := len(cp)

	if !dashesIsRoman(cp[left]) || !dashesIsRoman(cp[right]) {
		return false
	}

	a := left
	for a > 0 && dashesIsRoman(cp[a-1]) {
		a--
	}
	if a > 0 && engine.IsLetter(cp[a-1]) {
		return false
	}

	b := right
	for b+1 < n && dashesIsRoman(cp[b+1]) {
		b++
	}
	if b+1 < n && engine.IsLetter(cp[b+1]) {
		return false
	}

	return true
}

func scanDashes(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	var edits []engine.Edit
	style := locale.Dash.Parenthetical

	for _, token := range dshFindTokens(cp) {
		// A digit-flanked token is `ranges`' territory, never `dashes`' -- declined
		// unconditionally, whether or not `ranges` is enabled (operator decision, spec 0.5.0).
		if dshIsDigit(token.leftCp) && dshIsDigit(token.rightCp) {
			continue
		}

		// dashes.md 3.4 P5 -- authored en-dash mark-identity veto (spec 0.6.0). A run
		// consisting of exactly one U+2013 is declined unconditionally: every locale, tight or
		// spaced, regardless of dash.parenthetical's target glyph.
		if token.e-token.s == 1 && cp[token.s] == dshEnDash {
			continue
		}

		// dashes.md 3.4 P1 -- a bare hyphen-shaped stroke must be spaced (the compound-word
		// guard): well-known, e-mail, Jean-Luc, well-being, and their U+2010/U+2212 spellings.
		if token.e-token.s == 1 &&
			(cp[token.s] == dshHyphenMinus || cp[token.s] == dshHyphen || cp[token.s] == dshMinusSign) &&
			token.lsp == 0 {
			continue
		}

		// dashes.md 3.4 P4 -- Roman-numeral veto.
		if token.lsp == 0 && token.rsp == 0 && dashesIsRomanFlanked(cp, token.left, token.right) {
			continue
		}

		// "none": the locale has no verified convention, so nothing is substituted.
		if style == "none" {
			continue
		}

		if dshIsSpacedStyle(style) {
			// T1: a tight token may not become spaced across a digit run that has a far dash.
			if token.lsp == 0 && token.rsp == 0 && dshIsSpacingTransitionBlocked(cp, token.left, token.right) {
				continue
			}
			// T2: the emitted U+0020 must not land where `spaces` (order 10) would delete it.
			if dshIsStripBeforeOrCloseBracket(token.rightCp) {
				continue
			}
			if dshIsOpenBracket(token.leftCp) {
				continue
			}
		}

		// `dashes` never binds: an interrupting dash is exactly where a line may break
		// (dashes.md 3.3.1's binding is `ranges`-only). Every token this rule accepts has
		// crossedJoiner=false (dshFindTokens never returns a crossed-joiner, non-digit-flanked
		// token), so the plain s-lsp/e+rsp span is always exactly the token's own span here.
		replacement := dshBuildReplacement(style, false)
		spanStart := token.s - token.lsp
		spanEnd := token.e + token.rsp
		if dshSameContent(cp, spanStart, spanEnd, replacement) {
			continue
		}

		edits = append(edits, engine.Edit{
			Start:       spanStart,
			End:         spanEnd,
			Replacement: replacement,
			RuleID:      "dashes",
		})
	}

	return edits
}
