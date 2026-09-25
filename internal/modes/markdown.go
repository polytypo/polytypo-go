package modes

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extAst "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/polytypo/polytypo-go/internal/engine"
)

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()

// ResolveMarkdownDialect implements spec/rules/modes.md 3.7.1: dialect is required, has no
// default, and is never detected. This runtime supports only "commonmark" (CommonMark 0.31 plus
// GFM — tables, strikethrough, task lists, autolink literals); "mdx" raises CodeInvalidDialect
// like any other unsupported dialect, since no Go MDX/JSX parser has been evaluated — a narrower,
// honestly-declared conformance claim (see spec/CONFORMANCE.md), not a silent mishandling.
func ResolveMarkdownDialect(dialect string) error {
	switch dialect {
	case "":
		return engine.NewError(engine.CodeInvalidDialect,
			`mode "markdown" requires a dialect; there is no default`)
	case "commonmark":
		return nil
	case "mdx":
		return engine.NewError(engine.CodeInvalidDialect,
			`dialect "mdx" is not supported by this runtime (no MDX/JSX parser available); only "commonmark" is supported`)
	default:
		return engine.NewError(engine.CodeInvalidDialect,
			`unknown dialect "`+dialect+`"; this runtime supports "commonmark"`)
	}
}

// detectFrontmatterEnd implements spec/rules/modes.md 3.7.3's frontmatter skip: a metadata block
// at the very start of the document, delimited by "---" (YAML) or "+++" (TOML) alone on their own
// lines, skipped whole including its delimiters. goldmark has no frontmatter concept and
// (mis)parses the block as ordinary CommonMark (typically a thematic break followed by a setext
// heading, or a multi-line paragraph) — this is detected independently, before parsing, and used
// to suppress any span the walk would otherwise produce inside that byte range. Returns 0 (no
// frontmatter) if the document does not open with a bare delimiter line immediately closed by a
// matching one.
func detectFrontmatterEnd(source string) int {
	block, _ := detectFrontmatter(source)
	return block.end
}

// frontmatterBlock is what detectFrontmatter found: the block's end byte, the delimiter it used,
// and the byte range of its content — from after the opening delimiter line's terminator to the
// byte that begins the closing delimiter line (modes.md 3.7.4). Both delimiter lines and every
// line terminator lie outside that range.
type frontmatterBlock struct {
	end          int
	delim        string
	contentStart int
	contentEnd   int
}

// detectFrontmatter is detectFrontmatterEnd's scan, reporting the content range as well, so that
// spec 1.7.0's FrontmatterKeys (modes.md 3.7.4) locates exactly what 3.7.3 skips. One scan, one
// answer: a second implementation of "where does the block start and end" is how the skip and the
// option would come to disagree.
func detectFrontmatter(source string) (frontmatterBlock, bool) {
	for _, delim := range [2]string{"---", "+++"} {
		if !strings.HasPrefix(source, delim) {
			continue
		}
		afterDelim := len(delim)
		firstLineEnd := strings.IndexByte(source[afterDelim:], '\n')
		if firstLineEnd == -1 {
			continue
		}
		if strings.TrimRight(source[afterDelim:afterDelim+firstLineEnd], "\r") != "" {
			continue // not a bare delimiter line, e.g. "---" thematic break followed by text
		}
		contentStart := afterDelim + firstLineEnd + 1
		cursor := contentStart
		rest := source[cursor:]
		for {
			nl := strings.IndexByte(rest, '\n')
			var line string
			var consumed int
			if nl == -1 {
				line, consumed = rest, len(rest)
			} else {
				line, consumed = rest[:nl], nl+1
			}
			if strings.TrimRight(line, "\r") == delim {
				return frontmatterBlock{
					end:          cursor + consumed,
					delim:        delim,
					contentStart: contentStart,
					contentEnd:   cursor,
				}, true
			}
			if nl == -1 {
				break
			}
			rest = rest[consumed:]
			cursor += consumed
		}
	}
	return frontmatterBlock{}, false
}

func isTagNameStop(b byte) bool {
	return b == '>' || b == '/' || b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// parseTag parses "<name ...>", "</name>" or "<name .../>" into (name, closing, selfClosing).
// Comments, declarations and processing instructions ("<!--", "<!", "<?") have no element name
// and return name == "".
func parseTag(raw []byte) (name string, closing bool, selfClosing bool) {
	if len(raw) == 0 || raw[0] != '<' {
		return "", false, false
	}
	i := 1
	if i < len(raw) && raw[i] == '/' {
		closing = true
		i++
	}
	if i >= len(raw) || raw[i] == '!' || raw[i] == '?' {
		return "", false, false
	}
	if !isASCIIAlpha(raw[i]) {
		return "", false, false
	}
	nameStart := i
	for i < len(raw) && !isTagNameStop(raw[i]) {
		i++
	}
	name = strings.ToLower(string(raw[nameStart:i]))
	trimmed := strings.TrimRight(string(raw), " \t\r\n")
	selfClosing = strings.HasSuffix(trimmed, "/>")
	return name, closing, selfClosing
}

type mdWalker struct {
	src            []byte
	offsets        *ByteOffsets
	frontmatterEnd int
	spans          []Span
	htmlStack      []string
	err            error
}

func (w *mdWalker) emit(startByte, endByte int) {
	if endByte <= startByte || startByte < w.frontmatterEnd || len(w.htmlStack) > 0 {
		return
	}
	w.spans = append(w.spans, Span{
		Start: w.offsets.CodePointOf(startByte),
		End:   w.offsets.CodePointOf(endByte),
	})
}

// handleRawHTML implements modes.md 3.7.3's stack-of-open-skipped-elements model for inline raw
// HTML: "isolated tags with Markdown between them, not a tree", so the html skip list's subtree
// rule is a push-on-open, pop-on-matching-close stack rather than a tree walk. No span is ever
// emitted for the tag text itself (it is markup, not prose, in every case).
func (w *mdWalker) handleRawHTML(n *ast.RawHTML) {
	segs := n.Segments
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		name, closing, selfClosing := parseTag(w.src[seg.Start:seg.Stop])
		if name == "" {
			continue
		}
		if closing {
			if len(w.htmlStack) > 0 && w.htmlStack[len(w.htmlStack)-1] == name {
				w.htmlStack = w.htmlStack[:len(w.htmlStack)-1]
			}
			continue
		}
		if !selfClosing && skippedElements[name] {
			w.htmlStack = append(w.htmlStack, name)
		}
	}
}

// handleHTMLBlock hands an HTML block's raw bytes to the HTML span extractor rather than
// processing it as Markdown (modes.md 3.7.3: "handed to the html skip list ... rather than
// processed as markdown") — the prose inside e.g. <div>...</div> is typeset, the markup is not.
func (w *mdWalker) handleHTMLBlock(n *ast.HTMLBlock) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return
	}
	start := lines.At(0).Start
	end := lines.At(lines.Len() - 1).Stop
	if n.ClosureLine.Start >= 0 && n.ClosureLine.Stop > end {
		end = n.ClosureLine.Stop
	}
	if start < w.frontmatterEnd {
		return
	}

	inner, err := HTMLSpans(string(w.src[start:end]))
	if err != nil {
		w.err = err
		return
	}
	base := w.offsets.CodePointOf(start)
	for _, s := range inner {
		w.spans = append(w.spans, Span{Start: base + s.Start, End: base + s.End})
	}
}

// walk recurses over the goldmark AST. Only ast.Text nodes carry a byte Segment for prose content
// — every structural byte (emphasis delimiters, link brackets and destination, table pipes and
// the delimiter row, list markers, blockquote markers, heading hashes, fence lines and info
// strings) is consumed during parsing and simply never appears as a child node or a Text segment
// at all, so — unlike a fully-positioned tree such as tree-sitter's — no explicit "gap filling"
// step is needed here: emitting a span for exactly every Text node's Segment already reconstructs
// precisely the processable prose, nothing more.
func (w *mdWalker) walk(n ast.Node) {
	if w.err != nil {
		return
	}
	switch n.Kind() {
	case ast.KindCodeSpan, ast.KindAutoLink, ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindThematicBreak:
		return
	case ast.KindRawHTML:
		w.handleRawHTML(n.(*ast.RawHTML))
		return
	case ast.KindHTMLBlock:
		w.handleHTMLBlock(n.(*ast.HTMLBlock))
		return
	case ast.KindText:
		t := n.(*ast.Text)
		w.emit(t.Segment.Start, t.Segment.Stop)
		return
	case ast.KindParagraph, ast.KindHeading, ast.KindTextBlock, extAst.KindTableCell:
		// A fresh top-level inline-bearing container: the raw-HTML skip stack resets here (an
		// unclosed skipped start tag skips to the end of the block, modes.md 3.7.3), rather than
		// persisting across unrelated blocks or leaking from one into the next.
		w.htmlStack = w.htmlStack[:0]
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		w.walk(c)
	}
}

// MarkdownSpans locates the processable spans of a Markdown document. dialect must already be
// validated via ResolveMarkdownDialect (== "commonmark"); this function does not re-check it.
// FrontmatterSpans is the frontmatter block's own spans, which form a SECOND TEXT UNIT
// (modes.md 3.1, 3.7.4, spec 1.7.0): the pipeline runs over them separately from the body's, so
// an unbalanced mark in a metadata field can never pair with one in the first paragraph, and the
// option cannot change a byte outside the block.
//
// Spans come from the scan of modes.md 3.8 — frontmatter IS YAML, and implementing that grammar
// twice is how two implementations of one spec drift — with keys as step 8's key predicate. The
// block is the construct MarkdownSpans skips (3.7.3, the same detectFrontmatter scan), so the
// option only ever adds spans where the skip removed them: no source position belongs to both
// units. A TOML block yields nothing, with the option or without it — its quoting is a second
// grammar this scan does not claim (modes.md 7.13).
func FrontmatterSpans(source string, keys map[string]struct{}) []Span {
	if len(keys) == 0 {
		return nil
	}
	block, ok := detectFrontmatter(source)
	if !ok || block.delim != "---" {
		return nil
	}
	cp := engine.ToCodePoints(source)
	offsets := NewByteOffsets(cp)
	start := offsets.CodePointOf(block.contentStart)
	end := offsets.CodePointOf(block.contentEnd)
	if end <= start {
		return nil
	}
	spans := YAMLSpans(cp[start:end], keys)
	shifted := make([]Span, 0, len(spans))
	for _, span := range spans {
		shifted = append(shifted, Span{Start: span.Start + start, End: span.End + start})
	}
	return shifted
}

func MarkdownSpans(source string) ([]Span, error) {
	src := []byte(source)
	cp := engine.ToCodePoints(source)
	offsets := NewByteOffsets(cp)

	w := &mdWalker{
		src:            src,
		offsets:        offsets,
		frontmatterEnd: detectFrontmatterEnd(source),
	}

	var perr error
	func() {
		defer RecoverParsePanic(&perr)
		root := markdownParser.Parse(text.NewReader(src))
		w.walk(root)
	}()
	if perr != nil {
		return nil, perr
	}
	if w.err != nil {
		return nil, w.err
	}
	return w.spans, nil
}
