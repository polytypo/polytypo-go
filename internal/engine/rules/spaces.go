// Package rules holds the per-rule scan functions, each registering itself with the engine's
// rule registry via init().
package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/spaces.md 3.1. Explicit code points only: no regex, no character-class shorthand
// (ARCHITECTURE.md 4.1).
const (
	spSpace = rune(0x20)

	spLF  = rune(0x0A)
	spCR  = rune(0x0D)
	spVT  = rune(0x0B)
	spFF  = rune(0x0C)
	spNEL = rune(0x85)
	spLS  = rune(0x2028)
	spPS  = rune(0x2029)

	spComma       = rune(0x2C)
	spFullStop    = rune(0x2E)
	spSemicolon   = rune(0x3B)
	spColon       = rune(0x3A)
	spExclamation = rune(0x21)
	spQuestion    = rune(0x3F)
	spEllipsis    = rune(0x2026)

	spParenOpen   = rune(0x28)
	spParenClose  = rune(0x29)
	spSquareOpen  = rune(0x5B)
	spSquareClose = rune(0x5D)
	spCurlyOpen   = rune(0x7B)
	spCurlyClose  = rune(0x7D)

	spHyphenMinus    = rune(0x2D)
	spCaret          = rune(0x5E)
	spSolidus        = rune(0x2F)
	spReverseSolidus = rune(0x5C)
	spVerticalLine   = rune(0x7C)
	spAsterisk       = rune(0x2A)
	spDigitZero      = rune(0x30)
	spDigitNine      = rune(0x39)
	spLetterDUpper   = rune(0x44)
	spLetterDLower   = rune(0x64)
	spLetterPUpper   = rune(0x50)
	spLetterPLower   = rune(0x70)
	spLetterOUpper   = rune(0x4F)
	spLetterOLower   = rune(0x6F)
)

func init() {
	engine.RegisterRule("spaces", scanSpaces)
}

// spacesAt returns cp[i], or engine.None if i is out of bounds — the spec's own boundary value.
func spacesAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

// isSpacesBreak is BREAK (spaces.md 3.1), including engine.LineMarker: a member of BREAK for
// every rule, everywhere (modes.md 3.2).
func isSpacesBreak(cp rune) bool {
	switch cp {
	case spLF, spCR, spVT, spFF, spNEL, spLS, spPS, engine.LineMarker:
		return true
	default:
		return false
	}
}

// isStripBefore is STRIP-BEFORE: exactly six code points. U+2026 is deliberately absent — with
// it, "Wait ..." would keep its space here, `ellipsis` would yield "Wait …", and a second
// pipeline pass would then strip that space, a composition divergence (spaces.md 3.4).
func isStripBefore(cp rune) bool {
	switch cp {
	case spComma, spFullStop, spSemicolon, spColon, spExclamation, spQuestion:
		return true
	default:
		return false
	}
}

func isDotlike(cp rune) bool {
	return cp == spFullStop || cp == spEllipsis
}

func isOpenBracket(cp rune) bool {
	switch cp {
	case spParenOpen, spSquareOpen, spCurlyOpen:
		return true
	default:
		return false
	}
}

func isCloseBracket(cp rune) bool {
	switch cp {
	case spParenClose, spSquareClose, spCurlyClose:
		return true
	default:
		return false
	}
}

func matchingCloser(open rune) rune {
	switch open {
	case spParenOpen:
		return spParenClose
	case spSquareOpen:
		return spSquareClose
	default:
		return spCurlyClose
	}
}

// isEmptyBracketGuarded is spaces.md 3.3: "- [ ] item" must not become "- [] item". The run may
// still collapse to length 1.
func isEmptyBracketGuarded(left, right rune) bool {
	return isOpenBracket(left) && right == matchingCloser(left)
}

// isLoneDot is spaces.md 3.4. A run of dots is a different token from a terminal full stop — a
// relative path, a truncation, a typed ellipsis — and deleting the space before it merges the
// run with a preceding abbreviation dot ("See ../docs" -> "See../docs").
//
// e indexes right in the input array, and the run is measured there. Measuring it after any
// edit had been applied would break the Chicago spaced ellipsis "Hello . . .", where every dot
// is a lone dot at decision time and all three spaces must still strip.
//
// The word-start clause (spec 1.2.0): a single dot followed directly by a letter or an ASCII
// digit starts a token — ".NET", ".gitignore", ".5" — so "Use .NET, .NET Core" keeps both
// spaces. The accepted cost is "end .Next sentence", which keeps its stray space. A span boundary
// marker after the dot is neither a letter nor a digit, so the space before it still strips.
func isLoneDot(cp []rune, e int) bool {
	if spacesAt(cp, e) != spFullStop {
		return true
	}
	next := spacesAt(cp, e+1)
	if engine.IsLetter(next) || isDigitASCII(next) {
		return false
	}
	return !isDotlike(next)
}

func isDigitASCII(cp rune) bool {
	return cp >= spDigitZero && cp <= spDigitNine
}

// isEmoticonMouth is spaces.md 3.6: the recognised "mouth" glyphs of a Western text emoticon.
func isEmoticonMouth(cp rune) bool {
	switch cp {
	case spParenOpen, spParenClose, spSquareOpen, spSquareClose,
		spLetterDUpper, spLetterDLower, spLetterPUpper, spLetterPLower,
		spLetterOUpper, spLetterOLower, spSolidus, spReverseSolidus,
		spVerticalLine, spAsterisk:
		return true
	default:
		return false
	}
}

// isEmoticonEyeSideFires is spaces.md 3.6, the emoticon guard's eye side. A colon or semicolon
// immediately followed by an optional "nose" and a recognised "mouth" is the eye of a Western
// text emoticon (":-)", ":)", ";-)"), not sentence punctuation, and the space in front of it
// must survive.
//
// The mouth must not itself run into a letter or an ASCII digit — ":Deal" is a colon before a
// capitalised word, not a face — which is the one check needed to keep this from firing on
// ordinary prose. e indexes right, exactly as isLoneDot does.
func isEmoticonEyeSideFires(cp []rune, e int) bool {
	eye := spacesAt(cp, e)
	if eye != spColon && eye != spSemicolon {
		return false
	}

	i := e + 1
	nose := spacesAt(cp, i)
	if nose == spHyphenMinus || nose == spCaret {
		i++
	}

	if !isEmoticonMouth(spacesAt(cp, i)) {
		return false
	}
	i++

	after := spacesAt(cp, i)
	return !engine.IsLetter(after) && !isDigitASCII(after)
}

// isEmoticonMouthSideFires is spaces.md 3.6, the mouth side. "(" and "[" are EMOTICON-MOUTH
// members and OPEN-BRACKET members at once, so without this the opening-bracket clause deleted
// the space after an emoticon the eye side had just recognised: "a :( b" became "a :(b", and
// then "a:(b" on a second pass, because a letter after the mouth stops the eye side firing.
//
// The walk mirrors the eye side's, backwards from s, and needs no trailing check: the code point
// after the mouth is the space run itself, which is neither a letter nor a digit. A nose with no
// eye behind it is not a face — EMOTICON-NOSE and EMOTICON-EYE are disjoint, so the walk cannot
// mistake one for the other.
func isEmoticonMouthSideFires(cp []rune, s int) bool {
	if !isEmoticonMouth(spacesAt(cp, s-1)) {
		return false
	}

	j := s - 2
	nose := spacesAt(cp, j)
	if nose == spHyphenMinus || nose == spCaret {
		j--
	}

	eye := spacesAt(cp, j)
	return eye == spColon || eye == spSemicolon
}

// spacesReplacementLength is spaces.md 3.2 step 5: the replacement length is a pure function of
// the two bounding code points, computed once. A two-pass "collapse then strip" formulation
// would need a fixed-point loop, which two runtimes would iterate differently.
func spacesReplacementLength(cp []rune, s, e int, left, right rune) int {
	// The guard is a clause of this decision, not a "skip the run" branch: "(  )" collapses to
	// "( )" (spaces.md 3.3, normative reading).
	if isEmptyBracketGuarded(left, right) {
		return 1
	}
	if isOpenBracket(left) && !isEmoticonMouthSideFires(cp, s) {
		return 0
	}
	if isCloseBracket(right) {
		return 0
	}
	if isStripBefore(right) && isLoneDot(cp, e) && !isEmoticonEyeSideFires(cp, e) {
		return 0
	}
	return 1
}

func scanSpaces(cp []rune, locale spec.LocaleData, ctx engine.RuleContext) []engine.Edit {
	n := len(cp)
	var edits []engine.Edit
	i := 0

	for i < n {
		if spacesAt(cp, i) != spSpace {
			i++
			continue
		}

		s := i
		e := s
		for e < n && spacesAt(cp, e) == spSpace {
			e++
		}
		k := e - s

		left := spacesAt(cp, s-1)
		right := spacesAt(cp, e)

		// Boundary guard (3.2 step 4): indentation, Markdown hard breaks and text-unit edges
		// are structural. A span boundary marker counts as NONE here — the one place in the
		// whole spec where a marker is not opaque content, per modes.md 3.3.
		if left == engine.None || engine.IsMarker(left) || isSpacesBreak(left) ||
			right == engine.None || engine.IsMarker(right) || isSpacesBreak(right) {
			i = e
			continue
		}

		length := spacesReplacementLength(cp, s, e, left, right)
		if length != k {
			replacement := []rune{}
			if length == 1 {
				replacement = []rune{spSpace}
			}
			edits = append(edits, engine.Edit{
				Start:       s,
				End:         e,
				Replacement: replacement,
				RuleID:      "spaces",
			})
		}
		i = e
	}

	return edits
}
