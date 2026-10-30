package s7client_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/snap7-interop/internal/s7client"
	"github.com/otfabric/snap7-interop/internal/s7fake"
)

var db1 = s7fake.Area{Type: "db", Number: 1}

func serve(t *testing.T, reply func([]byte) []byte) *s7fake.Server {
	t.Helper()
	if reply == nil {
		reply = s7fake.NewMemory(nil).Reply
	}
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

// jobReply serves the handshake from a Memory and answers S7 jobs with data as
// the data part of an ack-data PDU.
func jobReply(data []byte) func([]byte) []byte {
	m := s7fake.NewMemory(nil)
	return func(p []byte) []byte {
		if p[1] != 0xE0 && p[13] != 0xF0 {
			out := []byte{0x02, 0xF0, 0x80, 0x32, 0x03, 0, 0, 0, 1, 0, 0, byte(len(data) >> 8), byte(len(data)), 0, 0}
			return append(out, data...)
		}
		return m.Reply(p)
	}
}

func TestDialFailures(t *testing.T) {
	s := serve(t, nil)
	addr := s.Addr()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := s7client.Dial(addr, 0, 2, time.Second)
	if err == nil || !strings.Contains(err.Error(), string(s7client.OutcomeTransportFailure)) {
		t.Fatalf("refused: %v", err)
	}

	m := s7fake.NewMemory(nil)
	tests := []struct {
		name  string
		reply func([]byte) []byte
		want  s7client.Outcome
	}{
		{"connection closed at connect", func([]byte) []byte { return nil }, s7client.OutcomeTransportFailure},
		{"no connection confirm", func([]byte) []byte { return []byte{0x06, 0xF0} }, s7client.OutcomeProtocolFailure},
		{"closed at setup", func(p []byte) []byte {
			if p[1] == 0xE0 {
				return m.Reply(p)
			}
			return nil
		}, s7client.OutcomeTransportFailure},
		{"bad setup answer", func(p []byte) []byte {
			if p[1] == 0xE0 {
				return m.Reply(p)
			}
			return []byte{0x02, 0xF0, 0x80, 0x31}
		}, s7client.OutcomeProtocolFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := serve(t, tt.reply)
			_, err := s7client.Dial(s.Addr(), 0, 2, time.Second)
			if err == nil || !strings.Contains(err.Error(), string(tt.want)) {
				t.Fatalf("err %v, want %s", err, tt.want)
			}
		})
	}
}

func TestArgumentChecks(t *testing.T) {
	c := dial(t, serve(t, s7fake.NewMemory(nil).Reply), time.Second)
	if _, outcome, err := c.ReadBit("db", 1, 0, 8); err == nil || outcome != s7client.OutcomeUnsupported {
		t.Fatalf("read bit 8: %s, %v", outcome, err)
	}
	if _, outcome, err := c.ReadBit("db", 1, 0, -1); err == nil || outcome != s7client.OutcomeUnsupported {
		t.Fatalf("read bit -1: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteBit("db", 1, 0, 8, true); err == nil || outcome != s7client.OutcomeUnsupported {
		t.Fatalf("write bit 8: %s, %v", outcome, err)
	}
	if _, outcome, err := c.ReadArea("timers", 0, 0, 1); err == nil || outcome != s7client.OutcomeUnsupported {
		t.Fatalf("read area: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteArea("timers", 0, 0, []byte{1}); err == nil || outcome != s7client.OutcomeUnsupported {
		t.Fatalf("write area: %s, %v", outcome, err)
	}
}

func TestReadBitAndWriteBit(t *testing.T) {
	m := s7fake.NewMemory(map[s7fake.Area][]byte{db1: {0b0100}})
	c := dial(t, serve(t, m.Reply), time.Second)
	for bit, want := range []byte{0, 0, 1, 0} {
		got, outcome, err := c.ReadBit("db", 1, 0, bit)
		if err != nil || outcome != s7client.OutcomeSuccess || !bytes.Equal(got, []byte{want}) {
			t.Fatalf("bit %d: %x, %s, %v", bit, got, outcome, err)
		}
	}
	if outcome, err := c.WriteBit("db", 1, 0, 0, true); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("write: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteBit("db", 1, 0, 2, false); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("write: %s, %v", outcome, err)
	}
	if got := m.Bytes(db1); !bytes.Equal(got, []byte{0b0001}) {
		t.Fatalf("memory %08b", got)
	}
}

func TestReadResponses(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    []byte
		outcome s7client.Outcome
	}{
		{"octets, length in bytes", []byte{0xFF, 0x09, 0, 2, 7, 8}, []byte{7, 8}, s7client.OutcomeSuccess},
		{"bytes, length in bits", []byte{0xFF, 0x04, 0, 16, 7, 8}, []byte{7, 8}, s7client.OutcomeSuccess},
		{"integers, length in bits", []byte{0xFF, 0x05, 0, 16, 7, 8}, []byte{7, 8}, s7client.OutcomeSuccess},
		{"no data header", []byte{0xFF, 0x04}, nil, s7client.OutcomeProtocolFailure},
		{"data shorter than declared", []byte{0xFF, 0x04, 0, 32, 7, 8}, nil, s7client.OutcomeProtocolFailure},
		{"access denied", []byte{0x03, 0, 0, 0}, nil, s7client.OutcomeWriteProtected},
		{"hardware fault", []byte{0x01, 0, 0, 0}, nil, s7client.OutcomeProtocolFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := dial(t, serve(t, jobReply(tt.data)), time.Second)
			got, outcome, err := c.ReadArea("db", 1, 0, 2)
			if outcome != tt.outcome || !bytes.Equal(got, tt.want) || (err != nil) != (tt.outcome != s7client.OutcomeSuccess) {
				t.Fatalf("%x, %s, %v", got, outcome, err)
			}
		})
	}
}

func TestReadBitKeepsTheLowestBitOnly(t *testing.T) {
	c := dial(t, serve(t, jobReply([]byte{0xFF, 0x03, 0, 1, 0xFF})), time.Second)
	got, outcome, err := c.ReadBit("db", 1, 0, 0)
	if err != nil || outcome != s7client.OutcomeSuccess || !bytes.Equal(got, []byte{1}) {
		t.Fatalf("%x, %s, %v", got, outcome, err)
	}
}

func TestWriteResponses(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		outcome s7client.Outcome
	}{
		{"accepted", []byte{0xFF}, s7client.OutcomeSuccess},
		{"empty", nil, s7client.OutcomeProtocolFailure},
		{"read-only", []byte{0x03}, s7client.OutcomeWriteProtected},
		{"no such address", []byte{0x05}, s7client.OutcomeAddressOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := dial(t, serve(t, jobReply(tt.data)), time.Second)
			outcome, err := c.WriteArea("db", 1, 0, []byte{1})
			if outcome != tt.outcome || (err != nil) != (tt.outcome != s7client.OutcomeSuccess) {
				t.Fatalf("%s, %v", outcome, err)
			}
		})
	}
}

func TestMalformedAck(t *testing.T) {
	m := s7fake.NewMemory(nil)
	reply := func(p []byte) []byte {
		if p[1] == 0xE0 || p[13] == 0xF0 {
			return m.Reply(p)
		}
		return []byte{0x02, 0xF0, 0x80, 0x31}
	}
	c := dial(t, serve(t, reply), time.Second)
	if _, outcome, err := c.ReadArea("db", 1, 0, 1); err == nil || outcome != s7client.OutcomeProtocolFailure {
		t.Fatalf("read: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteArea("db", 1, 0, []byte{1}); err == nil || outcome != s7client.OutcomeProtocolFailure {
		t.Fatalf("write: %s, %v", outcome, err)
	}
}

func TestTransportFailures(t *testing.T) {
	m := s7fake.NewMemory(nil)
	drop := func(p []byte) []byte {
		if p[1] == 0xE0 || p[13] == 0xF0 {
			return m.Reply(p)
		}
		return nil
	}
	c := dial(t, serve(t, drop), time.Second)
	if _, outcome, err := c.ReadArea("db", 1, 0, 1); err == nil || outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("no answer to a read: %s, %v", outcome, err)
	}
	c = dial(t, serve(t, drop), time.Second)
	if outcome, err := c.WriteArea("db", 1, 0, []byte{1}); err == nil || outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("no answer to a write: %s, %v", outcome, err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := c.ReadArea("db", 1, 0, 1); err == nil || outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("read after Close: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteArea("db", 1, 0, []byte{1}); err == nil || outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("write after Close: %s, %v", outcome, err)
	}
}
