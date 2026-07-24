// Package replay executes normalized Cardano Plutus evaluation cases and
// compares plutigo results with a reference ledger implementation.
package replay

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

const SchemaVersion = 1

type Corpus struct {
	SchemaVersion int       `json:"schema_version"`
	Network       string    `json:"network"`
	Reference     Reference `json:"reference"`
	Cases         []Case    `json:"cases"`
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
	program   *syn.Program[syn.DeBruijn]
	arguments []data.PlutusData
}

func LoadFile(path string) (*Corpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open replay corpus: %w", err)
	}
	defer f.Close()

	corpus, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("load replay corpus %q: %w", path, err)
	}
	return corpus, nil
}

func Load(r io.Reader) (*Corpus, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var corpus Corpus
	if err := decoder.Decode(&corpus); err != nil {
		return nil, fmt.Errorf("decode replay corpus: %w", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("decode replay corpus: multiple JSON values")
		}
		return nil, fmt.Errorf("decode replay corpus trailing data: %w", err)
	}

	if err := corpus.Validate(); err != nil {
		return nil, err
	}
	return &corpus, nil
}

func (c *Corpus) Validate() error {
	_, err := c.validate()
	return err
}

func (c *Corpus) validate() ([]decodedCase, error) {
	if c == nil {
		return nil, errors.New("replay corpus is required")
	}
	if c.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf(
			"unsupported replay corpus schema version %d (want %d)",
			c.SchemaVersion,
			SchemaVersion,
		)
	}
	if strings.TrimSpace(c.Network) == "" {
		return nil, errors.New("replay corpus network is required")
	}
	if strings.TrimSpace(c.Reference.Implementation) == "" {
		return nil, errors.New(
			"replay corpus reference implementation is required",
		)
	}
	if strings.TrimSpace(c.Reference.Version) == "" {
		return nil, errors.New("replay corpus reference version is required")
	}
	if len(c.Cases) == 0 {
		return nil, errors.New("replay corpus must contain at least one case")
	}

	ids := make(map[string]struct{}, len(c.Cases))
	decodedCases := make([]decodedCase, 0, len(c.Cases))
	for i := range c.Cases {
		replayCase := &c.Cases[i]
		decoded, err := replayCase.validate()
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
		decodedCases = append(decodedCases, decoded)
	}
	return decodedCases, nil
}

func (c *Case) validate() (decodedCase, error) {
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
	if _, err := c.Language.Version(); err != nil {
		return decodedCase{}, err
	}
	flatProgram, err := decodeHex("flat program", c.FlatProgramHex)
	if err != nil {
		return decodedCase{}, err
	}
	program, err := syn.Decode[syn.DeBruijn](flatProgram)
	if err != nil {
		return decodedCase{}, fmt.Errorf("decode FLAT program: %w", err)
	}
	arguments := make([]data.PlutusData, 0, len(c.ArgumentsCBORHex))
	for i, arg := range c.ArgumentsCBORHex {
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
	return decodedCase{
		program:   program,
		arguments: arguments,
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
