# Vendored spec subset

This directory is a manually-synced copy of a subset of `polytypo/polytypo`'s canonical `spec/`:
`locales/`, `fixtures/`, the whole of `rules/`, `schema/`, `VERSION` and `UNICODE`. Code and tests
here read only part of that — the locale and fixture data, `rules/order.json` and
`rules/dashes.md`; the other twelve rule documents are carried as the normative prose for the
behaviour the data drives, next to the data. It is **not** the canonical spec: `validate-spec.mjs`
lives only in `polytypo/polytypo`, and so does the authority — a change starts there and arrives
here by re-copying, never the other way round.

Named `internal/spec`, not `vendor/polytypo-spec` like the JS and Python ports: a top-level
directory literally named `vendor` has reserved meaning to the Go toolchain (`go mod vendor`
output), and `//go:embed` directives cannot reference a parent directory, so the package that
embeds this data (`embed.go`) has to live inside the copied tree rather than beside it. `internal/`
also gives it real, compiler-enforced privacy: nothing outside this module can import it.

Editing a file here does not change the spec; it only drifts this copy from canonical. When
canonical's `spec/` changes, re-copy the affected files here.

CI checks that it was done. `scripts/check-vendored-spec.sh` compares every file in this directory
against canonical `polytypo/polytypo` at tag `spec-v` + this directory's own `VERSION`, and fails
on any difference. Five details. The files this repository authors itself are listed in
`.not-canonical` and skipped — and a listed path that canonical *does* have is an error, so that
list cannot be used to keep a forked copy of a canonical file. A `locales/*.json` file is compared
in full when it carries its `sources` array, and against canonical minus that array when it does
not; the script reads which case it is off the file rather than being told. A file here with no canonical counterpart is a failure, so
a canonical rename cannot pass unnoticed. Completeness is checked from canonical's side rather
than this one: every file canonical has at that tag must be here unless `.not-vendored` names it,
so a vendored file that was deleted, or a canonical file that arrived later and was never copied,
fails naming itself, and an entry canonical does not have at that tag, or one that is also present
here, fails too. Here that list names `CONFORMANCE.md`, and nothing else. And the version itself
is checked: a tree faithful to the tag it claims while canonical has tagged a newer one is a
warning in CI and, with `--require-current`, a refusal in the release job, because a package must
not be published claiming a spec version canonical has moved past.

Before that check existed, this half of the tree went stale unnoticed in four of the five
ports at once: the data half is proved by the test suite, and nothing at all read the prose.
See `polytypo/polytypo` issue #56. How this vendoring will work
long-term (submodule, per-ecosystem spec package, or something else) is an open decision tracked
in `polytypo/polytypo`'s `docs/ROADMAP.md`; this is the interim, manually-synced form — the same
status `polytypo-js`'s and `polytypo-python`'s own vendored copies have.

## `locales/*.json` here carry `sources`

These are whole copies of the canonical files, including the mandatory `sources` array
(`spec/schema/locale.schema.json` requires it with `minItems: 1`), and `scripts/check-vendored-spec.sh`
compares them in full. The array is embedded into the binary with the rest of the file; no rule
reads it. The citations are evidence and they are read where they are rendered, on the project's
Locales page, from the canonical `spec/locales/`. Re-syncing this directory means copying the
canonical files as they are.

The check script can also compare a file against canonical minus `sources`, and reads which case
applies off the file. That form exists for a runtime that trims the array before embedding; none of
the five vendors a trimmed copy.
