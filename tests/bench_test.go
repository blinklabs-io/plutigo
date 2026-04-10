package tests

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func BenchmarkFlatFiles(b *testing.B) {
	benchFlatFiles(b, true, true)
}

func BenchmarkFlatFilesFresh(b *testing.B) {
	benchFlatFilesFresh(b, true, true)
}

func BenchmarkFlatFilesDecode(b *testing.B) {
	benchFlatFiles(b, true, false)
}

func BenchmarkFlatFilesDecodeFresh(b *testing.B) {
	benchFlatFilesFresh(b, true, false)
}

func BenchmarkFlatFilesEval(b *testing.B) {
	benchFlatFiles(b, false, true)
}

func BenchmarkFlatFilesEvalFresh(b *testing.B) {
	benchFlatFilesFresh(b, false, true)
}

func benchFlatFiles(b *testing.B, includeDecode, includeEval bool) {
	forEachBenchFile(b, func(b *testing.B, benchmarkName string, content []byte) {
		var decodedProgram *syn.Program[syn.DeBruijn]
		if !includeDecode || includeEval {
			var err error
			decodedProgram, err = syn.Decode[syn.DeBruijn](content)
			if err != nil {
				b.Fatalf("decode error: %v", err)
			}
		}

		var (
			programVersion lang.LanguageVersion
			evalContext    *cek.EvalContext
		)
		if includeEval {
			programVersion = benchmarkProgramVersion(b, decodedProgram)
			evalContext = benchmarkEvalContext(programVersion)
		}

		// Register a sub-benchmark for this file.
		b.Run(benchmarkName, func(b *testing.B) {
			var machine *cek.Machine[syn.DeBruijn]
			var decoder *syn.DeBruijnDecoder
			if includeEval {
				// Reuse one machine per file so eval benchmarks measure steady-state
				// execution, while decode benchmarks still decode inside the loop.
				machine = cek.NewMachine[syn.DeBruijn](
					programVersion,
					0,
					evalContext,
				)
			}
			if includeDecode {
				decoder = syn.NewDeBruijnDecoder()
			}

			for b.Loop() {
				program := decodedProgram
				if includeDecode {
					var decodeErr error
					program, decodeErr = decoder.Decode(content)
					if decodeErr != nil {
						b.Fatalf("decode error: %v", decodeErr)
					}
				}

				if includeEval {
					// Use an eval context with protocol major 200 so benchmark
					// coverage includes every builtin without availability gating.
					_, err := machine.Run(program.Term)
					if err != nil {
						b.Fatalf("eval error: %v", err)
					}
				}
			}
		})
	})
}

func benchFlatFilesFresh(b *testing.B, includeDecode, includeEval bool) {
	forEachBenchFile(b, func(b *testing.B, benchmarkName string, content []byte) {
		var (
			decodedProgram *syn.Program[syn.DeBruijn]
			programVersion lang.LanguageVersion
			evalContext    *cek.EvalContext
		)
		if includeEval {
			var err error
			decodedProgram, err = syn.Decode[syn.DeBruijn](content)
			if err != nil {
				b.Fatalf("decode error: %v", err)
			}
			programVersion = benchmarkProgramVersion(b, decodedProgram)
			evalContext = benchmarkEvalContext(programVersion)
		}

		b.Run(benchmarkName, func(b *testing.B) {
			for b.Loop() {
				program := decodedProgram
				if includeDecode {
					var decodeErr error
					program, decodeErr = syn.Decode[syn.DeBruijn](content)
					if decodeErr != nil {
						b.Fatalf("decode error: %v", decodeErr)
					}
				}

				if includeEval {
					machine := cek.NewMachine[syn.DeBruijn](
						programVersion,
						0,
						evalContext,
					)
					_, err := machine.Run(program.Term)
					if err != nil {
						b.Fatalf("eval error: %v", err)
					}
				}
			}
		})
	})
}

func forEachBenchFile(
	b *testing.B,
	fn func(b *testing.B, benchmarkName string, content []byte),
) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("Could not get caller information")
	}

	testRoot := filepath.Join(filepath.Dir(filename), "bench")
	testRoot = filepath.Clean(testRoot)

	if _, err := os.Stat(testRoot); os.IsNotExist(err) {
		b.Fatalf("Test directory not found: %s", testRoot)
	}

	files, err := os.ReadDir(testRoot)
	if err != nil {
		b.Fatalf("failed to read directory %s: %v", testRoot, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(testRoot, file.Name())
		benchmarkName := strings.TrimSuffix(
			file.Name(),
			filepath.Ext(file.Name()),
		)

		content, err := loadFile(filePath)
		if err != nil {
			b.Fatalf("failed to load file %s: %v", filePath, err)
		}
		fn(b, benchmarkName, content)
	}
}

func benchmarkEvalContext(version lang.LanguageVersion) *cek.EvalContext {
	return cek.NewDefaultEvalContext(version, cek.ProtoVersion{Major: 200})
}

func benchmarkProgramVersion(
	b *testing.B,
	program *syn.Program[syn.DeBruijn],
) lang.LanguageVersion {
	b.Helper()

	if program == nil {
		b.Fatal("decoded program is required when includeEval is true")
	}
	return program.Version
}

// loadFile reads the entire file into a byte slice.
func loadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}
