package modes

import (
	"sort"

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

	replacements := make([]replacement, 0, len(normalized))
	for i, span := range normalized {
		replacements = append(replacements, replacement{span: span, piece: pieces[i]})
	}
	return emit(sourceCP, replacements), nil
}

// replacement pairs a span with what the pipeline made of it.
type replacement struct {
	span  Span
	piece []rune
}

// emit is modes.md 4: the source with disjoint replacements applied at recorded offsets, and
// nothing else changed. Shared by RunOverSpans and RunOverUnits, because step 5 of 3.5 runs once
// per document however many units step 3 ran over.
func emit(sourceCP []rune, replacements []replacement) string {
	out := make([]rune, 0, len(sourceCP))
	cursor := 0
	for _, r := range replacements {
		original := sourceCP[r.span.Start:r.span.End]
		out = append(out, sourceCP[cursor:r.span.Start]...)
		if string(r.piece) == string(original) {
			out = append(out, original...)
		} else {
			out = append(out, r.piece...)
		}
		cursor = r.span.End
	}
	out = append(out, sourceCP[cursor:]...)
	return string(out)
}

// RunOverUnits is modes.md 3.1 and 3.5 step 3 (spec 1.7.0). A document has one text unit, except
// in "markdown" with FrontmatterKeys, where the frontmatter block's spans form a unit of their
// own. The pipeline runs once per unit and the two edit sets are disjoint, because no span of one
// unit lies inside the other — which is what the body's walk skipping the block guarantees. Only
// step 5 is shared: the source is emitted once, in document order.
func RunOverUnits(sourceCP []rune, units [][]Span, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) (string, error) {
	var replacements []replacement
	for _, spans := range units {
		normalized, err := NormalizeSpans(spans)
		if err != nil {
			return "", err
		}
		if len(normalized) == 0 {
			continue
		}
		transformed, err := runRulesOverSpans(ConcatenateSpans(sourceCP, normalized), plan, localeData, ctx)
		if err != nil {
			return "", err
		}
		pieces, err := SplitOnMarker(transformed, len(normalized))
		if err != nil {
			return "", err
		}
		for i, span := range normalized {
			replacements = append(replacements, replacement{span: span, piece: pieces[i]})
		}
	}
	sort.SliceStable(replacements, func(i, j int) bool {
		return replacements[i].span.Start < replacements[j].span.Start
	})
	return emit(sourceCP, replacements), nil
}

// AnalyzeOverUnits is AnalyzeOverSpans per text unit (modes.md 3.1), reported in document order.
func AnalyzeOverUnits(sourceCP []rune, units [][]Span, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) ([]engine.Change, error) {
	changes := []engine.Change{}
	for _, spans := range units {
		unitChanges, err := AnalyzeOverSpans(sourceCP, spans, plan, localeData, ctx)
		if err != nil {
			return nil, err
		}
		changes = append(changes, unitChanges...)
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Start < changes[j].Start })
	return changes, nil
}

// AnalyzeOverSpans is RunOverSpans, reporting instead of applying (analyze.md section 1). The
// span table supplies the origin map, so every change comes back in DOCUMENT coordinates —
// analyze.md section 6 names a runtime that reports span-local offsets here as the mistake that
// passes every text-mode test.
func AnalyzeOverSpans(sourceCP []rune, spans []Span, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) ([]engine.Change, error) {
	normalized, err := NormalizeSpans(spans)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return []engine.Change{}, nil
	}
	return engine.RunRulesRecording(
		ConcatenateSpans(sourceCP, normalized),
		plan,
		localeData,
		ctx,
		OriginOfSpans(normalized),
		len(sourceCP),
		func(current []rune, edits []engine.Edit) []engine.Edit {
			return FilterBoundaryEdits(current, edits, SpanRangesOf(current))
		},
	)
}
