// Package engine is the L1 rule pipeline (docs/ARCHITECTURE.md section 2): a pure, code-point
// scanning engine with no I/O, no clock, no package-level mutable state. It is unexported
// (internal/) so the public API surface stays exactly the root polytypo package.
package engine

// ErrorCode is one of the seven stable, cross-runtime error codes (ARCHITECTURE.md section 4.6).
// Messages are English and are not part of the contract; codes are.
type ErrorCode string

const (
	CodeUnknownLocale       ErrorCode = "POLYTYPO_UNKNOWN_LOCALE"
	CodeInvalidMode         ErrorCode = "POLYTYPO_INVALID_MODE"
	CodeInvalidDialect      ErrorCode = "POLYTYPO_INVALID_DIALECT"
	CodeUnknownRule         ErrorCode = "POLYTYPO_UNKNOWN_RULE"
	CodeMalformedLocaleData ErrorCode = "POLYTYPO_MALFORMED_LOCALE_DATA"
	CodeRuleContract        ErrorCode = "POLYTYPO_RULE_CONTRACT"
	CodeMalformedInput      ErrorCode = "POLYTYPO_MALFORMED_INPUT"
	// CodeInvalidOption is spec 1.3.0 and deliberately general: every option added from 1.3.0
	// on shares it, while Mode and Dialect keep their own because callers branch on them.
	CodeInvalidOption ErrorCode = "POLYTYPO_INVALID_OPTION"
)

// Error is the only error type this module ever returns. It is defined here, the lowest layer
// that needs it, and re-exported by the public polytypo package via a type alias, so external
// callers never import internal/engine directly (Go's usual idiom for hiding an internal package
// behind a stable public name).
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func newError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewError constructs an *Error, exported for internal/modes and the root polytypo package, which
// cannot use the unexported constructor.
func NewError(code ErrorCode, message string) *Error {
	return newError(code, message)
}

// NewMalformedInputError is a convenience constructor for the one error code a mode adapter (as
// opposed to the pipeline itself) ever raises directly.
func NewMalformedInputError(message string) *Error {
	return newError(CodeMalformedInput, message)
}
