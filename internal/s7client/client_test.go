package s7client_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/otfabric/snap7-interop/internal/s7client"
	"github.com/otfabric/snap7-interop/internal/s7fake"
)

var db1 = s7fake.Area{Type: "db", Number: 1}

func serve(t *testing.T, reply func([]byte) []byte) *s7fake.Server {
	t.Helper()
	s, err := s7fake.Serve(reply)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func dial(t *testing.T, s *s7fake.Server, timeout time.Duration) *s7client.Conn {
	t.Helper()
	c, err := s7client.Dial(s.Addr(), 0, 2, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// Regression: the deadline of Dial stayed on the connection, so that a session
// failed with a transport error once the per-request timeout had passed.
func TestTimeoutIsPerRequest(t *testing.T) {
	m := s7fake.NewMemory(map[s7fake.Area][]byte{db1: {1}})
	s := serve(t, m.Reply)
	const timeout = 300 * time.Millisecond
	c := dial(t, s, timeout)
	time.Sleep(timeout + 100*time.Millisecond)
	got, outcome, err := c.ReadArea("db", 1, 0, 1)
	if err != nil || outcome != s7client.OutcomeSuccess || !bytes.Equal(got, []byte{1}) {
		t.Fatalf("read: %x, %s, %v", got, outcome, err)
	}
	time.Sleep(timeout + 100*time.Millisecond)
	if outcome, err := c.WriteArea("db", 1, 0, []byte{2}); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("write: %s, %v", outcome, err)
	}
}
