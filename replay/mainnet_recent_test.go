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

package replay

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// recentMainnetCorpusGlob matches the corpus of every validator run in 150
// consecutive mainnet blocks, split into files under the per-corpus case
// limit.
const recentMainnetCorpusGlob = "testdata/mainnet-2026-11-03-*.json.gz"

// recentMainnetCorpusCases is the number of cases across those files.
const recentMainnetCorpusCases = 1360

func loadRecentMainnetCorpora(tb testing.TB) []*Corpus {
	tb.Helper()
	paths, err := filepath.Glob(recentMainnetCorpusGlob)
	if err != nil {
		tb.Fatal(err)
	}
	if len(paths) == 0 {
		tb.Fatalf("no corpus matches %s", recentMainnetCorpusGlob)
	}
	corpora := make([]*Corpus, 0, len(paths))
	for _, path := range paths {
		corpus, err := LoadFile(context.Background(), path)
		if err != nil {
			tb.Fatalf("LoadFile(%s) failed: %v", path, err)
		}
		corpora = append(corpora, corpus)
	}
	return corpora
}

// TestRecentMainnetCorpusParity requires plutigo to reproduce cardano-node's
// outcome and exact ExUnits for every validator in the recent mainnet
// corpus.
func TestRecentMainnetCorpusParity(t *testing.T) {
	total := 0
	for _, corpus := range loadRecentMainnetCorpora(t) {
		report, err := Run(context.Background(), corpus, DefaultLimits())
		if err != nil {
			t.Fatalf("Run() failed: %v", err)
		}
		total += report.Summary.Total
		for _, result := range report.Cases {
			if !result.Passed {
				t.Errorf("%s: mismatches: %v", result.ID, result.Mismatches)
			}
		}
	}
	if total != recentMainnetCorpusCases {
		t.Fatalf("recent mainnet corpus has %d cases, want %d", total, recentMainnetCorpusCases)
	}
}

func BenchmarkRecentMainnetCorpus(b *testing.B) {
	corpora := loadRecentMainnetCorpora(b)
	cases := 0
	for _, corpus := range corpora {
		cases += len(corpus.Cases)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, corpus := range corpora {
			report, err := Run(context.Background(), corpus, DefaultLimits())
			if err != nil {
				b.Fatalf("Run() failed: %v", err)
			}
			if report.Summary.Passed != report.Summary.Total {
				b.Fatalf("parity passed %d/%d cases", report.Summary.Passed, report.Summary.Total)
			}
		}
	}
	b.ReportMetric(float64(cases), "validators/op")
	b.ReportMetric(
		float64(b.Elapsed().Nanoseconds())/float64(b.N*cases),
		"ns/validator",
	)
}

// TestLoadFileDecompressesGzip checks that a ".gz" corpus loads to the
// same corpus as its uncompressed bytes.
func TestLoadFileDecompressesGzip(t *testing.T) {
	plain, err := os.ReadFile(mainnetCorpusPath)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := io.Copy(zw, bytes.NewReader(plain)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "mainnet.json.gz")
	if err := os.WriteFile(path, compressed.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(context.Background(), path)
	if err != nil {
		t.Fatalf("LoadFile(.gz) failed: %v", err)
	}
	want, err := LoadFile(context.Background(), mainnetCorpusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("gzip corpus differs from the uncompressed corpus")
	}

	// A file named .gz that is not gzip fails to load.
	bad := filepath.Join(t.TempDir(), "plain.json.gz")
	if err := os.WriteFile(bad, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(context.Background(), bad); err == nil {
		t.Fatal("LoadFile accepted a .gz path holding plain JSON")
	}
}

// TestLoadFileCanceledBeforeGzipHeader checks that cancellation stops a
// malformed ".gz" corpus while its header is read.
func TestLoadFileCanceledBeforeGzipHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json.gz")
	if err := os.WriteFile(path, []byte("not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadFile(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadFile() error = %v, want context.Canceled", err)
	}
}
