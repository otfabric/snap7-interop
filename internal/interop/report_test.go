package interop

import (
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
