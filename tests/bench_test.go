package tests

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

func BenchmarkFlatFiles(b *testing.B) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("Could not get caller information")
	}

	testRoot := filepath.Join(filepath.Dir(filename), "bench")
	testRoot = filepath.Clean(testRoot)

	if _, err := os.Stat(testRoot); os.IsNotExist(err) {
		b.Fatalf("Test directory not found: %s", testRoot)
	}

	// Get list of files in dataDir.
	files, err := os.ReadDir(testRoot)
	if err != nil {
		b.Fatalf("failed to read directory %s: %v", testRoot, err)
	}

	for _, file := range files {
		// Skip directories, only process files.
		if file.IsDir() {
			continue
		}

		// Get file path and name.
		filePath := filepath.Join(testRoot, file.Name())

		// Use file name (without extension) as benchmark name.
		benchmarkName := strings.TrimSuffix(
			file.Name(),
			filepath.Ext(file.Name()),
		)

		// Load file contents before benchmark.
		content, err := loadFile(filePath)
		if err != nil {
			b.Fatalf("failed to load file %s: %v", filePath, err)
		}

		// Register a sub-benchmark for this file.
		b.Run(benchmarkName, func(b *testing.B) {

			for b.Loop() {
				program, err := syn.Decode[syn.DeBruijn](content)
				if err != nil {
					log.Fatalf("decode error: %v\n\n", err)
				}

				machine := cek.NewMachine[syn.DeBruijn](program.Version, 200, nil)

				_, err = machine.Run(program.Term)
				if err != nil {
					log.Fatalf("eval error: %v\n\n", err)
				}
			}
		})
	}
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
