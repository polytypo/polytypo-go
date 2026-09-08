package engine

// ToCodePoints converts a string to its Unicode code points (docs/ARCHITECTURE.md section 4.2:
// rules index an explicit code-point array, never a native string). Go's string-to-[]rune
// conversion already decodes UTF-8 to code points one rune at a time, so this is close to free —
// unlike the JS reference implementation, which must undo UTF-16 surrogate pairs by hand.
//
// One JS-specific behaviour has no Go equivalent and is not attempted here: the JS engine carries
// an unpaired UTF-16 surrogate through as its own "code point" so malformed UTF-16 input still
// round-trips. Surrogate code points (U+D800-U+DFFF) are not valid Unicode scalar values and have
// no standalone UTF-8 encoding — Go's decoder replaces any byte sequence that would decode to one
// with U+FFFD, the same as every other Go program. No spec fixture exercises this JS-only case
// (it is a JS engine unit test, not a cross-runtime conformance fixture); this is a documented,
// structural non-goal for this runtime, not a gap.
func ToCodePoints(s string) []rune {
	return []rune(s)
}

// FromCodePoints converts a code-point array back to a string.
func FromCodePoints(cp []rune) string {
	return string(cp)
}

// IsValidCodePoint reports whether value is a real Unicode code point (used to validate rule
// output, not input — see edits.go).
func IsValidCodePoint(value rune) bool {
	return value >= 0 && value <= 0x10FFFF
}
