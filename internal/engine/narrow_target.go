package engine

import "fmt"

// nbsp.md 3.1a: NARROW-TARGET's two possible values.
const (
	// NarrowNoBreakSpace is U+202F, the default NARROW-TARGET.
	NarrowNoBreakSpace = rune(0x202F)
	// NoBreakSpace is U+00A0, what NarrowNbsp "nbsp" substitutes for it.
	NoBreakSpace = rune(0x00A0)
)

// ResolveNarrowTarget turns the NarrowNbsp option into the code point the rule writes
// (nbsp.md 3.1a). Done once, at the call boundary, so no rule ever sees the option's string.
//
// Checked immediately after Mode and before Rules — the two checks that read nothing but the call
// itself come first (ARCHITECTURE.md 7). It runs whether or not `nbsp` is enabled, so a
// misspelled value still raises rather than being silently ignored.
func ResolveNarrowTarget(value string) (rune, error) {
	switch value {
	case "", "narrow":
		return NarrowNoBreakSpace, nil
	case "nbsp":
		return NoBreakSpace, nil
	default:
		return 0, NewError(CodeInvalidOption,
			fmt.Sprintf("unknown narrowNbsp %q. Expected \"narrow\" or \"nbsp\"", value))
	}
}
