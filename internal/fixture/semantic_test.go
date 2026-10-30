package fixture

import (
	"math"
	"strings"
	"testing"
)

func intp(n int) *int { return &n }

func hasCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// The schema rejects most of these first. The semantic checks stand on their
// own, for callers that decode a fixture without it.
func TestSemanticValidate(t *testing.T) {
	ok := func() SourceFixture {
		return SourceFixture{ID: "ok", Areas: []Area{{Type: "db", Number: intp(1), Size: 1}}}
	}
	tests := []struct {
		name   string
		mutate func(*SourceFixture)
		code   string
	}{
		{"id", func(s *SourceFixture) { s.ID = "Bad_ID" }, CodeIDFormat},
		{"duplicate", func(s *SourceFixture) { s.Areas = append(s.Areas, s.Areas[0]) }, CodeDuplicateArea},
		{"no number", func(s *SourceFixture) { s.Areas[0].Number = nil }, CodeMissingDBNumber},
		{"number 0", func(s *SourceFixture) { s.Areas[0].Number = intp(0) }, CodeInvalidDBNumber},
		{"number too large", func(s *SourceFixture) { s.Areas[0].Number = intp(65536) }, CodeInvalidDBNumber},
		{"number on markers", func(s *SourceFixture) { s.Areas[0].Type = "markers" }, CodeDBNumberOnNonDB},
		{"size 0", func(s *SourceFixture) { s.Areas[0].Size = 0 }, CodeAllocationLimit},
		{"size too large", func(s *SourceFixture) { s.Areas[0].Size = MaxAreaBytes + 1 }, CodeAllocationLimit},
		{"too many areas", func(s *SourceFixture) { s.Areas = make([]Area, MaxAreas+1) }, CodeAllocationLimit},
		{"too many expectations", func(s *SourceFixture) { s.Expectations = make([]Expectation, MaxExpectations+1) }, CodeAllocationLimit},
		{"negative offset", func(s *SourceFixture) {
			s.Areas[0].Values = []ValuePatch{{Offset: -1, DataType: "uint8", Value: float64(1)}}
		}, CodeBounds},
	}
	if diags := semanticValidate(ok()); len(diags) != 0 {
		t.Fatalf("valid fixture: %+v", diags)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ok()
			tt.mutate(&s)
			if diags := semanticValidate(s); !hasCode(diags, tt.code) {
				t.Fatalf("want %s, got %+v", tt.code, diags)
			}
		})
	}
}

func TestBulkLength(t *testing.T) {
	tests := []struct {
		name string
		init BulkInitializer
		want int
		code string
	}{
		{"hex", BulkInitializer{Hex: "0102"}, 2, ""},
		{"base64", BulkInitializer{Base64: "AQI="}, 2, ""},
		{"bytes", BulkInitializer{Bytes: []int{1, 2, 3}}, 3, ""},
		{"none", BulkInitializer{}, 0, CodeBulkInitializer},
		{"two", BulkInitializer{Hex: "00", Bytes: []int{1}}, 0, CodeBulkInitializer},
		{"odd hex", BulkInitializer{Hex: "012"}, 0, CodeHexLength},
		{"bad hex", BulkInitializer{Hex: "zz"}, 0, CodeHexLength},
		{"bad base64", BulkInitializer{Base64: "@"}, 0, CodeBulkInitializer},
		{"byte too large", BulkInitializer{Bytes: []int{256}}, 0, CodeBulkInitializer},
		{"negative byte", BulkInitializer{Bytes: []int{-1}}, 0, CodeBulkInitializer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, d := bulkLength(&tt.init, "/p")
			if tt.code == "" {
				if d != nil || got != tt.want {
					t.Fatalf("%d, %+v", got, d)
				}
				return
			}
			if d == nil || d.Code != tt.code {
				t.Fatalf("diagnostic %+v, want %s", d, tt.code)
			}
		})
	}
}

func TestValidateAreaContentCodes(t *testing.T) {
	area := func(p ValuePatch) Area { return Area{Size: 8, Values: []ValuePatch{p}} }
	tests := []struct {
		name string
		area Area
		code string
	}{
		{"unknown data type", area(ValuePatch{DataType: "int128"}), CodeTypeEncoding},
		{"bad initializer", Area{Size: 8, Initial: &BulkInitializer{}}, CodeBulkInitializer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diags := validateAreaContent(tt.area, "/a"); !hasCode(diags, tt.code) {
				t.Fatalf("want %s, got %+v", tt.code, diags)
			}
		})
	}
	if err := validateTypedValue(ValuePatch{DataType: "int128"}); err == nil {
		t.Fatal("unknown data type accepted")
	}
}

// applyPatch runs on validated fixtures; these are the guards behind that.
func TestApplyPatchErrors(t *testing.T) {
	bit := 0
	tests := []struct {
		name  string
		patch ValuePatch
		want  string
	}{
		{"unknown type", ValuePatch{DataType: "int128"}, "unsupported dataType"},
		{"out of bounds", ValuePatch{Offset: 8, DataType: "uint8"}, "out of bounds"},
		{"negative offset", ValuePatch{Offset: -1, DataType: "uint8"}, "out of bounds"},
		{"bool value", ValuePatch{DataType: "bool", Bit: &bit, Value: 1}, "bool value type"},
		{"bool bit", ValuePatch{DataType: "bool", Value: true}, "bool requires bit"},
		{"int8", ValuePatch{DataType: "int8", Value: true}, "expected integer"},
		{"uint8", ValuePatch{DataType: "uint8", Value: true}, "expected integer"},
		{"int16", ValuePatch{DataType: "int16", Value: true}, "expected integer"},
		{"uint16", ValuePatch{DataType: "uint16", Value: true}, "expected integer"},
		{"int32", ValuePatch{DataType: "int32", Value: true}, "expected integer"},
		{"uint32", ValuePatch{DataType: "uint32", Value: true}, "expected unsigned"},
		{"int64", ValuePatch{DataType: "int64", Value: true}, "expected integer"},
		{"uint64", ValuePatch{DataType: "uint64", Value: true}, "expected unsigned"},
		{"float32", ValuePatch{DataType: "float32", Value: "x"}, "float32 value type"},
		{"float64", ValuePatch{DataType: "float64", Value: "x"}, "float64 value type"},
		{"bytes", ValuePatch{DataType: "bytes", Length: 1, Value: "x"}, "bytes value type"},
		{"bytes element", ValuePatch{DataType: "bytes", Length: 1, Value: []any{true}}, "expected integer"},
		{"ascii", ValuePatch{DataType: "ascii", Length: 1, Value: 1}, "ascii value type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := applyPatch(make([]byte, 8), tt.patch); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMaterializeAreaErrors(t *testing.T) {
	if _, err := materializeArea(Area{Size: 1, Initial: &BulkInitializer{}}); err == nil || !strings.Contains(err.Error(), "empty bulk") {
		t.Fatalf("err %v", err)
	}
	if _, err := materializeArea(Area{Size: 1, Initial: &BulkInitializer{Hex: "0000"}}); err == nil || !strings.Contains(err.Error(), "exceeds area size") {
		t.Fatalf("err %v", err)
	}
	if _, err := materializeArea(Area{Size: 1, Values: []ValuePatch{{DataType: "x"}}}); err == nil || !strings.Contains(err.Error(), "values[0]") {
		t.Fatalf("err %v", err)
	}
}

func TestNormalizeExpectations(t *testing.T) {
	got, err := normalizeExpectations([]byte(`{}`))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("absent: %v, %v", got, err)
	}
	for name, in := range map[string]string{
		"not JSON":      `{`,
		"not an array":  `{"expectations":1}`,
		"not an object": `{"expectations":[1]}`,
	} {
		if _, err := normalizeExpectations([]byte(in)); err == nil {
			t.Fatalf("%s: want an error", name)
		}
	}
}

func TestEncodeCanonical(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{map[string]any{"b": 1.0, "a": []any{nil, true, "x"}}, `{"a":[null,true,"x"],"b":1}`},
		{1.5, `1.5`},
		{1e16, `10000000000000000`},
	}
	for _, tt := range tests {
		got, err := encodeCanonical(tt.in)
		if err != nil || string(got) != tt.want {
			t.Fatalf("%v: %s, %v", tt.in, got, err)
		}
	}
	if _, err := encodeCanonical(math.Inf(1)); err == nil {
		t.Fatal("infinity accepted")
	}
	if _, err := encodeCanonical(map[string]any{"a": math.Inf(1)}); err == nil {
		t.Fatal("infinity in a map accepted")
	}
	if _, err := encodeCanonical([]any{math.Inf(1)}); err == nil {
		t.Fatal("infinity in an array accepted")
	}
	if _, err := marshalCanonical(struct{}{}); err != nil {
		t.Fatal(err)
	}
}

func TestToJSONValuePanicsOnUnencodable(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want a panic")
		}
	}()
	toJSONValue(make(chan int))
}
