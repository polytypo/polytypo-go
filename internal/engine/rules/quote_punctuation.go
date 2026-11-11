package rules

import (
	engine "github.com/polytypo/polytypo-go/internal/engine"
	spec "github.com/polytypo/polytypo-go/internal/spec"
)

// `quote-punctuation` -- spec/rules/quote-punctuation.md, order 45. Explicit opt-in: off by default
// (order.json's "default" field is what enforces that). Reads quotes.punctuationPlacement and the
// primary close glyph; "none" makes it a no-op.
const (
	qpComma    = rune(0x2C)
	qpFullStop = rune(0x2E)
)

func init() {
	engine.RegisterRule("quote-punctuation", scanQuotePunctuation)
}

// qpAt returns cp[i], or engine.None if i is out of bounds -- the spec's own boundary value.
func qpAt(cp []rune, i int) rune {
	if i < 0 || i >= len(cp) {
		return engine.None
	}
	return cp[i]
}

// scanQuotePunctuation is quote-punctuation.md 3.2: C1-C4 in one left-to-right scan; after an edit
// continue from i+2.
func scanQuotePunctuation(cp []rune, locale spec.LocaleData, _ engine.RuleContext) []engine.Edit {
	if locale.Quotes.PunctuationPlacement != "inside" {
		return nil
	}
	closeRunes := []rune(locale.Quotes.Primary.Close)
	if len(closeRunes) == 0 {
		return nil
	}
	closeCP := closeRunes[0]

	var edits []engine.Edit
	n := len(cp)
	for i := 0; i+1 < n; {
		next := cp[i+1]
		after := qpAt(cp, i+2)
		if cp[i] == closeCP &&
			(next == qpComma || next == qpFullStop) &&
			i >= 1 && engine.IsLetter(cp[i-1]) &&
			(after == engine.None || qtIsSpacelike(after)) {
			edits = append(edits, engine.Edit{
				Start:       i,
				End:         i + 2,
				Replacement: []rune{next, closeCP},
				RuleID:      "quote-punctuation",
			})
			i += 2
			continue
		}
		i++
	}
	return edits
}
