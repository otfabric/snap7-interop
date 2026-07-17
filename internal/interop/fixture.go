package interop

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/snap-interop/internal/s7client"
)

// CompiledFixture is the subset of the compiled fixture needed by the runner.
type CompiledFixture struct {
	ID           string
	Revision     int
	Requires     []string
	Rack         int
	Slot         int
	Expectations []Expectation
}

type Expectation struct {
	Operation string
	AreaType  string
	AreaNum   int
	Offset    int
	Length    int
	Data      []byte
	// WantOutcome is set for string results like "accepted".
	WantOutcome s7client.Outcome
	// WantBytes is set when result is a byte payload.
	WantBytes []byte
	HasBytes  bool
}

func LoadCompiledFixture(path string) (*CompiledFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	meta, _ := doc["fixture"].(map[string]any)
	server, _ := doc["server"].(map[string]any)
	fx := &CompiledFixture{
		ID:       asString(meta["id"]),
		Revision: asInt(meta["revision"], 1),
		Rack:     asInt(server["rack"], 0),
		Slot:     asInt(server["slot"], 2),
	}
	if reqs, ok := doc["requires"].([]any); ok {
		for _, r := range reqs {
			fx.Requires = append(fx.Requires, fmt.Sprint(r))
		}
	}
	exps, _ := doc["expectations"].([]any)
	for i, item := range exps {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expectations[%d]: not an object", i)
		}
		exp, err := parseExpectation(m)
		if err != nil {
			return nil, fmt.Errorf("expectations[%d]: %w", i, err)
		}
		fx.Expectations = append(fx.Expectations, exp)
	}
	return fx, nil
}

func parseExpectation(m map[string]any) (Expectation, error) {
	area, _ := m["area"].(map[string]any)
	exp := Expectation{
		Operation: asString(m["operation"]),
		AreaType:  asString(area["type"]),
		AreaNum:   asInt(area["number"], 0),
		Offset:    asInt(m["offset"], 0),
		Length:    asInt(m["length"], 0),
	}
	if data, ok := m["data"].(map[string]any); ok {
		b, err := decodePayload(data)
		if err != nil {
			return exp, err
		}
		exp.Data = b
	}
	switch result := m["result"].(type) {
	case string:
		exp.WantOutcome = normalizeResultString(result)
	case map[string]any:
		b, err := decodePayload(result)
		if err != nil {
			return exp, err
		}
		exp.WantBytes = b
		exp.HasBytes = true
		exp.WantOutcome = s7client.OutcomeSuccess
		if exp.Length == 0 {
			exp.Length = len(b)
		}
	default:
		return exp, fmt.Errorf("unsupported result type %T", m["result"])
	}
	return exp, nil
}

func normalizeResultString(s string) s7client.Outcome {
	switch s {
	case "accepted", "success":
		return s7client.OutcomeSuccess
	case "address-out-of-range":
		return s7client.OutcomeAddressOutOfRange
	case "write-protected":
		return s7client.OutcomeWriteProtected
	case "unsupported":
		return s7client.OutcomeUnsupported
	case "transport-failure":
		return s7client.OutcomeTransportFailure
	case "protocol-failure":
		return s7client.OutcomeProtocolFailure
	default:
		return s7client.Outcome(s)
	}
}

func decodePayload(m map[string]any) ([]byte, error) {
	if h, ok := m["hex"].(string); ok {
		return hex.DecodeString(strings.ReplaceAll(h, " ", ""))
	}
	return nil, fmt.Errorf("payload requires hex")
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func asInt(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
	default:
		return def
	}
}
