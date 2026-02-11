package cek

import (
	"errors"
	"fmt"
)

// EvalError is the base interface for all evaluation errors.
// It allows consumers to programmatically classify errors and determine
// appropriate recovery strategies.
type EvalError interface {
	error
	// IsRecoverable returns true if the error might succeed with more resources
	// (e.g., budget exhaustion). Non-recoverable errors indicate logic failures.
	IsRecoverable() bool
	// ErrorCode returns a numeric code for programmatic error classification.
	ErrorCode() ErrorCode
}

// ErrorCode represents categories of evaluation errors.
type ErrorCode uint16

const (
	// Budget errors (recoverable with more budget)
	ErrCodeBudgetExhausted ErrorCode = 100
	ErrCodeMemoryExhausted ErrorCode = 101
	ErrCodeCPUExhausted    ErrorCode = 102

	// Script errors (contract logic failures)
	ErrCodeExplicitError      ErrorCode = 200
	ErrCodeMissingCaseBranch  ErrorCode = 201
	ErrCodeInvalidCaseBranch  ErrorCode = 202
	ErrCodeCaseOnNegativeInt  ErrorCode = 203
	ErrCodeCaseIntOutOfRange  ErrorCode = 204
	ErrCodeCaseOnByteString   ErrorCode = 205
	ErrCodeMaxIntExceeded     ErrorCode = 206
	ErrCodeInvalidBranchCount ErrorCode = 207

	// Type errors (malformed script structure)
	ErrCodeOpenTerm             ErrorCode = 300
	ErrCodeTypeMismatch         ErrorCode = 301
	ErrCodeNonConstrScrutinized ErrorCode = 302
	ErrCodeNonPolymorphic       ErrorCode = 303
	ErrCodeNonFunctionalApp     ErrorCode = 304
	ErrCodeUnexpectedBuiltinArg ErrorCode = 305
	ErrCodeBuiltinForceExpected ErrorCode = 306

	// Builtin errors (builtin function failures)
	ErrCodeBuiltinFailure   ErrorCode = 400
	ErrCodeDivisionByZero   ErrorCode = 401
	ErrCodeOverflow         ErrorCode = 402
	ErrCodeDecodeFailure    ErrorCode = 403
	ErrCodeOutOfBounds      ErrorCode = 404
	ErrCodeInvalidArgument  ErrorCode = 405
	ErrCodeUnimplemented    ErrorCode = 406
	ErrCodeValidationFailed ErrorCode = 407

	// Internal errors (VM implementation issues)
	ErrCodeInternalError ErrorCode = 500
)

// BudgetError indicates resource exhaustion during evaluation.
// This is the only recoverable error type - retrying with more budget may succeed.
type BudgetError struct {
	Code      ErrorCode
	Requested ExBudget
	Available ExBudget
	Message   string
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf(
		"%s: requested (cpu=%d, mem=%d), available (cpu=%d, mem=%d)",
		e.Message,
		e.Requested.Cpu,
		e.Requested.Mem,
		e.Available.Cpu,
		e.Available.Mem,
	)
}

func (e *BudgetError) IsRecoverable() bool  { return true }
func (e *BudgetError) ErrorCode() ErrorCode { return e.Code }

// ScriptError indicates contract logic failure.
// The script explicitly failed or encountered an invalid case branch.
type ScriptError struct {
	Code    ErrorCode
	Message string
}

func (e *ScriptError) Error() string {
	return e.Message
}

func (e *ScriptError) IsRecoverable() bool  { return false }
func (e *ScriptError) ErrorCode() ErrorCode { return e.Code }

// TypeError indicates malformed script structure.
// The script has structural issues like open terms or type mismatches.
type TypeError struct {
	Code     ErrorCode
	Expected string
	Got      string
	Message  string
}

func (e *TypeError) Error() string {
	if e.Expected != "" && e.Got != "" {
		return fmt.Sprintf(
			"%s: expected %s, got %s",
			e.Message,
			e.Expected,
			e.Got,
		)
	}
	return e.Message
}

func (e *TypeError) IsRecoverable() bool  { return false }
func (e *TypeError) ErrorCode() ErrorCode { return e.Code }

// BuiltinError indicates a builtin function failure.
// The builtin received invalid arguments or encountered a runtime error.
type BuiltinError struct {
	Code    ErrorCode
	Builtin string
	Message string
}

func (e *BuiltinError) Error() string {
	if e.Builtin != "" {
		return fmt.Sprintf("%s: %s", e.Builtin, e.Message)
	}
	return e.Message
}

func (e *BuiltinError) IsRecoverable() bool  { return false }
func (e *BuiltinError) ErrorCode() ErrorCode { return e.Code }

// InternalError indicates a VM implementation issue.
// These errors should not occur during normal operation and may indicate bugs.
type InternalError struct {
	Code    ErrorCode
	Message string
}

func (e *InternalError) Error() string {
	return "internal error: " + e.Message
}

func (e *InternalError) IsRecoverable() bool  { return false }
func (e *InternalError) ErrorCode() ErrorCode { return e.Code }

// Error classification helpers for consumers

// IsBudgetError returns true if the error is a budget exhaustion error.
func IsBudgetError(err error) bool {
	var budgetErr *BudgetError
	return errors.As(err, &budgetErr)
}

// IsScriptError returns true if the error is a script logic error.
func IsScriptError(err error) bool {
	var scriptErr *ScriptError
	return errors.As(err, &scriptErr)
}

// IsTypeError returns true if the error is a type/structure error.
func IsTypeError(err error) bool {
	var typeErr *TypeError
	return errors.As(err, &typeErr)
}

// IsBuiltinError returns true if the error is a builtin function error.
func IsBuiltinError(err error) bool {
	var builtinErr *BuiltinError
	return errors.As(err, &builtinErr)
}

// IsInternalError returns true if the error is an internal VM error.
func IsInternalError(err error) bool {
	var internalErr *InternalError
	return errors.As(err, &internalErr)
}

// IsRecoverable returns true if the error might succeed with more resources.
// Currently, only BudgetError is recoverable.
func IsRecoverable(err error) bool {
	var evalErr EvalError
	if errors.As(err, &evalErr) {
		return evalErr.IsRecoverable()
	}
	return false
}

// GetErrorCode returns the error code if the error implements EvalError.
func GetErrorCode(err error) (ErrorCode, bool) {
	var evalErr EvalError
	if errors.As(err, &evalErr) {
		return evalErr.ErrorCode(), true
	}
	return 0, false
}
