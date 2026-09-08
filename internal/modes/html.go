package modes

import (
	"io"
	"strings"

	"golang.org/x/net/html"

	"github.com/polytypo/polytypo-go/internal/engine"
)

// skippedElements is spec/rules/modes.md 3.6, exhaustive and CLOSED: extending it is a spec
// change, not an implementation decision. svg/math are here because in MathML a quotation mark,
// a hyphen and a prime are operators and identifiers — substituting a curly glyph changes what
// the expression means.
var skippedElements = map[string]bool{
	"code": true, "pre": true, "kbd": true, "samp": true, "var": true,
	"script": true, "style": true, "textarea": true, "svg": true, "math": true,
}

// voidElements never get pushed onto the element stack, since an author is not required to close
// them and x/net/html's Tokenizer reports one only as a StartTagToken (or SelfClosingTagToken if
// written with a trailing "/>"), never with a matching EndTagToken. None are skip-listed, so this
// never affects skip tracking, only stack hygiene.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "param": true,
	"source": true, "track": true, "wbr": true,
}

// findEntityRefs returns the byte ranges [start, end) of every well-formed character reference in
// raw — &name;, &#1234;, &#x2014; — as spec/rules/modes.md 3.6 requires them treated: opaque
// units, split out of the surrounding text span so their exact source spelling survives untouched
// (the only way &nbsp; does not become a literal U+00A0 and back). "Well-formed" is syntactic
// (shape only, matching the html5lib/Python html.parser precedent this project's other ports
// follow), not validated against the registered named-character-reference table: a bare & that
// begins no well-formed reference is deliberately left inside its span (modes.md 3.6: "Tom &
// Jerry's \"book\"" must not lose the pairing of its quotation marks to a spurious boundary).
func findEntityRefs(raw []byte) [][2]int {
	var refs [][2]int
	n := len(raw)
	i := 0
	for i < n {
		if raw[i] != '&' {
			i++
			continue
		}
		start := i
		j := i + 1
		switch {
		case j < n && raw[j] == '#' && j+1 < n && (raw[j+1] == 'x' || raw[j+1] == 'X'):
			k := j + 2
			for k < n && isHexDigit(raw[k]) {
				k++
			}
			if k > j+2 && k < n && raw[k] == ';' {
				refs = append(refs, [2]int{start, k + 1})
				i = k + 1
				continue
			}
		case j < n && raw[j] == '#':
			k := j + 1
			for k < n && isASCIIDigit(raw[k]) {
				k++
			}
			if k > j+1 && k < n && raw[k] == ';' {
				refs = append(refs, [2]int{start, k + 1})
				i = k + 1
				continue
			}
		case j < n && isASCIIAlpha(raw[j]):
			k := j + 1
			for k < n && isASCIIAlphaNumeric(raw[k]) {
				k++
			}
			if k < n && raw[k] == ';' {
				refs = append(refs, [2]int{start, k + 1})
				i = k + 1
				continue
			}
		}
		i++
	}
	return refs
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
func isHexDigit(b byte) bool {
	return isASCIIDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
func isASCIIAlpha(b byte) bool        { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isASCIIAlphaNumeric(b byte) bool { return isASCIIAlpha(b) || isASCIIDigit(b) }

func lastStackIndex(stack []string, tag string) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == tag {
			return i
		}
	}
	return -1
}

// HTMLSpans locates the processable spans of an HTML document. The parser is used only to locate
// spans and is then discarded (modes.md 4: "the document is never serialised") — attributes, tag
// syntax and entity spelling are never touched.
func HTMLSpans(source string) ([]Span, error) {
	cp := engine.ToCodePoints(source)
	offsets := NewByteOffsets(cp)

	z := html.NewTokenizer(strings.NewReader(source))
	var spans []Span
	var stack []string
	skipDepth := 0
	byteOff := 0
	var perr error

	func() {
		defer RecoverParsePanic(&perr)
		for {
			tt := z.Next()
			if tt == html.ErrorToken {
				if err := z.Err(); err != nil && err != io.EOF {
					perr = WrapParseError(err)
				}
				return
			}
			raw := z.Raw()
			rawLen := len(raw)

			switch tt {
			case html.StartTagToken:
				name, _ := z.TagName()
				tag := string(name)
				if !voidElements[tag] {
					stack = append(stack, tag)
					if skippedElements[tag] {
						skipDepth++
					}
				}
			case html.EndTagToken:
				name, _ := z.TagName()
				tag := string(name)
				if idx := lastStackIndex(stack, tag); idx >= 0 {
					popped := stack[idx:]
					stack = stack[:idx]
					for _, n := range popped {
						if skippedElements[n] {
							skipDepth--
						}
					}
				}
			case html.SelfClosingTagToken:
				// Opens and closes atomically in the raw token stream; no persistent stack or
				// skip-depth effect regardless of the tag's identity, since there is no
				// subsequent content inside it to skip.
			case html.TextToken:
				if skipDepth == 0 && rawLen > 0 {
					refs := findEntityRefs(raw)
					cursor := 0
					for _, ref := range refs {
						if ref[0] > cursor {
							spans = append(spans, Span{
								Start: offsets.CodePointOf(byteOff + cursor),
								End:   offsets.CodePointOf(byteOff + ref[0]),
							})
						}
						cursor = ref[1]
					}
					if cursor < rawLen {
						spans = append(spans, Span{
							Start: offsets.CodePointOf(byteOff + cursor),
							End:   offsets.CodePointOf(byteOff + rawLen),
						})
					}
				}
			}
			byteOff += rawLen
		}
	}()

	if perr != nil {
		return nil, perr
	}
	return spans, nil
}
