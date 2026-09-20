package rules

import (
	"fmt"
	"sort"

	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/nbsp.md -- order 70 (last), default on. Ten independent sub-rules, N1 through N10,
// evaluated in that fixed order (3.2); each produces candidate edits keyed by the index of the
// space (or insertion point) it claims, and the first sub-rule to claim an index wins. The claim
// table is a positional array indexed 0..len(cp), never a map -- a map-iteration implementation
// would resolve conflicts differently in Go (ARCHITECTURE.md 4.5). No regex, no native-string
// indexing (ARCHITECTURE.md 4.1, 4.2).
const (
	nbSpace    = rune(0x20)
	nbTab      = rune(0x09)
	nbNBSP     = rune(0xA0)
	nbNNBSP    = rune(0x202F)
	nbFullStop = rune(0x2E)

	nbDigitZero = rune(0x30)
	nbDigitNine = rune(0x39)

	nbParenOpen   = rune(0x28)
	nbSquareOpen  = rune(0x5B)
	nbBraceOpen   = rune(0x7B)
	nbParenClose  = rune(0x29)
	nbSquareClose = rune(0x5D)
	nbBraceClose  = rune(0x7D)

	// nbMaxCharacterReferenceName is the longest HTML named reference (31 code points,
	// "CounterClockwiseContourIntegral") plus one, bounding the guard's left walk.
	nbSemicolon                 = rune(0x3B)
	nbAmpersand                 = rune(0x26)
	nbHash                      = rune(0x23)
	nbMaxCharacterReferenceName = 32

	nbEnDash   = rune(0x2013)
	nbEmDash   = rune(0x2014)
	nbEllipsis = rune(0x2026)
)

func init() {
	engine.RegisterRule("nbsp", scanNbsp)
}

// nbspAt returns cp[i], or engine.None if i is out of bounds -- the spec's own boundary value.
func nbspAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

func nbIsDigit(cp rune) bool {
	return cp >= nbDigitZero && cp <= nbDigitNine
}

func nbIsAlnum(cp rune) bool {
	return cp != engine.None && (nbIsDigit(cp) || engine.IsLetter(cp))
}

// nbIsBreak is BREAK (nbsp.md 3.1), including engine.LineMarker -- a member of BREAK for every
// rule everywhere (modes.md 3.2).
//
// engine.Marker is NOT a member here. Since spec 1.2.0 it is a member of this rule's CLOSEISH and
// not of its OPENISH (nbsp.md 3.1, 7 item 12; modes.md 3.3): CLOSEISH membership lets N1/N2's
// right-context guard put back the space `spaces` deleted in `<strong>gel :</strong>`, while
// OPENISH membership would make the quote-glyph guard decline `<em>non</em> !`. The fixture
// ru-nbsp-span-boundary-not-openish-short-word pins the OPENISH half.
func nbIsBreak(cp rune) bool {
	switch cp {
	case 0x0A, 0x0D, 0x0B, 0x0C, 0x85, 0x2028, 0x2029, engine.LineMarker:
		return true
	default:
		return false
	}
}

func nbIsNoBreak(cp rune) bool {
	return cp == nbNBSP || cp == nbNNBSP
}

// nbIsOtherSpace is OTHER-SPACE (nbsp.md 3.1): the fixed-width spaces. A member of SPACELIKE for
// boundary purposes, but never converted and never an "already correct" state -- a thin or
// figure space the author placed stays exactly where it is.
func nbIsOtherSpace(cp rune) bool {
	return (cp >= 0x2000 && cp <= 0x200A) || cp == 0x205F || cp == 0x3000
}

// nbIsSpaceLike is SPACELIKE, with NOBREAK included. Every boundary test in this rule uses this
// predicate and never U+0020 alone; that single decision is what makes the rule idempotent
// (nbsp.md 3.1).
func nbIsSpaceLike(cp rune) bool {
	return cp == nbSpace || nbIsNoBreak(cp) || cp == nbTab || nbIsOtherSpace(cp) || nbIsBreak(cp)
}

// nbIsSentenceDash is SENTENCE-DASH (nbsp.md 3.1): U+2013 and U+2014 only, never a hyphen. A
// hyphen marks an intra-word position by construction, so the token after one is not a
// free-standing word -- without this exclusion "из-за дождя" would bind twice over, once by
// `hyphen` producing "из‑за" and once by N3 reading the compound's tail "за" as a listed
// preposition (nbsp.md 3.5 step 2).
func nbIsSentenceDash(cp rune) bool {
	return cp == nbEnDash || cp == nbEmDash
}

// nbQuoteTarget is one locale quote pair N8 owns: the open/close glyphs and the no-break space
// (or narrow no-break space) that belongs on their inner side.
type nbQuoteTarget struct {
	open   rune
	close  rune
	target rune
}

// nbPrepared is the locale data this rule reads, resolved to code points once per call. No
// package-level mutable state (ARCHITECTURE.md 7): everything here is local to one scanNbsp call.
type nbPrepared struct {
	beforePunctuation       []rune
	narrowBeforePunctuation []rune
	shortWords              [][]rune
	abbreviations           [][]rune
	units                   [][]rune
	beforeNumber            [][]rune
	beforeWord              [][]rune
	symbols                 [][]rune
	initialBinding          string
	opens                   []rune
	closes                  []rune
	quotePairs              []nbQuoteTarget
	// nbsp.md 3.1a NARROW-TARGET: what N2 writes, and what N8 writes for a narrow-nbsp pair.
	narrowTarget rune
}

// nbSingleCodePoint validates that a locale-data string entry is exactly one code point, as
// every entry in the fields read here must be (locale.schema.json). A locale file that violates
// its own schema is reported as POLYTYPO_MALFORMED_LOCALE_DATA.
//
// RuleFunc (registry.go) has no error return -- rules produce edits, not errors, everywhere else
// in this engine -- so this is signalled by panicking with an *engine.Error rather than by a
// return value. This mirrors modes.RecoverParsePanic's existing pattern of using a panic to carry
// a typed engine error across a boundary that cannot otherwise return one; whoever composes the
// top-level Transform entry point must recover it into a normal error the same way parse panics
// already are.
func nbSingleCodePoint(entry, field string) rune {
	cps := []rune(entry)
	if len(cps) != 1 {
		panic(engine.NewError(engine.CodeMalformedLocaleData,
			fmt.Sprintf("nbsp.%s entry %q is not exactly one code point", field, entry)))
	}
	return cps[0]
}

func nbContains(list []rune, cp rune) bool {
	for _, v := range list {
		if v == cp {
			return true
		}
	}
	return false
}

// nbPrepareList sorts entries longest-first, so each sub-rule's "longest match wins at a given a"
// (nbsp.md 3.5) is a linear search that returns on the first match.
func nbPrepareList(entries []string) [][]rune {
	out := make([][]rune, len(entries))
	for i, entry := range entries {
		out[i] = []rune(entry)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// prepareNbsp resolves the locale's nbsp and quotes fields to code points once. See nbsp.md 2 for
// the field list and 2.1 for why the mechanism (U+00A0 vs U+202F, convert-only vs insert) lives
// here and not in the locale file.
func prepareNbsp(locale spec.LocaleData, narrowTarget rune) nbPrepared {
	data := locale.NBSP

	beforePunctuation := make([]rune, len(data.BeforePunctuation))
	for i, entry := range data.BeforePunctuation {
		beforePunctuation[i] = nbSingleCodePoint(entry, "beforePunctuation")
	}
	narrowBeforePunctuation := make([]rune, len(data.NarrowBeforePunctuation))
	for i, entry := range data.NarrowBeforePunctuation {
		narrowBeforePunctuation[i] = nbSingleCodePoint(entry, "narrowBeforePunctuation")
	}
	// Precondition (nbsp.md 2): the two arrays must be disjoint. locale.schema.json does not
	// enforce this; an implementation that finds a code point in both must raise
	// POLYTYPO_MALFORMED_LOCALE_DATA rather than pick a winner silently.
	for _, cp := range beforePunctuation {
		if nbContains(narrowBeforePunctuation, cp) {
			panic(engine.NewError(engine.CodeMalformedLocaleData, fmt.Sprintf(
				"nbsp.beforePunctuation and nbsp.narrowBeforePunctuation both list U+%04X; they must be disjoint (spec/rules/nbsp.md 2)",
				cp)))
		}
	}

	primary := locale.Quotes.Primary
	secondary := locale.Quotes.Secondary
	opens := []rune{
		nbParenOpen, nbSquareOpen, nbBraceOpen,
		nbSingleCodePoint(primary.Open, "quotes.primary.open"),
		nbSingleCodePoint(secondary.Open, "quotes.secondary.open"),
	}
	closes := []rune{
		nbParenClose, nbSquareClose, nbBraceClose,
		nbSingleCodePoint(primary.Close, "quotes.primary.close"),
		nbSingleCodePoint(secondary.Close, "quotes.secondary.close"),
	}

	var quotePairs []nbQuoteTarget
	for _, pair := range [2]spec.QuotePair{primary, secondary} {
		if pair.InnerSpace == "none" {
			continue
		}
		open := nbSingleCodePoint(pair.Open, "quotes.open")
		close := nbSingleCodePoint(pair.Close, "quotes.close")
		// 3.10 sidedness precondition: an open glyph equal to its close glyph cannot be told
		// apart without the pairing information only `quotes` has. Documented no-op (7.5).
		if open == close {
			continue
		}
		target := narrowTarget
		if pair.InnerSpace == "nbsp" {
			target = nbNBSP
		}
		quotePairs = append(quotePairs, nbQuoteTarget{open: open, close: close, target: target})
	}

	return nbPrepared{
		beforePunctuation:       beforePunctuation,
		narrowBeforePunctuation: narrowBeforePunctuation,
		shortWords:              nbPrepareList(data.AfterShortWords),
		abbreviations:           nbPrepareList(data.Abbreviations),
		units:                   nbPrepareList(data.BeforeUnits),
		beforeNumber:            nbPrepareList(data.BeforeNumber),
		beforeWord:              nbPrepareList(data.BeforeWord),
		symbols:                 nbPrepareList(data.AfterSymbols),
		initialBinding:          data.InitialBinding,
		opens:                   opens,
		closes:                  closes,
		quotePairs:              quotePairs,
		narrowTarget:            narrowTarget,
	}
}

// nbIsOpenish / nbIsCloseish are OPENISH / CLOSEISH (nbsp.md 3.1): the ASCII brackets plus every
// locale quote glyph. engine.Marker is a member of CLOSEISH only; see nbIsBreak's comment.
func nbIsOpenish(prep nbPrepared, cp rune) bool {
	return nbContains(prep.opens, cp)
}

func nbIsCloseish(prep nbPrepared, cp rune) bool {
	return cp == engine.Marker || nbContains(prep.closes, cp)
}

func nbIsMark(prep nbPrepared, cp rune) bool {
	return nbContains(prep.beforePunctuation, cp) || nbContains(prep.narrowBeforePunctuation, cp)
}

// nbClaimConversion and nbClaimInsertion are the only writers into the claims table. Both no-op
// if the index is already claimed, which is what makes sub-rule evaluation order equal
// first-claim-wins (nbsp.md 3.2).
func nbClaimConversion(claims []*engine.Edit, index int, target rune) {
	if claims[index] != nil {
		return
	}
	claims[index] = &engine.Edit{Start: index, End: index + 1, Replacement: []rune{target}, RuleID: "nbsp"}
}

func nbClaimInsertion(claims []*engine.Edit, index int, target rune) {
	if claims[index] != nil {
		return
	}
	claims[index] = &engine.Edit{Start: index, End: index, Replacement: []rune{target}, RuleID: "nbsp"}
}

type nbMatcher func(cp []rune, a int, w []rune) bool

func nbMatchExact(cp []rune, a int, w []rune) bool {
	if a+len(w) > len(cp) {
		return false
	}
	for j, want := range w {
		if cp[a+j] != want {
			return false
		}
	}
	return true
}

// nbMatchFirstCharLenient is nbsp.md 3.5 step 1: exact except that the pattern's first code point
// may also match its Unicode simple uppercase mapping -- a plain code-point-to-code-point table,
// never a locale-sensitive case operation (ARCHITECTURE.md 4.4).
func nbMatchFirstCharLenient(cp []rune, a int, w []rune) bool {
	if len(w) == 0 || a+len(w) > len(cp) {
		return false
	}
	head := cp[a]
	if head != w[0] && head != engine.SimpleUppercase(w[0]) {
		return false
	}
	for j := 1; j < len(w); j++ {
		if cp[a+j] != w[j] {
			return false
		}
	}
	return true
}

// nbMatchSpaceLenient is nbsp.md 3.6 step 1: exact except that a pattern U+0020 also matches an
// existing U+00A0 or U+202F in the input, so a previously-converted abbreviation still matches on
// a later run (the idempotency property nbsp.md 5 item 2 requires of N4).
func nbMatchSpaceLenient(cp []rune, a int, w []rune) bool {
	if a+len(w) > len(cp) {
		return false
	}
	for j, want := range w {
		got := cp[a+j]
		if got == want {
			continue
		}
		if want == nbSpace && nbIsNoBreak(got) {
			continue
		}
		return false
	}
	return true
}

// nbLongestMatch returns the first pattern (from a longest-first-sorted list) that matches at a,
// implementing "longest match wins at a given a, with no backtracking" (nbsp.md 3.5) for every
// list-driven sub-rule: N3, N4, N5, N6, N9, N10.
func nbLongestMatch(patterns [][]rune, cp []rune, a int, matcher nbMatcher) ([]rune, bool) {
	for _, w := range patterns {
		if matcher(cp, a, w) {
			return w, true
		}
	}
	return nil, false
}

// nbPunctuationSubRule is N1 (3.3, beforePunctuation -> U+00A0) and N2 (3.4,
// narrowBeforePunctuation -> U+202F): identical shape with target/other exchanged.
// nbEndsCharacterReference reports whether the code points left of this ";" have the shape of a
// character reference: a bounded left walk over ASCII alphanumerics, optionally one "#", then "&".
// Shape, not the HTML named-reference table -- declining on "&notaname;" costs nothing, and no
// runtime carries thousands of entries for it. The bound is the longest named reference plus one.
func nbEndsCharacterReference(cp []rune, i int) bool {
	j := i - 1
	for j >= 0 && nbIsASCIIAlphanumeric(cp[j]) {
		j--
	}
	length := i - 1 - j
	if length < 1 || length > nbMaxCharacterReferenceName {
		return false
	}
	if j >= 0 && cp[j] == nbHash {
		j--
	}
	return j >= 0 && cp[j] == nbAmpersand
}

func nbIsASCIIAlphanumeric(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func nbPunctuationSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit, marks []rune, target, other rune) {
	if len(marks) == 0 {
		return
	}
	for i := 0; i < len(cp); i++ {
		if !nbContains(marks, cp[i]) {
			continue
		}
		left := nbspAt(cp, i-1)
		// Step 1 -- run guard: only the first mark of "?!" or "!!!" takes the space.
		if left != engine.None && nbIsMark(prep, left) {
			continue
		}
		// Step 2 -- right-context guard: this is what protects "http://" and "12:30". U+2026
		// is accepted because the guard exists to catch punctuation *inside a token*, and an
		// ellipsis after a question mark is not that (nbsp.md 3.3 step 2).
		after := nbspAt(cp, i+1)
		if after != engine.None && !nbIsSpaceLike(after) && !nbIsCloseish(prep, after) &&
			after != nbEllipsis && !nbIsMark(prep, after) {
			continue
		}
		// Step 3 -- quote-glyph guard. The space beside an opening quotation glyph is
		// quotes.innerSpace and belongs to N8 alone; without this N2 and N8 alternate for
		// ever on the French input "«?" (3.2, 3.10.1).
		if nbIsOpenish(prep, left) {
			continue
		}
		if left != engine.None && nbIsSpaceLike(left) && nbIsOpenish(prep, nbspAt(cp, i-2)) {
			continue
		}
		// Step 4 (spec 1.3.0) -- character-reference guard. text mode has no markup concept, so
		// a locale listing ";" used to insert before the ";" that *ends* a reference and
		// "Bonjour&#160;: oui" stopped being what it was (nbsp.md 3.3 step 4).
		if cp[i] == nbSemicolon && nbEndsCharacterReference(cp, i) {
			continue
		}
		// Step 5.
		if left == target {
			continue
		}
		if left == nbSpace || left == other {
			nbClaimConversion(claims, i-1, target)
			continue
		}
		if nbIsOtherSpace(left) {
			continue
		}
		if left == engine.None || nbIsBreak(left) || left == nbTab {
			continue
		}
		nbClaimInsertion(claims, i, target)
	}
}

// nbShortWordsSubRule is N3 (nbsp.md 3.5, afterShortWords -> U+00A0, conversion only).
func nbShortWordsSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	if len(prep.shortWords) == 0 {
		return
	}
	for a := 0; a < len(cp); a++ {
		w, ok := nbLongestMatch(prep.shortWords, cp, a, nbMatchFirstCharLenient)
		if !ok {
			continue
		}
		k := len(w)
		before := nbspAt(cp, a-1)
		if !(before == engine.None || nbIsSpaceLike(before) || nbIsOpenish(prep, before) || nbIsSentenceDash(before)) {
			continue
		}
		separator := nbspAt(cp, a+k)
		if separator == nbNBSP {
			continue // already correct
		}
		if separator != nbSpace {
			continue
		}
		next := nbspAt(cp, a+k+1)
		if !nbIsAlnum(next) && !nbIsOpenish(prep, next) {
			continue
		}
		nbClaimConversion(claims, a+k, nbNBSP)
	}
}

// nbAbbreviationsSubRule is N4 (nbsp.md 3.6, abbreviations -> U+00A0 for every internal space).
func nbAbbreviationsSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	if len(prep.abbreviations) == 0 {
		return
	}
	for a := 0; a < len(cp); a++ {
		w, ok := nbLongestMatch(prep.abbreviations, cp, a, nbMatchSpaceLenient)
		if !ok {
			continue
		}
		k := len(w)
		if nbIsAlnum(nbspAt(cp, a-1)) || nbIsAlnum(nbspAt(cp, a+k)) {
			continue
		}
		for j := 0; j < k; j++ {
			if w[j] != nbSpace {
				continue
			}
			if cp[a+j] == nbNBSP {
				continue // already correct at this internal position
			}
			nbClaimConversion(claims, a+j, nbNBSP)
		}
	}
}

// nbUnitsSubRule is N5 (nbsp.md 3.7, beforeUnits -> U+00A0). Converts an existing space; never
// inserts one (§7.2).
func nbUnitsSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	if len(prep.units) == 0 {
		return
	}
	for a := 0; a < len(cp); a++ {
		w, ok := nbLongestMatch(prep.units, cp, a, nbMatchExact)
		if !ok {
			continue
		}
		k := len(w)
		if nbIsAlnum(nbspAt(cp, a+k)) {
			continue
		}
		left := nbspAt(cp, a-1)
		if left == nbNBSP {
			continue // already correct
		}
		if left != nbSpace {
			continue
		}
		if !nbIsDigit(nbspAt(cp, a-2)) {
			continue
		}
		b := a - 2
		for b-1 >= 0 && nbIsDigit(cp[b-1]) {
			b--
		}
		// The letter guard: "H2 O", "A4", "MP3" are not measurements.
		if engine.IsLetter(nbspAt(cp, b-1)) {
			continue
		}
		nbClaimConversion(claims, a-1, nbNBSP)
	}
}

// nbSymbolsSubRule is N6 (nbsp.md 3.8, afterSymbols -> U+00A0). Conversion only.
func nbSymbolsSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	if len(prep.symbols) == 0 {
		return
	}
	for a := 0; a < len(cp); a++ {
		w, ok := nbLongestMatch(prep.symbols, cp, a, nbMatchExact)
		if !ok {
			continue
		}
		k := len(w)
		if nbIsAlnum(nbspAt(cp, a-1)) {
			continue
		}
		separator := nbspAt(cp, a+k)
		if separator == nbNBSP {
			continue // already correct
		}
		if separator != nbSpace {
			continue
		}
		if !nbIsDigit(nbspAt(cp, a+k+1)) {
			continue
		}
		nbClaimConversion(claims, a+k, nbNBSP)
	}
}

// nbIsInitialAt is nbsp.md 3.9: one uppercase letter, one full stop, at a token start.
func nbIsInitialAt(cp []rune, prep nbPrepared, p int) bool {
	if p < 0 {
		return false
	}
	if !engine.IsUpper(nbspAt(cp, p)) {
		return false
	}
	if nbspAt(cp, p+1) != nbFullStop {
		return false
	}
	before := nbspAt(cp, p-1)
	return before == engine.None || nbIsSpaceLike(before) || nbIsOpenish(prep, before)
}

// nbIsAbbreviationTail is guard C1-a (nbsp.md 3.9): an uppercase letter plus a dot that is itself
// preceded by a lower-case letter plus a dot is the second token of an abbreviation, not an
// initial. Without it the shipped de-DE data turns "z. B. Berlin" into "z.⍽B.⍽Berlin" -- a false
// positive on ordinary prose. "А. С. Пушкин" is unaffected: cp[p-3] there is uppercase.
func nbIsAbbreviationTail(cp []rune, p int) bool {
	if !nbIsSpaceLike(nbspAt(cp, p-1)) {
		return false
	}
	if nbspAt(cp, p-2) != nbFullStop {
		return false
	}
	head := nbspAt(cp, p-3)
	return engine.IsLetter(head) && !engine.IsUpper(head)
}

// nbHasPrecedingInitial is the "chain" mode confirmation (nbsp.md 3.9): is the initial whose
// letter sits at p itself immediately preceded by another initial? Used only by "chain" mode's
// C1, to require Chicago's own "two or more initials" before the space leading into a following
// non-initial word (a candidate surname) is bound.
func nbHasPrecedingInitial(cp []rune, prep nbPrepared, p int) bool {
	gap := nbspAt(cp, p-1)
	if gap != nbSpace && gap != nbNBSP {
		return false
	}
	if nbspAt(cp, p-2) != nbFullStop {
		return false
	}
	return nbIsInitialAt(cp, prep, p-3)
}

// nbInitialsSubRule is N7 (nbsp.md 3.9, initialBinding -> U+00A0), skipped entirely when the
// locale's initialBinding is "none".
func nbInitialsSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	mode := prep.initialBinding
	if mode == "none" {
		return
	}
	for q := 0; q < len(cp); q++ {
		here := cp[q]
		if here != nbSpace && here != nbNBSP {
			continue
		}

		// C1 -- an initial on the left and an uppercase letter on the right, unless C1-a
		// declines.
		leftInitialP := q - 2
		c1Shape := nbspAt(cp, q-1) == nbFullStop &&
			nbIsInitialAt(cp, prep, leftInitialP) &&
			engine.IsUpper(nbspAt(cp, q+1)) &&
			!nbIsAbbreviationTail(cp, leftInitialP)
		// "chain" mode additionally requires either that the right side is itself an initial
		// (the between-initials case, e.g. "E.|B.", always safe) or that the left initial is
		// itself preceded by another initial (a confirmed chain of two or more, e.g.
		// "E. B.|White") before binding to a plain following word. "single" mode keeps the
		// unconditional shape check -- the behaviour fr/fr-CA need for "N. Bourbaki"/
		// "M. Dupont" (nbsp.md 3.9, Jacques André), structurally indistinguishable from a
		// sentence-boundary collision.
		c1 := c1Shape && (mode == "single" ||
			nbIsInitialAt(cp, prep, q+1) ||
			nbHasPrecedingInitial(cp, prep, leftInitialP))

		// C2 -- a word on the left and two consecutive initials on the right ("Пушкин А. С.").
		// Already requires two initials by construction, so it is unaffected by
		// "chain" vs "single".
		rightSpace := nbspAt(cp, q+3)
		c2 := engine.IsLetter(nbspAt(cp, q-1)) &&
			nbIsInitialAt(cp, prep, q+1) &&
			(rightSpace == nbSpace || rightSpace == nbNBSP) &&
			nbIsInitialAt(cp, prep, q+4)

		if !c1 && !c2 {
			continue
		}
		if here == nbNBSP {
			continue // already correct
		}
		nbClaimConversion(claims, q, nbNBSP)
	}
}

// nbQuotesSubRule is N8 (nbsp.md 3.10, quotes.innerSpace). The only sub-rule besides N1/N2 that
// may insert.
func nbQuotesSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit) {
	for _, pair := range prep.quotePairs {
		for i := 0; i < len(cp); i++ {
			here := cp[i]
			if here == pair.open {
				right := nbspAt(cp, i+1)
				switch {
				case right == pair.target:
					// already correct
				case right == nbSpace || nbIsNoBreak(right):
					nbClaimConversion(claims, i+1, pair.target)
				case right == engine.None || nbIsBreak(right):
					// skip: never insert at a line boundary or the end of the text
				default:
					nbClaimInsertion(claims, i+1, pair.target)
				}
				continue
			}
			if here != pair.close {
				continue
			}
			left := nbspAt(cp, i-1)
			switch {
			case left == pair.target:
				// already correct
			case left == nbSpace || nbIsNoBreak(left):
				nbClaimConversion(claims, i-1, pair.target)
			case left == engine.None || nbIsBreak(left):
				// skip
			default:
				nbClaimInsertion(claims, i, pair.target)
			}
		}
	}
}

// nbForwardBindingSubRule is N9 (nbsp.md 3.11, beforeNumber, wantsDigit=true) and N10 (3.12,
// beforeWord, wantsDigit=false). They share every guard except what must follow the separator: a
// digit for N9, a letter for N10.
func nbForwardBindingSubRule(cp []rune, prep nbPrepared, claims []*engine.Edit, patterns [][]rune, wantsDigit bool) {
	if len(patterns) == 0 {
		return
	}
	for a := 0; a < len(cp); a++ {
		w, ok := nbLongestMatch(patterns, cp, a, nbMatchExact)
		if !ok {
			continue
		}
		k := len(w)

		// G-D (3.12 step 5, N10 only): in a locale where N7 is active, an *uppercase* letter
		// plus a dot is structurally an initial, and N7 owns that shape with better evidence
		// (it inspects what follows for a second initial or a surname). The UPPER test is
		// load-bearing: without it a lower-case entry such as "ул." would be inert in an
		// initialBinding-active locale.
		if !wantsDigit && prep.initialBinding != "none" && k == 2 &&
			engine.IsUpper(w[0]) && w[1] == nbFullStop {
			continue
		}

		// G-L -- stronger than "not ALNUM": it is what stops "S." matching inside "Fig.S. 3".
		// A hyphen fails it, per 3.5 step 2 -- an abbreviation cannot begin immediately after
		// an intra-word hyphen.
		before := nbspAt(cp, a-1)
		if !(before == engine.None || nbIsSpaceLike(before) || nbIsOpenish(prep, before) || nbIsSentenceDash(before)) {
			continue
		}

		// G-S -- exactly one separator, and it must already be a space.
		separator := nbspAt(cp, a+k)
		if separator == nbNBSP {
			continue // already correct
		}
		if separator != nbSpace {
			continue
		}
		next := nbspAt(cp, a+k+1)
		if nbIsSpaceLike(next) {
			continue
		}

		// G-W / "a following number": one code point, tested for membership. NONE fails
		// both, which is also the line-boundary guard G-B.
		if wantsDigit {
			if !nbIsDigit(next) {
				continue
			}
		} else if !engine.IsLetter(next) {
			continue
		}

		nbClaimConversion(claims, a+k, nbNBSP)
	}
}

// scanNbsp is nbsp.md 3.2: N1 through N10, in that fixed order, first claim wins. The order is
// positional and total, never an artefact of map iteration.
//
// First-claim-wins only settles a conflict when both sub-rules actually emit an edit; an
// "already correct" branch emits nothing and therefore claims nothing, silently yielding the
// index to a lower-priority sub-rule. Sub-rules wanting *different* code points at a shared index
// are therefore made disjoint by construction elsewhere (N1/N2's quote-glyph guard, nbsp.md
// 3.10.1) rather than relying on ordering alone.
func scanNbsp(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	prep := prepareNbsp(locale, ctx.NarrowTarget)
	claims := make([]*engine.Edit, len(cp)+1)

	nbPunctuationSubRule(cp, prep, claims, prep.beforePunctuation, nbNBSP, nbNNBSP) // N1
	// N2's target is NARROW-TARGET (nbsp.md 3.1a); `other` is the NOBREAK member that is not the
	// target, which is what the sub-rule converts. With the substitution on, N2 and N1 want the
	// same character — never different ones.
	nbOther := nbNBSP
	if prep.narrowTarget == nbNBSP {
		nbOther = nbNNBSP
	}
	nbPunctuationSubRule(cp, prep, claims, prep.narrowBeforePunctuation, prep.narrowTarget, nbOther) // N2
	nbShortWordsSubRule(cp, prep, claims)                                                            // N3
	nbAbbreviationsSubRule(cp, prep, claims)                                                         // N4
	nbUnitsSubRule(cp, prep, claims)                                                                 // N5
	nbSymbolsSubRule(cp, prep, claims)                                                               // N6
	nbInitialsSubRule(cp, prep, claims)                                                              // N7
	nbQuotesSubRule(cp, prep, claims)                                                                // N8
	nbForwardBindingSubRule(cp, prep, claims, prep.beforeNumber, true)                               // N9
	nbForwardBindingSubRule(cp, prep, claims, prep.beforeWord, false)                                // N10

	var edits []engine.Edit
	for i := 0; i <= len(cp); i++ {
		if claims[i] != nil {
			edits = append(edits, *claims[i])
		}
	}
	return edits
}
