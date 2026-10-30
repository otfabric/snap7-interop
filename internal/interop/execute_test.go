package interop_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otfabric/snap7-interop/internal/interop"
	"github.com/otfabric/snap7-interop/internal/s7client"
	"github.com/otfabric/snap7-interop/internal/s7fake"
)

var db1 = s7fake.Area{Type: "db", Number: 1}

func server(t *testing.T, m *s7fake.Memory) *s7fake.Server {
	t.Helper()
	s, err := s7fake.Serve(m.Reply)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func memory() *s7fake.Memory {
	return s7fake.NewMemory(map[s7fake.Area][]byte{db1: {1, 2, 3, 4}, {Type: "markers"}: {0b100}})
}

// Regression: after a failed reconnect the connection was nil, and the next
// step panicked instead of reporting a transport failure.
func TestRunEndpoint_stepAfterFailedReconnect(t *testing.T) {
	s := server(t, memory())
	s.Limit(1)
	fx := &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{
		{Operation: "reconnect", WantOutcome: s7client.OutcomeSuccess},
		{Operation: "read", AreaType: "db", AreaNum: 1, Length: 1, WantOutcome: s7client.OutcomeSuccess},
		{Operation: "reconnect", WantOutcome: s7client.OutcomeSuccess},
	}}
	got := interop.RunEndpoint("native", s.Addr(), fx, time.Second)
	if got.Passed || len(got.Steps) != 3 {
		t.Fatalf("result %+v", got)
	}
	for i, step := range got.Steps {
		if step.Pass || step.Outcome != s7client.OutcomeTransportFailure {
			t.Fatalf("step %d: %+v", i, step)
		}
	}
	if got.Steps[1].Error != "not connected" || got.Steps[1].AreaType != "db" {
		t.Fatalf("step 1: %+v", got.Steps[1])
	}
}

func exp(op string, want s7client.Outcome) interop.Expectation {
	return interop.Expectation{Operation: op, AreaType: "db", AreaNum: 1, WantOutcome: want}
}

func TestRunEndpoint_operations(t *testing.T) {
	ok := s7client.OutcomeSuccess
	bytesOf := func(e interop.Expectation, b ...byte) interop.Expectation {
		e.WantBytes, e.HasBytes = b, true
		return e
	}
	withLen := func(e interop.Expectation, n int) interop.Expectation {
		e.Length = n
		return e
	}
	withBit := func(e interop.Expectation, offset, bit int) interop.Expectation {
		e.Offset, e.Bit, e.HasBit = offset, bit, true
		return e
	}
	withData := func(e interop.Expectation, b ...byte) interop.Expectation {
		e.Data = b
		return e
	}
	marker := func(e interop.Expectation) interop.Expectation {
		e.AreaType, e.AreaNum = "markers", 0
		return e
	}
	tests := []struct {
		name string
		exp  interop.Expectation
		pass bool
		out  s7client.Outcome
		hex  string
	}{
		{"read bytes", bytesOf(exp("read", ok), 1, 2), true, ok, "0102"},
		{"read wrong bytes", bytesOf(exp("read", ok), 9, 9), false, ok, "0102"},
		{"read without expected bytes", withLen(exp("read", ok), 3), true, ok, "010203"},
		{"final state", bytesOf(exp("final-state", ok), 1), true, ok, "01"},
		{"read bit", withBit(marker(bytesOf(exp("read", ok), 1)), 0, 2), true, ok, "01"},
		{"read outcome mismatch", withLen(exp("read", s7client.OutcomeAddressOutOfRange), 3), false, ok, "010203"},
		{"read an error", withLen(exp("read", ok), 99), false, s7client.OutcomeAddressOutOfRange, ""},
		{"write", withData(exp("write", ok), 9), true, ok, ""},
		{"write bit", withBit(withData(marker(exp("write", ok)), 1), 0, 0), true, ok, ""},
		{"write an error", withData(withBit(exp("write", ok), 99, 0), 0), false, s7client.OutcomeAddressOutOfRange, ""},
		{"rejected read", withLen(exp("rejected-read", s7client.OutcomeAddressOutOfRange), 99), true, s7client.OutcomeAddressOutOfRange, ""},
		{"rejected read that succeeds", withLen(exp("rejected-read", ok), 1), false, ok, ""},
		{"rejected read of a bit", withBit(exp("rejected-read", s7client.OutcomeAddressOutOfRange), 99, 0), true, s7client.OutcomeAddressOutOfRange, ""},
		{"rejected write", withData(exp("rejected-write", s7client.OutcomeAddressOutOfRange), make([]byte, 9)...), true, s7client.OutcomeAddressOutOfRange, ""},
		{"rejected write of a bit", withData(withBit(exp("rejected-write", s7client.OutcomeAddressOutOfRange), 99, 0), 1), true, s7client.OutcomeAddressOutOfRange, ""},
		{"rejected write that succeeds", withData(exp("rejected-write", ok), 1), false, ok, ""},
		{"unsupported operation", exp("explode", s7client.OutcomeUnsupported), true, s7client.OutcomeUnsupported, ""},
		{"unsupported operation unexpected", exp("explode", ok), false, s7client.OutcomeUnsupported, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := server(t, memory())
			fx := &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{tt.exp}}
			got := interop.RunEndpoint("native", s.Addr(), fx, time.Second)
			if got.Passed != tt.pass || len(got.Steps) != 1 {
				t.Fatalf("result %+v", got)
			}
			step := got.Steps[0]
			if step.Pass != tt.pass || step.Outcome != tt.out || step.BytesHex != tt.hex {
				t.Fatalf("step %+v", step)
			}
			if step.Outcome != s7client.OutcomeSuccess && step.Error == "" {
				t.Fatalf("no error text: %+v", step)
			}
		})
	}
}

func TestRunEndpoint_writeThenReconnectKeepsState(t *testing.T) {
	s := server(t, memory())
	ok := s7client.OutcomeSuccess
	write := exp("write", ok)
	write.Data = []byte{0xAB}
	final := exp("final-state", ok)
	final.WantBytes, final.HasBytes = []byte{0xAB}, true
	fx := &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{write, exp("reconnect", ok), final}}
	got := interop.RunEndpoint("native", s.Addr(), fx, time.Second)
	if !got.Passed {
		t.Fatalf("result %+v", got)
	}
}

func TestRunEndpoint_reconnectExpectedToFail(t *testing.T) {
	s := server(t, memory())
	fx := &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{exp("reconnect", s7client.OutcomeTransportFailure)}}
	got := interop.RunEndpoint("native", s.Addr(), fx, time.Second)
	if got.Passed || got.Steps[0].Outcome != s7client.OutcomeSuccess {
		t.Fatalf("result %+v", got)
	}
}

func TestRunEndpoint_concurrentRead(t *testing.T) {
	ok := s7client.OutcomeSuccess
	base := interop.Expectation{Operation: "concurrent-read", AreaType: "db", AreaNum: 1, Clients: 4, Length: 2, WantOutcome: ok}
	withBytes := base
	withBytes.Length = 0
	withBytes.WantBytes, withBytes.HasBytes = []byte{1, 2}, true
	wrongBytes := base
	wrongBytes.WantBytes, wrongBytes.HasBytes = []byte{9, 9}, true
	wrongOutcome := base
	wrongOutcome.WantOutcome = s7client.OutcomeAddressOutOfRange
	two := base
	two.Clients = 0
	tests := []struct {
		name string
		exp  interop.Expectation
		pass bool
	}{
		{"same bytes", base, true},
		{"length from the expected bytes", withBytes, true},
		{"at least two clients", two, true},
		{"wrong bytes", wrongBytes, false},
		{"wrong outcome", wrongOutcome, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := server(t, memory())
			fx := &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{tt.exp}}
			got := interop.RunEndpoint("native", s.Addr(), fx, time.Second)
			if got.Passed != tt.pass || got.Steps[0].Pass != tt.pass {
				t.Fatalf("result %+v", got)
			}
		})
	}
}

func TestRunEndpoint_concurrentClientRefused(t *testing.T) {
	s := server(t, memory())
	s.Limit(1) // the session itself
	e := interop.Expectation{Operation: "concurrent-read", AreaType: "db", AreaNum: 1, Length: 1, Clients: 2, WantOutcome: s7client.OutcomeSuccess}
	got := interop.RunEndpoint("native", s.Addr(), &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{e}}, time.Second)
	if got.Passed || got.Steps[0].Outcome != s7client.OutcomeTransportFailure || got.Steps[0].Error == "" {
		t.Fatalf("result %+v", got)
	}
}

// Readers that disagree fail the step, though each meets the expected outcome.
func TestRunEndpoint_concurrentClientsDiffer(t *testing.T) {
	m := memory()
	var n atomic.Int32
	reply := func(p []byte) []byte {
		out := m.Reply(p)
		if len(p) > 13 && p[13] == 0x04 && n.Add(1) == 2 {
			out[len(out)-1] ^= 0xFF
		}
		return out
	}
	s, err := s7fake.Serve(reply)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := interop.Expectation{Operation: "concurrent-read", AreaType: "db", AreaNum: 1, Length: 1, Clients: 2, WantOutcome: s7client.OutcomeSuccess}
	got := interop.RunEndpoint("native", s.Addr(), &interop.CompiledFixture{Slot: 2, Expectations: []interop.Expectation{e}}, time.Second)
	if got.Passed || got.Steps[0].Error != "concurrent clients returned differing bytes" {
		t.Fatalf("result %+v", got)
	}
}

func TestRunEndpoint_skipAndDialFailure(t *testing.T) {
	fx := &interop.CompiledFixture{Requires: []string{"area.timers"}}
	got := interop.RunEndpoint("native", "127.0.0.1:1", fx, 100*time.Millisecond)
	if !got.Passed || !strings.Contains(got.SkipReason, "area.timers") {
		t.Fatalf("skip: %+v", got)
	}
	s := server(t, memory())
	addr := s.Addr()
	_ = s.Close()
	got = interop.RunEndpoint("native", addr, &interop.CompiledFixture{Slot: 2}, 100*time.Millisecond)
	if got.Passed || len(got.Steps) != 1 || got.Steps[0].Index != -1 || got.Steps[0].Outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("dial failure: %+v", got)
	}
}

func TestWaitReady(t *testing.T) {
	s := server(t, memory())
	if err := interop.WaitReady(s.Addr(), 0, 2, time.Second); err != nil {
		t.Fatal(err)
	}
	addr := s.Addr()
	_ = s.Close()
	if err := interop.WaitReady(addr, 0, 2, 50*time.Millisecond); err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("err %v", err)
	}
}

func TestMissingCapabilities(t *testing.T) {
	got := interop.MissingCapabilities([]string{"area.db", "area.timers"}, interop.DefaultCapabilities)
	if len(got) != 1 || got[0] != "area.timers" {
		t.Fatalf("missing %v", got)
	}
}
