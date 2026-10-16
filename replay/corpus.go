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

// Package replay executes normalized Cardano Plutus evaluation cases and
// compares plutigo results with a reference ledger implementation.
package replay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

const (
	SchemaVersion = 1

	maxCorpusJSONBytes       = 128 << 20
	maxCaseJSONBytes         = 8 << 20
	maxCorpusCases           = 1_000
	maxCorpusTextBytes       = 1 << 10
	maxCaseTextBytes         = 4 << 10
	maxFlatProgramBytes      = 1 << 20
	maxArgumentsPerCase      = 32
	maxArgumentBytes         = 1 << 20
	maxCostModelParameters   = 1_024
	maxCaseMetadataBytes     = 1 << 20
	maxCorpusDecodedBytes    = 64 << 20
	fingerprintCheckInterval = 64 << 10
)

var errCorpusJSONTooLarge = errors.New("replay corpus input exceeds byte limit")

type Corpus struct {
	SchemaVersion int       `json:"schema_version"`
	Network       string    `json:"network"`
	Reference     Reference `json:"reference"`
	Cases         []Case    `json:"cases"`

	cache *decodedCorpus
}

type Reference struct {
	Implementation string `json:"implementation"`
	Version        string `json:"version"`
}

type Case struct {
	ID                 string          `json:"id"`
	Transaction        TransactionRef  `json:"transaction"`
	Language           Language        `json:"language"`
	ProtocolVersion    ProtocolVersion `json:"protocol_version"`
	FlatProgramHex     string          `json:"flat_program_hex"`
	ArgumentsCBORHex   []string        `json:"arguments_cbor_hex"`
	CostModel          CostModel       `json:"cost_model"`
	BudgetLimit        ExUnits         `json:"budget_limit"`
	Expected           Expected        `json:"expected"`
	AdditionalMetadata json.RawMessage `json:"metadata,omitempty"`
}

type TransactionRef struct {
	ID       string `json:"id"`
	Slot     uint64 `json:"slot,omitempty"`
	Block    string `json:"block,omitempty"`
	Redeemer string `json:"redeemer"`
}

type ProtocolVersion struct {
	Major uint `json:"major"`
	Minor uint `json:"minor"`
}

type Language string

const (
	PlutusV1 Language = "PlutusV1"
	PlutusV2 Language = "PlutusV2"
	PlutusV3 Language = "PlutusV3"
	PlutusV4 Language = "PlutusV4"
)

func (l Language) Version() (lang.LanguageVersion, error) {
	switch l {
	case PlutusV1:
		return lang.LanguageVersionV1, nil
	case PlutusV2:
		return lang.LanguageVersionV2, nil
	case PlutusV3:
		return lang.LanguageVersionV3, nil
	case PlutusV4:
		return lang.LanguageVersionV4, nil
	default:
		return lang.LanguageVersion{}, fmt.Errorf("unsupported Plutus language %q", l)
	}
}

type CostModel struct {
	UseDefault bool    `json:"use_default,omitempty"`
	Parameters []int64 `json:"parameters,omitempty"`
}

// ExUnits uses the cardano-node field names: steps are plutigo's CPU units.
type ExUnits struct {
	Steps  int64 `json:"steps"`
	Memory int64 `json:"memory"`
}

type Expected struct {
	Success   bool           `json:"success"`
	ExUnits   ExUnits        `json:"ex_units"`
	ErrorCode *cek.ErrorCode `json:"error_code,omitempty"`
}

type decodedCase struct {
	program     *syn.Program[syn.DeBruijn]
	arguments   []data.PlutusData
	evalContext *cek.EvalContext
}

type decodedCorpus struct {
	fingerprint [sha256.Size]byte
	cases       []decodedCase
}

type corpusDecodeLimits struct {
	jsonBytes           int64
	caseJSONBytes       int
	cases               int
	corpusTextBytes     int
	caseTextBytes       int
	flatProgramBytes    int
	argumentsPerCase    int
	argumentBytes       int
	costModelParameters int
	caseMetadataBytes   int
	decodedBytes        int64
}

var defaultCorpusDecodeLimits = corpusDecodeLimits{
	jsonBytes:           maxCorpusJSONBytes,
	caseJSONBytes:       maxCaseJSONBytes,
	cases:               maxCorpusCases,
	corpusTextBytes:     maxCorpusTextBytes,
	caseTextBytes:       maxCaseTextBytes,
	flatProgramBytes:    maxFlatProgramBytes,
	argumentsPerCase:    maxArgumentsPerCase,
	argumentBytes:       maxArgumentBytes,
	costModelParameters: maxCostModelParameters,
	caseMetadataBytes:   maxCaseMetadataBytes,
	decodedBytes:        maxCorpusDecodedBytes,
}

// LoadFile loads and validates the corpus at path. Loading stops with the
// context's error once ctx is canceled.
func LoadFile(ctx context.Context, path string) (*Corpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open replay corpus: %w", err)
	}
	defer f.Close()

	corpus, err := Load(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("load replay corpus %q: %w", path, err)
	}
	return corpus, nil
}

// Load decodes and validates a bounded corpus, stopping with the context's
// error once ctx is canceled. A caller whose reader can block closes it to
// interrupt a read.
func Load(ctx context.Context, r io.Reader) (*Corpus, error) {
	return load(ctx, r, defaultCorpusDecodeLimits)
}

func load(
	ctx context.Context,
	r io.Reader,
	limits corpusDecodeLimits,
) (*Corpus, error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}
	limitedReader := &byteLimitReader{
		r:         contextReader{ctx: ctx, r: r},
		remaining: limits.jsonBytes,
	}
	decoder := json.NewDecoder(limitedReader)
	decoder.DisallowUnknownFields()

	var corpus Corpus
	decodedCases := make([]decodedCase, 0)
	ids := make(map[string]struct{})
	var decodedBytes int64
	if err := decodeCorpusObject(
		ctx,
		decoder,
		&corpus,
		&decodedCases,
		ids,
		&decodedBytes,
		limits,
	); err != nil {
		if limitedReader.exceeded {
			return nil, fmt.Errorf("decode replay corpus: %w", errCorpusJSONTooLarge)
		}
		return nil, fmt.Errorf("decode replay corpus: %w", err)
	}
	if limitedReader.exceeded {
		return nil, fmt.Errorf("decode replay corpus: %w", errCorpusJSONTooLarge)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if limitedReader.exceeded {
			return nil, fmt.Errorf(
				"decode replay corpus trailing data: %w",
				errCorpusJSONTooLarge,
			)
		}
		if err == nil {
			return nil, errors.New("decode replay corpus: multiple JSON values")
		}
		return nil, fmt.Errorf("decode replay corpus trailing data: %w", err)
	}

	if err := corpus.validateHeader(limits); err != nil {
		return nil, err
	}
	fingerprint, err := corpus.fingerprint(ctx)
	if err != nil {
		return nil, fmt.Errorf("fingerprint replay corpus: %w", err)
	}
	corpus.cache = &decodedCorpus{
		fingerprint: fingerprint,
		cases:       decodedCases,
	}
	return &corpus, nil
}

func (l corpusDecodeLimits) validate() error {
	if l.jsonBytes <= 0 || l.caseJSONBytes <= 0 || l.cases <= 0 ||
		l.corpusTextBytes <= 0 || l.caseTextBytes <= 0 ||
		l.flatProgramBytes <= 0 ||
		l.argumentsPerCase <= 0 || l.argumentBytes <= 0 ||
		l.costModelParameters <= 0 || l.caseMetadataBytes <= 0 ||
		l.decodedBytes <= 0 {
		return errors.New("replay corpus decode limits must be positive")
	}
	return nil
}

func decodeCorpusObject(
	ctx context.Context,
	decoder *json.Decoder,
	corpus *Corpus,
	decodedCases *[]decodedCase,
	ids map[string]struct{},
	decodedBytes *int64,
	limits corpusDecodeLimits,
) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return errors.New("replay corpus must be a JSON object")
	}

	seen := make(map[string]struct{}, 4)
	for decoder.More() {
		if err := ctx.Err(); err != nil {
			return err
		}
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("replay corpus field name must be a string")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate field %q", key)
		}
		seen[key] = struct{}{}

		switch key {
		case "schema_version":
			err = decoder.Decode(&corpus.SchemaVersion)
		case "network":
			err = decoder.Decode(&corpus.Network)
		case "reference":
			err = decoder.Decode(&corpus.Reference)
		case "cases":
			err = decodeCorpusCases(
				ctx,
				decoder,
				corpus,
				decodedCases,
				ids,
				decodedBytes,
				limits,
			)
		default:
			return fmt.Errorf("json: unknown field %q", key)
		}
		if err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}

func decodeCorpusCases(
	ctx context.Context,
	decoder *json.Decoder,
	corpus *Corpus,
	decodedCases *[]decodedCase,
	ids map[string]struct{},
	decodedBytes *int64,
	limits corpusDecodeLimits,
) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return errors.New("cases must be a JSON array")
	}
	for decoder.More() {
		caseIndex := len(corpus.Cases)
		if caseIndex >= limits.cases {
			return fmt.Errorf(
				"case count exceeds preprocessing limit %d",
				limits.cases,
			)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var rawCase json.RawMessage
		if err := decoder.Decode(&rawCase); err != nil {
			return fmt.Errorf("case %d: %w", caseIndex, err)
		}
		if len(rawCase) > limits.caseJSONBytes {
			return fmt.Errorf(
				"case %d exceeds JSON byte limit %d",
				caseIndex,
				limits.caseJSONBytes,
			)
		}
		var replayCase Case
		caseDecoder := json.NewDecoder(bytes.NewReader(rawCase))
		caseDecoder.DisallowUnknownFields()
		if err := caseDecoder.Decode(&replayCase); err != nil {
			return fmt.Errorf("case %d: %w", caseIndex, err)
		}
		if _, exists := ids[replayCase.ID]; exists {
			return fmt.Errorf("case %d: duplicate id %q", caseIndex, replayCase.ID)
		}
		decoded, err := replayCase.decode(ctx, decodedBytes, limits)
		if err != nil {
			return fmt.Errorf("case %d: %w", caseIndex, err)
		}
		ids[replayCase.ID] = struct{}{}
		corpus.Cases = append(corpus.Cases, replayCase)
		*decodedCases = append(*decodedCases, decoded)
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}

// Validate checks the corpus, including that every case's cost model builds
// into a usable evaluation context. It checks ctx throughout preprocessing.
func (c *Corpus) Validate(ctx context.Context) error {
	_, err := c.validateCases(ctx)
	return err
}

func (c *Corpus) validateCases(ctx context.Context) ([]decodedCase, error) {
	if c == nil {
		return nil, errors.New("replay corpus is required")
	}
	limits := defaultCorpusDecodeLimits
	if err := c.validateBounds(ctx, limits); err != nil {
		return nil, err
	}
	fingerprint, err := c.fingerprint(ctx)
	if err != nil {
		return nil, fmt.Errorf("fingerprint replay corpus: %w", err)
	}
	if c.cache != nil && c.cache.fingerprint == fingerprint {
		return c.cache.cases, nil
	}

	ids := make(map[string]struct{}, len(c.Cases))
	decodedCases := make([]decodedCase, len(c.Cases))
	for i := range c.Cases {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("validate replay corpus: %w", err)
		}
		replayCase := &c.Cases[i]
		decoded, err := replayCase.decodeChecked(ctx)
		if err != nil {
			return nil, fmt.Errorf("replay case %d: %w", i, err)
		}
		if _, exists := ids[replayCase.ID]; exists {
			return nil, fmt.Errorf(
				"replay case %d: duplicate id %q",
				i,
				replayCase.ID,
			)
		}
		ids[replayCase.ID] = struct{}{}
		decodedCases[i] = decoded
	}
	return decodedCases, nil
}

func (c *Corpus) validateHeader(limits corpusDecodeLimits) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf(
			"unsupported replay corpus schema version %d (want %d)",
			c.SchemaVersion,
			SchemaVersion,
		)
	}
	if strings.TrimSpace(c.Network) == "" {
		return errors.New("replay corpus network is required")
	}
	if strings.TrimSpace(c.Reference.Implementation) == "" {
		return errors.New(
			"replay corpus reference implementation is required",
		)
	}
	if strings.TrimSpace(c.Reference.Version) == "" {
		return errors.New("replay corpus reference version is required")
	}
	if len(c.Cases) == 0 {
		return errors.New("replay corpus must contain at least one case")
	}
	if len(c.Cases) > limits.cases {
		return fmt.Errorf(
			"replay corpus has %d cases, exceeds preprocessing limit %d",
			len(c.Cases),
			limits.cases,
		)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "network", value: c.Network},
		{name: "reference implementation", value: c.Reference.Implementation},
		{name: "reference version", value: c.Reference.Version},
	} {
		if len(field.value) > limits.corpusTextBytes {
			return fmt.Errorf(
				"replay corpus %s exceeds byte limit",
				field.name,
			)
		}
	}
	return nil
}

func (c *Corpus) validateBounds(
	ctx context.Context,
	limits corpusDecodeLimits,
) error {
	if err := c.validateHeader(limits); err != nil {
		return err
	}
	var decodedBytes int64
	for i := range c.Cases {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("validate replay corpus bounds: %w", err)
		}
		if err := c.Cases[i].validateBounds(
			ctx,
			&decodedBytes,
			limits,
		); err != nil {
			return fmt.Errorf("replay case %d: %w", i, err)
		}
	}
	return nil
}

func (c *Case) validateBounds(
	ctx context.Context,
	decodedBytes *int64,
	limits corpusDecodeLimits,
) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "id", value: c.ID},
		{name: "transaction id", value: c.Transaction.ID},
		{name: "transaction block", value: c.Transaction.Block},
		{name: "transaction redeemer", value: c.Transaction.Redeemer},
		{name: "language", value: string(c.Language)},
	} {
		if len(field.value) > limits.caseTextBytes {
			return fmt.Errorf("%s exceeds byte limit", field.name)
		}
		if err := addDecodedBytes(
			decodedBytes,
			int64(len(field.value)),
			limits.decodedBytes,
		); err != nil {
			return err
		}
	}

	programBytes, err := boundedHexSize(
		"flat program",
		c.FlatProgramHex,
		limits.flatProgramBytes,
	)
	if err != nil {
		return err
	}
	if err := addDecodedBytes(
		decodedBytes,
		int64(programBytes),
		limits.decodedBytes,
	); err != nil {
		return err
	}
	if len(c.ArgumentsCBORHex) > limits.argumentsPerCase {
		return fmt.Errorf(
			"argument count %d exceeds limit %d",
			len(c.ArgumentsCBORHex),
			limits.argumentsPerCase,
		)
	}
	for i, argument := range c.ArgumentsCBORHex {
		if err := ctx.Err(); err != nil {
			return err
		}
		argumentBytes, err := boundedHexSize(
			fmt.Sprintf("argument %d CBOR", i),
			argument,
			limits.argumentBytes,
		)
		if err != nil {
			return err
		}
		if err := addDecodedBytes(
			decodedBytes,
			int64(argumentBytes),
			limits.decodedBytes,
		); err != nil {
			return err
		}
	}
	if len(c.CostModel.Parameters) > limits.costModelParameters {
		return fmt.Errorf(
			"cost model parameter count %d exceeds limit %d",
			len(c.CostModel.Parameters),
			limits.costModelParameters,
		)
	}
	if err := addDecodedBytes(
		decodedBytes,
		int64(len(c.CostModel.Parameters))*8,
		limits.decodedBytes,
	); err != nil {
		return err
	}
	if len(c.AdditionalMetadata) > limits.caseMetadataBytes {
		return errors.New("metadata exceeds byte limit")
	}
	return addDecodedBytes(
		decodedBytes,
		int64(len(c.AdditionalMetadata)),
		limits.decodedBytes,
	)
}

func boundedHexSize(name, value string, limit int) (int, error) {
	if len(value) > 2*limit+2 {
		return 0, fmt.Errorf("%s exceeds decoded byte limit %d", name, limit)
	}
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	if len(value) > 2*limit {
		return 0, fmt.Errorf("%s exceeds decoded byte limit %d", name, limit)
	}
	if len(value)%2 != 0 {
		return 0, fmt.Errorf("decode %s hex: odd length", name)
	}
	return hex.DecodedLen(len(value)), nil
}

func addDecodedBytes(total *int64, amount, limit int64) error {
	if amount < 0 || amount > limit-*total {
		return fmt.Errorf("replay corpus decoded data exceeds byte limit %d", limit)
	}
	*total += amount
	return nil
}

func (c *Corpus) fingerprint(ctx context.Context) ([sha256.Size]byte, error) {
	f := corpusFingerprinter{ctx: ctx, hash: sha256.New()}
	f.writeInt64(int64(c.SchemaVersion))
	if err := f.writeString(c.Network); err != nil {
		return [sha256.Size]byte{}, err
	}
	if err := f.writeString(c.Reference.Implementation); err != nil {
		return [sha256.Size]byte{}, err
	}
	if err := f.writeString(c.Reference.Version); err != nil {
		return [sha256.Size]byte{}, err
	}
	f.writeInt64(int64(len(c.Cases)))
	for i := range c.Cases {
		if err := f.writeCase(&c.Cases[i]); err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	var result [sha256.Size]byte
	copy(result[:], f.hash.Sum(nil))
	return result, nil
}

type corpusFingerprinter struct {
	ctx  context.Context
	hash hash.Hash
}

func (f *corpusFingerprinter) writeCase(c *Case) error {
	for _, value := range []string{
		c.ID,
		c.Transaction.ID,
		c.Transaction.Block,
		c.Transaction.Redeemer,
		string(c.Language),
		c.FlatProgramHex,
	} {
		if err := f.writeString(value); err != nil {
			return err
		}
	}
	f.writeUint64(c.Transaction.Slot)
	f.writeUint64(uint64(c.ProtocolVersion.Major))
	f.writeUint64(uint64(c.ProtocolVersion.Minor))
	f.writeInt64(int64(len(c.ArgumentsCBORHex)))
	for _, argument := range c.ArgumentsCBORHex {
		if err := f.writeString(argument); err != nil {
			return err
		}
	}
	f.writeBool(c.CostModel.UseDefault)
	f.writeInt64(int64(len(c.CostModel.Parameters)))
	for _, parameter := range c.CostModel.Parameters {
		f.writeInt64(parameter)
	}
	f.writeInt64(c.BudgetLimit.Steps)
	f.writeInt64(c.BudgetLimit.Memory)
	f.writeBool(c.Expected.Success)
	f.writeInt64(c.Expected.ExUnits.Steps)
	f.writeInt64(c.Expected.ExUnits.Memory)
	if c.Expected.ErrorCode == nil {
		f.writeBool(false)
	} else {
		f.writeBool(true)
		f.writeInt64(int64(*c.Expected.ErrorCode))
	}
	return f.writeBytes(c.AdditionalMetadata)
}

func (f *corpusFingerprinter) writeString(value string) error {
	f.writeInt64(int64(len(value)))
	for len(value) > 0 {
		if err := f.ctx.Err(); err != nil {
			return err
		}
		chunkSize := min(len(value), fingerprintCheckInterval)
		_, _ = f.hash.Write([]byte(value[:chunkSize]))
		value = value[chunkSize:]
	}
	return nil
}

func (f *corpusFingerprinter) writeBytes(value []byte) error {
	f.writeInt64(int64(len(value)))
	for len(value) > 0 {
		if err := f.ctx.Err(); err != nil {
			return err
		}
		chunkSize := min(len(value), fingerprintCheckInterval)
		_, _ = f.hash.Write(value[:chunkSize])
		value = value[chunkSize:]
	}
	return nil
}

func (f *corpusFingerprinter) writeBool(value bool) {
	if value {
		_, _ = f.hash.Write([]byte{1})
	} else {
		_, _ = f.hash.Write([]byte{0})
	}
}

func (f *corpusFingerprinter) writeInt64(value int64) {
	f.writeUint64(uint64(value))
}

func (f *corpusFingerprinter) writeUint64(value uint64) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	_, _ = f.hash.Write(encoded[:])
}

func (c *Case) validate() (decodedCase, error) {
	var decodedBytes int64
	return c.decode(
		context.Background(),
		&decodedBytes,
		defaultCorpusDecodeLimits,
	)
}

func (c *Case) decode(
	ctx context.Context,
	decodedBytes *int64,
	limits corpusDecodeLimits,
) (decodedCase, error) {
	if err := ctx.Err(); err != nil {
		return decodedCase{}, err
	}
	if err := c.validateBounds(ctx, decodedBytes, limits); err != nil {
		return decodedCase{}, err
	}
	return c.decodeChecked(ctx)
}

func (c *Case) decodeChecked(ctx context.Context) (decodedCase, error) {
	if err := ctx.Err(); err != nil {
		return decodedCase{}, err
	}
	if strings.TrimSpace(c.ID) == "" {
		return decodedCase{}, errors.New("id is required")
	}
	if strings.TrimSpace(c.Transaction.ID) == "" {
		return decodedCase{}, errors.New("transaction id is required")
	}
	if strings.TrimSpace(c.Transaction.Redeemer) == "" {
		return decodedCase{}, errors.New("transaction redeemer is required")
	}
	if c.ProtocolVersion.Major == 0 {
		return decodedCase{}, errors.New("protocol major version must be positive")
	}
	languageVersion, err := c.Language.Version()
	if err != nil {
		return decodedCase{}, err
	}
	flatProgram, err := decodeHex("flat program", c.FlatProgramHex)
	if err != nil {
		return decodedCase{}, err
	}
	if err := ctx.Err(); err != nil {
		return decodedCase{}, err
	}
	programContext := syn.ProgramContext{
		LedgerLanguage: languageVersion,
		ProtocolMajor:  c.ProtocolVersion.Major,
	}
	program, err := syn.DecodeDeBruijnWithContext(flatProgram, programContext)
	if err != nil {
		return decodedCase{}, fmt.Errorf("decode FLAT program: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return decodedCase{}, err
	}
	// Every replay case is executed through the CEK machine below (evaluate,
	// in runner.go), unlike a transaction's own stored-but-unexecuted
	// reference-script output. DecodeDeBruijnWithContext only performs
	// phase-1 well-formedness checks, so the phase-2, execution-time term
	// version legality gate must be checked separately here, before this
	// program is ever handed to the machine.
	if err := syn.ValidateTermVersionForExecution(program.Version, programContext); err != nil {
		return decodedCase{}, fmt.Errorf("validate term version for execution: %w", err)
	}
	arguments := make([]data.PlutusData, 0, len(c.ArgumentsCBORHex))
	for i, arg := range c.ArgumentsCBORHex {
		if err := ctx.Err(); err != nil {
			return decodedCase{}, err
		}
		argCBOR, err := decodeHex(fmt.Sprintf("argument %d CBOR", i), arg)
		if err != nil {
			return decodedCase{}, err
		}
		decodedArgument, err := data.Decode(argCBOR)
		if err != nil {
			return decodedCase{}, fmt.Errorf(
				"decode argument %d PlutusData: %w",
				i,
				err,
			)
		}
		arguments = append(arguments, decodedArgument)
	}
	if err := ctx.Err(); err != nil {
		return decodedCase{}, err
	}
	if c.CostModel.UseDefault == (len(c.CostModel.Parameters) > 0) {
		return decodedCase{}, errors.New(
			"cost model must select exactly one of use_default or parameters",
		)
	}
	if c.BudgetLimit.Steps <= 0 || c.BudgetLimit.Memory <= 0 {
		return decodedCase{}, errors.New(
			"budget limit steps and memory must be positive",
		)
	}
	if c.Expected.ExUnits.Steps < 0 || c.Expected.ExUnits.Memory < 0 {
		return decodedCase{}, errors.New(
			"expected execution units cannot be negative",
		)
	}
	if c.Expected.Success && c.Expected.ErrorCode != nil {
		return decodedCase{}, errors.New(
			"successful expected result cannot include an error code",
		)
	}
	if c.Expected.ExUnits.Steps > c.BudgetLimit.Steps ||
		c.Expected.ExUnits.Memory > c.BudgetLimit.Memory {
		return decodedCase{}, errors.New(
			"expected execution units exceed the budget limit",
		)
	}
	protoVersion := cek.ProtoVersion{
		Major: c.ProtocolVersion.Major,
		Minor: c.ProtocolVersion.Minor,
	}
	var evalContext *cek.EvalContext
	if c.CostModel.UseDefault {
		evalContext = cek.NewDefaultEvalContext(languageVersion, protoVersion)
	} else {
		evalContext, err = cek.NewEvalContext(
			languageVersion,
			protoVersion,
			c.CostModel.Parameters,
		)
		if err != nil {
			return decodedCase{}, fmt.Errorf("build evaluation context: %w", err)
		}
	}
	return decodedCase{
		program:     program,
		arguments:   arguments,
		evalContext: evalContext,
	}, nil
}

func decodeHex(name, value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	if value == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode %s hex: %w", name, err)
	}
	return decoded, nil
}

// contextReader stops reading once its context is canceled.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

type byteLimitReader struct {
	r         io.Reader
	remaining int64
	exceeded  bool
}

func (r *byteLimitReader) Read(p []byte) (int, error) {
	if int64(len(p)) < r.remaining {
		n, err := r.r.Read(p)
		r.remaining -= int64(n)
		return n, err
	}

	probe := make([]byte, int(r.remaining)+1)
	n, err := r.r.Read(probe)
	allowed := min(n, int(r.remaining))
	copy(p, probe[:allowed])
	r.remaining -= int64(allowed)
	if n > allowed {
		r.exceeded = true
		return allowed, errCorpusJSONTooLarge
	}
	return allowed, err
}
