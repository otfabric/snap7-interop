package interop

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/otfabric/snap-interop/internal/s7client"
)

// StepResult is the outcome of one expectation on one endpoint.
type StepResult struct {
	Index       int               `json:"index"`
	Operation   string            `json:"operation"`
	AreaType    string            `json:"areaType"`
	AreaNumber  int               `json:"areaNumber"`
	Offset      int               `json:"offset"`
	Length      int               `json:"length"`
	Outcome     s7client.Outcome  `json:"outcome"`
	BytesHex    string            `json:"bytesHex,omitempty"`
	Error       string            `json:"error,omitempty"`
	WantOutcome s7client.Outcome  `json:"wantOutcome"`
	WantHex     string            `json:"wantHex,omitempty"`
	Pass        bool              `json:"pass"`
}

// EndpointResult is the full run against one server.
type EndpointResult struct {
	Name     string       `json:"name"`
	Addr     string       `json:"addr"`
	Steps    []StepResult `json:"steps"`
	Passed   bool         `json:"passed"`
	SkipReason string     `json:"skipReason,omitempty"`
}

// WaitReady dials until success or timeout.
func WaitReady(addr string, rack, slot int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		c, err := s7client.Dial(addr, rack, slot, 2*time.Second)
		if err == nil {
			_ = c.Close()
			return nil
		}
		last = err
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("not ready within %s: %v", timeout, last)
}

// RunEndpoint executes fixture expectations against one endpoint.
func RunEndpoint(name, addr string, fx *CompiledFixture, timeout time.Duration) EndpointResult {
	out := EndpointResult{Name: name, Addr: addr, Passed: true}
	missing := MissingCapabilities(fx.Requires, DefaultCapabilities)
	if len(missing) > 0 {
		out.Passed = true
		out.SkipReason = "skip-unsupported: missing " + fmt.Sprint(missing)
		return out
	}

	c, err := s7client.Dial(addr, fx.Rack, fx.Slot, timeout)
	if err != nil {
		out.Passed = false
		out.Steps = append(out.Steps, StepResult{
			Index:   -1,
			Outcome: s7client.OutcomeTransportFailure,
			Error:   err.Error(),
			Pass:    false,
		})
		return out
	}
	defer c.Close()

	for i, exp := range fx.Expectations {
		step := executeStep(c, i, exp)
		out.Steps = append(out.Steps, step)
		if !step.Pass {
			out.Passed = false
		}
	}
	return out
}

func executeStep(c *s7client.Conn, index int, exp Expectation) StepResult {
	step := StepResult{
		Index:       index,
		Operation:   exp.Operation,
		AreaType:    exp.AreaType,
		AreaNumber:  exp.AreaNum,
		Offset:      exp.Offset,
		Length:      exp.Length,
		WantOutcome: exp.WantOutcome,
	}
	if exp.HasBytes {
		step.WantHex = hex.EncodeToString(exp.WantBytes)
	}

	switch exp.Operation {
	case "read", "final-state":
		length := exp.Length
		if length == 0 && exp.HasBytes {
			length = len(exp.WantBytes)
		}
		data, outcome, err := c.ReadArea(exp.AreaType, exp.AreaNum, exp.Offset, length)
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		if data != nil {
			step.BytesHex = hex.EncodeToString(data)
		}
		step.Pass = outcome == exp.WantOutcome
		if exp.HasBytes && outcome == s7client.OutcomeSuccess {
			step.Pass = step.Pass && step.BytesHex == step.WantHex
		}
	case "write":
		outcome, err := c.WriteArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Data)
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		step.Pass = outcome == exp.WantOutcome
	case "rejected-read":
		_, outcome, err := c.ReadArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Length)
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		step.Pass = outcome == exp.WantOutcome && outcome != s7client.OutcomeSuccess
	case "rejected-write":
		outcome, err := c.WriteArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Data)
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		step.Pass = outcome == exp.WantOutcome && outcome != s7client.OutcomeSuccess
	default:
		step.Outcome = s7client.OutcomeUnsupported
		step.Error = "unsupported operation " + exp.Operation
		step.Pass = exp.WantOutcome == s7client.OutcomeUnsupported
	}
	return step
}
