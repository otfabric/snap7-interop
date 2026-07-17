package fixture_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileDeterministic(t *testing.T) {
	v := newValidator(t)
	path := filepath.Join(repoRoot(t), "fixtures", "canonical", "basic-db.json")

	a, err := v.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes, b.Bytes) {
		t.Fatal("compiled output is not deterministic")
	}
	if a.Compiled.Fixture.SourceSha256 == "" || a.Compiled.Fixture.CompiledSha256 == "" {
		t.Fatal("digests must be populated")
	}
	if a.Compiled.Fixture.CompiledSha256 == a.Compiled.Fixture.SourceSha256 {
		t.Fatal("source and compiled digests should differ")
	}
}

func TestCompileBasicDBBytes(t *testing.T) {
	v := newValidator(t)
	path := filepath.Join(repoRoot(t), "fixtures", "canonical", "basic-db.json")
	cr, err := v.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cr.Compiled.Areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(cr.Compiled.Areas))
	}
	raw, err := base64.StdEncoding.DecodeString(cr.Compiled.Areas[0].DataBase64)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 64 {
		t.Fatalf("expected 64 bytes, got %d", len(raw))
	}
	wantPrefix := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	if !bytes.Equal(raw[:8], wantPrefix) {
		t.Fatalf("prefix = %x, want %x", raw[:8], wantPrefix)
	}
	for i := 8; i < len(raw); i++ {
		if raw[i] != 0 {
			t.Fatalf("byte %d = %x, want 0", i, raw[i])
		}
	}
}

func TestCompileTypedValues(t *testing.T) {
	v := newValidator(t)
	path := filepath.Join(repoRoot(t), "fixtures", "canonical", "typed-values.json")
	cr, err := v.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(cr.Compiled.Areas[0].DataBase64)
	if err != nil {
		t.Fatal(err)
	}
	// uint16 4660 = 0x1234 big-endian
	if raw[0] != 0x12 || raw[1] != 0x34 {
		t.Fatalf("uint16 bytes = %02x%02x, want 1234", raw[0], raw[1])
	}
	// float32 12.5 big-endian = 0x41480000
	if raw[2] != 0x41 || raw[3] != 0x48 || raw[4] != 0x00 || raw[5] != 0x00 {
		t.Fatalf("float32 bytes = %x, want 41480000", raw[2:6])
	}
	// bool bit 0 at offset 6
	if raw[6]&0x01 == 0 {
		t.Fatalf("bool bit not set: %02x", raw[6])
	}
}

func TestGoldenCompiledFixtures(t *testing.T) {
	v := newValidator(t)
	root := repoRoot(t)
	entries, err := filepath.Glob(filepath.Join(root, "fixtures", "canonical", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range entries {
		name := filepath.Base(src)
		id := name[:len(name)-len(filepath.Ext(name))]
		golden := filepath.Join(root, "fixtures", "compiled", id+".compiled.json")
		t.Run(id, func(t *testing.T) {
			cr, err := v.CompileFile(src)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run make fixtures-update): %v", err)
			}
			if !bytes.Equal(cr.Bytes, want) {
				t.Fatalf("compiled output drifted from golden\n--- got ---\n%s\n--- want ---\n%s", cr.Bytes, want)
			}
			// Ensure golden is valid JSON object.
			var doc map[string]any
			if err := json.Unmarshal(want, &doc); err != nil {
				t.Fatal(err)
			}
		})
	}
}
