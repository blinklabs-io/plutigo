package cek

import (
	"errors"
	"testing"
)

func TestBudgetError(t *testing.T) {
	err := &BudgetError{
		Code:      ErrCodeBudgetExhausted,
		Requested: ExBudget{Cpu: 1000, Mem: 500},
		Available: ExBudget{Cpu: 100, Mem: 50},
		Message:   "out of budget",
	}

	// Test error message
	expectedMsg := "out of budget: requested (cpu=1000, mem=500), available (cpu=100, mem=50)"
	if err.Error() != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, err.Error())
	}

	// Test IsRecoverable
	if !err.IsRecoverable() {
		t.Error("BudgetError should be recoverable")
	}

	// Test ErrorCode
	if err.ErrorCode() != ErrCodeBudgetExhausted {
		t.Errorf(
			"expected code %d, got %d",
			ErrCodeBudgetExhausted,
			err.ErrorCode(),
		)
	}
}

func TestScriptError(t *testing.T) {
	err := &ScriptError{
		Code:    ErrCodeExplicitError,
		Message: "error explicitly called",
	}

	// Test error message
	if err.Error() != "error explicitly called" {
		t.Errorf("expected %q, got %q", "error explicitly called", err.Error())
	}

	// Test IsRecoverable
	if err.IsRecoverable() {
		t.Error("ScriptError should not be recoverable")
	}

	// Test ErrorCode
	if err.ErrorCode() != ErrCodeExplicitError {
		t.Errorf(
			"expected code %d, got %d",
			ErrCodeExplicitError,
			err.ErrorCode(),
		)
	}
}

func TestTypeError(t *testing.T) {
	err := &TypeError{
		Code:     ErrCodeTypeMismatch,
		Expected: "Integer",
		Got:      "ByteString",
		Message:  "type mismatch",
	}

	// Test error message with expected/got
	expectedMsg := "type mismatch: expected Integer, got ByteString"
	if err.Error() != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, err.Error())
	}

	// Test IsRecoverable
	if err.IsRecoverable() {
		t.Error("TypeError should not be recoverable")
	}

	// Test ErrorCode
	if err.ErrorCode() != ErrCodeTypeMismatch {
		t.Errorf(
			"expected code %d, got %d",
			ErrCodeTypeMismatch,
			err.ErrorCode(),
		)
	}

	// Test without expected/got
	err2 := &TypeError{
		Code:    ErrCodeOpenTerm,
		Message: "open term evaluated",
	}
	if err2.Error() != "open term evaluated" {
		t.Errorf("expected %q, got %q", "open term evaluated", err2.Error())
	}
}

func TestBuiltinError(t *testing.T) {
	err := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Builtin: "divideInteger",
		Message: "division by zero",
	}

	// Test error message
	expectedMsg := "divideInteger: division by zero"
	if err.Error() != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, err.Error())
	}

	// Test IsRecoverable
	if err.IsRecoverable() {
		t.Error("BuiltinError should not be recoverable")
	}

	// Test ErrorCode
	if err.ErrorCode() != ErrCodeDivisionByZero {
		t.Errorf(
			"expected code %d, got %d",
			ErrCodeDivisionByZero,
			err.ErrorCode(),
		)
	}

	// Test without builtin name
	err2 := &BuiltinError{
		Code:    ErrCodeBuiltinFailure,
		Message: "unknown failure",
	}
	if err2.Error() != "unknown failure" {
		t.Errorf("expected %q, got %q", "unknown failure", err2.Error())
	}
}

func TestInternalError(t *testing.T) {
	err := &InternalError{
		Code:    ErrCodeInternalError,
		Message: "compute returned nil state",
	}

	// Test error message
	expectedMsg := "internal error: compute returned nil state"
	if err.Error() != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, err.Error())
	}

	// Test IsRecoverable
	if err.IsRecoverable() {
		t.Error("InternalError should not be recoverable")
	}

	// Test ErrorCode
	if err.ErrorCode() != ErrCodeInternalError {
		t.Errorf(
			"expected code %d, got %d",
			ErrCodeInternalError,
			err.ErrorCode(),
		)
	}
}

func TestIsBudgetError(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}
	internalErr := &InternalError{
		Code:    ErrCodeInternalError,
		Message: "internal",
	}

	if !IsBudgetError(budgetErr) {
		t.Error("IsBudgetError should return true for BudgetError")
	}
	if IsBudgetError(scriptErr) {
		t.Error("IsBudgetError should return false for ScriptError")
	}
	if IsBudgetError(typeErr) {
		t.Error("IsBudgetError should return false for TypeError")
	}
	if IsBudgetError(builtinErr) {
		t.Error("IsBudgetError should return false for BuiltinError")
	}
	if IsBudgetError(internalErr) {
		t.Error("IsBudgetError should return false for InternalError")
	}
}

func TestIsScriptError(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}

	if IsScriptError(budgetErr) {
		t.Error("IsScriptError should return false for BudgetError")
	}
	if !IsScriptError(scriptErr) {
		t.Error("IsScriptError should return true for ScriptError")
	}
	if IsScriptError(typeErr) {
		t.Error("IsScriptError should return false for TypeError")
	}
	if IsScriptError(builtinErr) {
		t.Error("IsScriptError should return false for BuiltinError")
	}
}

func TestIsTypeError(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}

	if IsTypeError(budgetErr) {
		t.Error("IsTypeError should return false for BudgetError")
	}
	if IsTypeError(scriptErr) {
		t.Error("IsTypeError should return false for ScriptError")
	}
	if !IsTypeError(typeErr) {
		t.Error("IsTypeError should return true for TypeError")
	}
	if IsTypeError(builtinErr) {
		t.Error("IsTypeError should return false for BuiltinError")
	}
}

func TestIsBuiltinError(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}

	if IsBuiltinError(budgetErr) {
		t.Error("IsBuiltinError should return false for BudgetError")
	}
	if IsBuiltinError(scriptErr) {
		t.Error("IsBuiltinError should return false for ScriptError")
	}
	if IsBuiltinError(typeErr) {
		t.Error("IsBuiltinError should return false for TypeError")
	}
	if !IsBuiltinError(builtinErr) {
		t.Error("IsBuiltinError should return true for BuiltinError")
	}
}

func TestIsInternalError(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	internalErr := &InternalError{
		Code:    ErrCodeInternalError,
		Message: "internal",
	}

	if IsInternalError(budgetErr) {
		t.Error("IsInternalError should return false for BudgetError")
	}
	if !IsInternalError(internalErr) {
		t.Error("IsInternalError should return true for InternalError")
	}
}

func TestIsRecoverable(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}
	internalErr := &InternalError{
		Code:    ErrCodeInternalError,
		Message: "internal",
	}

	if !IsRecoverable(budgetErr) {
		t.Error("BudgetError should be recoverable")
	}
	if IsRecoverable(scriptErr) {
		t.Error("ScriptError should not be recoverable")
	}
	if IsRecoverable(typeErr) {
		t.Error("TypeError should not be recoverable")
	}
	if IsRecoverable(builtinErr) {
		t.Error("BuiltinError should not be recoverable")
	}
	if IsRecoverable(internalErr) {
		t.Error("InternalError should not be recoverable")
	}

	// Test with non-EvalError
	plainErr := errors.New("plain error")
	if IsRecoverable(plainErr) {
		t.Error("plain error should not be recoverable")
	}
}

func TestGetErrorCode(t *testing.T) {
	budgetErr := &BudgetError{
		Code:    ErrCodeBudgetExhausted,
		Message: "out of budget",
	}
	scriptErr := &ScriptError{Code: ErrCodeExplicitError, Message: "error"}
	typeErr := &TypeError{Code: ErrCodeOpenTerm, Message: "open term"}
	builtinErr := &BuiltinError{
		Code:    ErrCodeDivisionByZero,
		Message: "div zero",
	}
	internalErr := &InternalError{
		Code:    ErrCodeInternalError,
		Message: "internal",
	}

	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
		wantOK   bool
	}{
		{"BudgetError", budgetErr, ErrCodeBudgetExhausted, true},
		{"ScriptError", scriptErr, ErrCodeExplicitError, true},
		{"TypeError", typeErr, ErrCodeOpenTerm, true},
		{"BuiltinError", builtinErr, ErrCodeDivisionByZero, true},
		{"InternalError", internalErr, ErrCodeInternalError, true},
		{"plain error", errors.New("plain"), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := GetErrorCode(tt.err)
			if ok != tt.wantOK {
				t.Errorf("GetErrorCode() ok = %v, want %v", ok, tt.wantOK)
			}
			if code != tt.wantCode {
				t.Errorf("GetErrorCode() code = %v, want %v", code, tt.wantCode)
			}
		})
	}
}

func TestErrorsAs(t *testing.T) {
	// Test that errors.As works correctly with our error types
	budgetErr := &BudgetError{
		Code:      ErrCodeBudgetExhausted,
		Requested: ExBudget{Cpu: 1000, Mem: 500},
		Available: ExBudget{Cpu: 100, Mem: 50},
		Message:   "out of budget",
	}

	var target *BudgetError
	if !errors.As(budgetErr, &target) {
		t.Fatal("errors.As should match BudgetError")
	}
	if target == nil {
		t.Fatal("target should not be nil after successful errors.As")
	}
	if target.Requested.Cpu != 1000 {
		t.Errorf("expected Cpu=1000, got %d", target.Requested.Cpu)
	}

	// Test EvalError interface matching
	var evalErr EvalError
	if !errors.As(budgetErr, &evalErr) {
		t.Fatal("errors.As should match EvalError interface")
	}
	if evalErr == nil {
		t.Fatal("evalErr should not be nil after successful errors.As")
	}
	if !evalErr.IsRecoverable() {
		t.Error("matched EvalError should be recoverable")
	}
}

func TestErrorCodeRanges(t *testing.T) {
	// Verify error code ranges are correct
	tests := []struct {
		code     ErrorCode
		minRange ErrorCode
		maxRange ErrorCode
	}{
		{ErrCodeBudgetExhausted, 100, 199},
		{ErrCodeMemoryExhausted, 100, 199},
		{ErrCodeCPUExhausted, 100, 199},
		{ErrCodeExplicitError, 200, 299},
		{ErrCodeMissingCaseBranch, 200, 299},
		{ErrCodeOpenTerm, 300, 399},
		{ErrCodeTypeMismatch, 300, 399},
		{ErrCodeDivisionByZero, 400, 499},
		{ErrCodeBuiltinFailure, 400, 499},
		{ErrCodeInternalError, 500, 599},
	}

	for _, tt := range tests {
		if tt.code < tt.minRange || tt.code > tt.maxRange {
			t.Errorf(
				"error code %d not in expected range [%d, %d]",
				tt.code,
				tt.minRange,
				tt.maxRange,
			)
		}
	}
}
