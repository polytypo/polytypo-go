package engine

// analyze.md section 2: every reported offset is a code-point offset into the input the caller
// passed, in every mode. The rules, however, run over an array that is not the input — in "text"
// mode edits from earlier rules have already shifted it, and in "html"/"markdown" mode it is the
// marker-joined concatenation of the processable spans (modes.md 3.5). This file carries the one
// structure that bridges the two: an origin map, parallel to the current code-point array,
// holding the input offset each code point came from, or NoOrigin for one the pipeline itself
// produced. Mirrors polytypo-js's src/engine/origin.ts.

// NoOrigin marks a code point the pipeline produced, which came from nowhere in the input.
const NoOrigin = -1

// Change is one entry of Analyze's result, in input coordinates (analyze.md section 2).
type Change struct {
	// RuleID is a rule id from spec/rules/order.json.
	RuleID string
	// Start is a code-point offset into the input, inclusive; End is exclusive. Start == End is
	// a pure insertion, and Before is then empty.
	Start int
	End   int
	// Before is the text this rule replaced, After what it replaced it with.
	Before string
	After  string
}

// OriginAt returns the input offset an edit boundary at index addresses. Synthetic code points
// have no origin of their own, so the scan runs forward to the first that has one — an insertion
// between two earlier insertions still lands where the next real character is. Falling off the
// end means the boundary is at the end of the input.
func OriginAt(origin []int, index, inputLength int) int {
	for i := index; i < len(origin); i++ {
		if origin[i] != NoOrigin {
			return origin[i]
		}
	}
	return inputLength
}

// ApplyEditsToOrigin returns the origin map for the array ApplyEdits is about to produce. A
// replacement of equal length keeps its origins position by position, which is what makes a
// conversion (U+0020 -> U+00A0) still point at the character it converted; anything longer is
// synthetic beyond the positions it covers.
func ApplyEditsToOrigin(origin []int, edits []Edit) []int {
	if len(edits) == 0 {
		out := make([]int, len(origin))
		copy(out, origin)
		return out
	}
	out := make([]int, 0, len(origin))
	cursor := 0
	for _, edit := range edits {
		out = append(out, origin[cursor:edit.Start]...)
		for k := range edit.Replacement {
			source := edit.Start + k
			if source < edit.End {
				out = append(out, origin[source])
			} else {
				out = append(out, NoOrigin)
			}
		}
		cursor = edit.End
	}
	out = append(out, origin[cursor:]...)
	return out
}

// RecordChanges renders one rule's edits, in the coordinates that rule saw, as Changes in input
// coordinates. Before is the text this rule replaced and After what it replaced it with
// (analyze.md section 2), so on a text two rules have both touched, Before is what the second
// rule saw rather than what the caller typed — section 5 says so and shows the French case where
// it matters.
func RecordChanges(cp []rune, edits []Edit, origin []int, inputLength int, ruleID string) []Change {
	changes := make([]Change, 0, len(edits))
	for _, edit := range edits {
		changes = append(changes, Change{
			RuleID: ruleID,
			Start:  OriginAt(origin, edit.Start, inputLength),
			End:    OriginAt(origin, edit.End, inputLength),
			Before: string(cp[edit.Start:edit.End]),
			After:  string(edit.Replacement),
		})
	}
	return changes
}
