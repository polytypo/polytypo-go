package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/ellipsis.md 3.1.
const (
	elDot      = rune(0x2E)
	elEll      = rune(0x2026)
	elExclaim  = rune(0x21)
	elQuestion = rune(0x3F)
)

func init() {
	engine.RegisterRule("ellipsis", scanEllipsis)
}

// ellipsisAt returns cp[i], or engine.None if i is out of bounds — the spec's own boundary
// value.
func ellipsisAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

func isEllipsisDotlike(cp rune) bool {
	return cp == elDot || cp == elEll
}

func isTerminal(cp rune) bool {
	return cp == elExclaim || cp == elQuestion
}

// isSameRun reports whether cp[s:e] is already exactly target, so a would-be no-op edit is
// never emitted.
func isSameRun(cp []rune, s, e int, target []rune) bool {
	if e-s != len(target) {
		return false
	}
	for j, want := range target {
		if ellipsisAt(cp, s+j) != want {
			return false
		}
	}
	return true
}

func scanEllipsis(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	n := len(cp)
	abbreviated := locale.Ellipsis.AbbreviatedAfterTerminal
	var edits []engine.Edit
	i := 0

	for i < n {
		if !isEllipsisDotlike(ellipsisAt(cp, i)) {
			i++
			continue
		}

		s := i
		e := s
		q := 0
		for e < n && isEllipsisDotlike(ellipsisAt(cp, e)) {
			if ellipsisAt(cp, e) == elEll {
				q++
			}
			e++
		}
		k := e - s
		left := ellipsisAt(cp, s-1)

		// One decision per run, one edit per run (ellipsis.md 3.3 step 6).
		target := ellipsisRunTarget(k, q, left, abbreviated)
		if target != nil && !isSameRun(cp, s, e, target) {
			edits = append(edits, engine.Edit{
				Start:       s,
				End:         e,
				Replacement: target,
				RuleID:      "ellipsis",
			})
		}
		i = e
	}

	return edits
}

// ellipsisRunTarget is ellipsis.md 3.3 steps 4-6. nil means "emit nothing"; otherwise the final
// form of the whole run.
func ellipsisRunTarget(k, q int, left rune, abbreviated bool) []rune {
	if k == 2 && q == 0 {
		// The two-dot run is unconditionally inert where "?.." is the correct output form;
		// that is what stops "?.." <-> "?…" from oscillating (ellipsis.md 5).
		if abbreviated {
			return nil
		}
		if isTerminal(left) {
			return []rune{elEll}
		}
		return nil
	}
	if k == 1 && q == 0 {
		return nil
	}
	// k = 1 with an existing U+2026, or any run of 2+ containing one: normalise, then decide
	// the abbreviated form on the same span.
	if abbreviated && isTerminal(left) {
		return []rune{elDot, elDot}
	}
	return []rune{elEll}
}
