<p align="center">
  <img src="https://raw.githubusercontent.com/polytypo/polytypo/main/brand/logo/polytypo-lockup-stacked.svg" alt="polytypo" width="260">
</p>

<h1 align="center">polytypo</h1>

<p align="center">
  <a href="https://pkg.go.dev/github.com/polytypo/polytypo-go"><img src="https://pkg.go.dev/badge/github.com/polytypo/polytypo-go.svg" alt="Go Reference"></a>
  <a href="https://github.com/polytypo/polytypo-go/actions/workflows/ci.yml"><img src="https://github.com/polytypo/polytypo-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
</p>

<p align="center">
  Locale-correct quotes, dashes, ellipses, apostrophes, symbols and no-break spaces —<br>
  one portable spec, designed for byte-identical output across runtimes.
</p>

<p align="center">
  <strong>Try it live, no install: <a href="https://polytypo.dev/">polytypo.dev</a></strong>
</p>

This is the Go implementation. The full spec — all locales, all rules, worked examples in each —
lives in [polytypo/polytypo](https://github.com/polytypo/polytypo). This runtime supports the
`text` and `html` modes fully, and `markdown` for the `commonmark` dialect only — `mdx` returns
`CodeInvalidDialect` (no MDX/JSX parser is available for Go; see
[Supported dialects](#supported-dialects)).

## Install

```sh
go get github.com/polytypo/polytypo-go
```

## Usage

```go
import polytypo "github.com/polytypo/polytypo-go"

out, err := polytypo.Transform(`She said, "it's fine" -- but I wasn't sure...`, polytypo.Options{
    Locale: "en-US",
})
// out == `She said, “it’s fine”—but I wasn’t sure…`
```

Same input, one locale changed — quotes, dash spacing and all follow the target locale, not a
single hardcoded style:

```go
out, err := polytypo.Transform(`Sie sagte: "Alles gut" -- aber ich war mir nicht sicher...`, polytypo.Options{
    Locale: "de-DE",
})
// out == `Sie sagte: „Alles gut“ – aber ich war mir nicht sicher…`
```

HTML and Markdown are first-class modes, not an afterthought — tags, attributes and fenced code
are left alone; only text content is touched:

```go
out, err := polytypo.Transform(`<a title="test... wait">Wait... she said "go on."</a>`, polytypo.Options{
    Locale: "en-US",
    Mode:   "html",
})
// out == `<a title="test... wait">Wait… she said “go on.”</a>`
// (the attribute value's straight quotes and ellipsis are untouched; only the text content is)
```

`Locale` has no default anywhere and must always be set explicitly — there is no silent fallback
to English. `Mode` defaults to `"text"` if left empty.

```go
out, err := polytypo.Transform(input, polytypo.Options{
    Locale:  "fr",
    Mode:    "markdown",
    Dialect: "commonmark",
})
```

Unlike the JS and Python ports, there is no `polytypo/text`, `polytypo/html` or
`polytypo/markdown` subpath split: Go's linker already dead-code-eliminates unreached functions
and `go.sum` entries are cheap, so the bundle-size/import-time motivation that justifies a
subpath split in those two runtimes doesn't transfer here. A single `Transform` function in one
package is the better fit for Go. Importing this package does pull in `golang.org/x/net/html`
and `github.com/yuin/goldmark` transitively, regardless of which mode a given caller actually
uses.

### Errors

Every error `Transform` returns is a `*polytypo.Error` carrying one of seven stable codes — check
`Code`, not the message text, which is English and informative but not part of the contract:

```go
var perr *polytypo.Error
if errors.As(err, &perr) {
    switch perr.Code {
    case polytypo.CodeUnknownLocale:
        // ...
    }
}
```

## Supported dialects

`markdown` mode requires a `Dialect`, exactly as the spec requires (no default, detection is
forbidden). This runtime supports `Dialect: "commonmark"` (CommonMark plus GFM — tables,
strikethrough, task lists, autolink literals). `Dialect: "mdx"` is a real dialect the spec names,
but this runtime has no MDX/JSX parser for it and returns `CodeInvalidDialect` immediately rather
than silently mishandling it — a narrower, honest conformance claim, not a port defect.

## Concurrency

`Transform` is a pure function with no package-level mutable state: safe to call concurrently
from any number of goroutines with no external synchronization.

## Licence

MIT. See [LICENSE](LICENSE).
