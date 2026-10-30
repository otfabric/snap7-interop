package s7fake_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/otfabric/snap7-interop/internal/s7client"
	"github.com/otfabric/snap7-interop/internal/s7fake"
)

var (
	db1     = s7fake.Area{Type: "db", Number: 1}
	markers = s7fake.Area{Type: "markers"}
)

func start(t *testing.T) (*s7fake.Memory, *s7fake.Server, *s7client.Conn) {
	t.Helper()
	m := s7fake.NewMemory(map[s7fake.Area][]byte{
		db1:               {1, 2, 3, 4},
		markers:           {0b10},
		{Type: "inputs"}:  {5},
		{Type: "outputs"}: {6},
	})
	s, err := s7fake.Serve(m.Reply)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	c, err := s7client.Dial(s.Addr(), 0, 2, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return m, s, c
}

func TestMemoryThroughClient(t *testing.T) {
	m, _, c := start(t)
	for _, tt := range []struct {
		area string
		want byte
	}{{"inputs", 5}, {"outputs", 6}, {"markers", 2}} {
		got, outcome, err := c.ReadArea(tt.area, 0, 0, 1)
		if err != nil || outcome != s7client.OutcomeSuccess || !bytes.Equal(got, []byte{tt.want}) {
			t.Fatalf("%s: %x, %s, %v", tt.area, got, outcome, err)
		}
	}
	if got, outcome, err := c.ReadBit("markers", 0, 0, 1); err != nil || outcome != s7client.OutcomeSuccess || !bytes.Equal(got, []byte{1}) {
		t.Fatalf("read bit: %x, %s, %v", got, outcome, err)
	}
	if outcome, err := c.WriteArea("db", 1, 1, []byte{9, 8}); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("write: %s, %v", outcome, err)
	}
	if got := m.Bytes(db1); !bytes.Equal(got, []byte{1, 9, 8, 4}) {
		t.Fatalf("memory %x", got)
	}
	if outcome, err := c.WriteBit("markers", 0, 0, 0, true); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("set bit: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteBit("markers", 0, 0, 1, false); err != nil || outcome != s7client.OutcomeSuccess {
		t.Fatalf("clear bit: %s, %v", outcome, err)
	}
	if got := m.Bytes(markers); !bytes.Equal(got, []byte{1}) {
		t.Fatalf("markers %x", got)
	}
}

func TestMemoryRejections(t *testing.T) {
	m, _, c := start(t)
	m.SetReadOnly(db1)
	if outcome, err := c.WriteArea("db", 1, 0, []byte{9}); err == nil || outcome != s7client.OutcomeWriteProtected {
		t.Fatalf("read-only: %s, %v", outcome, err)
	}
	if got := m.Bytes(db1); got[0] != 1 {
		t.Fatalf("read-only memory changed: %x", got)
	}
	if _, outcome, err := c.ReadArea("db", 9, 0, 1); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("unknown db read: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteArea("db", 9, 0, []byte{1}); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("unknown db write: %s, %v", outcome, err)
	}
	if _, outcome, err := c.ReadArea("db", 1, 3, 2); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("read past the end: %s, %v", outcome, err)
	}
	if _, outcome, err := c.ReadBit("markers", 0, 5, 0); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("bit past the end: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteArea("markers", 0, 0, []byte{1, 2}); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("write past the end: %s, %v", outcome, err)
	}
	if outcome, err := c.WriteBit("markers", 0, 5, 0, true); err == nil || outcome != s7client.OutcomeAddressOutOfRange {
		t.Fatalf("set bit past the end: %s, %v", outcome, err)
	}
}

func TestReplyMalformed(t *testing.T) {
	m := s7fake.NewMemory(nil)
	short := func(param ...byte) []byte {
		out := []byte{0x02, 0xF0, 0x80, 0x32, 1, 0, 0, 0, 1, 0, byte(len(param)), 0, 0}
		return append(out, param...)
	}
	for name, in := range map[string][]byte{
		"empty":         {},
		"no S7 PDU":     {0x02, 0xF0, 0x80},
		"not S7":        {0x02, 0xF0, 0x80, 0x31, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"overrun":       {0x02, 0xF0, 0x80, 0x32, 1, 0, 0, 0, 1, 0, 9, 0, 0},
		"no parameters": short(),
		"unknown job":   short(0x28),
		"short read":    short(0x04, 0x01),
		"short write":   short(0x05, 0x01),
	} {
		if got := m.Reply(in); got != nil {
			t.Fatalf("%s: answered %x", name, got)
		}
	}
	unknown := []byte{0x04, 0x01, 0x12, 0x0A, 0x10, 0x02, 0, 1, 0, 0, 0x99, 0, 0, 0}
	got := m.Reply(short(unknown...))
	if len(got) < 4 || got[len(got)-4] != 0x0A {
		t.Fatalf("unknown area read: %x", got)
	}
	unknown[0] = 0x05
	in := short(unknown...)
	in[12] = 5 // data length
	got = m.Reply(append(in, 0, 4, 0, 8, 1))
	if len(got) < 1 || got[len(got)-1] != 0x0A {
		t.Fatalf("unknown area write: %x", got)
	}
}

func TestServerLimitAndClose(t *testing.T) {
	_, s, _ := start(t)
	s.Limit(0)
	if _, err := s7client.Dial(s.Addr(), 0, 2, 2*time.Second); err == nil {
		t.Fatal("a connection beyond the limit must fail")
	}
	s.Limit(1)
	c, err := s7client.Dial(s.Addr(), 0, 2, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := c.ReadArea("db", 1, 0, 1); err == nil || outcome != s7client.OutcomeTransportFailure {
		t.Fatalf("read after Close: %s, %v", outcome, err)
	}
}

func TestServerDropsMalformedFrames(t *testing.T) {
	s, err := s7fake.Serve(func([]byte) []byte { return []byte{1} })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s7client.Dial(s.Addr(), 0, 2, time.Second); err == nil {
		t.Fatal("a one-byte COTP answer must fail the handshake")
	}
}
