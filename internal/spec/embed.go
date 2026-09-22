// Package spec embeds the vendored copy of polytypo's canonical spec (locale data, rule order,
// and conformance fixtures) into the compiled binary via go:embed, so the shipped module never
// reads from the filesystem at runtime (docs/ARCHITECTURE.md section 3.1). This is a manually
// synced interim copy of github.com/polytypo/polytypo spec/, the same status polytypo-js's and
// polytypo-python's own vendored copies carry -- see this directory's README.md.
//
// Named internal/spec, not vendor/polytypo-spec like the other two ports: a top-level directory
// literally named "vendor" has reserved meaning to the Go toolchain (go mod vendor output), and
// //go:embed patterns cannot reference a parent directory, so the embedding package must live
// inside the copied tree rather than beside it.
package spec

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed VERSION UNICODE
var metaFS embed.FS

//go:embed locales/*.json
var localesFS embed.FS

//go:embed rules/order.json
var orderFS embed.FS

//go:embed fixtures/*.json
var fixturesFS embed.FS

// Version is the spec version this vendored copy implements (spec/VERSION, trimmed).
var Version = mustReadTrimmed(metaFS, "VERSION")

// QuotePair is one open/close quotation-mark pair (locale.schema.json #/$defs/quotePair).
type QuotePair struct {
	Open       string `json:"open"`
	Close      string `json:"close"`
	InnerSpace string `json:"innerSpace"` // "none" | "nbsp" | "narrow-nbsp"
}

// ElisionIdiom is a closed-set elision veto entry (locale.schema.json #/$defs/elisionIdiom).
type ElisionIdiom struct {
	Left   string `json:"left"`
	Elided string `json:"elided"`
	Right  string `json:"right"`
}

// Quotes is a locale's quotes field.
type Quotes struct {
	Primary        QuotePair      `json:"primary"`
	Secondary      QuotePair      `json:"secondary"`
	ElisionIdioms  []ElisionIdiom `json:"elisionIdioms"`
	ElisionClitics ElisionClitics `json:"elisionClitics"`
}

// ElisionClitics is a locale's quotes.elisionClitics field (quotes.md 3.2's span-boundary elision
// veto, spec 1.4.0). Both lists are POSITIONAL: Before is matched against the maximal LETTER run
// ending before the mark, After against the run beginning after it. Both may be empty, and empty
// is a total no-op.
type ElisionClitics struct {
	Before []string `json:"before"`
	After  []string `json:"after"`
}

// Dash is a locale's dash field. Parenthetical/Range are one of "em-tight", "em-spaced",
// "en-tight", "en-spaced", "none".
type Dash struct {
	Parenthetical string `json:"parenthetical"`
	Range         string `json:"range"`
}

// Ellipsis is a locale's ellipsis field.
type Ellipsis struct {
	AbbreviatedAfterTerminal bool `json:"abbreviatedAfterTerminal"`
}

// Hyphen is a locale's hyphen field -- morphological forms whose hyphen must not break.
type Hyphen struct {
	Prefixes  []string `json:"prefixes"`
	Suffixes  []string `json:"suffixes"`
	Compounds []string `json:"compounds"`
}

// NBSP is a locale's nbsp field.
type NBSP struct {
	BeforePunctuation       []string `json:"beforePunctuation"`
	NarrowBeforePunctuation []string `json:"narrowBeforePunctuation"`
	AfterShortWords         []string `json:"afterShortWords"`
	Abbreviations           []string `json:"abbreviations"`
	BeforeUnits             []string `json:"beforeUnits"`
	BeforeNumber            []string `json:"beforeNumber"`
	BeforeWord              []string `json:"beforeWord"`
	AfterSymbols            []string `json:"afterSymbols"`
	InitialBinding          string   `json:"initialBinding"` // "none" | "chain" | "single"
}

// Source is one normative citation (locale.schema.json "sources").
type Source struct {
	Rule string `json:"rule"`
	Cite string `json:"cite"`
	URL  string `json:"url,omitempty"`
	Note string `json:"note,omitempty"`
}

// LocaleData is the full declarative shape of one spec/locales/<code>.json file.
type LocaleData struct {
	Locale   string   `json:"locale"`
	Name     string   `json:"name"`
	Quotes   Quotes   `json:"quotes"`
	Dash     Dash     `json:"dash"`
	Ellipsis Ellipsis `json:"ellipsis"`
	Hyphen   Hyphen   `json:"hyphen"`
	NBSP     NBSP     `json:"nbsp"`
	Sources  []Source `json:"sources"`
}

// Registry is spec/locales/registry.json: the set of known locales and their aliases.
type Registry struct {
	Locales []string          `json:"locales"`
	Aliases map[string]string `json:"aliases"`
}

// Rule is one entry of spec/rules/order.json.
type Rule struct {
	ID         string   `json:"id"`
	Order      int      `json:"order"`
	Default    string   `json:"default"` // "on" | "off"
	Modes      []string `json:"modes"`
	LocaleData []string `json:"localeData"`
	Summary    string   `json:"summary"`
}

// Order is spec/rules/order.json.
type Order struct {
	Spec  string `json:"spec"`
	Rules []Rule `json:"rules"`
}

// FixtureCase is one case of a spec/fixtures/<locale>.json file (fixtures.schema.json).
type FixtureCase struct {
	ID      string          `json:"id"`
	Rule    string          `json:"rule"`
	Mode    string          `json:"mode"`
	Dialect string          `json:"dialect,omitempty"`
	In      string          `json:"in"`
	Out     *string         `json:"out,omitempty"`
	Throws  string          `json:"throws,omitempty"`
	Note    string          `json:"note,omitempty"`
	Rules   map[string]bool `json:"rules,omitempty"`
	// NarrowNbsp is spec 1.3.0's case-level option (nbsp.md 3.1a), passed through to Transform
	// on BOTH calls — the idempotency re-run carries the case's own options, not the defaults.
	NarrowNbsp string `json:"narrowNbsp,omitempty"`
	// Keys is spec 1.3.0's yaml-mode option (modes.md 3.8.2), required exactly when Mode is
	// "yaml" and forbidden otherwise. A nil slice is "not supplied"; an empty, non-nil slice
	// is legal and processes nothing, so the JSON null/[] distinction is load-bearing here.
	Keys []string `json:"keys,omitempty"`
}

// Fixtures is a spec/fixtures/<locale>.json file.
type Fixtures struct {
	Spec   string        `json:"spec"`
	Locale string        `json:"locale"`
	Cases  []FixtureCase `json:"cases"`
}

// ResolutionCase is one case of spec/fixtures/locale-resolution.json (resolution.schema.json).
type ResolutionCase struct {
	ID        string `json:"id"`
	Tag       string `json:"tag,omitempty"`
	TagAbsent bool   `json:"tagAbsent,omitempty"`
	Resolves  string `json:"resolves,omitempty"`
	Throws    string `json:"throws,omitempty"`
	Note      string `json:"note,omitempty"`
}

// Resolution is spec/fixtures/locale-resolution.json.
type Resolution struct {
	Spec  string           `json:"spec"`
	Cases []ResolutionCase `json:"cases"`
}

// LoadRegistry parses locales/registry.json.
func LoadRegistry() (Registry, error) {
	var reg Registry
	if err := readJSON(localesFS, "locales/registry.json", &reg); err != nil {
		return Registry{}, err
	}
	return reg, nil
}

// LoadLocale parses locales/<id>.json. id must already be a canonical locale id (e.g. "en-US").
func LoadLocale(id string) (LocaleData, error) {
	var data LocaleData
	if err := readJSON(localesFS, "locales/"+id+".json", &data); err != nil {
		return LocaleData{}, err
	}
	return data, nil
}

// LoadOrder parses rules/order.json.
func LoadOrder() (Order, error) {
	var order Order
	if err := readJSON(orderFS, "rules/order.json", &order); err != nil {
		return Order{}, err
	}
	return order, nil
}

// LoadFixtures parses fixtures/<locale>.json.
func LoadFixtures(locale string) (Fixtures, error) {
	var f Fixtures
	if err := readJSON(fixturesFS, "fixtures/"+locale+".json", &f); err != nil {
		return Fixtures{}, err
	}
	return f, nil
}

// LoadResolutionFixtures parses fixtures/locale-resolution.json.
func LoadResolutionFixtures() (Resolution, error) {
	var r Resolution
	if err := readJSON(fixturesFS, "fixtures/locale-resolution.json", &r); err != nil {
		return Resolution{}, err
	}
	return r, nil
}

// FixtureLocales lists the locale codes with a fixtures/<code>.json file, excluding the
// locale-resolution fixture file which has a different shape.
func FixtureLocales() ([]string, error) {
	entries, err := fixturesFS.ReadDir("fixtures")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if name == "locale-resolution" {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func readJSON(fsys embed.FS, path string, out any) error {
	b, err := fsys.ReadFile(path)
	if err != nil {
		return fmt.Errorf("spec: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("spec: parse %s: %w", path, err)
	}
	return nil
}

func mustReadTrimmed(fsys embed.FS, path string) string {
	b, err := fsys.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("spec: read %s: %v", path, err))
	}
	return strings.TrimSpace(string(b))
}
