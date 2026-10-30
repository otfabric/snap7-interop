package s7client

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestTPKTRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCOTPData(&buf, []byte{0x32, 0x01}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x03, 0x00, 0x00, 0x09, 0x02, 0xF0, 0x80, 0x32, 0x01}) {
		t.Fatalf("frame %x", buf.Bytes())
	}
	got, err := readCOTPData(&buf)
	if err != nil || !bytes.Equal(got, []byte{0x32, 0x01}) {
		t.Fatalf("%x, %v", got, err)
	}
	if err := writeTPKT(failWriter{}, nil); err == nil {
		t.Fatal("a write error must be returned")
	}
}

func TestReadTPKTRejects(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"cut header", []byte{0x03, 0x00}, "EOF"},
		{"bad version", []byte{0x04, 0x00, 0x00, 0x04}, "bad TPKT version"},
		{"bad length", []byte{0x03, 0x00, 0x00, 0x03}, "bad TPKT length"},
		{"cut payload", []byte{0x03, 0x00, 0x00, 0x08, 0x01}, "EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readTPKT(bytes.NewReader(tt.in)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
		})
	}
}

func TestReadCOTPDataRejects(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"no frame", nil, "EOF"},
		{"short", []byte{0x03, 0x00, 0x00, 0x06, 0x02, 0xF0}, "short COTP DT"},
		{"bad length indicator", []byte{0x03, 0x00, 0x00, 0x07, 0x09, 0xF0, 0x80}, "bad COTP LI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readCOTPData(bytes.NewReader(tt.in)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseS7Lengths(t *testing.T) {
	ack := []byte{0x32, 0x03, 0, 0, 0, 1, 0, 2, 0, 1, 0, 0, 0xAA, 0xBB, 0xCC}
	hdr, param, data, err := parseS7Lengths(ack)
	if err != nil || hdr != 12 || param != 2 || data != 1 {
		t.Fatalf("%d %d %d %v", hdr, param, data, err)
	}
	job := []byte{0x32, 0x01, 0, 0, 0, 1, 0, 0, 0, 0}
	if hdr, _, _, err := parseS7Lengths(job); err != nil || hdr != 10 {
		t.Fatalf("job: %d, %v", hdr, err)
	}
	for name, in := range map[string][]byte{
		"short":        {0x32},
		"not S7":       {0x31, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"short header": {0x32, 0x03, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		"overrun":      {0x32, 0x03, 0, 0, 0, 0, 0, 9, 0, 0, 0, 0},
	} {
		if _, _, _, err := parseS7Lengths(in); err == nil {
			t.Fatalf("%s: want an error", name)
		}
	}
}

func TestAreaCodes(t *testing.T) {
	for name, want := range map[string]byte{"inputs": 0x81, "outputs": 0x82, "markers": 0x83, "db": 0x84} {
		if got, ok := areaCode(name); !ok || got != want {
			t.Fatalf("%s: %x, %v", name, got, ok)
		}
	}
	if _, ok := areaCode("timers"); ok {
		t.Fatal("timers accepted")
	}
}

func TestEncodeAnyPointer(t *testing.T) {
	got := encodeAnyPointer(0x84, 0x0102, 0x030405, 0x0607, transportByte)
	want := []byte{0x12, 0x0A, 0x10, 0x02, 0x06, 0x07, 0x01, 0x02, 0x84, 0x03, 0x04, 0x05}
	if !bytes.Equal(got, want) {
		t.Fatalf("db: %x", got)
	}
	if got := encodeAnyPointer(0x83, 5, 0, 1, transportBit); got[6] != 0 || got[7] != 0 || got[3] != transportBit {
		t.Fatalf("a db number must only reach db areas: %x", got)
	}
}

func TestMapItemReturnCode(t *testing.T) {
	for code, want := range map[byte]Outcome{
		0x05: OutcomeAddressOutOfRange, 0x0A: OutcomeAddressOutOfRange, 0x03: OutcomeWriteProtected, 0x06: OutcomeProtocolFailure,
	} {
		if got := mapItemReturnCode(code); got != want {
			t.Fatalf("0x%02x: %s", code, got)
		}
	}
}
