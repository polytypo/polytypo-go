package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// The two decline-only predicates quotes.go reads — spec/rules/quotes.md 3.2 ("Listed elision
// veto" and "Universal medial-`n` elision veto"). Both have the same outcome: the marks survive
// pass 2 unmatched and apostrophe converts each by its own case ladder (apostrophe.md 3.3 cases
// 4 then 3), giving `rock 'n' roll` → `rock ’n’ roll`. Mirrors ref-js's
// src/rules/quote-ambiguity.ts and ref-python's _quote_ambiguity.py.
//
// The universal shape (quotes.md 3.2, spec 1.1.0): a pair of NARROW marks enclosing exactly one
// code point, U+006E `n` or U+004E `N`, with at least one INLINE-SPACE code point immediately
// outside each mark. Only the single adjacent code point is tested on each side; a longer run of
// inline spaces further out does not invalidate the match (quotes.md 3.2's "at least one,
// deliberately not exactly one").
//
// Only quotes.go consumes this file. Spec 0.5.0 had apostrophe.go consume it too, through a
// preserve set withdrawn in 1.1.0 (apostrophe.md 3.4) — conversion is now the specified outcome
// for every position either predicate vetoes.

// qaLowerN and qaUpperN are quotes.md 3.2's one enclosed code point, in either case.
const (
	qaLowerN = rune(0x6E)
	qaUpperN = rune(0x4E)
)

// qaNarrow is quotes.md 3.1 NARROW — every glyph an elision mark may appear as across pipeline
// passes (straight, or already curled by an earlier pass). Shared with quotes.go so the two rules
// cannot define two slightly different NARROW sets. For the universal medial-n veto, matching the
// whole class is an IDEMPOTENCY obligation rather than a preference: its marks are converted to
// U+2019 by apostrophe, so a straight-ASCII-only predicate would not recognise its own output and
// pass 2 would pair `rock ’n’ roll` as an ordinary NARROW quotation on the next run — measured as
// `rock «n» roll` in ru and `rock ”n” roll` in fi.
var qaNarrow = map[rune]bool{
	0x27:   true,
	0x2018: true,
	0x2019: true,
	0x201A: true,
	0x201B: true,
	0x2039: true,
	0x203A: true,
}

// qaInlineSpace is quotes.md 3.1 INLINE-SPACE, deliberately excluding BREAK/Marker/LineMarker so
// this shape never crosses a line or span boundary (modes.md 3.3) — the same anchor the existing
// elisionIdioms matcher already uses.
var qaInlineSpace = map[rune]bool{
	0x20:   true,
	0x09:   true,
	0xA0:   true,
	0x202F: true,
	0x2007: true,
	0x2009: true,
	0x200A: true,
}

// qaAt returns cp[i], or engine.None if i is out of bounds — the spec's own boundary value.
func qaAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

func qaIsAlnum(cp rune) bool {
	return (cp >= 0x30 && cp <= 0x39) || engine.IsLetter(cp)
}

// qaAsciiLower folds ASCII A-Z to a-z, ASCII-only — the same convention nbsp's afterShortWords
// and the existing idiom matcher already use (ARCHITECTURE.md 4.4: never a platform locale
// case-fold).
func qaAsciiLower(cp rune) rune {
	if cp >= 0x41 && cp <= 0x5A {
		return cp + 0x20
	}
	return cp
}

// qaElidedMatches compares cp[start : start+len(elided)] against elided exactly, code point for
// code point — no case leniency, ever, on the elided content (quotes.md 3.2). Every comparison
// goes through this function rather than ever assembling the candidate span into a Go string:
// the equivalent Python port crashed with `ValueError: chr() arg not in range` when a span
// straddled a span-boundary marker (a negative sentinel, not a valid code point) and was then
// compared to a literal string. Go's rune equality cannot panic the way Python's chr() does, but
// a marker must still never be treated as if it could equal a real code point — checking
// IsValidCodePoint first keeps that guarantee explicit rather than accidental, and correctly
// makes the match fail across a span boundary (quotes.md 3.2 / apostrophe.md 3.4, rows H1-H3:
// modes.md's MARKER is not INLINE-SPACE, so a context split by an element boundary must not
// match).
func qaElidedMatches(cp []rune, start int, elided []rune) bool {
	for w, want := range elided {
		c := qaAt(cp, start+w)
		if !engine.IsValidCodePoint(c) {
			return false
		}
		if c != want {
			return false
		}
	}
	return true
}

// qaWordEndsAt reports whether the len(word) code points immediately before index `end`
// (exclusive) match word exactly — except the first code point, compared ASCII-case-insensitive
// — and have a legal outer (left) word boundary: NONE or not LETTER/DIGIT (quotes.md 3.2's Word
// definition). The caller is responsible for having already verified the code point at `end`
// itself is a legal right-hand boundary (a single INLINE-SPACE code point).
func qaWordEndsAt(cp []rune, end int, word []rune) bool {
	start := end - len(word)
	if start < 0 {
		return false
	}
	for k, want := range word {
		c := qaAt(cp, start+k)
		if !engine.IsValidCodePoint(c) {
			return false
		}
		if k == 0 {
			if qaAsciiLower(c) != qaAsciiLower(want) {
				return false
			}
		} else if c != want {
			return false
		}
	}
	before := qaAt(cp, start-1)
	return before == engine.None || !qaIsAlnum(before)
}

// qaWordStartsAt is qaWordEndsAt's mirror image: word must start exactly at `start`, first code
// point ASCII-case-insensitive, with a legal outer (right) word boundary immediately after it.
func qaWordStartsAt(cp []rune, start int, word []rune) bool {
	n := len(cp)
	for k, want := range word {
		c := qaAt(cp, start+k)
		if !engine.IsValidCodePoint(c) {
			return false
		}
		if k == 0 {
			if qaAsciiLower(c) != qaAsciiLower(want) {
				return false
			}
		} else if c != want {
			return false
		}
	}
	after := start + len(word)
	if after >= n {
		return true
	}
	return !qaIsAlnum(cp[after])
}

// qaComputeIdiomMatchedIndices is the listed elision veto (quotes.md 3.2, spec 0.4.0), locale
// data quotes.elisionIdioms. Bounded literal scan for `left, NARROW, elided, NARROW, right`
// (`rock 'n' roll`'s {left: "rock", elided: "n", right: "roll"}). Both marks of a match are
// returned. Matches on NARROW quote marks generally (U+0027 and already-curly U+2018/U+2019),
// not only straight ASCII — required for quotes' own idempotency (an idiom must still veto
// pairing on a second pipeline pass, after apostrophe has curled the marks).
func qaComputeIdiomMatchedIndices(cp []rune, idioms []spec.ElisionIdiom) map[int]struct{} {
	vetoed := map[int]struct{}{}
	if len(idioms) == 0 {
		return vetoed
	}

	n := len(cp)
	type compiledIdiom struct {
		left, elided, right []rune
	}
	compiled := make([]compiledIdiom, len(idioms))
	for i, idiom := range idioms {
		compiled[i] = compiledIdiom{
			left:   []rune(idiom.Left),
			elided: []rune(idiom.Elided),
			right:  []rune(idiom.Right),
		}
	}

	for i := 0; i < n; i++ {
		g := cp[i]
		if !qaNarrow[g] {
			continue
		}

		lLit := qaAt(cp, i-1)
		if lLit == engine.None || !qaInlineSpace[lLit] {
			continue
		}

		for _, idiom := range compiled {
			k := len(idiom.elided)
			j := i + 1 + k
			if j >= n {
				continue
			}
			if !qaElidedMatches(cp, i+1, idiom.elided) {
				continue
			}
			if !qaNarrow[cp[j]] {
				continue
			}

			rLit := qaAt(cp, j+1)
			if rLit == engine.None || !qaInlineSpace[rLit] {
				continue
			}

			if !qaWordEndsAt(cp, i-1, idiom.left) {
				continue
			}
			if !qaWordStartsAt(cp, j+2, idiom.right) {
				continue
			}

			vetoed[i] = struct{}{}
			vetoed[j] = struct{}{}
		}
	}

	return vetoed
}

// qaComputeAmbiguousShapeIndices is the universal medial-n elision shape, locale-independent
// (quotes.md 3.2, spec 1.1.0): a pair of NARROW marks enclosing exactly one code point, U+006E or
// U+004E, with at least one INLINE-SPACE code point immediately outside each mark. Both mark
// positions are returned for every match. A superset of qaComputeIdiomMatchedIndices's output for
// every idiom whose elided field is a single n (true of every idiom shipped so far), but computed
// independently rather than assumed, since a future idiom's elided field is not required to be
// that short.
func qaComputeAmbiguousShapeIndices(cp []rune) map[int]struct{} {
	ambiguous := map[int]struct{}{}
	n := len(cp)

	for i := 0; i < n; i++ {
		if !qaNarrow[qaAt(cp, i)] {
			continue
		}

		lLit := qaAt(cp, i-1)
		if lLit == engine.None || !qaInlineSpace[lLit] {
			continue
		}

		enclosed := qaAt(cp, i+1)
		if enclosed != qaLowerN && enclosed != qaUpperN {
			continue
		}

		j := i + 2
		if !qaNarrow[qaAt(cp, j)] {
			continue
		}

		rLit := qaAt(cp, j+1)
		if rLit == engine.None || !qaInlineSpace[rLit] {
			continue
		}

		ambiguous[i] = struct{}{}
		ambiguous[j] = struct{}{}
	}

	return ambiguous
}
