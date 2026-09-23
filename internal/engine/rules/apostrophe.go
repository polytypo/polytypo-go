package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/apostrophe.md (spec 1.2.0), order 50.
//
// Converts a straight U+0027 to U+2019 where it is genuinely an apostrophe: a contraction, an
// elision, a possessive, or a decade elision. Runs immediately after `quotes` (order 40) and
// sees only the U+0027 marks quotes declined to claim. Every edit is one code point replacing
// one code point; the rule never inserts, never deletes, and never touches U+2019 itself.
//
// As of spec 1.1.0 this rule reads no locale data and skips no position (apostrophe.md 3.4).
// Spec 0.5.0's preserve set existed to stop the case ladder from converting the marks quotes had
// vetoed; conversion is now the specified outcome for exactly those marks — cases 4 and 3 are
// what turn `rock 'n' roll` into `rock ’n’ roll`, in every locale. Case 3a (spec 1.2.0) reads
// no locale data either: OPENQUOTE and CLOSEDELIM are fixed sets.

const apoSQ = rune(0x27)
const apoRightSingle = rune(0x2019)

// apoOpenish is apostrophe.md 3.1 OPENISH. Unlike quotes.md's OPENISH, this is not "every
// QUOTEMARK" — only the specific opening-shaped glyphs the spec lists. engine.Marker is a member
// (modes.md 3.3's table names this rule explicitly).
var apoOpenish = map[rune]bool{
	engine.Marker: true,
	0x28:          true,
	0x5B:          true,
	0x7B:          true,
	0xAB:          true,
	0x2018:        true,
	0x201A:        true,
	0x201B:        true,
	0x201C:        true,
	0x201E:        true,
	0x201F:        true,
	0x2039:        true,
	0x2D:          true,
	0x2011:        true,
	0x2013:        true,
	0x2014:        true,
}

// apoCloseish is apostrophe.md 3.1 CLOSEISH. U+2019 is a member and U+0027 is a member of
// neither this nor apoOpenish — apostrophe.md 5 turns exactly that asymmetry into the
// idempotency argument. U+2011 sits beside U+002D because `hyphen` (order 35) converts one to
// the other.
var apoCloseish = map[rune]bool{
	engine.Marker: true,
	0x29:          true,
	0x5D:          true,
	0x7D:          true,
	0xBB:          true,
	0x2019:        true,
	0x201D:        true,
	0x203A:        true,
	0x2C:          true,
	0x2E:          true,
	0x3B:          true,
	0x3A:          true,
	0x21:          true,
	0x3F:          true,
	0x2026:        true,
	0x2D:          true,
	0x2011:        true,
	0x2013:        true,
	0x2014:        true,
}

// apoIsSpacelike is apostrophe.md 3.1 SPACELIKE, including engine.LineMarker as a member of
// BREAK for every rule everywhere (modes.md 3.2).
func apoIsSpacelike(cp rune) bool {
	switch cp {
	case 0x20, 0x09, 0xA0, 0x202F, 0x2007, 0x2009, 0x200A,
		0x0A, 0x0D, 0x0B, 0x0C, 0x85, 0x2028, 0x2029, engine.LineMarker:
		return true
	default:
		return false
	}
}

// apoIsOpenquote is apostrophe.md 3.1 OPENQUOTE (spec 1.2.0): the quotation glyphs of
// apoOpenish, without its brackets and dashes — `f'(x)` is a prime and must stay as typed.
// engine.Marker is not a member: case 3 already accepts it through apoCloseish (modes.md 3.3).
func apoIsOpenquote(cp rune) bool {
	switch cp {
	case 0xAB, 0x2018, 0x201A, 0x201B, 0x201C, 0x201E, 0x201F, 0x2039:
		return true
	default:
		return false
	}
}

// apoIsClosedelim is apostrophe.md 3.1 CLOSEDELIM (spec 1.5.0, case 2a): the bracket and
// quotation members of apoCloseish, without its sentence punctuation and without the dashes
// apoOpenish already carries. These are exactly the closing delimiters case 3 has always
// accepted on the mark's RIGHT; before 1.5.0 no left-hand test accepted any of them.
// engine.Marker is not a member (modes.md 3.3): it is in apoOpenish, so a mark against a span
// boundary already reaches case 4 and emits the same U+2019.
func apoIsClosedelim(cp rune) bool {
	switch cp {
	case 0x29, 0x5D, 0x7D, 0xBB, 0x2019, 0x201D, 0x203A:
		return true
	default:
		return false
	}
}

func apoIsDigit(cp rune) bool {
	return cp >= 0x30 && cp <= 0x39
}

func apoIsAlnum(cp rune) bool {
	return apoIsDigit(cp) || engine.IsLetter(cp)
}

// apoIsApostrophe is the case ladder of apostrophe.md 3.3, first match wins. Every verdict is a
// pure function of exactly two neighbouring code points; there is no lookahead and no state
// carried between candidates.
func apoIsApostrophe(left, right rune) bool {
	// Case 1 — prime guard, first so it wins over case 3: `6' 2"`, `55° 40' N`, `6'2"`. A foot
	// mark is not an apostrophe.
	if apoIsDigit(left) && !engine.IsLetter(right) {
		return false
	}
	// Case 2 — medial apostrophe: `don't`, `l'été`, `O'Brien`, `1990's`.
	if apoIsAlnum(left) && apoIsAlnum(right) {
		return true
	}
	// Case 2a — suffix or possessive after a closing delimiter (spec 1.5.0): `(order 90)'s`,
	// `“Hamlet”'s`, `{user}'s`. Disjoint from every other case, so its position in the ladder
	// carries no behaviour.
	if apoIsClosedelim(left) && apoIsAlnum(right) {
		return true
	}
	// Case 3 — trailing elision or possessive: `the dogs' bowls`, `Jesus'`, `rock 'n'` (the
	// trailing mark).
	if left != engine.None && engine.IsLetter(left) &&
		(right == engine.None || apoIsSpacelike(right) || apoCloseish[right]) {
		return true
	}
	// Case 3a — elision before a quotation: `d'« urine »`, `l'“idea”`, `dell'‘arte’`; also the
	// de-DE possessive `„Hans'“`, whose closing U+201C is not in CLOSEISH.
	if engine.IsLetter(left) && apoIsOpenquote(right) {
		return true
	}
	// Case 4 — leading elision: `'90s`, `'tis`, `'em`, `'n'` (the leading mark). The replacement
	// is U+2019, never U+2018 — a leading elision is a raised comma, not an opening quotation
	// mark, and `quotes` has already had its chance to claim the mark as a quotation and
	// declined (quotes.md 3.2, 5).
	if (left == engine.None || apoIsSpacelike(left) || apoOpenish[left]) && apoIsAlnum(right) {
		return true
	}
	// Case 5 — nothing inferable: `a ' b`, `''`. Leave it.
	return false
}

func init() {
	engine.RegisterRule("apostrophe", scanApostrophe)
}

func scanApostrophe(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	n := len(cp)

	var edits []engine.Edit
	for i := 0; i < n; i++ {
		if cp[i] != apoSQ {
			continue
		}

		left := qaAt(cp, i-1)
		right := qaAt(cp, i+1)
		if apoIsApostrophe(left, right) {
			edits = append(edits, engine.Edit{Start: i, End: i + 1, Replacement: []rune{apoRightSingle}, RuleID: "apostrophe"})
		}
	}

	return edits
}
