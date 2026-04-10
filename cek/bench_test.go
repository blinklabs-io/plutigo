package cek

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

var (
	benchmarkValueSink Value[syn.DeBruijn]
	benchmarkBoolSink  bool
	benchmarkTermSink  syn.Term[syn.DeBruijn]
)

func BenchmarkEnvLookupDepth(b *testing.B) {
	depths := []int{1, 4, 8, 16, 32, 64}

	for _, depth := range depths {
		env := buildBenchmarkEnv(depth)
		target := max(1, depth/2)

		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				v, ok := env.Lookup(target)
				benchmarkValueSink = v
				benchmarkBoolSink = ok
			}
		})
	}
}

func BenchmarkMachineLambdaChain(b *testing.B) {
	for _, depth := range []int{4, 8, 16, 32} {
		term := mustBenchmarkTerm(b, buildLambdaChainProgram(depth))
		// Reuse one machine per case so the benchmark measures steady-state
		// evaluation rather than repeated machine construction.
		machine := NewMachine[syn.DeBruijn](
			lang.LanguageVersionV3,
			200,
			nil,
		)

		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				result, err := machine.Run(term)
				if err != nil {
					b.Fatalf("Run failed: %v", err)
				}
				benchmarkTermSink = result
			}
		})
	}
}

func BenchmarkMachineBuiltinHeavy(b *testing.B) {
	for _, count := range []int{8, 32, 128, 256} {
		term := mustBenchmarkTerm(b, buildBuiltinHeavyProgram(count))
		// Reuse one machine per case so the benchmark measures steady-state
		// evaluation rather than repeated machine construction.
		machine := NewMachine[syn.DeBruijn](
			lang.LanguageVersionV3,
			200,
			nil,
		)

		b.Run(fmt.Sprintf("ops=%d", count-1), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				result, err := machine.Run(term)
				if err != nil {
					b.Fatalf("Run failed: %v", err)
				}
				benchmarkTermSink = result
			}
		})
	}
}

func BenchmarkDischarge(b *testing.B) {
	for _, depth := range []int{4, 8, 16, 32} {
		value := buildDischargeValue(depth)

		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				discharged, err := dischargeValue[syn.DeBruijn](value)
				if err != nil {
					b.Fatalf("dischargeValue failed: %v", err)
				}
				benchmarkTermSink = discharged
			}
		})
	}
}

func BenchmarkBuiltinIntegerOps(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   builtin.DefaultFunction
		args []int64
	}{
		{
			name: "AddSmall",
			fn:   builtin.AddInteger,
			args: []int64{123, 456},
		},
		{
			name: "SubtractSmall",
			fn:   builtin.SubtractInteger,
			args: []int64{456, 123},
		},
		{
			name: "EqualsSmall",
			fn:   builtin.EqualsInteger,
			args: []int64{456, 456},
		},
		{
			name: "LessThanSmall",
			fn:   builtin.LessThanInteger,
			args: []int64{123, 456},
		},
		{
			name: "LessThanEqualsSmall",
			fn:   builtin.LessThanEqualsInteger,
			args: []int64{123, 123},
		},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()

			machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
			builtinValue := &Builtin[syn.DeBruijn]{
				Func: bench.fn,
			}
			for _, arg := range bench.args {
				builtinValue = builtinValue.ApplyArg(benchmarkIntValue(arg))
			}

			b.ResetTimer()

			for b.Loop() {
				machine.ExBudget = ExBudget{
					Cpu: 1 << 62,
					Mem: 1 << 62,
				}
				result, err := machine.evalBuiltinApp(builtinValue)
				if err != nil {
					b.Fatalf("evalBuiltinApp failed: %v", err)
				}
				benchmarkValueSink = result
			}
		})
	}
}

func buildBenchmarkEnv(depth int) *Env[syn.DeBruijn] {
	var env *Env[syn.DeBruijn]

	for i := 0; i < depth; i++ {
		value := benchmarkIntValue(int64(i))
		if env == nil {
			env = (*Env[syn.DeBruijn])(nil).Extend(value)
			continue
		}
		env = env.Extend(value)
	}

	return env
}

func benchmarkIntValue(v int64) *Constant {
	return &Constant{
		Constant: &syn.Integer{
			Inner: big.NewInt(v),
		},
	}
}

func mustBenchmarkTerm(
	b *testing.B,
	program *syn.Program[syn.Name],
) syn.Term[syn.DeBruijn] {
	b.Helper()

	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		b.Fatalf("NameToDeBruijn failed: %v", err)
	}

	return dbProgram.Term
}

func buildLambdaChainProgram(depth int) *syn.Program[syn.Name] {
	names := make([]string, depth)
	for i := range depth {
		names[i] = fmt.Sprintf("x%d", i)
	}

	body := syn.NewRawVar(names[0])
	for i := 1; i < depth; i++ {
		body = addTerms(body, syn.NewRawVar(names[i]))
	}

	term := body
	for i := depth - 1; i >= 0; i-- {
		term = syn.NewLambda(syn.NewRawName(names[i]), term)
	}

	for i := 0; i < depth; i++ {
		term = syn.NewApply(term, syn.NewSimpleInteger(i+1))
	}

	return syn.NewProgram(lang.LanguageVersionV3, term)
}

func buildBuiltinHeavyProgram(count int) *syn.Program[syn.Name] {
	term := syn.NewSimpleInteger(1)
	for i := 2; i <= count; i++ {
		term = addTerms(term, syn.NewSimpleInteger(i))
	}
	return syn.NewProgram(lang.LanguageVersionV3, term)
}

func addTerms(left, right syn.Term[syn.Name]) syn.Term[syn.Name] {
	return syn.NewApply(
		syn.NewApply(syn.NewBuiltin(builtin.AddInteger), left),
		right,
	)
}

func buildDischargeValue(depth int) Value[syn.DeBruijn] {
	env := buildBenchmarkEnv(depth)

	body := syn.Term[syn.DeBruijn](&syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(2)})
	for i := 3; i <= depth+1; i++ {
		body = &syn.Apply[syn.DeBruijn]{
			Function: &syn.Apply[syn.DeBruijn]{
				Function: &syn.Builtin{DefaultFunction: builtin.AddInteger},
				Argument: body,
			},
			Argument: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(i)},
		}
	}

	bodyDelay := syn.Term[syn.DeBruijn](&syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)})
	for i := 2; i <= depth; i++ {
		bodyDelay = &syn.Apply[syn.DeBruijn]{
			Function: &syn.Apply[syn.DeBruijn]{
				Function: &syn.Builtin{DefaultFunction: builtin.AddInteger},
				Argument: bodyDelay,
			},
			Argument: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(i)},
		}
	}

	var args *BuiltinArgs[syn.DeBruijn]
	args = args.Extend(benchmarkIntValue(11))
	args = args.Extend(benchmarkIntValue(29))

	return &Constr[syn.DeBruijn]{
		Tag: 0,
		Fields: []Value[syn.DeBruijn]{
			&Lambda[syn.DeBruijn]{
				AST: &syn.Lambda[syn.DeBruijn]{
					ParameterName: syn.DeBruijn(0),
					Body:          body,
				},
				Env: env,
			},
			&Delay[syn.DeBruijn]{
				AST: &syn.Delay[syn.DeBruijn]{
					Term: bodyDelay,
				},
				Env: env,
			},
			&Builtin[syn.DeBruijn]{
				Func:     builtin.AddInteger,
				ArgCount: 2,
				Args:     args,
			},
			&Constr[syn.DeBruijn]{
				Tag: 1,
				Fields: []Value[syn.DeBruijn]{
					benchmarkIntValue(7),
					benchmarkIntValue(13),
				},
			},
		},
	}
}
