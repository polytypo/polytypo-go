package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/symbols.md 3.1. No locale data (order.json declares "localeData": []). Explicit
// code points only: no regex, no case folding (ARCHITECTURE.md 4.1, 4.4 -- Turkish dotless i
// would otherwise make "(TM)" behave differently in a Turkish process).
const (
	symParenOpen   = rune(0x28)
	symParenClose  = rune(0x29)
	symSquareOpen  = rune(0x5B)
	symSquareClose = rune(0x5D)

	symCopyright      = rune(0xA9)
	symRegistered     = rune(0xAE)
	symTrademark      = rune(0x2122)
	symMultiplication = rune(0xD7)

	symSpace = rune(0x20)
	symNBSP  = rune(0xA0)
	symNNBSP = rune(0x202F)

	symLowerX          = rune(0x78)
	symUpperX          = rune(0x58)
	symCyrillicLowerHa = rune(0x445)
	symCyrillicUpperHa = rune(0x425)
	symDigitZero       = rune(0x30)
	symDigitNine       = rune(0x39)

	symPlus        = rune(0x2B)
	symSolidus     = rune(0x2F)
	symHyphenMinus = rune(0x2D)
	symPlusMinus   = rune(0xB1)
)

func init() {
	engine.RegisterRule("symbols", scanSymbols)
}

// symAt returns cp[i], or engine.None if i is out of bounds -- the spec's own boundary value.
func symAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

func symIsDigit(cp rune) bool {
	return cp >= symDigitZero && cp <= symDigitNine
}

func symIsAlnum(cp rune) bool {
	return cp != engine.None && (symIsDigit(cp) || engine.IsLetter(cp))
}

// symIsSpaceLike is SPACE ∪ NOBREAK-SPACE (symbols.md 3.1): U+0020, U+00A0, U+202F only -- no
// tabs, no other Unicode spaces, no line breaks. Narrower than nbsp's SPACELIKE on purpose; this
// rule only needs to recognise the spacing a multiplication chain can legally carry.
func symIsSpaceLike(cp rune) bool {
	return cp == symSpace || cp == symNBSP || cp == symNNBSP
}

// symIsMulLetter is MUL-LETTER (symbols.md 3.1): exactly four code points, enumerated, never
// case-folded and never locale data. The Cyrillic pair is unconditional -- U+0445 between two
// ASCII digits is a Russian dimension typed on a Cyrillic layout or keyboard debris, and the
// glyphs are visually identical to the Latin ones in every font, so no human review can catch a
// missed conversion.
func symIsMulLetter(cp rune) bool {
	return cp == symLowerX || cp == symUpperX || cp == symCyrillicLowerHa || cp == symCyrillicUpperHa
}

// symTrademarkRow is one row of the trademark table (symbols.md 3.1). guardedByS1 is true only
// for the (c)/(r) rows -- the (tm) rows are exempt from S1 (3.2 step 3).
type symTrademarkRow struct {
	literal     []rune
	replacement rune
	guardedByS1 bool
}

// symTrademarkTable is exhaustive and case-explicit; nothing else matches. Longest literals
// first (the 4-code-point (tm) rows before the 3-code-point (c)/(r) rows), per 3.2 step 1.
var symTrademarkTable = []symTrademarkRow{
	{literal: []rune{symParenOpen, 0x74, 0x6D, symParenClose}, replacement: symTrademark, guardedByS1: false}, // (tm)
	{literal: []rune{symParenOpen, 0x54, 0x4D, symParenClose}, replacement: symTrademark, guardedByS1: false}, // (TM)
	{literal: []rune{symParenOpen, 0x54, 0x6D, symParenClose}, replacement: symTrademark, guardedByS1: false}, // (Tm)
	{literal: []rune{symParenOpen, 0x74, 0x4D, symParenClose}, replacement: symTrademark, guardedByS1: false}, // (tM)
	{literal: []rune{symParenOpen, 0x63, symParenClose}, replacement: symCopyright, guardedByS1: true},        // (c)
	{literal: []rune{symParenOpen, 0x43, symParenClose}, replacement: symCopyright, guardedByS1: true},        // (C)
	{literal: []rune{symParenOpen, 0x72, symParenClose}, replacement: symRegistered, guardedByS1: true},       // (r)
	{literal: []rune{symParenOpen, 0x52, symParenClose}, replacement: symRegistered, guardedByS1: true},       // (R)
}

func symMatchesAt(cp []rune, i int, literal []rune) bool {
	if i+len(literal) > len(cp) {
		return false
	}
	for j, want := range literal {
		if symAt(cp, i+j) != want {
			return false
		}
	}
	return true
}

// symTrademarkAt is symbols.md 3.2. Returns the edit and true, or (zero value, false) when no
// row matches or a guard rejects the candidate.
func symTrademarkAt(cp []rune, i int) (engine.Edit, bool) {
	for _, row := range symTrademarkTable {
		if !symMatchesAt(cp, i, row.literal) {
			continue
		}
		end := i + len(row.literal)
		before := symAt(cp, i-1)
		after := symAt(cp, end)

		// S1 -- left adjacency, (c)/(r) rows only: a one-letter argument list ("f(c)") is
		// common, "(tm)" tucked against a product name is not a call. The three replacement
		// signs are listed so that "(c)(r)" converges to "©(r)" in one run (symbols.md 5).
		if row.guardedByS1 && before != engine.None &&
			(symIsAlnum(before) || before == symParenClose || before == symSquareClose ||
				before == symCopyright || before == symRegistered || before == symTrademark) {
			return engine.Edit{}, false
		}
		// S2 -- right adjacency: "(r)evolution", "(c)ompiler".
		if after != engine.None && symIsAlnum(after) {
			return engine.Edit{}, false
		}
		// S3 -- no nesting: "((c))" is ASCII art or code.
		if before == symParenOpen {
			return engine.Edit{}, false
		}

		return engine.Edit{Start: i, End: end, Replacement: []rune{row.replacement}, RuleID: "symbols"}, true
	}
	return engine.Edit{}, false
}

// symChainLink is one MUL-LETTER position within a multiplication chain, plus whether it carried
// a space on each side (0 or 1 code point -- symbols.md 3.3 never allows more than one).
type symChainLink struct {
	letterIndex int
	leftSpace   int
	rightSpace  int
}

// symChain is the result of reading a whole DIGIT+ (MUL-LETTER DIGIT+)+ shape starting at a
// maximal digit run (symbols.md 3.3 step 1).
type symChain struct {
	// end is the index of the last code point read, whether or not any link was completed --
	// the caller resumes scanning at end+1 either way.
	end int
	// firstRunEnd is the index of the last digit of the very first digit run (before any
	// link), used only by guard M4's hex-literal check.
	firstRunEnd int
	links       []symChainLink
}

// symReadChain reads the chain greedily and unconditionally; guard decisions happen afterward in
// symChainEdits, never here, so the scan shape itself cannot express "how many links converted" --
// only "whole chain or nothing" (symbols.md 3.3 step 2, 5).
func symReadChain(cp []rune, a int) symChain {
	p := a
	for symIsDigit(symAt(cp, p)) {
		p++
	}
	firstRunEnd := p - 1
	end := p - 1
	var links []symChainLink

	for {
		q := p
		leftSpace := 0
		if symIsSpaceLike(symAt(cp, q)) {
			leftSpace = 1
			q++
		}
		if !symIsMulLetter(symAt(cp, q)) {
			break
		}
		letterIndex := q
		q++
		rightSpace := 0
		if symIsSpaceLike(symAt(cp, q)) {
			rightSpace = 1
			q++
		}
		if !symIsDigit(symAt(cp, q)) {
			break
		}
		for symIsDigit(symAt(cp, q)) {
			q++
		}
		links = append(links, symChainLink{letterIndex: letterIndex, leftSpace: leftSpace, rightSpace: rightSpace})
		end = q - 1
		p = q
	}

	return symChain{end: end, firstRunEnd: firstRunEnd, links: links}
}

// symSameSpan reports whether cp[start:end] already equals replacement, so a would-be no-op
// edit is never emitted (symbols.md 3.3 step 8, last sentence). In practice a link's middle
// position is always a MUL-LETTER, never U+00D7 (isMulLetter excludes it), so this can never
// actually fire -- it is kept as a literal implementation of the spec sentence rather than
// dropped as dead code.
func symSameSpan(cp []rune, start, end int, replacement []rune) bool {
	if end-start != len(replacement) {
		return false
	}
	for j, want := range replacement {
		if symAt(cp, start+j) != want {
			return false
		}
	}
	return true
}

// symChainEdits applies guards M1-M4 to a chain read by symReadChain. Returns (edits, true) when
// the chain converts, or (nil, false) when it is declined whole -- symbols.md 3.3 never
// half-converts a chain.
func symChainEdits(cp []rune, a int, chain symChain) ([]engine.Edit, bool) {
	if len(chain.links) == 0 {
		return nil, false
	}
	first := chain.links[0]

	// M1 -- every link symmetric, and every link agreeing with the first. "5x4 x 3" is
	// ambiguous input and is declined whole rather than half-converted.
	sp := first.leftSpace
	for _, link := range chain.links {
		if link.leftSpace != link.rightSpace {
			return nil, false
		}
		if link.leftSpace != sp {
			return nil, false
		}
	}

	// M2/M3 -- the chain's OUTER boundaries, not each link. Applying them per link is exactly
	// what made the pairwise form reject chains: the letter past the middle digit run is
	// itself a MUL-LETTER, hence a LETTER.
	before := symAt(cp, a-1)
	after := symAt(cp, chain.end+1)
	if before != engine.None && engine.IsLetter(before) {
		return nil, false
	}
	if after != engine.None && engine.IsLetter(after) {
		return nil, false
	}

	// M4 -- hexadecimal literal veto. Latin lowercase only: a hex literal is never written
	// with Cyrillic. Inspects the first link alone.
	if sp == 0 &&
		symAt(cp, first.letterIndex) == symLowerX &&
		chain.firstRunEnd == a &&
		symAt(cp, a) == symDigitZero {
		return nil, false
	}

	// There is no M5. It was removed rather than extended (symbols.md 3.3 step 7, 7.3).
	edits := make([]engine.Edit, 0, len(chain.links))
	for _, link := range chain.links {
		j := link.letterIndex
		var replacement []rune
		if sp == 0 {
			replacement = []rune{symMultiplication}
		} else {
			replacement = []rune{symAt(cp, j-1), symMultiplication, symAt(cp, j+1)}
		}
		start := j - sp
		end := j + sp + 1
		if symSameSpan(cp, start, end, replacement) {
			continue
		}
		edits = append(edits, engine.Edit{Start: start, End: end, Replacement: replacement, RuleID: "symbols"})
	}
	return edits, true
}

// symPlusMinusAt is symbols.md 3.4. Only the literal "+/-"; the bare "+-" is never converted, in
// any context (§3.4, §7.11).
func symPlusMinusAt(cp []rune, i int) (engine.Edit, bool) {
	if symAt(cp, i+1) != symSolidus || symAt(cp, i+2) != symHyphenMinus {
		return engine.Edit{}, false
	}
	// F1 -- not a character class: "[+/-]" is a regular expression.
	if symAt(cp, i-1) == symSquareOpen {
		return engine.Edit{}, false
	}
	// F2 -- numeric context, with one optional intervening space so "+/-5" and "+/- 5" both
	// work. Without it, prose that names the characters ("lines marked +/- were edited") is
	// corrupted.
	j := i + 3
	if symAt(cp, j) == symSpace {
		j++
	}
	if !symIsDigit(symAt(cp, j)) {
		return engine.Edit{}, false
	}
	return engine.Edit{Start: i, End: i + 3, Replacement: []rune{symPlusMinus}, RuleID: "symbols"}, true
}

// scanSymbols is symbols.md 3.5: one left-to-right scan. The three branches key on different
// code points -- U+0028, a DIGIT, U+002B -- so no two can match at the same index; on a
// successful edit the scan continues from the index after the matched span.
func scanSymbols(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	n := len(cp)
	var edits []engine.Edit
	i := 0

	for i < n {
		current := symAt(cp, i)

		switch {
		case current == symParenOpen:
			if edit, ok := symTrademarkAt(cp, i); ok {
				edits = append(edits, edit)
				i = edit.End
				continue
			}
		case symIsDigit(current) && !symIsDigit(symAt(cp, i-1)):
			// Keyed on the start of a maximal digit run, not on the letter.
			chain := symReadChain(cp, i)
			if chainResult, ok := symChainEdits(cp, i, chain); ok {
				edits = append(edits, chainResult...)
			}
			// Continue past the chain whether or not it converted. A declined chain has no
			// convertible sub-chain: any sub-chain starts right after a MUL-LETTER, which is
			// a LETTER, so M2 would reject it too.
			i = chain.end + 1
			continue
		case current == symPlus:
			if edit, ok := symPlusMinusAt(cp, i); ok {
				edits = append(edits, edit)
				i = edit.End
				continue
			}
		}
		i++
	}

	return edits
}
