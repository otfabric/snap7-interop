package interop

import (
	"encoding/json"
	"strings"
	"testing"
)

// Regression: an invalid fixture wrote a <failure> element under failures="0".
func TestJUnitCountsInvalidAsFailure(t *testing.T) {
	got := junitReport(Report{FixtureID: "x", Status: "invalid"})
	if !strings.Contains(got, `failures="1"`) || !strings.Contains(got, "<failure") {
		t.Fatalf("junit:\n%s", got)
	}
}

func TestMarkdownSteps(t *testing.T) {
	rep := Report{
		Native: EndpointResult{Steps: []StepResult{{Operation: "read"}}},
		Python: EndpointResult{Steps: []StepResult{{Operation: "read"}, {Operation: "write"}}},
		Diffs:  []Diff{{Kind: "length", Message: "step count differs"}},
	}
	got := markdownReport(rep)
	if !strings.Contains(got, "| 1 | write |") || !strings.Contains(got, "step count differs") {
		t.Fatalf("report.md:\n%s", got)
	}
}

func TestJUnitEscapes(t *testing.T) {
	got := junitReport(Report{FixtureID: `a<b&"c'`, Status: "fail", Diffs: []Diff{{Message: "x>y"}}})
	for _, want := range []string{`a&lt;b&amp;&quot;c&apos;`, `x&gt;y`} {
		if !strings.Contains(got, want) {
			t.Fatalf("junit lacks %q:\n%s", want, got)
		}
	}
	if got := junitReport(Report{Status: "fail"}); !strings.Contains(got, `message="interop failed"`) {
		t.Fatalf("junit:\n%s", got)
	}
}

func TestAsIntForms(t *testing.T) {
	if asInt(float64(3), 0) != 3 || asInt(4, 0) != 4 || asInt(json.Number("5"), 0) != 5 {
		t.Fatal("number forms")
	}
	if asInt(json.Number("x"), 7) != 7 || asInt("6", 7) != 7 || asInt(nil, 7) != 7 {
		t.Fatal("defaults")
	}
}
