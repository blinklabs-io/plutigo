package syn

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
)

func TestIntern(t *testing.T) {
	// Create a simple term
	term := NewVar("x", Unique(0))

	// Intern the term
	interned := Intern(term)

	// Check that the same term is returned
	if interned != term {
		t.Error("Intern should return the same term object")
	}

	// Check that it's still a Var
	if _, ok := interned.(*Var[Name]); !ok {
		t.Error("Intern should preserve term type")
	}
}

func TestNewProgram(t *testing.T) {
	version := [3]uint32{1, 0, 0}
	term := &Var[Name]{Name: NewRawName("test")}

	program := NewProgram(version, term)

	if program.Version != version {
		t.Errorf("Expected version %v, got %v", version, program.Version)
	}

	if program.Term != term {
		t.Error("Term not set correctly")
	}
}

func TestNewName(t *testing.T) {
	name := NewName("testVar", Unique(42))

	if name.Text != "testVar" {
		t.Errorf("Expected text 'testVar', got %q", name.Text)
	}

	if name.Unique != 42 {
		t.Errorf("Expected unique 42, got %d", name.Unique)
	}
}

func TestNewRawName(t *testing.T) {
	name := NewRawName("testVar")

	if name.Text != "testVar" {
		t.Errorf("Expected text 'testVar', got %q", name.Text)
	}

	if name.Unique != 0 {
		t.Errorf("Expected unique 0, got %d", name.Unique)
	}
}

func TestNewVar(t *testing.T) {
	term := NewVar("testVar", Unique(42))

	varTerm, ok := term.(*Var[Name])
	if !ok {
		t.Fatal("Expected Var term")
	}

	if varTerm.Name.Text != "testVar" {
		t.Errorf("Expected name 'testVar', got %q", varTerm.Name.Text)
	}

	if varTerm.Name.Unique != 42 {
		t.Errorf("Expected unique 42, got %d", varTerm.Name.Unique)
	}
}

func TestNewRawVar(t *testing.T) {
	term := NewRawVar("testVar")

	varTerm, ok := term.(*Var[Name])
	if !ok {
		t.Fatal("Expected Var term")
	}

	if varTerm.Name.Text != "testVar" {
		t.Errorf("Expected name 'testVar', got %q", varTerm.Name.Text)
	}

	if varTerm.Name.Unique != 0 {
		t.Errorf("Expected unique 0, got %d", varTerm.Name.Unique)
	}
}

func TestNewApply(t *testing.T) {
	function := NewRawVar("f")
	argument := NewRawVar("x")

	apply := NewApply(function, argument)

	applyTerm, ok := apply.(*Apply[Name])
	if !ok {
		t.Fatal("Expected Apply term")
	}

	if applyTerm.Function != function {
		t.Error("Function not set correctly")
	}

	if applyTerm.Argument != argument {
		t.Error("Argument not set correctly")
	}
}

func TestNewLambda(t *testing.T) {
	paramName := NewRawName("x")
	body := NewRawVar("x")

	lambda := NewLambda(paramName, body)

	lambdaTerm, ok := lambda.(*Lambda[Name])
	if !ok {
		t.Fatal("Expected Lambda term")
	}

	if lambdaTerm.ParameterName != paramName {
		t.Error("ParameterName not set correctly")
	}

	if lambdaTerm.Body != body {
		t.Error("Body not set correctly")
	}
}

func TestNewDelay(t *testing.T) {
	innerTerm := NewRawVar("x")

	delay := NewDelay(innerTerm)

	delayTerm, ok := delay.(*Delay[Name])
	if !ok {
		t.Fatal("Expected Delay term")
	}

	if delayTerm.Term != innerTerm {
		t.Error("Term not set correctly")
	}
}

func TestNewForce(t *testing.T) {
	innerTerm := NewRawVar("x")

	force := NewForce(innerTerm)

	forceTerm, ok := force.(*Force[Name])
	if !ok {
		t.Fatal("Expected Force term")
	}

	if forceTerm.Term != innerTerm {
		t.Error("Term not set correctly")
	}
}

func TestNewConstant(t *testing.T) {
	integerConst := &Integer{Inner: big.NewInt(42)}

	constant := NewConstant(integerConst)

	constantTerm, ok := constant.(*Constant)
	if !ok {
		t.Fatal("Expected Constant term")
	}

	if constantTerm.Con != integerConst {
		t.Error("Constant not set correctly")
	}
}

func TestNewSimpleInteger(t *testing.T) {
	term := NewSimpleInteger(42)

	constantTerm, ok := term.(*Constant)
	if !ok {
		t.Fatal("Expected Constant term")
	}

	integerConst, ok := constantTerm.Con.(*Integer)
	if !ok {
		t.Fatal("Expected Integer constant")
	}

	if integerConst.Inner.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected value 42, got %s", integerConst.Inner.String())
	}
}

func TestNewInteger(t *testing.T) {
	value := big.NewInt(123456789)
	term := NewInteger(value)

	constantTerm, ok := term.(*Constant)
	if !ok {
		t.Fatal("Expected Constant term")
	}

	integerConst, ok := constantTerm.Con.(*Integer)
	if !ok {
		t.Fatal("Expected Integer constant")
	}

	if integerConst.Inner.Cmp(value) != 0 {
		t.Errorf("Expected value %s, got %s", value.String(), integerConst.Inner.String())
	}
}

func TestNewBool(t *testing.T) {
	term := NewBool(true)

	constantTerm, ok := term.(*Constant)
	if !ok {
		t.Fatal("Expected Constant term")
	}

	boolConst, ok := constantTerm.Con.(*Bool)
	if !ok {
		t.Fatal("Expected Bool constant")
	}

	if boolConst.Inner != true {
		t.Error("Expected true, got false")
	}
}

func TestNewBuiltin(t *testing.T) {
	term := NewBuiltin(builtin.AddInteger)

	builtinTerm, ok := term.(*Builtin)
	if !ok {
		t.Fatal("Expected Builtin term")
	}

	if builtinTerm.DefaultFunction != builtin.AddInteger {
		t.Error("Builtin function not set correctly")
	}
}

func TestAddInteger(t *testing.T) {
	term := AddInteger()

	builtinTerm, ok := term.(*Builtin)
	if !ok {
		t.Fatal("Expected Builtin term")
	}

	if builtinTerm.DefaultFunction != builtin.AddInteger {
		t.Error("Expected AddInteger builtin")
	}
}

func TestSubtractInteger(t *testing.T) {
	term := SubtractInteger()

	builtinTerm, ok := term.(*Builtin)
	if !ok {
		t.Fatal("Expected Builtin term")
	}

	if builtinTerm.DefaultFunction != builtin.SubtractInteger {
		t.Error("Expected SubtractInteger builtin")
	}
}

func TestIfThenElse(t *testing.T) {
	term := IfThenElse()

	builtinTerm, ok := term.(*Builtin)
	if !ok {
		t.Fatal("Expected Builtin term")
	}

	if builtinTerm.DefaultFunction != builtin.IfThenElse {
		t.Error("Expected IfThenElse builtin")
	}
}
