package interop_test

import (
	"strings"
	"testing"

	"github.com/otfabric/snap-interop/internal/interop"
	"github.com/otfabric/snap-interop/internal/s7client"
)

func TestCompareEndpointsDetectsByteMismatch(t *testing.T) {
	native := interop.EndpointResult{
		Name:   "native",
		Passed: true,
		Steps: []interop.StepResult{{
			Index:      0,
			Operation:  "read",
			AreaType:   "db",
			AreaNumber: 1,
			Offset:     0,
			Outcome:    s7client.OutcomeSuccess,
			BytesHex:   "0102030405060708",
			WantHex:    "0102030405060708",
			Pass:       true,
		}},
	}
	python := interop.EndpointResult{
		Name:   "python",
		Passed: true,
		Steps: []interop.StepResult{{
			Index:      0,
			Operation:  "read",
			AreaType:   "db",
			AreaNumber: 1,
			Offset:     0,
			Outcome:    s7client.OutcomeSuccess,
			BytesHex:   "ffffffffffffffff", // deliberate mismatch
			WantHex:    "0102030405060708",
			Pass:       false,
		}},
	}

	diffs, ok := interop.CompareEndpoints(native, python)
	if ok {
		t.Fatal("expected compare failure")
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Kind != "bytes" {
		t.Fatalf("kind=%s", d.Kind)
	}
	if d.AreaType != "db" || d.Offset != 0 {
		t.Fatalf("area/offset not recorded: %#v", d)
	}
	if d.Native != "0102030405060708" || d.Python != "ffffffffffffffff" {
		t.Fatalf("bytes not recorded: %#v", d)
	}
	if d.Expected != "0102030405060708" {
		t.Fatalf("expected bytes missing: %#v", d)
	}
	if !strings.Contains(d.Message, "byte mismatch") {
		t.Fatalf("message=%q", d.Message)
	}
}

func TestClassifySkipUnsupported(t *testing.T) {
	native := interop.EndpointResult{SkipReason: "skip-unsupported: missing [area.timers]"}
	python := interop.EndpointResult{SkipReason: "skip-unsupported: missing [area.timers]"}
	status := interop.ClassifyFixture(native, python, true, false)
	if status != "skip-unsupported" {
		t.Fatalf("status=%s", status)
	}
}
