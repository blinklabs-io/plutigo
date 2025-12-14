package syn

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
)

func TestIntern(t *testing.T) {
	// Test interning a simple Var
	name := Name{Text: "test", Unique: 0}
	varTerm := &Var[Name]{Name: name}

	interned := Intern(varTerm)

	// The unique should be assigned (0 for the first name)
	if interned.(*Var[Name]).Name.Unique != 0 {
		t.Errorf(
			"Expected first interned name to have unique 0, got %d",
			interned.(*Var[Name]).Name.Unique,
		)
	}

	// Test interning a complex term with duplicate names
	name1 := Name{Text: "x", Unique: 0}
	name2 := Name{Text: "x", Unique: 0} // Same text

	var1 := &Var[Name]{Name: name1}
	var2 := &Var[Name]{Name: name2}

	// Create a lambda that uses both vars
	lambda := &Lambda[Name]{
		ParameterName: Name{Text: "param", Unique: 0},
		Body: &Apply[Name]{
			Function: var1,
			Argument: var2,
		},
	}

	internedLambda := Intern(lambda)

	// Check that duplicate names now have the same unique
	paramUnique := internedLambda.(*Lambda[Name]).ParameterName.Unique
	varXUnique := internedLambda.(*Lambda[Name]).Body.(*Apply[Name]).Function.(*Var[Name]).Name.Unique
	varXUnique2 := internedLambda.(*Lambda[Name]).Body.(*Apply[Name]).Argument.(*Var[Name]).Name.Unique

	if varXUnique != varXUnique2 {
		t.Errorf(
			"Duplicate variable names should have the same unique: %d != %d",
			varXUnique,
			varXUnique2,
		)
	}

	// Different names should have different uniques
	if paramUnique == varXUnique {
		t.Errorf(
			"Different names should have different uniques: param=%d, x=%d",
			paramUnique,
			varXUnique,
		)
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
		t.Errorf(
			"Expected value %s, got %s",
			value.String(),
			integerConst.Inner.String(),
		)
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

func TestNameToNamedDeBruijn(t *testing.T) {
	// Create a simple program: (lam x x)
	program := NewProgram(
		[3]uint32{1, 0, 0},
		NewLambda(NewName("x", 0), NewVar("x", 0)),
	)

	converted, err := NameToNamedDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToNamedDeBruijn failed: %v", err)
	}

	// Check that the lambda parameter has index 0
	lambda := converted.Term.(*Lambda[NamedDeBruijn])
	if lambda.ParameterName.Index != 0 {
		t.Errorf(
			"Expected parameter index 0, got %d",
			lambda.ParameterName.Index,
		)
	}
	if lambda.ParameterName.Text != "x" {
		t.Errorf(
			"Expected parameter text 'x', got %s",
			lambda.ParameterName.Text,
		)
	}

	// Check that the body variable has index 1
	bodyVar := lambda.Body.(*Var[NamedDeBruijn])
	if bodyVar.Name.Index != 1 {
		t.Errorf("Expected body var index 1, got %d", bodyVar.Name.Index)
	}
	if bodyVar.Name.Text != "x" {
		t.Errorf("Expected body var text 'x', got %s", bodyVar.Name.Text)
	}
}

func TestNameToDeBruijn(t *testing.T) {
	// Create a simple program: (lam x x)
	program := NewProgram(
		[3]uint32{1, 0, 0},
		NewLambda(NewName("x", 0), NewVar("x", 0)),
	)

	converted, err := NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn failed: %v", err)
	}

	// Check that the lambda parameter has index 0
	lambda := converted.Term.(*Lambda[DeBruijn])
	if lambda.ParameterName != 0 {
		t.Errorf("Expected parameter index 0, got %d", lambda.ParameterName)
	}

	// Check that the body variable has index 1 (1-based DeBruijn)
	bodyVar := lambda.Body.(*Var[DeBruijn])
	if bodyVar.Name != 1 {
		t.Errorf("Expected body var index 1, got %d", bodyVar.Name)
	}
}

func TestNameToDeBruijnComplex(t *testing.T) {
	// Create a program with delay and force: (lam x (delay (force x)))
	force := NewForce(NewVar("x", 0))
	delay := NewDelay(force)

	program := NewProgram(
		[3]uint32{1, 0, 0},
		NewLambda(NewName("x", 0), delay),
	)

	converted, err := NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn failed: %v", err)
	}

	// Check the structure
	lambda := converted.Term.(*Lambda[DeBruijn])
	if lambda.ParameterName != 0 {
		t.Errorf(
			"Expected lambda parameter index 0, got %d",
			lambda.ParameterName,
		)
	}

	delayTerm := lambda.Body.(*Delay[DeBruijn])
	forceTerm := delayTerm.Term.(*Force[DeBruijn])
	varTerm := forceTerm.Term.(*Var[DeBruijn])

	if varTerm.Name != 1 {
		t.Errorf("Expected var index 1, got %d", varTerm.Name)
	}
}

func TestNameToDeBruijnWithConstants(t *testing.T) {
	// Create a program with constants and builtins: (lam x ((addInteger 1) x))
	addInt := NewBuiltin(builtin.AddInteger)
	one := NewSimpleInteger(1)
	apply := NewApply(addInt, one)
	apply2 := NewApply(apply, NewVar("x", 0))

	program := NewProgram(
		[3]uint32{1, 0, 0},
		NewLambda(NewName("x", 0), apply2),
	)

	converted, err := NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn failed: %v", err)
	}

	// Check that constants and builtins are preserved
	lambda := converted.Term.(*Lambda[DeBruijn])
	applyTerm := lambda.Body.(*Apply[DeBruijn])
	applyTerm2 := applyTerm.Function.(*Apply[DeBruijn])

	// Check builtin
	builtinTerm := applyTerm2.Function.(*Builtin)
	if builtinTerm.DefaultFunction != builtin.AddInteger {
		t.Errorf("Expected AddInteger builtin")
	}

	// Check constant
	constTerm := applyTerm2.Argument.(*Constant)
	intConst := constTerm.Con.(*Integer)
	if intConst.Inner.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected constant value 1")
	}

	// Check variable
	varTerm := applyTerm.Argument.(*Var[DeBruijn])
	if varTerm.Name != 1 {
		t.Errorf("Expected var index 1, got %d", varTerm.Name)
	}
}

func TestNameToDeBruijnWithError(t *testing.T) {
	// Create a program with an error term: (lam x error)
	program := NewProgram(
		[3]uint32{1, 0, 0},
		NewLambda(NewName("x", 0), &Error{}),
	)

	converted, err := NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn failed: %v", err)
	}

	// Check that error is preserved
	lambda := converted.Term.(*Lambda[DeBruijn])
	errorTerm := lambda.Body.(*Error)
	_ = errorTerm // Just check it's the right type
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
