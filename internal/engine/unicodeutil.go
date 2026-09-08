package engine

import "unicode"

// Unicode general-category predicates for the rule engine.
//
// The JS reference implementation hand-rolls these as static binary-searched code-point-range
// tables (src/engine/unicode.ts), because Go's own RE2 has no \p{L}-style Unicode property
// escapes (docs/ARCHITECTURE.md section 4.1) — but that rationale is about not depending on a
// *regex engine*, not about avoiding the standard library's own Unicode Character Database.
// Go's unicode package is a direct, deterministic UCD lookup, never locale-dependent in the
// ARCHITECTURE.md section 4.4 sense (it has no locale parameter at all, so there is no Turkish
// dotless-i problem: unicode.ToUpper('i') is always 'I'). The Python port made the same choice
// for the same reason ("more correct than porting JS's frozen-at-one-UCD-version table, not a
// shortcut" — internal/_engine/unicode.py).
//
// UCD version note (matches spec/rules/apostrophe.md 7 and nbsp.md 7's documented gap, and
// mirrors Python's own equivalent note): unicode.Version reports the Unicode Character Database
// version compiled into the *building* Go toolchain, which this module does not control. The Go
// toolchain used during this port (1.27.1) reports "17.0.0", matching spec/UNICODE exactly and
// the JS reference implementation's own frozen table — but a module built with an older Go
// toolchain compiles against an older UCD and can disagree on newly assigned code points. The
// spec does not yet pin a UCD version across runtimes, so this is an acknowledged, spec-level
// gap, not a defect in this port.

var letterCategories = []*unicode.RangeTable{unicode.L, unicode.M}
var upperCategories = []*unicode.RangeTable{unicode.Lu, unicode.Lt}

// IsLetter is true for a code point in Lu, Ll, Lt, Lm, Lo, Mn, Mc or Me.
func IsLetter(cp rune) bool {
	if cp < 0 || cp > 0x10FFFF {
		return false
	}
	return unicode.IsOneOf(letterCategories, cp)
}

// IsUpper is true for a code point in Lu or Lt — spec 3.1 UPPER, read by nbsp's N7.
func IsUpper(cp rune) bool {
	if cp < 0 || cp > 0x10FFFF {
		return false
	}
	return unicode.IsOneOf(upperCategories, cp)
}

// SimpleUppercase returns the Unicode simple (one-code-point-to-one-code-point) uppercase mapping
// of cp, or cp itself if it has none. Needed by hyphen.md 3.3 and nbsp.md 3.5 for matching a
// pattern's first character case-insensitively — applied to the *pattern*, never to the input,
// and never via a whole-string case conversion (ARCHITECTURE.md section 4.4). Go's
// unicode.ToUpper is already a simple (rune-to-rune) mapping with no expansion case (unlike
// German ß -> "SS", which only exists in *full* case mapping and has no single-rune result), so —
// unlike the Python port, which must guard against Python's str.upper() sometimes expanding to
// more than one character — this needs no extra guard.
func SimpleUppercase(cp rune) rune {
	if cp < 0 || cp > 0x10FFFF {
		return cp
	}
	return unicode.ToUpper(cp)
}
