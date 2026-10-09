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
	"encoding/json"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
)

func TestCorpusValidateRejectsSuccessfulResultWithErrorCode(t *testing.T) {
	replayCase := successfulCase(t)
	errorCode := cek.ErrCodeExplicitError
	replayCase.Expected.ErrorCode = &errorCode

	corpus := validCorpus(replayCase)
	err := corpus.Validate(context.Background())
	if err == nil || !strings.Contains(
		err.Error(),
		"successful expected result cannot include an error code",
	) {
		t.Fatalf("Validate() error = %v, want contradictory-result error", err)
	}
}

func TestCorpusValidateRejectsMalformedEncodedPayloads(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Case)
		wantError string
	}{
		{
			name: "FLAT program",
			mutate: func(replayCase *Case) {
				replayCase.FlatProgramHex = "00"
			},
			wantError: "decode FLAT program",
		},
		{
			name: "PlutusData argument",
			mutate: func(replayCase *Case) {
				replayCase.ArgumentsCBORHex = []string{"ff"}
			},
			wantError: "decode argument 0 PlutusData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayCase := successfulCase(t)
			tt.mutate(&replayCase)

			corpus := validCorpus(replayCase)
			encoded, err := json.Marshal(corpus)
			if err != nil {
				t.Fatalf("json.Marshal() failed: %v", err)
			}
			_, err = Load(context.Background(), bytes.NewReader(encoded))
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Load(context.Background(), ) error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func FuzzLoadEncodedPayloads(f *testing.F) {
	replayCase := successfulCase(f)
	f.Add(replayCase.FlatProgramHex, replayCase.ArgumentsCBORHex[0])
	f.Add("00", "ff")
	f.Add("not-hex", "0")

	f.Fuzz(func(t *testing.T, flatProgramHex, argumentCBORHex string) {
		testCase := replayCase
		testCase.FlatProgramHex = flatProgramHex
		testCase.ArgumentsCBORHex = []string{argumentCBORHex}
		encoded, err := json.Marshal(validCorpus(testCase))
		if err != nil {
			t.Fatalf("json.Marshal() failed: %v", err)
		}

		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("Load(context.Background(), ) panicked: %v", recovered)
			}
		}()
		_, _ = Load(context.Background(), bytes.NewReader(encoded))
	})
}

func validCorpus(replayCase Case) *Corpus {
	return &Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{replayCase},
	}
}
