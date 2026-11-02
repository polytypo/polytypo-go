package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
)

// `dash.parenthetical = "em-paired"` -- spec/rules/dashes.md 3.4a. The pair is written with a
// space outside it and none inside (`Llegó —por fin— a casa`), which no per-token rule can
// express: which stroke opens and which closes is a property of the sentence, not of the token.
// A segment is converted only when it holds exactly two counted strokes and both are eligible;
// anything else is left as typed.

type pairedRun struct {
	s, e        int
	lsp, rsp    int
	left, right int
	eligible    bool
}

func dshIsTerminal(cp rune) bool {
	return cp == dshFullStop || cp == dshQuestion || cp == dshExclamation || cp == dshEllipsisChar
}

// dshEndsSegment: a terminal ends a segment only before a space (U+0020 or a no-break space,
// which `nbsp` puts there later), a line break, a marker or the end of input.
func dshEndsSegment(cp []rune, i int) bool {
	if !dshIsTerminal(cp[i]) {
		return false
	}
	if i+1 >= len(cp) {
		return true
	}
	next := cp[i+1]
	return next == dshSpace || dshIsNoBreakSpace(next) || dshIsBreak(next) || engine.IsMarker(next)
}

// classifyPairedRun returns ok=false for a run that is not a stroke at all and is not counted.
func classifyPairedRun(cp []rune, s, e int) (pairedRun, bool) {
	n := len(cp)
	k := e - s
	if k > 3 {
		return pairedRun{}, false
	}

	lsp, rsp := 0, 0
	if s > 0 && cp[s-1] == dshSpace {
		lsp = 1
	}
	if e < n && cp[e] == dshSpace {
		rsp = 1
	}
	first := cp[s]
	isHyphenShaped := first == dshHyphenMinus || first == dshHyphen || first == dshMinusSign
	// A hyphen inside a word is a compound, not a stroke.
	if k == 1 && isHyphenShaped && lsp == 0 && rsp == 0 {
		return pairedRun{}, false
	}
	// The hyphen of `+/-`, which `symbols` (order 80) turns into `±`.
	if k == 1 && first == dshHyphenMinus && s >= 2 && cp[s-1] == '/' && cp[s-2] == '+' {
		return pairedRun{}, false
	}

	counted := func(eligible bool, left, right int) (pairedRun, bool) {
		return pairedRun{s: s, e: e, lsp: lsp, rsp: rsp, left: left, right: right, eligible: eligible}, true
	}

	left := s - 1 - lsp
	right := e + rsp
	if left < 0 || right >= n {
		return counted(false, left, right)
	}

	joinStart := left + 1
	joinEnd := right
	for left >= 0 && cp[left] == dshWordJoiner {
		left--
	}
	for right < n && cp[right] == dshWordJoiner {
		right++
	}
	if left < 0 || right >= n {
		return counted(false, left, right)
	}
	crossedJoiner := left+1 != joinStart || right != joinEnd

	if _, isCandidate := rngFlanks(cp, left, right); isCandidate {
		return pairedRun{}, false
	}
	if crossedJoiner {
		return counted(false, left, right)
	}

	leftCp, rightCp := cp[left], cp[right]
	if dshIsBreak(leftCp) || dshIsBreak(rightCp) {
		return counted(false, left, right)
	}
	if dshIsInertDash(leftCp) || dshIsInertDash(rightCp) {
		return counted(false, left, right)
	}
	if dshIsDash(leftCp) || dshIsDash(rightCp) {
		return counted(false, left, right)
	}
	if leftCp == dshSpace || dshIsNoBreakSpace(leftCp) || rightCp == dshSpace || dshIsNoBreakSpace(rightCp) {
		return counted(false, left, right)
	}
	if dshIsClusterInert(cp, s, e) {
		return counted(false, left, right)
	}

	// A stroke spaced on one side only is eligible solely as the one em dash that is already
	// half of the target form.
	if lsp != rsp && !(k == 1 && first == dshEmDash) {
		return counted(false, left, right)
	}
	// P4: a tight stroke between two Roman numerals is a range already in its correct form.
	if lsp == 0 && rsp == 0 && dashesIsRomanFlanked(cp, left, right) {
		return counted(false, left, right)
	}
	// P5: an authored lone en dash is declined in every locale.
	if k == 1 && first == dshEnDash {
		return counted(false, left, right)
	}
	return counted(true, left, right)
}

func dshWantsSpace(cp rune) bool {
	return engine.IsLetter(cp) || dshIsDigit(cp)
}

func scanPairedDashes(cp []rune) []engine.Edit {
	n := len(cp)
	var edits []engine.Edit
	var segment []pairedRun

	flush := func() {
		if len(segment) == 2 {
			edits = emitPairedDashes(cp, segment[0], segment[1], edits)
		}
		segment = segment[:0]
	}

	i := 0
	for i < n {
		if dshIsDash(cp[i]) {
			s := i
			e := s
			for e < n && dshIsDash(cp[e]) {
				e++
			}
			i = e
			if run, ok := classifyPairedRun(cp, s, e); ok {
				segment = append(segment, run)
			}
			continue
		}
		if dshEndsSegment(cp, i) {
			flush()
		}
		i++
	}
	flush()

	return edits
}

func emitPairedDashes(cp []rune, a, b pairedRun, edits []engine.Edit) []engine.Edit {
	if !a.eligible || !b.eligible {
		return edits
	}
	if a.lsp == 0 && a.rsp == 1 { // the first stroke is shaped like a closer
		return edits
	}
	if b.lsp == 1 && b.rsp == 0 { // the second stroke is shaped like an opener
		return edits
	}
	for j := a.e; j < b.s; j++ {
		if dshIsBreak(cp[j]) {
			return edits
		}
	}

	insertBefore := a.lsp == 0 && dshWantsSpace(cp[a.left])
	insertAfter := b.rsp == 0 && dshWantsSpace(cp[b.right])
	if insertBefore && dshIsSpacingTransitionBlockedLeft(cp, a.left) {
		return edits
	}
	if insertAfter && dshIsSpacingTransitionBlockedRight(cp, b.right) {
		return edits
	}

	opener := []rune{dshEmDash}
	if a.lsp == 1 || insertBefore {
		opener = []rune{dshSpace, dshEmDash}
	}
	closer := []rune{dshEmDash}
	if b.rsp == 1 || insertAfter {
		closer = []rune{dshEmDash, dshSpace}
	}

	for _, p := range []struct {
		run  pairedRun
		repl []rune
	}{{a, opener}, {b, closer}} {
		start := p.run.s - p.run.lsp
		end := p.run.e + p.run.rsp
		if dshSameContent(cp, start, end, p.repl) {
			continue
		}
		edits = append(edits, engine.Edit{Start: start, End: end, Replacement: p.repl, RuleID: "dashes"})
	}
	return edits
}
