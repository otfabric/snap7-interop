package fixture_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/snap7-interop/internal/fixture"
)

const areaTail = `"access":"read-write"`

// doc is a fixture with the given JSON for areas.
func doc(areas string) string {
	return `{"schemaVersion":"1.0.0","id":"case","description":"d","server":{"rack":0,"slot":2},"areas":[` + areas + `]}`
}

// db is a 64-byte db area with the given extra members.
func db(members string) string {
	if members != "" {
		members = "," + members
	}
	return `{"type":"db","number":1,"size":64,` + areaTail + members + `}`
}

func patch(fields string) string {
	return db(`"values":[{` + fields + `}]`)
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompileEveryDataType(t *testing.T) {
	v := newValidator(t)
	path := writeFile(t, doc(db(`"values":[
		{"offset":0,"dataType":"bool","bit":3,"value":true},
		{"offset":1,"dataType":"bool","bit":0,"value":false},
		{"offset":2,"dataType":"int8","value":-2},
		{"offset":3,"dataType":"uint8","value":200},
		{"offset":4,"dataType":"int16","byteOrder":"big-endian","value":-2},
		{"offset":6,"dataType":"uint16","byteOrder":"little-endian","value":513},
		{"offset":8,"dataType":"int32","byteOrder":"big-endian","value":-2},
		{"offset":12,"dataType":"uint32","byteOrder":"little-endian","value":67305985},
		{"offset":16,"dataType":"int64","byteOrder":"big-endian","value":"-2"},
		{"offset":24,"dataType":"uint64","byteOrder":"little-endian","value":"18446744073709551615"},
		{"offset":32,"dataType":"float32","byteOrder":"big-endian","value":1.5},
		{"offset":36,"dataType":"float64","byteOrder":"big-endian","value":-0.5},
		{"offset":44,"dataType":"bytes","length":2,"value":[1,255]},
		{"offset":46,"dataType":"ascii","length":2,"value":"Hi"}
	]`)))
	cr, err := v.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := base64.StdEncoding.DecodeString(cr.Compiled.Areas[0].DataBase64)
	if err != nil {
		t.Fatal(err)
	}
	f32 := math.Float32bits(1.5)
	f64 := math.Float64bits(-0.5)
	want := make([]byte, 64)
	copy(want, []byte{
		0x08, 0x00, 0xFE, 200,
		0xFF, 0xFE, 0x01, 0x02,
		0xFF, 0xFF, 0xFF, 0xFE, 1, 2, 3, 4,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		byte(f32 >> 24), byte(f32 >> 16), byte(f32 >> 8), byte(f32),
		byte(f64 >> 56), byte(f64 >> 48), byte(f64 >> 40), byte(f64 >> 32), byte(f64 >> 24), byte(f64 >> 16), byte(f64 >> 8), byte(f64),
		1, 255, 'H', 'i',
	})
	if !bytes.Equal(got, want) {
		t.Fatalf("area\n got %x\nwant %x", got, want)
	}
	c := cr.Compiled
	if c.Fixture.Revision != 1 || c.Server.PDUSize != 480 || len(c.Expectations) != 0 {
		t.Fatalf("defaults %+v", c)
	}
}

func TestCompileBulkInitializers(t *testing.T) {
	v := newValidator(t)
	for name, init := range map[string]string{
		"hex":    `"initial":{"hex":"0102"}`,
		"base64": `"initial":{"base64":"AQI="}`,
		"bytes":  `"initial":{"bytes":[1,2]}`,
	} {
		t.Run(name, func(t *testing.T) {
			cr, err := v.CompileFile(writeFile(t, doc(db(init))))
			if err != nil {
				t.Fatal(err)
			}
			got, _ := base64.StdEncoding.DecodeString(cr.Compiled.Areas[0].DataBase64)
			if got[0] != 1 || got[1] != 2 || got[2] != 0 {
				t.Fatalf("area %x", got[:4])
			}
		})
	}
}

func TestCompileKeepsExplicitValues(t *testing.T) {
	v := newValidator(t)
	src := `{"schemaVersion":"1.0.0","id":"case","description":"d","revision":4,"requires":["operation.write","area.db"],
		"server":{"rack":0,"slot":2,"pduSize":960},"areas":[` + db("") + `,{"type":"markers","size":1,` + areaTail + `}],
		"expectations":[{"operation":"read","area":{"type":"db","number":1},"offset":0,"length":1,"result":{"hex":"00"}}]}`
	cr, err := v.CompileFile(writeFile(t, src))
	if err != nil {
		t.Fatal(err)
	}
	c := cr.Compiled
	if c.Fixture.Revision != 4 || c.Server.PDUSize != 960 || c.Requires[0] != "area.db" {
		t.Fatalf("compiled %+v", c)
	}
	if c.Areas[0].Number == nil || *c.Areas[0].Number != 1 || c.Areas[1].Number != nil {
		t.Fatalf("areas %+v", c.Areas)
	}
	if len(c.Expectations) != 1 || c.Expectations[0]["operation"] != "read" {
		t.Fatalf("expectations %+v", c.Expectations)
	}
	var round map[string]any
	if err := json.Unmarshal(cr.Bytes, &round); err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
}

func TestCompileInvalidFixture(t *testing.T) {
	v := newValidator(t)
	_, err := v.CompileFile(writeFile(t, `{}`))
	var r fixture.Result
	if !errors.As(err, &r) || r.Valid {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(err.Error(), "invalid fixture") {
		t.Fatalf("message %q", err)
	}
}

func TestValidateDiagnostics(t *testing.T) {
	v := newValidator(t)
	many := func(n int, item func(i int) string) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = item(i)
		}
		return strings.Join(parts, ",")
	}
	numberedDB := func(i int) string {
		return fmt.Sprintf(`{"type":"db","number":%d,"size":1,%s}`, i+1, areaTail)
	}
	bigDB := func(i int) string {
		return fmt.Sprintf(`{"type":"db","number":%d,"size":%d,%s}`, i+1, fixture.MaxAreaBytes, areaTail)
	}
	bits := func(i int) string {
		return fmt.Sprintf(`{"offset":%d,"dataType":"uint8","value":1}`, i)
	}
	tests := []struct {
		name  string
		input string
		code  string
	}{
		{"not JSON", `not json`, fixture.CodeSchema},
		{"id format", strings.Replace(doc(db("")), `"case"`, `"Bad_ID"`, 1), fixture.CodeSchema},
		{"too many areas", doc(many(fixture.MaxAreas+1, numberedDB)), fixture.CodeAllocationLimit},
		{"total area bytes", doc(many(9, bigDB)), fixture.CodeAllocationLimit},
		{"db without number", doc(`{"type":"db","size":1,` + areaTail + `}`), fixture.CodeSchema},
		{"db number on markers", doc(`{"type":"markers","number":1,"size":1,` + areaTail + `}`), fixture.CodeSchema},
		{"area too large", doc(fmt.Sprintf(`{"type":"db","number":1,"size":%d,%s}`, fixture.MaxAreaBytes+1, areaTail)), fixture.CodeSchema},
		{"too many patches", doc(fmt.Sprintf(`{"type":"db","number":1,"size":%d,%s,"values":[%s]}`, fixture.MaxValuePatches+1, areaTail, many(fixture.MaxValuePatches+1, bits))), fixture.CodeAllocationLimit},
		{"two initializers", doc(db(`"initial":{"hex":"00","base64":"AA=="}`)), fixture.CodeSchema},
		{"bad base64", doc(db(`"initial":{"base64":"@@@@"}`)), fixture.CodeBulkInitializer},
		{"initializer too long", doc(`{"type":"db","number":1,"size":1,` + areaTail + `,"initial":{"hex":"0000"}}`), fixture.CodeBounds},
		{"bytes without length", doc(patch(`"offset":0,"dataType":"bytes","value":[1]`)), fixture.CodeTypeEncoding},
		{"patch out of bounds", doc(patch(`"offset":63,"dataType":"uint16","byteOrder":"big-endian","value":1`)), fixture.CodeBounds},
		{"bool value", doc(patch(`"offset":0,"dataType":"bool","bit":0,"value":1`)), fixture.CodeTypeEncoding},
		{"bool without bit", doc(patch(`"offset":0,"dataType":"bool","value":true`)), fixture.CodeTypeEncoding},
		{"int8 range", doc(patch(`"offset":0,"dataType":"int8","value":128`)), fixture.CodeTypeRange},
		{"uint8 range", doc(patch(`"offset":0,"dataType":"uint8","value":256`)), fixture.CodeTypeRange},
		{"int16 range", doc(patch(`"offset":0,"dataType":"int16","byteOrder":"big-endian","value":32768`)), fixture.CodeTypeRange},
		{"uint16 range", doc(patch(`"offset":0,"dataType":"uint16","byteOrder":"big-endian","value":-1`)), fixture.CodeTypeRange},
		{"int32 range", doc(patch(`"offset":0,"dataType":"int32","byteOrder":"big-endian","value":2147483648`)), fixture.CodeTypeRange},
		{"uint32 range", doc(patch(`"offset":0,"dataType":"uint32","byteOrder":"big-endian","value":4294967296`)), fixture.CodeTypeRange},
		{"int64 text", doc(patch(`"offset":0,"dataType":"int64","byteOrder":"big-endian","value":"x"`)), fixture.CodeTypeRange},
		{"uint64 text", doc(patch(`"offset":0,"dataType":"uint64","byteOrder":"big-endian","value":"-1"`)), fixture.CodeTypeRange},
		{"fraction", doc(patch(`"offset":0,"dataType":"int8","value":1.5`)), fixture.CodeTypeRange},
		{"uint8 from a bool", doc(patch(`"offset":0,"dataType":"uint8","value":true`)), fixture.CodeTypeRange},
		{"uint64 fraction", doc(patch(`"offset":0,"dataType":"uint64","byteOrder":"big-endian","value":1.5`)), fixture.CodeTypeRange},
		{"uint64 from a bool", doc(patch(`"offset":0,"dataType":"uint64","byteOrder":"big-endian","value":true`)), fixture.CodeTypeRange},
		{"float value", doc(patch(`"offset":0,"dataType":"float32","byteOrder":"big-endian","value":"x"`)), fixture.CodeTypeEncoding},
		{"no byte order", doc(patch(`"offset":0,"dataType":"uint16","value":1`)), fixture.CodeTypeEncoding},
		{"bad byte order", doc(patch(`"offset":0,"dataType":"uint16","byteOrder":"middle","value":1`)), fixture.CodeSchema},
		{"bytes value", doc(patch(`"offset":0,"dataType":"bytes","length":1,"value":"x"`)), fixture.CodeTypeEncoding},
		{"bytes length", doc(patch(`"offset":0,"dataType":"bytes","length":2,"value":[1]`)), fixture.CodeTypeEncoding},
		{"bytes element", doc(patch(`"offset":0,"dataType":"bytes","length":1,"value":[256]`)), fixture.CodeTypeEncoding},
		{"ascii value", doc(patch(`"offset":0,"dataType":"ascii","length":1,"value":1`)), fixture.CodeTypeEncoding},
		{"ascii length", doc(patch(`"offset":0,"dataType":"ascii","length":2,"value":"x"`)), fixture.CodeTypeEncoding},
		{"ascii 8-bit", doc(patch(`"offset":0,"dataType":"ascii","length":2,"value":"é"`)), fixture.CodeTypeRange},
		{"overlap", doc(db(`"initial":{"hex":"0000"},"values":[{"offset":1,"dataType":"uint8","value":1}]`)), fixture.CodeOverlap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := v.ValidateFile(writeFile(t, tt.input))
			if r.Valid {
				t.Fatal("expected an invalid fixture")
			}
			for _, d := range r.Diagnostics {
				if d.Code == tt.code {
					return
				}
			}
			t.Fatalf("want %s, got %+v", tt.code, r.Diagnostics)
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	v := newValidator(t)
	for name, input := range map[string]string{
		"replace over an initializer": doc(db(`"initial":{"hex":"0000"},"values":[{"offset":1,"dataType":"uint8","value":1,"replace":true}]`)),
		"bits of one byte":            doc(db(`"values":[{"offset":0,"dataType":"bool","bit":0,"value":true},{"offset":0,"dataType":"bool","bit":1,"value":true}]`)),
		"bytes initializer":           doc(db(`"initial":{"bytes":[1,2]}`)),
	} {
		t.Run(name, func(t *testing.T) {
			if r := v.ValidateFile(writeFile(t, input)); !r.Valid {
				t.Fatal(r.Diagnostics)
			}
		})
	}
}

func TestValidateMissingFile(t *testing.T) {
	r := newValidator(t).ValidateFile(filepath.Join(t.TempDir(), "missing.json"))
	if r.Valid || r.Diagnostics[0].Code != fixture.CodeIO {
		t.Fatalf("result %+v", r)
	}
}

func TestNewValidatorErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := fixture.NewValidator(filepath.Join(dir, "missing.json")); err == nil || !strings.Contains(err.Error(), "open schema") {
		t.Fatalf("missing: %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.NewValidator(bad); err == nil || !strings.Contains(err.Error(), "parse schema") {
		t.Fatalf("not JSON: %v", err)
	}
	wrong := filepath.Join(dir, "wrong.json")
	if err := os.WriteFile(wrong, []byte(`{"type":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.NewValidator(wrong); err == nil || !strings.Contains(err.Error(), "compile schema") {
		t.Fatalf("not a schema: %v", err)
	}
}

func TestDiagnosticStrings(t *testing.T) {
	d := fixture.Diagnostic{Code: "E_X", Message: "m"}
	if d.String() != "E_X: m" {
		t.Fatalf("%q", d.String())
	}
	d.Path = "/a"
	if d.String() != "E_X: m (/a)" {
		t.Fatalf("%q", d.String())
	}
	if got := (fixture.Result{Valid: true}).Error(); got != "" {
		t.Fatalf("valid result: %q", got)
	}
	got := fixture.Result{File: "f", Diagnostics: []fixture.Diagnostic{d}}.Error()
	if got != "f: invalid fixture\nE_X: m (/a)" {
		t.Fatalf("%q", got)
	}
}

type failWriter struct{ after int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.after == 0 {
		return 0, errors.New("sink failed")
	}
	w.after--
	return len(p), nil
}

func TestWriteText(t *testing.T) {
	results := []fixture.Result{
		{File: "a", Valid: true},
		{File: "b", Diagnostics: []fixture.Diagnostic{{Code: "E_X", Message: "m"}}},
	}
	var buf bytes.Buffer
	if err := fixture.WriteText(&buf, results); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "OK  a\nFAIL b\n  E_X: m\n"; got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	for after := 0; after < 3; after++ {
		if err := fixture.WriteText(&failWriter{after: after}, results); err == nil {
			t.Fatalf("after %d writes: want an error", after)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	in := []fixture.Result{{File: "a", Valid: true}}
	if err := fixture.WriteJSON(&buf, in); err != nil {
		t.Fatal(err)
	}
	var got []fixture.Result
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].File != "a" || !got[0].Valid {
		t.Fatalf("%+v", got)
	}
}
