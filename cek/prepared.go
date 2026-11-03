// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cek

import (
	"context"
	"errors"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// PreparedProgram is a decoded UPLC program whose structure has been checked
// once, so that it can be evaluated many times without the per-run structure
// and scope walk that [Machine.Run] and its variants perform.
//
// A PreparedProgram is immutable and safe to share between goroutines and
// machines. Its term is private, so the program cannot be changed through it
// after it has been checked, which is what makes skipping the walk sound.
//
// Preparing never fails because of the structure or scope check. A program
// that does not pass it is still prepared, and every evaluation then runs the
// full walk, so it fails at evaluation time with the same error, and the same
// budget, as [Machine.RunScriptContext] reports for it.
type PreparedProgram struct {
	term    syn.Term[syn.DeBruijn]
	version lang.LanguageVersion
	// validated reports that term passed validateTermStructure, in Full
	// scope mode when fullScope is set and in NoCaseConstr mode otherwise,
	// using usage of the structure limits.
	validated bool
	fullScope bool
	usage     termStructureUsage
	// retainedBytes estimates the memory the program keeps alive; zero
	// when unknown.
	retainedBytes int64
}

// PrepareProgram decodes a FLAT-encoded De Bruijn program for the given
// ledger context, like [syn.DecodeDeBruijnWithContext], and prepares it. It
// checks the program's structure as [PrepareDecodedProgram] does, but in the
// same pass as the decode, using [syn.DecodeDeBruijnWithContextInfo] in the
// scope mode of context.ProtocolMajor rather than walking the decoded term
// again. The returned program shares the decoder's read-only leaf nodes.
//
// It does not apply the execution-time version gate: ledger callers must
// still call [syn.ValidateTermVersionForExecution] with [PreparedProgram.Version]
// before evaluating the program.
func PrepareProgram(flat []byte, context syn.ProgramContext) (*PreparedProgram, error) {
	fullScope := fullScopeCheckFor(context.ProtocolMajor)
	program, info, err := syn.DecodeDeBruijnWithContextInfo(flat, context, fullScope)
	if err != nil {
		return nil, err
	}
	usage, validated := decodedStructureUsage(info)
	return &PreparedProgram{
		term:          program.Term,
		version:       program.Version,
		validated:     validated,
		fullScope:     fullScope,
		usage:         usage,
		retainedBytes: info.RetainedBytes + info.RetainedBytes/8 + preparedProgramOverhead,
	}, nil
}

// Besides the storage syn.DecodeInfo.RetainedBytes counts, a prepared
// program keeps the PreparedProgram and its program, and allocator
// size-class rounding. The eighth and preparedProgramOverhead cover them.
const preparedProgramOverhead = 2048

// decodedStructureUsage is the outcome of validateTermStructureUsage for a
// program the decoder produced, computed from what the decoder recorded
// rather than by walking the term. A decoded term has no nil, cyclic or
// unknown nodes, so the walk can fail only on scope, depth, its work limit or
// a constant. Its work is one unit per term node plus the work of each
// constant, which the constant walk itself measures.
func decodedStructureUsage(info *syn.DecodeInfo) (termStructureUsage, bool) {
	if info.ScopeError != nil || info.MaxDepth > maxDischargeDepth ||
		info.Terms > maxDischargeWork {
		return termStructureUsage{}, false
	}
	budget := structureValidationBudget{
		ctx:       contextBackground,
		remaining: maxDischargeWork - info.Terms,
	}
	for _, constant := range info.Constants {
		if err := validateConstantStructureWithBudget(constant.Con, &budget); err != nil {
			return termStructureUsage{}, false
		}
	}
	return termStructureUsage{
		work:     maxDischargeWork - budget.remaining,
		maxDepth: int32(info.MaxDepth),
	}, true
}

// PrepareDecodedProgram checks the structure of a program decoded for context
// once, in the scope mode of context.ProtocolMajor, and returns it prepared.
//
// It takes ownership of program: the program must come from a one-shot
// decode (not a reused [syn.DeBruijnDecoder], whose next decode overwrites
// it), and nothing may modify its term afterwards. That is the same rule
// that already applies to sharing one decoded term between machines.
func PrepareDecodedProgram(
	program *syn.Program[syn.DeBruijn],
	context syn.ProgramContext,
) (*PreparedProgram, error) {
	if program == nil || program.Term == nil {
		return nil, errors.New("program is required")
	}
	fullScope := fullScopeCheckFor(context.ProtocolMajor)
	var usage termStructureUsage
	validated := validateTermStructureUsage[syn.DeBruijn](
		contextBackground,
		false,
		program.Term,
		nil,
		fullScope,
		&usage,
	) == nil
	return &PreparedProgram{
		term:      program.Term,
		version:   program.Version,
		validated: validated,
		fullScope: fullScope,
		usage:     usage,
	}, nil
}

// RetainedBytes estimates the memory a program prepared by [PrepareProgram]
// keeps alive: the decode storage of its nodes and constants, plus an eighth
// and a fixed overhead for allocator rounding and bookkeeping. It returns 0
// for a program prepared by [PrepareDecodedProgram], whose storage it cannot
// see.
func (p *PreparedProgram) RetainedBytes() int64 {
	return p.retainedBytes
}

// Version returns the program's UPLC version.
func (p *PreparedProgram) Version() lang.LanguageVersion {
	return p.version
}

// RunPreparedScriptContext evaluates a prepared ledger script exactly like
// [Machine.RunScriptContext] evaluates the program's term: it applies the
// trusted data arguments, evaluates without discharging the result, and
// reports whether the result is the unit constant. The outcome, error and
// remaining budget are the same; only the structure and scope walk of the
// program is skipped when the program passed it in a scope mode at least as
// strict as the machine's.
func RunPreparedScriptContext(
	ctx context.Context,
	machine *Machine[syn.DeBruijn],
	program *PreparedProgram,
	arguments ...data.PlutusData,
) (resultIsUnit bool, err error) {
	if machine == nil {
		return false, errors.New("machine is required")
	}
	if program == nil {
		return false, errors.New("prepared program is required")
	}
	if ctx == nil {
		ctx = contextBackground
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// Full mode checks every position NoCaseConstr mode checks, so a program
	// that passed in Full mode is valid in either. The walk RunScriptContext
	// performs also covers the argument application chain, which nests the
	// program one level deeper per argument and adds three nodes per argument
	// (the application, its argument slot and the data constant), so the
	// program is walked again whenever the chain would exceed a limit the
	// program alone stayed within.
	skip := program.validated &&
		(program.fullScope || !fullScopeCheckFor(machine.protoMajor)) &&
		len(arguments) <= maxDischargeDepth-int(program.usage.maxDepth) &&
		len(arguments) <= (maxDischargeWork-program.usage.work)/3
	machine.skipDischarge = true
	machine.resultIsUnit = false
	defer func() {
		machine.skipDischarge = false
		machine.resultIsUnit = false
	}()
	if _, err := machine.runWithTrustedDataArguments(ctx, program.term, skip, arguments); err != nil {
		return false, err
	}
	return machine.resultIsUnit, nil
}

var contextBackground = context.Background()

// fullScopeCheckFor reports whether a protocol major version selects the Full
// scope-check mode. A zero major selects no protocol version and so the
// current (Full) mode.
func fullScopeCheckFor(protoMajor uint) bool {
	return protoMajor == 0 || protoMajor >= dijkstraProtoMajorVersion
}
