package modes

// spec/rules/modes.md 3.8. "yaml" differs from the other two document modes twice over.
//
// It uses NO PARSER (3.8.1): two of the five ecosystems' YAML libraries cannot report the source
// offsets the round-trip guarantee needs — this one, gopkg.in/yaml.v3, reports a start and no end
// — so the scan below is specified rather than delegated and is written the same way in every
// runtime.
//
// And the caller names the keys (3.8.2). YAML is a data format with islands of prose in it — the
// inverse of HTML and Markdown — and nothing in its syntax separates "description:" from "run:".
// A keyless draft of this file rewrote "if !" as "if!" inside a workflow's shell script; there is
// no content test that would not, because shell and template expressions are written in words.
//
// It also SKIPS BY DEFAULT — the inverse of 3.6's closed skip list. A construct this scan does not
// recognise with certainty yields no spans, so the worst outcome of a gap in it is prose left
// untypeset, never a changed byte.

const (
	tab      = '\t'
	lf       = '\n'
	cr       = '\r'
	sp       = ' '
	hash     = '#'
	percent  = '%'
	colon    = ':'
	dash     = '-'
	dot      = '.'
	pipe     = '|'
	greater  = '>'
	dquote   = '"'
	squote   = '\''
	backsl   = '\\'
	ampers   = '&'
	star     = '*'
	bang     = '!'
	lbrace   = '{'
	lbracket = '['
)

// yamlLine is a line of the source. End is the index of the line terminator, or of the end of the
// source — never inside a span.
type yamlLine struct {
	start int
	end   int
}

// splitYAMLLines is 3.8.4: a line ends at U+000A, and A U+000D IMMEDIATELY BEFORE IT IS NOT PART
// OF THE LINE — it is a terminator like the U+000A itself, so it lies outside every span and comes
// back untouched. Without that clause a CRLF file behaves differently from the same bytes with LF:
// the block header reads as '|' followed by U+000D and is unrecognised, and a plain scalar carries
// the carriage return inside its span. Five runtimes split lines with five different standard
// library calls, so the treatment has to be stated rather than inherited.
func splitYAMLLines(cp []rune) []yamlLine {
	lines := make([]yamlLine, 0, 16)
	start := 0
	endOf := func(i int) int {
		if i > start && cp[i-1] == cr {
			return i - 1
		}
		return i
	}
	for i, r := range cp {
		if r == lf {
			lines = append(lines, yamlLine{start, endOf(i)})
			start = i + 1
		}
	}
	if start < len(cp) {
		lines = append(lines, yamlLine{start, endOf(len(cp))})
	}
	return lines
}

func yamlFirstNonSpace(cp []rune, line yamlLine) int {
	i := line.start
	for i < line.end && cp[i] == sp {
		i++
	}
	return i
}

func yamlBlank(cp []rune, line yamlLine) bool {
	return yamlFirstNonSpace(cp, line) == line.end
}

// yamlHasContinuation is 3.8.4 step 7: a line of the value run that is not blank. Blank lines
// alone are not one.
func yamlHasContinuation(cp []rune, lines []yamlLine, li, runEnd int) bool {
	for k := li + 1; k < runEnd; k++ {
		if !yamlBlank(cp, lines[k]) {
			return true
		}
	}
	return false
}

func yamlHasCR(cp []rune, line yamlLine) bool {
	for i := line.start; i < line.end; i++ {
		if cp[i] == cr {
			return true
		}
	}
	return false
}

func yamlHasTab(cp []rune, line yamlLine) bool {
	for i := line.start; i < line.end; i++ {
		if cp[i] == tab {
			return true
		}
	}
	return false
}

// yamlDocumentMarker is 3.8.4 step 3. "---" and "..." at the head of a line, bare or introducing a
// node: the trailing-content form is declined too, so "--- key: value" never yields a key of
// "--- key".
func yamlDocumentMarker(cp []rune, from, to int) bool {
	if to-from < 3 {
		return false
	}
	c := cp[from]
	if c != dash && c != dot {
		return false
	}
	if cp[from+1] != c || cp[from+2] != c {
		return false
	}
	return from+3 == to || cp[from+3] == sp
}

// yamlIndicatorColon reports a colon that ends the line or is followed by U+0020 — the only colon
// YAML reads as an indicator.
func yamlIndicatorColon(cp []rune, j, to int) bool {
	if cp[j] != colon {
		return false
	}
	return j+1 == to || cp[j+1] == sp
}

func yamlSequenceDash(cp []rune, j, to int) bool {
	if cp[j] != dash {
		return false
	}
	return j+1 == to || cp[j+1] == sp
}

// yamlValueRunEnd is the value run of 3.8.4 step 7: every following line that is blank or indented
// more than the key line. THOSE LINES ARE NEVER SCANNED AGAIN — without that, a multi-line quoted
// scalar, a multi-line flow collection and a folded plain scalar all leak their continuation lines
// back into the scan as if they were mappings, and a span can end up holding a scalar's own
// closing delimiter.
func yamlValueRunEnd(cp []rune, lines []yamlLine, li, indent int) int {
	k := li + 1
	for k < len(lines) {
		next := lines[k]
		if !yamlBlank(cp, next) && yamlFirstNonSpace(cp, next)-next.start <= indent {
			break
		}
		k++
	}
	return k
}

// YAMLSpans returns the processable spans of a YAML source, as code-point offsets.
func YAMLSpans(cp []rune, keys map[string]struct{}) []Span {
	lines := splitYAMLLines(cp)
	spans := make([]Span, 0, 16)
	for li := 0; li < len(lines); {
		li = scanYAMLLine(cp, lines, li, keys, &spans)
	}
	return dropYAMLCarriageReturnRegions(cp, lines, spans)
}

// dropYAMLCarriageReturnRegions is 3.8.4's carriage-return region: a line containing a U+000D
// yields no spans, and neither does any later line up to the first one that is non-empty, does
// not begin with U+0020 and contains none. It removes spans and never adds one.
func dropYAMLCarriageReturnRegions(cp []rune, lines []yamlLine, spans []Span) []Span {
	declined := make([]bool, len(lines))
	inRegion, any := false, false
	for i, line := range lines {
		if yamlHasCR(cp, line) {
			inRegion = true
		} else if line.end > line.start && cp[line.start] != ' ' {
			inRegion = false
		}
		declined[i] = inRegion
		any = any || inRegion
	}
	if !any {
		return spans
	}
	kept := spans[:0:0]
	li := 0
	for _, span := range spans {
		for li+1 < len(lines) && span.Start >= lines[li+1].start {
			li++
		}
		if !declined[li] {
			kept = append(kept, span)
		}
	}
	return kept
}

// yamlClosingQuote is 3.8.4 step 5: the index of the quote that closes the one at q, or -1. A
// double-quoted scalar a U+005C and the code point after it never close the quote, which decodes
// nothing: no YAML escape ends in anything that would be a closing quote. A quote not closed on
// the line returns -1.
func yamlClosingQuote(cp []rune, line yamlLine, q int) int {
	quote := cp[q]
	for j := q + 1; j < line.end; j++ {
		c := cp[j]
		if quote == dquote && c == backsl {
			j++
			continue
		}
		if c != quote {
			continue
		}
		if quote == squote && j+1 < line.end && cp[j+1] == squote {
			j++
			continue
		}
		return j
	}
	return -1
}

// yamlDocumentRunEnd is 3.8.4 step 3: the next line that begins in column 0 with a document marker.
func yamlDocumentRunEnd(cp []rune, lines []yamlLine, li int) int {
	k := li + 1
	for k < len(lines) {
		if yamlDocumentMarker(cp, lines[k].start, lines[k].end) {
			break
		}
		k++
	}
	return k
}

// yamlOwnLineRunEnd is 3.8.4 step 5's own-line node: everything up to the first later line that is
// non-empty and does not begin with U+0020, because the scan does not track the key above.
func yamlOwnLineRunEnd(cp []rune, lines []yamlLine, li, indent int) int {
	if indent == 0 {
		return yamlDocumentRunEnd(cp, lines, li)
	}
	k := li + 1
	for k < len(lines) {
		next := lines[k]
		if next.end > next.start && cp[next.start] != sp {
			break
		}
		k++
	}
	return k
}

// yamlNodeMarker is 3.8.4 step 4: "-", "?" or ":" followed by U+0020 or the line end.
func yamlNodeMarker(cp []rune, j, to int) bool {
	if cp[j] != dash && cp[j] != '?' && cp[j] != colon {
		return false
	}
	return j+1 == to || cp[j+1] == sp
}

// scanYAMLLine is one step of 3.8.4. It returns the index of the next line to scan.
func scanYAMLLine(cp []rune, lines []yamlLine, li int, keys map[string]struct{}, spans *[]Span) int {
	line := lines[li]
	skipLine := li + 1

	// step 1 — blank. A tab makes indentation undecidable, so the line and whatever it absorbs
	// yield no spans, but steps 2 to 7 still decide what it absorbs.
	start := yamlFirstNonSpace(cp, line)
	if start == line.end {
		return skipLine
	}
	quiet := yamlHasTab(cp, line)
	indent := start - line.start

	// steps 2 and 3 — comment, directive, document marker. A marker with a node after it absorbs
	// the document.
	if cp[start] == hash || cp[start] == percent {
		return skipLine
	}
	if yamlDocumentMarker(cp, start, line.end) {
		p := start + 3
		for p < line.end && cp[p] == sp {
			p++
		}
		if p >= line.end || cp[p] == hash {
			return skipLine
		}
		if indent == 0 {
			return yamlDocumentRunEnd(cp, lines, li)
		}
		return skipLine
	}

	// step 4 — node markers are consumed, not skipped; "- - key: v" nests.
	i := start
	for i < line.end && yamlNodeMarker(cp, i, line.end) {
		i += 2
		for i < line.end && cp[i] == sp {
			i++
		}
	}
	if i >= line.end || cp[i] == hash {
		return skipLine
	}
	hasMarker := i > start

	// step 5 — find the key. A colon NOT followed by U+0020 or the line end is an ordinary key
	// character, so "a:b: v" has the key "a:b"; stating that is what keeps five scanners agreeing.
	keyStart := i
	first := cp[i]
	colonAt, keyEnd := -1, -1
	declined, openQuote := false, false
	switch {
	case first == dquote || first == squote:
		closeAt := yamlClosingQuote(cp, line, i)
		if closeAt < 0 {
			openQuote = true
		} else {
			p := closeAt + 1
			for p < line.end && cp[p] == sp {
				p++
			}
			if p < line.end && yamlIndicatorColon(cp, p, line.end) {
				colonAt, keyEnd, declined = p, closeAt+1, true
			}
		}
	case first == lbrace || first == lbracket || first == ampers || first == star || first == bang:
	default:
		for j := i; j < line.end; j++ {
			c := cp[j]
			if c == hash && cp[j-1] == sp {
				break
			}
			if yamlIndicatorColon(cp, j, line.end) {
				colonAt = j
				break
			}
			switch c {
			case dquote, squote, lbrace, lbracket, ampers, star, bang, hash:
				declined = true
			}
		}
		if colonAt >= 0 {
			keyEnd = colonAt
			for keyEnd > keyStart && cp[keyEnd-1] == sp {
				keyEnd--
			}
		}
	}

	// No key: the line is a node of its own, and it absorbs its value run. One that opens a flow
	// collection, a block scalar or a quote and sits on a line of its own absorbs the region.
	if colonAt < 0 {
		ownLine := !hasMarker && (openQuote || first == lbrace || first == lbracket ||
			first == pipe || first == greater)
		if ownLine {
			return yamlOwnLineRunEnd(cp, lines, li, indent)
		}
		return yamlValueRunEnd(cp, lines, li, indent)
	}

	// step 6 — an empty value means a nested node, whose lines ARE scanned on their own.
	v := colonAt + 1
	for v < line.end && cp[v] == sp {
		v++
	}
	if v >= line.end {
		return skipLine
	}

	// step 7 — the line carries an inline value, so its continuation lines belong to that value. A
	// declined key's value absorbs them only if it can continue on the next line.
	if declined {
		open := false
		switch cp[v] {
		case pipe, greater, lbrace, lbracket, ampers, bang:
			open = true
		case dquote, squote:
			closeAt := yamlClosingQuote(cp, line, v)
			p := closeAt + 1
			for p < line.end && cp[p] == sp {
				p++
			}
			open = closeAt < 0 || (p < line.end && cp[p] != hash)
		}
		if open {
			return yamlValueRunEnd(cp, lines, li, indent)
		}
		return skipLine
	}
	next := yamlValueRunEnd(cp, lines, li, indent)

	// step 8 — the key must be listed. Checked before the value's form, so an unlisted key costs
	// nothing to decline: this is what makes "run:", "if:" and "image:" unreachable.
	if _, ok := keys[string(cp[keyStart:keyEnd])]; !ok {
		return next
	}
	if quiet {
		return next
	}

	// step 9 — the scalar form.
	switch cp[v] {
	case hash, ampers, star, bang, lbrace, lbracket:
		return next
	case pipe, greater:
		yamlBlockScalar(cp, lines, li, next, indent, v, spans)
		return next
	case dquote, squote:
		if !yamlHasContinuation(cp, lines, li, next) {
			yamlQuotedScalar(cp, line, v, spans)
		}
		return next
	}
	if !yamlHasContinuation(cp, lines, li, next) {
		yamlPlainScalar(cp, line, v, spans)
	}
	return next
}

// yamlBlockScalar is 3.8.5. One span per non-blank content line, starting after the block's own
// indentation. The header, the indentation and every line terminator lie outside every span —
// including the run of line terminators at the end that the chomping indicator governs, which is
// why "|", "|-", "|+", ">", ">-" and ">+" are handled identically here.
func yamlBlockScalar(cp []rune, lines []yamlLine, li, runEnd, indent, v int, spans *[]Span) {
	line := lines[li]

	// The header: at most one chomping indicator and at most one indentation indicator, in either
	// order, then optional spaces and an optional comment. Anything else is unrecognised.
	h := v + 1
	explicitIndent := 0
	chomping := false
	for h < line.end {
		c := cp[h]
		if (c == dash || c == '+') && !chomping {
			chomping = true
			h++
			continue
		}
		if c >= '1' && c <= '9' && explicitIndent == 0 {
			explicitIndent = int(c - '0')
			h++
			continue
		}
		break
	}
	for h < line.end && cp[h] == sp {
		h++
	}
	if h < line.end && cp[h] != hash {
		return
	}

	// One definition of the run, and four conditions that make the whole block yield no spans.
	content := make([]yamlLine, 0, 8)
	contentIndent := -1
	for k := li + 1; k < runEnd; k++ {
		next := lines[k]
		if yamlBlank(cp, next) {
			continue // blank lines belong to the block and yield no span
		}
		if yamlHasTab(cp, next) || yamlHasCR(cp, next) {
			return
		}
		nextIndent := yamlFirstNonSpace(cp, next) - next.start
		if contentIndent < 0 {
			if explicitIndent > 0 {
				contentIndent = indent + explicitIndent
			} else {
				contentIndent = nextIndent
			}
		}
		// An explicit indicator that disagrees with the block as written, or a later line dedented
		// inside it, is ambiguous rather than guessable — bail rather than choose.
		if nextIndent < contentIndent {
			return
		}
		content = append(content, next)
	}
	if contentIndent <= 0 {
		return
	}

	for _, c := range content {
		from := c.start + contentIndent
		if c.end > from {
			*spans = append(*spans, Span{Start: from, End: c.end})
		}
	}
}

// yamlQuotedScalar is 3.8.6. The spans are the content between the quotes. A backslash in a
// double-quoted scalar is a bail, so that source characters and content characters are the same
// thing, which the offset model of 3.1 requires — the same constraint that makes an HTML character
// reference an opaque unit in 3.6. A doubled apostrophe in a single-quoted scalar is that opaque
// unit: it lies outside every span and the scalar's range is split around it. No colon test applies
// here: quoting neutralises the colon, and applying the plain-scalar test would decline
// `title: "Chapter 1: the beginning"`.
func yamlQuotedScalar(cp []rune, line yamlLine, v int, spans *[]Span) {
	quote := cp[v]
	closeAt := -1
	segment := v + 1
	var found []Span
	for j := v + 1; j < line.end; j++ {
		c := cp[j]
		if quote == dquote && c == backsl {
			return
		}
		if c != quote {
			continue
		}
		if quote == squote && j+1 < line.end && cp[j+1] == squote {
			if j > segment {
				found = append(found, Span{Start: segment, End: j})
			}
			segment = j + 2
			j++
			continue
		}
		closeAt = j
		break
	}
	if closeAt < 0 {
		return
	}

	after := closeAt + 1
	for after < line.end && cp[after] == sp {
		after++
	}
	if after < line.end && cp[after] != hash {
		return
	}

	if closeAt > segment {
		found = append(found, Span{Start: segment, End: closeAt})
	}
	*spans = append(*spans, found...)
}

// yamlPlainScalar is 3.8.6. In a plain scalar ':' and '#' are still live: U+0020 beside either of
// them is what turns a scalar into a mapping indicator or a comment, and dashes emits U+0020 in
// every "-spaced" locale. Lifting both out as opaque units puts the dash token at a span extremity,
// where the edge-growth rule of 3.4 discards the replacement that emits one.
func yamlPlainScalar(cp []rune, line yamlLine, v int, spans *[]Span) {
	// The scalar ends before a trailing comment, so a colon inside that comment is not the
	// scalar's and must not decline it.
	end := line.end
	for j := v; j < line.end; j++ {
		if cp[j] == hash && j > v && cp[j-1] == sp {
			end = j - 1
			break
		}
	}
	for end > v && cp[end-1] == sp {
		end--
	}
	if end <= v {
		return
	}

	// Compact nesting is not a value: "key: - item" opens a sequence, and "key: a .:" is a mapping
	// whose key is "a ." — a plain scalar can never contain a colon in that position.
	if yamlSequenceDash(cp, v, line.end) {
		return
	}
	for j := v; j < end; j++ {
		if yamlIndicatorColon(cp, j, line.end) {
			return
		}
	}

	segment := v
	for j := v; j <= end; j++ {
		if j == end || cp[j] == colon || cp[j] == hash {
			if j > segment {
				*spans = append(*spans, Span{Start: segment, End: j})
			}
			segment = j + 1
		}
	}
}
