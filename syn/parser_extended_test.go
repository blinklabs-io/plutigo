package syn

import (
	"testing"
)

func TestParseDelay(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple delay with constant",
			input:   "(delay (con integer 42))",
			wantErr: false,
		},
		{
			name:    "delay with nested force",
			input:   "(delay (force (delay (con integer 1))))",
			wantErr: false,
		},
		{
			name:    "delay with variable",
			input:   "(delay x)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseTerm()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTerm() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseForce(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple force with delay",
			input:   "(force (delay (con integer 42)))",
			wantErr: false,
		},
		{
			name:    "force with nested delay",
			input:   "(force (delay (force (delay (con integer 1)))))",
			wantErr: false,
		},
		{
			name:    "force with variable",
			input:   "(force x)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseTerm()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTerm() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseConstr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "constr with no fields",
			input:   "(program 1.1.0 (constr 0))",
			wantErr: false,
		},
		{
			name:    "constr with one field",
			input:   "(program 1.1.0 (constr 0 (con integer 42)))",
			wantErr: false,
		},
		{
			name:    "constr with multiple fields",
			input:   "(program 1.1.0 (constr 1 (con integer 1) (con integer 2) (con integer 3)))",
			wantErr: false,
		},
		{
			name:    "constr with nested terms",
			input:   "(program 1.1.0 (constr 0 (delay (con integer 1)) (force x)))",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseProgram()
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"ParseProgram() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestParseCase(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "case with one branch",
			input:   "(program 1.1.0 (case (constr 0) (con integer 1)))",
			wantErr: false,
		},
		{
			name:    "case with multiple branches",
			input:   "(program 1.1.0 (case (constr 0) (con integer 1) (con integer 2) (con integer 3)))",
			wantErr: false,
		},
		{
			name:    "case with variable constr",
			input:   "(program 1.1.0 (case x (con integer 1) (con integer 2)))",
			wantErr: false,
		},
		{
			name:    "case with nested terms",
			input:   "(program 1.1.0 (case (constr 0) (delay (con integer 1)) (force x)))",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseProgram()
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"ParseProgram() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple error",
			input:   "(error)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			term, err := p.ParseTerm()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTerm() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if _, ok := term.(*Error); !ok {
					t.Errorf("Expected *Error, got %T", term)
				}
			}
		})
	}
}

func TestParseComplexTerms(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "complex nested term",
			input:   "(program 1.1.0 (lam x (force (delay (case (constr 0 x) (con integer 1) (con integer 2))))))",
			wantErr: false,
		},
		{
			name:    "apply with delay and force",
			input:   "(program 1.0.0 [(force (delay (con integer 42))) (con integer 1)])",
			wantErr: false,
		},
		{
			name:    "case with constr in branches",
			input:   "(program 1.1.0 (case x (constr 0) (constr 1 (con integer 1))))",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseProgram()
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"ParseProgram() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
