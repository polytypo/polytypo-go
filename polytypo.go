// Package polytypo normalizes typography across languages: locale-correct quotes, dashes,
// ellipses, apostrophes, symbols and no-break spaces, from a spec shared across every polytypo
// runtime (github.com/polytypo/polytypo). See spec/CONFORMANCE.md there for exactly what this
// runtime implements.
//
// Unlike the JS and Python ports, this package has no per-mode subpath split
// (polytypo/text, polytypo/html, polytypo/markdown): Go's linker already dead-code-eliminates
// unreached functions, and go.sum entries are cheap, so the bundle-size/import-time motivation
// that justified a subpath split in those two runtimes barely applies here. A single idiomatic
// Transform function is the better fit for "idiomatic naming per runtime"
// (docs/ARCHITECTURE.md section 7) than fighting Go's per-package import model to imitate a
// pattern built for a different problem. Importing this package does pull in
// golang.org/x/net/html and github.com/yuin/goldmark transitively, unconditionally, regardless
// of which mode a given caller actually uses.
package polytypo

import (
	"fmt"

	"github.com/polytypo/polytypo-go/internal/engine"
	_ "github.com/polytypo/polytypo-go/internal/engine/rules" // side effect: registers all 9 rules
	"github.com/polytypo/polytypo-go/internal/modes"
)

// ErrorCode is one of the seven stable, cross-runtime error codes (ARCHITECTURE.md section 4.6).
type ErrorCode = engine.ErrorCode

// Error is the only error type Transform ever returns. No third-party parser's error type is
// ever allowed to escape (ARCHITECTURE.md section 4.6) — check the Code field, not the message,
// which is English and informative but not part of the contract:
//
//	var perr *polytypo.Error
//	if errors.As(err, &perr) && perr.Code == polytypo.CodeUnknownLocale { ... }
type Error = engine.Error

// The seven error codes every polytypo runtime raises.
const (
	CodeUnknownLocale       = engine.CodeUnknownLocale
	CodeInvalidMode         = engine.CodeInvalidMode
	CodeInvalidDialect      = engine.CodeInvalidDialect
	CodeUnknownRule         = engine.CodeUnknownRule
	CodeMalformedLocaleData = engine.CodeMalformedLocaleData
	CodeRuleContract        = engine.CodeRuleContract
	CodeMalformedInput      = engine.CodeMalformedInput
)

// Change is one entry of Analyze's result: a rule id, code-point offsets into the input, and the
// text on both sides of that one edit (spec/rules/analyze.md section 2).
type Change = engine.Change

// Options configures Transform. Matches docs/ARCHITECTURE.md section 7 exactly across every
// runtime, with idiomatic Go naming.
type Options struct {
	// Locale is required. An unknown locale returns CodeUnknownLocale; there is never a
	// fallback to English (section 4.7).
	Locale string
	// Mode is "text" (the default, if empty), "html", or "markdown".
	Mode string
	// Dialect is required iff Mode == "markdown" ("commonmark"; "mdx" is not implemented by
	// this runtime and returns CodeInvalidDialect). Rejected — must be empty — in the other two
	// modes.
	Dialect string
	// Rules is an opt-out map keyed by rule id; false disables a default-on rule, true opts a
	// default-off rule in (only "ranges" defaults off). Absence of a key always means "use that
	// rule's own default," never "off."
	Rules map[string]bool
}

func resolveMode(mode string) (string, error) {
	switch mode {
	case "", "text":
		return "text", nil
	case "html", "markdown":
		return mode, nil
	default:
		return "", engine.NewError(engine.CodeInvalidMode,
			fmt.Sprintf(`unknown mode %q. Expected "text", "html" or "markdown"`, mode))
	}
}

// Transform applies polytypo's rule pipeline to input and returns the result.
//
// Pure: no I/O, no environment, no clock, no globals, no filesystem, no network, no
// package-level mutable state — goroutine-safe and reentrant, callable concurrently from many
// goroutines with no external synchronization (ARCHITECTURE.md section 7).
func Transform(input string, opts Options) (out string, err error) {
	// A rule guarding a build-time invariant on the embedded, vendored locale data (e.g. an
	// entry violating its own schema in a way no valid spec release should) panics with an
	// *engine.Error rather than threading an error return through every RuleFunc signature —
	// this is the one place that panic is converted back into an ordinary returned error, so
	// it never surfaces as a crash to a caller of the public API.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*engine.Error); ok {
				out, err = "", e
				return
			}
			panic(r)
		}
	}()

	mode, err := resolveMode(opts.Mode)
	if err != nil {
		return "", err
	}

	switch mode {
	case "text":
		if opts.Dialect != "" {
			return "", engine.NewError(engine.CodeInvalidDialect, `"dialect" is only valid when mode is "markdown"`)
		}
		return runText(input, opts)
	case "html":
		if opts.Dialect != "" {
			return "", engine.NewError(engine.CodeInvalidDialect, `"dialect" is only valid when mode is "markdown"`)
		}
		return runHTML(input, opts)
	default: // "markdown"
		return runMarkdown(input, opts)
	}
}

func runText(input string, opts Options) (string, error) {
	_, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return "", err
	}
	cp := engine.ToCodePoints(input)
	ctx := engine.RuleContext{Mode: "text", Locale: opts.Locale}
	result, err := engine.RunRules(cp, plan, localeData, ctx)
	if err != nil {
		return "", err
	}
	return engine.FromCodePoints(result), nil
}

func runHTML(input string, opts Options) (string, error) {
	resolvedLocale, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return "", err
	}
	spans, err := modes.HTMLSpans(input)
	if err != nil {
		return "", err
	}
	ctx := engine.RuleContext{Mode: "html", Locale: resolvedLocale}
	return modes.RunOverSpans(engine.ToCodePoints(input), spans, plan, localeData, ctx)
}

func runMarkdown(input string, opts Options) (string, error) {
	// Validation order is public, tested behaviour, identical to the JS/Python reference
	// implementations: rules (an unknown rule id), then locale (an unknown locale), then
	// dialect/parsing.
	resolvedLocale, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return "", err
	}
	if err := modes.ResolveMarkdownDialect(opts.Dialect); err != nil {
		return "", err
	}
	spans, err := modes.MarkdownSpans(input)
	if err != nil {
		return "", err
	}
	ctx := engine.RuleContext{Mode: "markdown", Dialect: opts.Dialect, Locale: resolvedLocale}
	return modes.RunOverSpans(engine.ToCodePoints(input), spans, plan, localeData, ctx)
}

// Analyze runs the same pipeline as Transform and reports what it would do instead of doing it
// (spec/rules/analyze.md). Offsets are code-point offsets into input in every mode — into the
// document, in "html" and "markdown" mode, not into a span.
//
// What it guarantees: the list is empty exactly when Transform would return the input unchanged,
// every RuleID was enabled for the call, and every offset is inside the input. What it does not:
// the list is a report, not a patch — two rules may touch the same original range, so replaying
// it is not guaranteed to reproduce Transform's output. Call Transform for the text (analyze.md
// sections 4 and 5).
//
// Pure and goroutine-safe on the same terms as Transform.
func Analyze(input string, opts Options) (changes []Change, err error) {
	// The same conversion of a locale-data invariant panic into a returned error that Transform
	// does, and for the same reason: it must never surface as a crash to a caller.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*engine.Error); ok {
				changes, err = nil, e
				return
			}
			panic(r)
		}
	}()

	mode, err := resolveMode(opts.Mode)
	if err != nil {
		return nil, err
	}

	switch mode {
	case "text":
		if opts.Dialect != "" {
			return nil, engine.NewError(engine.CodeInvalidDialect, `"dialect" is only valid when mode is "markdown"`)
		}
		return analyzeText(input, opts)
	case "html":
		if opts.Dialect != "" {
			return nil, engine.NewError(engine.CodeInvalidDialect, `"dialect" is only valid when mode is "markdown"`)
		}
		return analyzeHTML(input, opts)
	default: // "markdown"
		return analyzeMarkdown(input, opts)
	}
}

func analyzeText(input string, opts Options) ([]Change, error) {
	_, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return nil, err
	}
	cp := engine.ToCodePoints(input)
	origin := make([]int, len(cp))
	for i := range cp {
		origin[i] = i
	}
	ctx := engine.RuleContext{Mode: "text", Locale: opts.Locale}
	return engine.RunRulesRecording(cp, plan, localeData, ctx, origin, len(cp), nil)
}

func analyzeHTML(input string, opts Options) ([]Change, error) {
	resolvedLocale, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return nil, err
	}
	spans, err := modes.HTMLSpans(input)
	if err != nil {
		return nil, err
	}
	ctx := engine.RuleContext{Mode: "html", Locale: resolvedLocale}
	return modes.AnalyzeOverSpans(engine.ToCodePoints(input), spans, plan, localeData, ctx)
}

func analyzeMarkdown(input string, opts Options) ([]Change, error) {
	// Validation order is public, tested behaviour and is shared with runMarkdown: rules, then
	// locale, then dialect/parsing (analyze.md section 4, A1).
	resolvedLocale, localeData, plan, err := engine.Prepare(opts.Locale, opts.Rules)
	if err != nil {
		return nil, err
	}
	if err := modes.ResolveMarkdownDialect(opts.Dialect); err != nil {
		return nil, err
	}
	spans, err := modes.MarkdownSpans(input)
	if err != nil {
		return nil, err
	}
	ctx := engine.RuleContext{Mode: "markdown", Dialect: opts.Dialect, Locale: resolvedLocale}
	return modes.AnalyzeOverSpans(engine.ToCodePoints(input), spans, plan, localeData, ctx)
}
