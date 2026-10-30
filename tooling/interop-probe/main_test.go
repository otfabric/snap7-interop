package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/snap7-interop/internal/s7fake"
)

var db1 = s7fake.Area{Type: "db", Number: 1}

func capture(t *testing.T, fn func() int) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	out, errf := open("out"), open("err")
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errf
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	code = fn()
	read := func(f *os.File) string {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return code, read(out), read(errf)
}

func serve(t *testing.T, m *s7fake.Memory) string {
	t.Helper()
	s, err := s7fake.Serve(m.Reply)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s.Addr()
}

func memory() *s7fake.Memory {
	return s7fake.NewMemory(map[s7fake.Area][]byte{db1: {1, 2, 3, 4}})
}

func TestRunDispatch(t *testing.T) {
	tests := []struct {
		args []string
		code int
		want string
	}{
		{nil, 2, "Usage:"},
		{[]string{"help"}, 0, "Usage:"},
		{[]string{"--help"}, 0, "Usage:"},
		{[]string{"smoke", "--bad"}, 2, "not defined"},
		{[]string{"run", "--bad"}, 2, "not defined"},
		{[]string{"interop"}, 2, "--fixture is required"},
	}
	for _, tt := range tests {
		code, _, stderr := capture(t, func() int { return run(tt.args) })
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Fatalf("%v: %d, %q", tt.args, code, stderr)
		}
	}
}

func TestSmokeRead(t *testing.T) {
	addr := serve(t, memory())
	code, stdout, _ := capture(t, func() int {
		return run([]string{"smoke", "-addr", addr, "-length", "2", "-expect-hex", "01 02"})
	})
	if code != 0 || !strings.Contains(stdout, "hex=0102") || !strings.Contains(stdout, "OK expect-hex match") {
		t.Fatalf("%d, %q", code, stdout)
	}
}

// Flags without a subcommand are smoke flags.
func TestSmokeWithoutSubcommand(t *testing.T) {
	addr := serve(t, memory())
	code, stdout, _ := capture(t, func() int { return run([]string{"-addr", addr, "-length", "1"}) })
	if code != 0 || !strings.Contains(stdout, "READ db=1") {
		t.Fatalf("%d, %q", code, stdout)
	}
}

func TestSmokeWrite(t *testing.T) {
	m := memory()
	addr := serve(t, m)
	code, stdout, _ := capture(t, func() int {
		return run([]string{"smoke", "-addr", addr, "-length", "1", "-offset", "1", "-write-hex", "aabb"})
	})
	if code != 0 || !strings.Contains(stdout, "OK write verified") {
		t.Fatalf("%d, %q", code, stdout)
	}
	if got := m.Bytes(db1); got[1] != 0xAA || got[2] != 0xBB {
		t.Fatalf("memory %x", got)
	}
}

func TestSmokeFailures(t *testing.T) {
	readOnly := memory()
	readOnly.SetReadOnly(db1)
	addr := serve(t, memory())
	roAddr := serve(t, readOnly)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"read", []string{"-addr", addr, "-db", "9"}, "read: outcome="},
		{"bad expected hex", []string{"-addr", addr, "-length", "1", "-expect-hex", "zz"}, "expect-hex:"},
		{"byte mismatch", []string{"-addr", addr, "-length", "1", "-expect-hex", "ff"}, "byte mismatch"},
		{"bad write hex", []string{"-addr", addr, "-length", "1", "-write-hex", "zz"}, "write-hex:"},
		{"write refused", []string{"-addr", roAddr, "-length", "1", "-write-hex", "ff"}, "write: outcome="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := capture(t, func() int { return cmdSmoke(tt.args) })
			if code != 1 || !strings.Contains(stderr, tt.want) {
				t.Fatalf("%d, %q", code, stderr)
			}
		})
	}
}

// A server that takes a write without storing it fails the read-back.
func TestSmokeWriteNotVerified(t *testing.T) {
	m := memory()
	reply := func(p []byte) []byte {
		if len(p) > 13 && p[13] == 0x05 {
			return []byte{0x02, 0xF0, 0x80, 0x32, 0x03, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0xFF}
		}
		return m.Reply(p)
	}
	s, err := s7fake.Serve(reply)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	code, _, stderr := capture(t, func() int { return cmdSmoke([]string{"-addr", s.Addr(), "-length", "1", "-write-hex", "ff"}) })
	if code != 1 || !strings.Contains(stderr, "write verify mismatch") {
		t.Fatalf("%d, %q", code, stderr)
	}
}

func TestSmokeConnectFailure(t *testing.T) {
	s, err := s7fake.Serve(memory().Reply)
	if err != nil {
		t.Fatal(err)
	}
	addr := s.Addr()
	_ = s.Close()
	code, _, stderr := capture(t, func() int { return cmdSmoke([]string{"-addr", addr, "-timeout", "200ms"}) })
	if code != 1 || !strings.Contains(stderr, "connect:") {
		t.Fatalf("%d, %q", code, stderr)
	}
}

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const readFixture = `{"fixture":{"id":"one"},"expectations":[{"operation":"read","area":{"type":"db","number":1},"result":{"hex":"01"}}]}`

func TestRunCommand(t *testing.T) {
	out := t.TempDir()
	code, stdout, _ := capture(t, func() int {
		return run([]string{"run", "--fixture", writeFixture(t, readFixture), "--native", serve(t, memory()), "--python", serve(t, memory()), "--out", out})
	})
	if code != 0 || !strings.Contains(stdout, "status=pass") || !strings.Contains(stdout, "reports="+out) {
		t.Fatalf("%d, %q", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(out, "report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunCommandSkip(t *testing.T) {
	code, stdout, _ := capture(t, func() int {
		return cmdRun([]string{"--fixture", writeFixture(t, `{"fixture":{"id":"s"},"requires":["area.timers"]}`),
			"--native", serve(t, memory()), "--python", serve(t, memory()), "--out", t.TempDir()})
	})
	if code != 0 || !strings.Contains(stdout, "SKIP unsupported") {
		t.Fatalf("%d, %q", code, stdout)
	}
}

func TestRunCommandDivergence(t *testing.T) {
	other := s7fake.NewMemory(map[s7fake.Area][]byte{db1: {9}})
	code, stdout, stderr := capture(t, func() int {
		return cmdRun([]string{"--fixture", writeFixture(t, readFixture), "--native", serve(t, memory()), "--python", serve(t, other), "--out", t.TempDir()})
	})
	if code != 1 || !strings.Contains(stdout, "DIFF: byte mismatch") || !strings.Contains(stderr, "FAIL:") {
		t.Fatalf("%d, %q, %q", code, stdout, stderr)
	}
}

func TestRunCommandDefaultReportDir(t *testing.T) {
	code, stdout, _ := capture(t, func() int {
		return cmdRun([]string{"--fixture", filepath.Join(t.TempDir(), "missing.json")})
	})
	if code != 1 || !strings.Contains(stdout, filepath.Join("reports", "")) {
		t.Fatalf("%d, %q", code, stdout)
	}
}
