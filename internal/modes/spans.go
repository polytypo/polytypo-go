// Package modes holds the span model shared by every mode adapter that is not "text"
// (spec/rules/modes.md 3.2-3.5). Mirrors polytypo-js's src/modes/spans.ts and the Python port's
// internal/_modes/spans.py exactly: markers are written here (mode layer), classified in
// engine/sentinels.go and in each rule's own class definitions (L1).
package modes

import (
	"fmt"

	"github.com/polytypo/polytypo-go/internal/engine"
)

var lineTerminators = map[rune]bool{
	0x0A: true, 0x0D: true, 0x0B: true, 0x0C: true, 0x85: true, 0x2028: true, 0x2029: true,
}

// Span is a processable span, identified by its offsets in the original source, addressed as
// code-point indices (ARCHITECTURE.md section 4.2) — never byte offsets, which is exactly what a
// Go-specific mode adapter must convert away from before constructing one of these.
type Span struct {
	Start int
	End   int
}

// SpanRange is a span's extent in the concatenated code-point array: s0/s1 of modes.md 3.4.
type SpanRange struct {
	First int
	Last  int
}

func gapIsLineBoundary(cp []rune, from, to int) bool {
	for i := from; i < to; i++ {
		if lineTerminators[cp[i]] {
			return true
		}
	}
	return false
}

// NormalizeSpans sorts, drops empties, and coalesces spans separated by nothing in the source
// (modes.md 7.5). Overlapping spans are an extractor bug and are rejected rather than silently
// merged.
func NormalizeSpans(spans []Span) ([]Span, error) {
	filtered := make([]Span, 0, len(spans))
	for _, s := range spans {
		if s.End > s.Start {
			filtered = append(filtered, s)
		}
	}
	// Stable insertion sort by Start — spans are typically already close to sorted and the
	// counts involved are small (a document's processable spans, not its full byte length).
	for i := 1; i < len(filtered); i++ {
		for j := i; j > 0 && filtered[j-1].Start > filtered[j].Start; j-- {
			filtered[j-1], filtered[j] = filtered[j], filtered[j-1]
		}
	}

	out := make([]Span, 0, len(filtered))
	for _, span := range filtered {
		if len(out) == 0 {
			out = append(out, span)
			continue
		}
		last := &out[len(out)-1]
		if span.Start < last.End {
			return nil, engine.NewError(engine.CodeRuleContract, fmt.Sprintf(
				"mode extractor produced overlapping spans (%d, %d) and (%d, %d)",
				last.Start, last.End, span.Start, span.End))
		}
		if span.Start == last.End {
			last.End = span.End
			continue
		}
		out = append(out, span)
	}
	return out, nil
}

// ConcatenateSpans builds S1 (marker) S2 ... Sm (modes.md 3.5 step 2), given the source's full
// code-point array (so offsets in spans, which are code-point indices, can be sliced directly).
func ConcatenateSpans(sourceCP []rune, spans []Span) []rune {
	cp := make([]rune, 0, len(sourceCP))
	hasPrevious := false
	var previous Span
	for _, span := range spans {
		if hasPrevious {
			if gapIsLineBoundary(sourceCP, previous.End, span.Start) {
				cp = append(cp, engine.LineMarker)
			} else {
				cp = append(cp, engine.Marker)
			}
		}
		cp = append(cp, sourceCP[span.Start:span.End]...)
		previous = span
		hasPrevious = true
	}
	return cp
}

// SpanRangesOf returns the span extents of the array as it stands. Recomputed after every rule,
// because applying edits shifts every index after the first one — the markers themselves always
// survive, since no edit may contain one.
func SpanRangesOf(cp []rune) []SpanRange {
	ranges := make([]SpanRange, 0)
	first := 0
	for i, value := range cp {
		if engine.IsMarker(value) {
			ranges = append(ranges, SpanRange{First: first, Last: i - 1})
			first = i + 1
		}
	}
	ranges = append(ranges, SpanRange{First: first, Last: len(cp) - 1})
	return ranges
}

func spanContaining(ranges []SpanRange, p int) (SpanRange, bool) {
	for _, r := range ranges {
		if r.First <= p && p <= r.Last+1 {
			return r, true
		}
	}
	return SpanRange{}, false
}

// space is U+0020, the one emitted code point whose meaning is positional (modes.md 3.4,
// 5 item 2).
const space = ' '

// FilterBoundaryEdits is modes.md 3.4, two safety nets, both pure functions of (p, q, r, s0, s1):
//
//  1. No edit may contain a marker — one that does is a bug, discarded rather than
//     redistributed.
//  2. The edge-growth rule: an edit is discarded if it would place code points at an extremity of
//     its span that were not there before. That sentence is the rule; "r > d" alone is an
//     incorrect formalisation of it and misses r == d. dashes P3 admits a run of THREE dashes, so
//     "---" -> U+0020 en-dash U+0020 is 3 -> 3: the length test sees nothing while U+0020 lands
//     on both extremities anyway. The second clause tests the CHARACTER, and only U+0020 needs
//     testing — it is the one code point any rule emits whose meaning comes from its position
//     rather than from itself (modes.md 5 item 2).
//
// Deletion at an edge is NOT restricted here — r > d is always false for a deletion, so this
// filter never sees one; that case is spaces.md 3.2 step 4's own edge-as-NONE clause instead.
func FilterBoundaryEdits(cp []rune, edits []engine.Edit, ranges []SpanRange) []engine.Edit {
	out := make([]engine.Edit, 0, len(edits))
	for _, edit := range edits {
		containsMarker := false
		for i := edit.Start; i < edit.End; i++ {
			if engine.IsMarker(cp[i]) {
				containsMarker = true
				break
			}
		}
		if containsMarker {
			continue
		}

		p := edit.Start
		q := edit.End - 1
		d := edit.End - edit.Start
		r := len(edit.Replacement)
		if span, ok := spanContaining(ranges, p); ok && (p == span.First || q == span.Last) {
			if r > d {
				continue
			}
			if r > 0 && p == span.First && edit.Replacement[0] == space && cp[p] != space {
				continue
			}
			if r > 0 && q == span.Last && edit.Replacement[r-1] == space && cp[q] != space {
				continue
			}
		}

		out = append(out, edit)
	}
	return out
}

// SplitOnMarker redistributes the transformed array back to one piece per span (modes.md 3.5
// step 4).
func SplitOnMarker(cp []rune, expected int) ([][]rune, error) {
	pieces := [][]rune{{}}
	for _, value := range cp {
		if engine.IsMarker(value) {
			pieces = append(pieces, []rune{})
			continue
		}
		pieces[len(pieces)-1] = append(pieces[len(pieces)-1], value)
	}
	if len(pieces) != expected {
		return nil, engine.NewError(engine.CodeRuleContract, fmt.Sprintf(
			"boundary markers did not survive the pipeline: expected %d spans, found %d",
			expected, len(pieces)))
	}
	return pieces, nil
}

// OriginOfSpans returns the origin map for ConcatenateSpans (analyze.md section 2): for every
// code point of the joined array, the code-point offset of the character it came from IN THE
// DOCUMENT, and engine.NoOrigin for the markers, which came from nowhere. A Span's bounds are
// already code-point offsets (a mode adapter converts its parser's byte offsets away before
// constructing one), so no coordinate conversion belongs here.
func OriginOfSpans(spans []Span) []int {
	origin := make([]int, 0)
	for i, span := range spans {
		if i > 0 {
			origin = append(origin, engine.NoOrigin)
		}
		for offset := span.Start; offset < span.End; offset++ {
			origin = append(origin, offset)
		}
	}
	return origin
}
