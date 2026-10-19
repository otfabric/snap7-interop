package interop

import (
	"fmt"

	"github.com/otfabric/snap7-interop/internal/s7client"
)

// Diff describes a divergence between two endpoint step results.
type Diff struct {
	Index      int    `json:"index"`
	Operation  string `json:"operation"`
	AreaType   string `json:"areaType"`
	AreaNumber int    `json:"areaNumber"`
	Offset     int    `json:"offset"`
	Kind       string `json:"kind"` // outcome | bytes | length
	Expected   string `json:"expected,omitempty"`
	Actual     string `json:"actual,omitempty"`
	Native     string `json:"native,omitempty"`
	Python     string `json:"python,omitempty"`
	Message    string `json:"message"`
}

// CompareEndpoints compares native vs python step-by-step.
func CompareEndpoints(native, python EndpointResult) (diffs []Diff, ok bool) {
	if native.SkipReason != "" || python.SkipReason != "" {
		if native.SkipReason != python.SkipReason {
			diffs = append(diffs, Diff{
				Kind:    "skip",
				Native:  native.SkipReason,
				Python:  python.SkipReason,
				Message: "skip reasons differ",
			})
			return diffs, false
		}
		return nil, true
	}
	n := len(native.Steps)
	if len(python.Steps) < n {
		n = len(python.Steps)
	}
	for i := 0; i < n; i++ {
		a := native.Steps[i]
		b := python.Steps[i]
		if a.Outcome != b.Outcome {
			diffs = append(diffs, Diff{
				Index:      i,
				Operation:  a.Operation,
				AreaType:   a.AreaType,
				AreaNumber: a.AreaNumber,
				Offset:     a.Offset,
				Kind:       "outcome",
				Native:     string(a.Outcome),
				Python:     string(b.Outcome),
				Message: fmt.Sprintf(
					"outcome mismatch at step %d %s %s#%d offset=%d: native=%s python=%s",
					i, a.Operation, a.AreaType, a.AreaNumber, a.Offset, a.Outcome, b.Outcome,
				),
			})
			continue
		}
		if a.BytesHex != b.BytesHex {
			diffs = append(diffs, Diff{
				Index:      i,
				Operation:  a.Operation,
				AreaType:   a.AreaType,
				AreaNumber: a.AreaNumber,
				Offset:     a.Offset,
				Kind:       "bytes",
				Expected:   a.WantHex,
				Actual:     b.BytesHex,
				Native:     a.BytesHex,
				Python:     b.BytesHex,
				Message: fmt.Sprintf(
					"byte mismatch at step %d %s %s#%d offset=%d: native=%s python=%s want=%s",
					i, a.Operation, a.AreaType, a.AreaNumber, a.Offset, a.BytesHex, b.BytesHex, a.WantHex,
				),
			})
		}
	}
	if len(native.Steps) != len(python.Steps) {
		diffs = append(diffs, Diff{
			Kind:    "length",
			Native:  fmt.Sprintf("%d", len(native.Steps)),
			Python:  fmt.Sprintf("%d", len(python.Steps)),
			Message: "step count differs",
		})
	}
	return diffs, len(diffs) == 0
}

// ClassifyFixture maps endpoint/compare results to a fixture-level status.
func ClassifyFixture(native, python EndpointResult, compareOK bool, invalid bool) string {
	if invalid {
		return "invalid"
	}
	if native.SkipReason != "" && python.SkipReason != "" {
		return "skip-unsupported"
	}
	if !native.Passed || !python.Passed {
		for _, s := range append(native.Steps, python.Steps...) {
			if s.Outcome == s7client.OutcomeTransportFailure {
				return "infrastructure-error"
			}
		}
		return "fail"
	}
	if !compareOK {
		return "fail"
	}
	return "pass"
}
