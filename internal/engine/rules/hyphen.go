package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/hyphen.md (spec 0.1.0), order 35.
//
// Replaces U+002D with U+2011 inside a closed list of locale-listed morphological forms
// (hyphen.prefixes/suffixes/compounds). Explicit index-based scanning over the code-point array
// only: no regex, no native-string indexing (docs/ARCHITECTURE.md section 4.1, 4.2). All
// package-level identifiers here are prefixed hyphen* so they cannot collide with sibling rule
// files in this same package.

const (
	hyphenHY   = rune(0x2D)   // U+002D hyphen-minus
	hyphenNBHY = rune(0x2011) // U+2011 non-breaking hyphen
)

func init() {
	engine.RegisterRule("hyphen", scanHyphen)
}

// hyphenPatternKind is 3.4's tie-break order when two entries match at the same index with the
// same length: compounds, then prefixes, then suffixes.
type hyphenPatternKind int

const (
	hyphenCompound hyphenPatternKind = iota
	hyphenPrefix
	hyphenSuffix
)

type hyphenPattern struct {
	cps  []rune
	kind hyphenPatternKind
}

// hyphenAt returns cp[i], or engine.None if i is out of bounds -- the spec's own NONE boundary
// value (3.1).
func hyphenAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

// hyphenIsHyphenish is 3.1 HYPHENISH. U+2010, U+00AD, U+2012, U+2013, U+2014 are deliberately
// absent.
func hyphenIsHyphenish(cp rune) bool {
	return cp == hyphenHY || cp == hyphenNBHY
}

func hyphenIsDigit(cp rune) bool {
	return cp >= '0' && cp <= '9'
}

// hyphenIsWordish is 3.1 WORDISH = ALNUM ∪ HYPHENISH. U+2011 is a member on purpose: without it,
// converting a hyphen would flip a neighbouring form's boundary verdict between pipeline runs
// (hyphen.md 5).
func hyphenIsWordish(cp rune) bool {
	return engine.IsLetter(cp) || hyphenIsDigit(cp) || hyphenIsHyphenish(cp)
}

// hyphenPreparePatterns converts one locale word list into match patterns of the given kind.
//
// hyphen.md 2 requires every entry to contain at least one U+002D and specifies the error code
// POLYTYPO_MALFORMED_LOCALE_DATA for one that doesn't. RuleFunc's signature has no error result
// (it returns only []Edit), so there is no channel to return that error through from inside a
// rule; this panics with *engine.Error instead, the same shape the JS reference implementation
// throws and the Python port raises. This is a judgment call: nothing in this module currently
// recovers a rule panic into an ordinary error, so today this would surface as a Go panic rather
// than a returned error until/unless the public entry point adds a recover for *engine.Error.
// Locale data is vendored and embedded (never user-supplied at runtime), so this only fires if
// the vendored spec copy itself is malformed -- a build-time invariant violation, not routine
// input handling.
func hyphenPreparePatterns(entries []string, field string, kind hyphenPatternKind, out []hyphenPattern) []hyphenPattern {
	for _, entry := range entries {
		cps := []rune(entry)
		hasHyphen := false
		for _, c := range cps {
			if c == hyphenHY {
				hasHyphen = true
				break
			}
		}
		if !hasHyphen {
			panic(&engine.Error{
				Code: engine.CodeMalformedLocaleData,
				Message: "hyphen." + field + ` entry "` + entry +
					`" contains no U+002D; there is nothing for the rule to convert (spec/rules/hyphen.md section 2)`,
			})
		}
		out = append(out, hyphenPattern{cps: cps, kind: kind})
	}
	return out
}

// hyphenMatchesAt is 3.3 -- hyphen-lenient, first-character-lenient literal matching. The hyphen
// leniency covers j = 0 too, because a suffix entry begins with its own hyphen and must keep
// matching its own output once that hyphen has already been converted to U+2011 (hyphen.md 3.3,
// 5 -- the idempotency argument depends on this). The first-character case leniency uses the
// Unicode simple uppercase mapping of the *pattern*, never a locale-dependent case fold of the
// input (hyphen.md 3.3, ARCHITECTURE.md section 4.4).
func hyphenMatchesAt(cp []rune, a int, w []rune) bool {
	if a+len(w) > len(cp) {
		return false
	}
	for j, p := range w {
		c := cp[a+j]
		if p == hyphenHY {
			if !hyphenIsHyphenish(c) {
				return false
			}
			continue
		}
		if c == p {
			continue
		}
		if j == 0 && engine.IsLetter(p) && c == engine.SimpleUppercase(p) {
			continue
		}
		return false
	}
	return true
}

// hyphenSelect picks 3.4's one candidate at index a: the longest entry that matches, ties broken
// compounds, prefixes, suffixes. patterns is built compounds-first, prefixes-second,
// suffixes-third, so keeping the first-seen entry on a length tie already yields that order.
func hyphenSelect(cp []rune, a int, patterns []hyphenPattern) (hyphenPattern, bool) {
	var best hyphenPattern
	found := false
	for _, p := range patterns {
		if !hyphenMatchesAt(cp, a, p.cps) {
			continue
		}
		if !found || len(p.cps) > len(best.cps) {
			best = p
			found = true
		}
	}
	return best, found
}

// hyphenGuardCompound is 3.4 C -- a compound must be a whole word.
func hyphenGuardCompound(cp []rune, a, k int) bool {
	before := hyphenAt(cp, a-1)
	if before != engine.None && hyphenIsWordish(before) {
		return false
	}
	after := hyphenAt(cp, a+k)
	return after == engine.None || !hyphenIsWordish(after)
}

// hyphenGuardPrefix is 3.4 P -- a prefix starts a word and must actually prefix something.
func hyphenGuardPrefix(cp []rune, a, k int) bool {
	before := hyphenAt(cp, a-1)
	if before != engine.None && hyphenIsWordish(before) {
		return false
	}
	return engine.IsLetter(hyphenAt(cp, a+k))
}

// hyphenGuardSuffix is 3.4 S -- a suffix must actually suffix something and must end the word.
func hyphenGuardSuffix(cp []rune, a, k int) bool {
	if !engine.IsLetter(hyphenAt(cp, a-1)) {
		return false
	}
	after := hyphenAt(cp, a+k)
	return after == engine.None || !hyphenIsWordish(after)
}

func hyphenBind(index int) engine.Edit {
	return engine.Edit{Start: index, End: index + 1, Replacement: []rune{hyphenNBHY}, RuleID: "hyphen"}
}

func scanHyphen(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	h := locale.Hyphen
	// hyphen.md 2: with all three lists empty the rule emits nothing for any input -- the
	// common case for every v1 locale except ru.
	if len(h.Compounds) == 0 && len(h.Prefixes) == 0 && len(h.Suffixes) == 0 {
		return nil
	}

	var patterns []hyphenPattern
	patterns = hyphenPreparePatterns(h.Compounds, "compounds", hyphenCompound, patterns)
	patterns = hyphenPreparePatterns(h.Prefixes, "prefixes", hyphenPrefix, patterns)
	patterns = hyphenPreparePatterns(h.Suffixes, "suffixes", hyphenSuffix, patterns)

	n := len(cp)
	var edits []engine.Edit
	a := 0
	for a < n {
		selected, ok := hyphenSelect(cp, a, patterns)
		if !ok {
			a++
			continue
		}

		// 3.4: matching and guarding are separate steps, and there is no backtracking. A guard
		// failure ends the position; no shorter entry is tried at a.
		w := selected.cps
		k := len(w)
		switch selected.kind {
		case hyphenCompound:
			if !hyphenGuardCompound(cp, a, k) {
				a++
				continue
			}
			for j, p := range w {
				if p == hyphenHY && cp[a+j] != hyphenNBHY {
					edits = append(edits, hyphenBind(a+j))
				}
			}
		case hyphenPrefix:
			if !hyphenGuardPrefix(cp, a, k) {
				a++
				continue
			}
			// The entry's own last code point is its hyphen (hyphen.md 2).
			if w[k-1] == hyphenHY && cp[a+k-1] != hyphenNBHY {
				edits = append(edits, hyphenBind(a+k-1))
			}
		default: // hyphenSuffix
			if !hyphenGuardSuffix(cp, a, k) {
				a++
				continue
			}
			// A suffix is matched at its own hyphen, which is its first code point.
			if w[0] == hyphenHY && cp[a] != hyphenNBHY {
				edits = append(edits, hyphenBind(a))
			}
		}
		a += k
	}
	return edits
}
