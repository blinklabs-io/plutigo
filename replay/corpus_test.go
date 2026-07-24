package replay

import (
	"bytes"
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
	err := corpus.Validate()
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
			_, err = Load(bytes.NewReader(encoded))
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantError)
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
				t.Fatalf("Load() panicked: %v", recovered)
			}
		}()
		_, _ = Load(bytes.NewReader(encoded))
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
