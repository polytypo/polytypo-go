package engine

// The three non-code-point values a rule can meet in the array it scans. They live together and
// must stay pairwise disjoint (mirrors src/engine/sentinels.ts in the JS reference
// implementation, and internal/engine/sentinels.py's equivalent in the Python port):
//
//   - NONE — there is nothing at that index; the array ends here.
//   - MARKER — a span boundary whose skipped region has no line terminator. Per modes.md 3.3 it
//     is opaque content everywhere except OPENISH/CLOSEISH: a member of both for quotes and
//     apostrophe, of CLOSEISH only for nbsp (spec 1.2.0). It is explicitly not NONE and not
//     SPACELIKE.
//   - LINE_MARKER — a span boundary whose skipped region contains a line terminator. A member of
//     BREAK for every rule, everywhere, so a mode's output on hard-wrapped prose is identical to
//     text mode's on the same characters (modes.md 3.2, 7.4).
//
// A new sentinel goes here and nowhere else, takes the next free negative integer, and is
// classified in modes.md before any rule reads it.
const (
	Marker     rune = -1
	LineMarker rune = -2
	None       rune = -3
)

// IsMarker is true for either span boundary. None is deliberately not a marker: it is not in the
// array.
func IsMarker(value rune) bool {
	return value == Marker || value == LineMarker
}
