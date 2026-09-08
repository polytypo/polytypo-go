# Vendored spec subset

This directory is a manually-synced copy of the subset of `polytypo/polytypo`'s canonical `spec/`
that this repository's build and test suite read: `locales/`, `fixtures/`, `rules/order.json`,
`rules/dashes.md`, `schema/`, `VERSION`, `UNICODE`. It is **not** the canonical spec — the rest of
the normative prose (`spec/rules/*.md` beyond `dashes.md`) and `validate-spec.mjs` live only in
`polytypo/polytypo`.

Named `internal/spec`, not `vendor/polytypo-spec` like the JS and Python ports: a top-level
directory literally named `vendor` has reserved meaning to the Go toolchain (`go mod vendor`
output), and `//go:embed` directives cannot reference a parent directory, so the package that
embeds this data (`embed.go`) has to live inside the copied tree rather than beside it. `internal/`
also gives it real, compiler-enforced privacy: nothing outside this module can import it.

Editing a file here does not change the spec; it only drifts this copy from canonical. When
canonical's `spec/` changes, re-copy the affected files here. How this vendoring will work
long-term (submodule, per-ecosystem spec package, or something else) is an open decision tracked
in `polytypo/polytypo`'s `docs/ROADMAP.md`; this is the interim, manually-synced form — the same
status `polytypo-js`'s and `polytypo-python`'s own vendored copies have.
