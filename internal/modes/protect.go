package modes

import (
	"github.com/polytypo/polytypo-go/internal/engine"
	"github.com/polytypo/polytypo-go/internal/spec"
)

// protectedLayout is modes.md 3.9.1: [S0] m1 S1 m2 ... mk [Sk], one marker per maximal protected
// run, edges included. Unlike the mode adapters a run at the very start or end of the input still
// yields a marker, and an input protected end to end is the one-element array [m]. origin is the
// analyze map for the same array.
func protectedLayout(cp []rune, holes []engine.Range) (array []rune, origin []int) {
	array = make([]rune, 0, len(cp))
	origin = make([]int, 0, len(cp))
	cursor := 0
	for _, hole := range holes {
		for i := cursor; i < hole.Start; i++ {
			array = append(array, cp[i])
			origin = append(origin, i)
		}
		if gapIsLineBoundary(cp, hole.Start, hole.End) {
			array = append(array, engine.LineMarker)
		} else {
			array = append(array, engine.Marker)
		}
		origin = append(origin, engine.NoOrigin)
		cursor = hole.End
	}
	for i := cursor; i < len(cp); i++ {
		array = append(array, cp[i])
		origin = append(origin, i)
	}
	return array, origin
}

// RunProtectedMapped runs the pipeline once over the protected layout and re-emits each protected
// run byte for byte at its marker. It also returns the runs located in the output, which is
// protect' of modes.md 3.9.5; Transform does not expose it, tests use it.
func RunProtectedMapped(cp []rune, holes []engine.Range, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) ([]rune, []engine.Range, error) {
	array, _ := protectedLayout(cp, holes)
	transformed, err := runRulesOverSpans(array, plan, localeData, ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	out := make([]rune, 0, len(transformed))
	moved := make([]engine.Range, 0, len(holes))
	for _, value := range transformed {
		if !engine.IsMarker(value) {
			out = append(out, value)
			continue
		}
		if len(moved) == len(holes) {
			break
		}
		hole := holes[len(moved)]
		start := len(out)
		out = append(out, cp[hole.Start:hole.End]...)
		moved = append(moved, engine.Range{Start: start, End: len(out)})
	}
	if len(moved) != len(holes) {
		return nil, nil, engine.NewError(engine.CodeRuleContract, "boundary markers did not survive the pipeline")
	}
	return out, moved, nil
}

// RunProtected is RunProtectedMapped without the moved runs.
func RunProtected(cp []rune, holes []engine.Range, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) (string, error) {
	out, _, err := RunProtectedMapped(cp, holes, plan, localeData, ctx)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// AnalyzeProtected is RunProtected reporting instead of applying (modes.md 3.9.6); offsets are
// into the input as passed.
func AnalyzeProtected(cp []rune, holes []engine.Range, plan []string, localeData spec.LocaleData, ctx engine.RuleContext) ([]engine.Change, error) {
	array, origin := protectedLayout(cp, holes)
	return engine.RunRulesRecording(array, plan, localeData, ctx, origin, len(cp),
		func(current []rune, edits []engine.Edit) []engine.Edit {
			return FilterBoundaryEdits(current, edits, SpanRangesOf(current), false, nil)
		})
}
