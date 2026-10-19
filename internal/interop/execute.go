package interop

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/otfabric/snap7-interop/internal/s7client"
)

// StepResult is the outcome of one expectation on one endpoint.
type StepResult struct {
	Index       int              `json:"index"`
	Operation   string           `json:"operation"`
	AreaType    string           `json:"areaType"`
	AreaNumber  int              `json:"areaNumber"`
	Offset      int              `json:"offset"`
	Length      int              `json:"length"`
	Outcome     s7client.Outcome `json:"outcome"`
	BytesHex    string           `json:"bytesHex,omitempty"`
	Error       string           `json:"error,omitempty"`
	WantOutcome s7client.Outcome `json:"wantOutcome"`
	WantHex     string           `json:"wantHex,omitempty"`
	Pass        bool             `json:"pass"`
}

// EndpointResult is the full run against one server.
type EndpointResult struct {
	Name       string       `json:"name"`
	Addr       string       `json:"addr"`
	Steps      []StepResult `json:"steps"`
	Passed     bool         `json:"passed"`
	SkipReason string       `json:"skipReason,omitempty"`
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
	defer func() {
		if c != nil {
			_ = c.Close()
		}
	}()

	for i, exp := range fx.Expectations {
		var step StepResult
		switch exp.Operation {
		case "reconnect":
			step = executeReconnect(&c, addr, fx, i, exp, timeout)
		case "concurrent-read":
			step = executeConcurrentRead(addr, fx, i, exp, timeout)
		default:
			step = executeStep(c, i, exp)
		}
		out.Steps = append(out.Steps, step)
		if !step.Pass {
			out.Passed = false
		}
	}
	return out
}

func executeReconnect(c **s7client.Conn, addr string, fx *CompiledFixture, index int, exp Expectation, timeout time.Duration) StepResult {
	step := StepResult{
		Index:       index,
		Operation:   exp.Operation,
		AreaType:    exp.AreaType,
		AreaNumber:  exp.AreaNum,
		WantOutcome: exp.WantOutcome,
	}
	if *c != nil {
		_ = (*c).Close()
		*c = nil
	}
	next, err := s7client.Dial(addr, fx.Rack, fx.Slot, timeout)
	if err != nil {
		step.Outcome = s7client.OutcomeTransportFailure
		step.Error = err.Error()
		step.Pass = false
		return step
	}
	*c = next
	step.Outcome = s7client.OutcomeSuccess
	step.Pass = exp.WantOutcome == s7client.OutcomeSuccess
	return step
}

func executeConcurrentRead(addr string, fx *CompiledFixture, index int, exp Expectation, timeout time.Duration) StepResult {
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
	n := exp.Clients
	if n < 2 {
		n = 2
	}
	length := exp.Length
	if length == 0 && exp.HasBytes {
		length = len(exp.WantBytes)
	}

	type result struct {
		hex     string
		outcome s7client.Outcome
		err     string
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			conn, err := s7client.Dial(addr, fx.Rack, fx.Slot, timeout)
			if err != nil {
				results[i] = result{outcome: s7client.OutcomeTransportFailure, err: err.Error()}
				return
			}
			defer conn.Close()
			data, outcome, err := conn.ReadArea(exp.AreaType, exp.AreaNum, exp.Offset, length)
			r := result{outcome: outcome}
			if err != nil {
				r.err = err.Error()
			}
			if data != nil {
				r.hex = hex.EncodeToString(data)
			}
			results[i] = r
		}(i)
	}
	wg.Wait()

	step.Outcome = results[0].outcome
	step.BytesHex = results[0].hex
	step.Error = results[0].err
	step.Pass = true
	for _, r := range results {
		if r.outcome != exp.WantOutcome {
			step.Pass = false
			step.Outcome = r.outcome
			step.Error = r.err
			break
		}
		if exp.HasBytes && r.hex != step.WantHex {
			step.Pass = false
			step.BytesHex = r.hex
			break
		}
		if r.hex != results[0].hex {
			step.Pass = false
			step.Error = "concurrent clients returned differing bytes"
			break
		}
	}
	return step
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
		var (
			data    []byte
			outcome s7client.Outcome
			err     error
		)
		if exp.HasBit {
			data, outcome, err = c.ReadBit(exp.AreaType, exp.AreaNum, exp.Offset, exp.Bit)
		} else {
			data, outcome, err = c.ReadArea(exp.AreaType, exp.AreaNum, exp.Offset, length)
		}
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
		var (
			outcome s7client.Outcome
			err     error
		)
		if exp.HasBit {
			set := len(exp.Data) > 0 && exp.Data[0] != 0
			outcome, err = c.WriteBit(exp.AreaType, exp.AreaNum, exp.Offset, exp.Bit, set)
		} else {
			outcome, err = c.WriteArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Data)
		}
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		step.Pass = outcome == exp.WantOutcome
	case "rejected-read":
		var (
			outcome s7client.Outcome
			err     error
		)
		if exp.HasBit {
			_, outcome, err = c.ReadBit(exp.AreaType, exp.AreaNum, exp.Offset, exp.Bit)
		} else {
			_, outcome, err = c.ReadArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Length)
		}
		step.Outcome = outcome
		if err != nil {
			step.Error = err.Error()
		}
		step.Pass = outcome == exp.WantOutcome && outcome != s7client.OutcomeSuccess
	case "rejected-write":
		var (
			outcome s7client.Outcome
			err     error
		)
		if exp.HasBit {
			set := len(exp.Data) > 0 && exp.Data[0] != 0
			outcome, err = c.WriteBit(exp.AreaType, exp.AreaNum, exp.Offset, exp.Bit, set)
		} else {
			outcome, err = c.WriteArea(exp.AreaType, exp.AreaNum, exp.Offset, exp.Data)
		}
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
