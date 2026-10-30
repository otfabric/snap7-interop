package interop_test

import (
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
