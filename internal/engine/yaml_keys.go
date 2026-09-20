package engine

import "fmt"

// ResolveYAMLKeys turns the Keys option into the set the yaml scan consults (modes.md 3.8.2).
//
// It has NO DEFAULT, for the reason Dialect has none. YAML is a data format with islands of prose
// in it, and nothing in its syntax marks them — "description" holds a sentence and "run" holds a
// shell script, spelled identically — so a default would be a guess about the schema above the
// document. A nil slice is "not supplied" and returns CodeInvalidOption; an EMPTY, non-nil slice
// is legal and yields no spans, because "process nothing" is a choice a caller may make.
//
// Checked after Locale, last of the option checks, alongside Dialect (ARCHITECTURE.md 7). The two
// never both apply, since each belongs to a different mode.
func ResolveYAMLKeys(keys []string) (map[string]struct{}, error) {
	if keys == nil {
		return nil, NewError(CodeInvalidOption,
			fmt.Sprintf(`%q is required when mode is "yaml" and must be a non-nil []string `+
				`(modes.md 3.8.2); an empty slice is legal and processes nothing`, "keys"))
	}
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set, nil
}
