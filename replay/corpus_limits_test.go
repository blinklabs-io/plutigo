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
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLoadEnforcesPreprocessingLimits(t *testing.T) {
	t.Parallel()

	base := validCorpus(successfulCase(t))
	tests := []struct {
		name      string
		mutate    func(*Corpus, *corpusDecodeLimits)
		wantError string
	}{
		{
			name: "case JSON bytes",
			mutate: func(_ *Corpus, limits *corpusDecodeLimits) {
				limits.caseJSONBytes = 1
			},
			wantError: "exceeds JSON byte limit",
		},
		{
			name: "case count before decoding next case",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				second := corpus.Cases[0]
				second.ID = "invalid-second"
				second.FlatProgramHex = "zz"
				corpus.Cases = append(corpus.Cases, second)
				limits.cases = 1
			},
			wantError: "case count exceeds preprocessing limit",
		},
		{
			name: "corpus text bytes",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				corpus.Network = "ab"
				limits.corpusTextBytes = 1
			},
			wantError: "network exceeds byte limit",
		},
		{
			name: "case text bytes",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				corpus.Cases[0].ID = "ab"
				corpus.Cases[0].Transaction.ID = "a"
				corpus.Cases[0].Transaction.Redeemer = "a"
				limits.caseTextBytes = 1
			},
			wantError: "id exceeds byte limit",
		},
		{
			name: "FLAT program bytes",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				decodedLength := hex.DecodedLen(len(corpus.Cases[0].FlatProgramHex))
				limits.flatProgramBytes = decodedLength - 1
			},
			wantError: "flat program exceeds decoded byte limit",
		},
		{
			name: "argument count",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				argument := corpus.Cases[0].ArgumentsCBORHex[0]
				corpus.Cases[0].ArgumentsCBORHex = []string{argument, argument}
				limits.argumentsPerCase = 1
			},
			wantError: "argument count 2 exceeds limit 1",
		},
		{
			name: "argument bytes",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				corpus.Cases[0].ArgumentsCBORHex = []string{"0000"}
				limits.argumentBytes = 1
			},
			wantError: "argument 0 CBOR exceeds decoded byte limit",
		},
		{
			name: "cost model parameters",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				corpus.Cases[0].CostModel = CostModel{Parameters: []int64{1, 2}}
				limits.costModelParameters = 1
			},
			wantError: "cost model parameter count 2 exceeds limit 1",
		},
		{
			name: "metadata bytes",
			mutate: func(corpus *Corpus, limits *corpusDecodeLimits) {
				corpus.Cases[0].AdditionalMetadata = json.RawMessage(`{"x":1}`)
				limits.caseMetadataBytes = 1
			},
			wantError: "metadata exceeds byte limit",
		},
		{
			name: "aggregate decoded bytes",
			mutate: func(_ *Corpus, limits *corpusDecodeLimits) {
				limits.decodedBytes = 1
			},
			wantError: "decoded data exceeds byte limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			corpus := *base
			corpus.Cases = append([]Case(nil), base.Cases...)
			limits := defaultCorpusDecodeLimits
			tt.mutate(&corpus, &limits)
			encoded, err := json.Marshal(&corpus)
			if err != nil {
				t.Fatalf("json.Marshal() failed: %v", err)
			}

			_, err = load(context.Background(), bytes.NewReader(encoded), limits)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("load() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadEnforcesInputByteLimit(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(validCorpus(successfulCase(t)))
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	t.Run("exact limit", func(t *testing.T) {
		limits := defaultCorpusDecodeLimits
		limits.jsonBytes = int64(len(encoded))
		if _, err := load(
			context.Background(),
			bytes.NewReader(encoded),
			limits,
		); err != nil {
			t.Fatalf("load() failed at the input byte limit: %v", err)
		}
	})
	t.Run("over limit", func(t *testing.T) {
		limits := defaultCorpusDecodeLimits
		limits.jsonBytes = int64(len(encoded) - 1)
		_, err := load(
			context.Background(),
			bytes.NewReader(encoded),
			limits,
		)
		if !errors.Is(err, errCorpusJSONTooLarge) {
			t.Fatalf("load() error = %v, want input byte limit error", err)
		}
	})
}

func TestLoadedCorpusReusesDecodedCases(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(validCorpus(successfulCase(t)))
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	corpus, err := Load(context.Background(), bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if corpus.cache == nil || len(corpus.cache.cases) != 1 {
		t.Fatal("Load() did not retain its decoded case")
	}

	decoded, err := corpus.validateCases(context.Background())
	if err != nil {
		t.Fatalf("validateCases() failed: %v", err)
	}
	if decoded[0].program != corpus.cache.cases[0].program {
		t.Fatal("validateCases() decoded an unchanged loaded case again")
	}

	corpus.Cases[0].FlatProgramHex = "zz"
	if err := corpus.Validate(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "decode flat program hex") {
		t.Fatalf("Validate() after mutation error = %v, want invalid hex", err)
	}
}

func TestRunCaseCancelsDuringPreprocessing(t *testing.T) {
	t.Parallel()

	replayCase := successfulCase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := RunCase(ctx, &replayCase)
	if !result.Actual.SetupError ||
		!strings.Contains(result.Actual.Error, context.Canceled.Error()) {
		t.Fatalf("RunCase() = %+v, want preprocessing cancellation", result)
	}
}
