package modes

import (
	"fmt"

	"github.com/polytypo/polytypo-go/internal/engine"
)

// WrapParseError wraps any error either mode parser produces into CodeMalformedInput — no
// x/net/html or goldmark error type is ever allowed to escape Transform.
func WrapParseError(err error) error {
	if err == nil {
		return nil
	}
	return engine.NewMalformedInputError(fmt.Sprintf("input did not parse: %v", err))
}

// RecoverParsePanic converts a panic raised while parsing (some parser libraries panic on
// sufficiently malformed input rather than returning an error) into a CodeMalformedInput error.
// Call as `defer modes.RecoverParsePanic(&err)` in any function that invokes a third-party parser.
func RecoverParsePanic(errp *error) {
	if r := recover(); r != nil {
		*errp = engine.NewMalformedInputError(fmt.Sprintf("input did not parse: %v", r))
	}
}
