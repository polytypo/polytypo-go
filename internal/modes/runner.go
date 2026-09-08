package modes

import (
	"github.com/polytypo/polytypo-go/internal/engine"
	"github.com/polytypo/polytypo-go/internal/spec"
)

// runRulesOverSpans is the same sequence as engine.RunRules, with the two boundary filters of
// modes.md 3.4 interposed. Span extents are recomputed after every rule, because applying an
// edit shifts every index after it; the markers themselves always survive, since no edit may
// contain one.
func runRulesOverSpans(cp []rune, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) ([]rune, error) {
	current := cp
	for _, ruleID := range plan {
		fn := engine.GetRule(ruleID)
		edits := fn(current, localeData, ctx)
		filtered := FilterBoundaryEdits(current, edits, SpanRangesOf(current))
		if len(filtered) == 0 {
			continue
		}
		next, err := engine.ApplyEdits(current, filtered, ruleID)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

// RunOverSpans runs the pipeline once, over the marker-separated concatenation of every
// processable span — not per span (would pair quotation marks in isolation), and not over a
// naive concatenation (would manufacture adjacencies the document does not have). The output is
// the input with a set of disjoint substring replacements applied and nothing else (modes.md 4):
// a span whose content the rules did not change contributes no replacement, so a document
// needing no changes comes back byte-identical.
//
// sourceCP is the whole input already converted to code points; spans address it by code-point
// index (never a byte offset — a mode-specific adapter must convert away from its parser's native
// offsets before constructing a Span).
func RunOverSpans(sourceCP []rune, spans []Span, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) (string, error) {
	normalized, err := NormalizeSpans(spans)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return string(sourceCP), nil
	}

	concatenated := ConcatenateSpans(sourceCP, normalized)
	transformed, err := runRulesOverSpans(concatenated, plan, localeData, ctx)
	if err != nil {
		return "", err
	}
	pieces, err := SplitOnMarker(transformed, len(normalized))
	if err != nil {
		return "", err
	}

	out := make([]rune, 0, len(sourceCP))
	cursor := 0
	for i, span := range normalized {
		piece := pieces[i]
		original := sourceCP[span.Start:span.End]
		out = append(out, sourceCP[cursor:span.Start]...)
		if string(piece) == string(original) {
			out = append(out, original...)
		} else {
			out = append(out, piece...)
		}
		cursor = span.End
	}
	out = append(out, sourceCP[cursor:]...)
	return string(out), nil
}
