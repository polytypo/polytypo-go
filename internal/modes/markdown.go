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

// frontmatterBlock is what detectFrontmatter found: the byte just past the closing delimiter
// line's terminator, the delimiter it used, and the byte range of its content — from after the
// opening delimiter line's terminator to the byte that begins the closing delimiter line
// (modes.md 3.7.4). Both delimiter lines and every line terminator lie outside the content range.
// The block has no start field because everything before end is masked, the leading U+FEFF of
// 3.7.3a step 1 included.
type frontmatterBlock struct {
	end          int
	delim        string
	contentStart int
	contentEnd   int
}

// isFenceTail reports whether what follows the delimiter on a delimiter line is only U+0020 and
// U+0009 (modes.md 3.7.3a steps 2 and 3). The terminator's U+000D has already been removed by the
// caller. Every character the test accepts or rejects on is ASCII and no UTF-8 continuation byte
// can equal one, so scanning bytes here gives the same answer as 3.7.3a's code-point scan.
func isFenceTail(tail string) bool {
	for i := 0; i < len(tail); i++ {
		if tail[i] != ' ' && tail[i] != '\t' {
			return false
		}
	}
	return true
}

// detectFrontmatter locates spec/rules/modes.md 3.7.3's frontmatter skip: a metadata block at the
// very start of the document, delimited by "---" (YAML) or "+++" (TOML) on lines of their own,
// skipped whole including its delimiters. goldmark has no frontmatter concept, so the block is
// located here, before parsing, and the same scan serves spec 1.7.0's FrontmatterKeys (modes.md
// 3.7.4), which processes exactly what 3.7.3 skips. One scan, one answer: a second implementation
// of "where does the block start and end" is how the skip and the option would come to disagree.
//
// The extent is modes.md 3.7.3a's scan (spec 1.8.0), not goldmark's opinion: the document must
// begin with the delimiter — after a single leading U+FEFF, which is stepped over — the rest of
// that line may be U+0020 and U+0009 and nothing else, and the closing line is the first later
// line that begins with the same delimiter and carries only U+0020 and U+0009 after it.
// Indentation disqualifies either line, "..." closes nothing, a fourth delimiter character is not
// whitespace, and with no closing line there is no block. Through spec 1.7.0 this runtime required
// both delimiter lines to be exactly the three characters and denied a block to every document
// carrying a byte-order mark, so one character an author cannot see handed the metadata to the
// rules as prose (polytypo/polytypo#58).
func detectFrontmatter(source string) (frontmatterBlock, bool) {
	start := 0
	if strings.HasPrefix(source, "\uFEFF") {
		start = len("\uFEFF") // 3.7.3a step 1: a byte-order mark is not content
	}
	for _, delim := range [2]string{"---", "+++"} {
		if !strings.HasPrefix(source[start:], delim) {
			continue
		}
		openLine, contentStart := frontmatterLineAt(source, start)
		if !isFenceTail(openLine[len(delim):]) {
			continue // not a delimiter line, e.g. "--- yaml" or "----": 3.7.3a step 2
		}
		for cursor := contentStart; cursor < len(source); {
			line, next := frontmatterLineAt(source, cursor)
			if strings.HasPrefix(line, delim) && isFenceTail(line[len(delim):]) {
				return frontmatterBlock{
					end:          next,
					delim:        delim,
					contentStart: contentStart,
					contentEnd:   cursor,
				}, true
			}
			cursor = next
		}
		// No closing line, so no block: 3.7.3a step 4.
	}
	return frontmatterBlock{}, false
}

// frontmatterLineAt returns the line beginning at from — its content, and the index just past its
// terminator — under modes.md 3.7.3a step 5's line model: a line ends at U+000A, at a U+000D not
// followed by U+000A, or at the end of input, and the terminator is never part of the line.
//
// That is CommonMark's model, and it is deliberately not 3.8.4's LF-only one. The block is a
// Markdown construct and ends its lines the way the language around it does; the YAML content
// inside it keeps the LF-only model, which is why YAMLSpans is not changed to match. A port that
// harmonises the two has silently changed one of them.
func frontmatterLineAt(source string, from int) (line string, next int) {
	for i := from; i < len(source); i++ {
		switch source[i] {
		case '\n':
			return source[from:i], i + 1
		case '\r':
			if i+1 < len(source) && source[i+1] == '\n' {
				return source[from:i], i + 2
			}
			return source[from:i], i + 1
		}
	}
	return source[from:], len(source)
}

// maskFrontmatter is modes.md 3.7.3a's second requirement: the source handed to the Markdown
// parser is the document with every code point of the located block — both delimiter lines
// included, and the leading U+FEFF of step 1 if there is one — replaced by U+0020, line
// terminators kept as they are. Suppressing the spans inside the block's range is not enough,
// because by then the parser has already read the block's characters: a fenced-code line in a
// metadata value pairs with the body's own fence, and the body's code block and its prose swap
// places. That is measured damage in this runtime, not a hypothetical, and no span-level test can
// see it.
//
// The mark is masked with the block because otherwise the first line of the parser's input is
// U+FEFF followed by spaces, which is not blank and which goldmark does not strip.
//
// The masking is byte-wise where the spec is code-point-wise, so that byte offsets — what goldmark
// reports and what ByteOffsets maps back to code points — are preserved exactly. A multi-byte code
// point becomes that many U+0020, and a line of nothing but U+0020 is blank to a parser whatever
// its length, so the two agree on everything a parser can observe.
func maskFrontmatter(src []byte, block frontmatterBlock) []byte {
	masked := make([]byte, len(src))
	copy(masked, src)
	for i := 0; i < block.end; i++ {
		if masked[i] != '\n' && masked[i] != '\r' {
			masked[i] = ' '
		}
	}
	return masked
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
	src       []byte
	offsets   *ByteOffsets
	spans     []Span
	htmlStack []string
	err       error
}

func (w *mdWalker) emit(startByte, endByte int) {
	if endByte <= startByte || len(w.htmlStack) > 0 {
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

	// 3.7.3a: the parser reads the block masked out, and that is the whole of the skip. The walk
	// no longer suppresses spans by byte range as well — a masked block is blank lines, so there
	// is nothing there for the walk to find, and a second mechanism the spec does not describe is
	// how two runtimes come to skip different things.
	block, hasBlock := detectFrontmatter(source)
	parsed := src
	if hasBlock {
		parsed = maskFrontmatter(src, block)
	}

	w := &mdWalker{
		src:     parsed,
		offsets: offsets,
	}

	var perr error
	func() {
		defer RecoverParsePanic(&perr)
		root := markdownParser.Parse(text.NewReader(parsed))
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
