package fixture

import (
	"strings"
	"testing"
)

// Regression: float64(math.MaxInt64) is 2^63, so 2^63 passed the range check
// and its conversion to int64 depended on the platform. 2^64 did the same for
// uint64. Larger values need the string form, which is exact.
func TestIntegerRangeAtTheFloat64Limit(t *testing.T) {
	if _, err := asInt64(float64(1 << 63)); err == nil || !strings.Contains(err.Error(), "int64 range") {
		t.Fatalf("2^63: %v", err)
	}
	if _, err := asUint64(float64(1<<63) * 2); err == nil || !strings.Contains(err.Error(), "uint64 range") {
		t.Fatalf("2^64: %v", err)
	}
	if got, err := asInt64(-float64(1 << 63)); err != nil || got != -1<<63 {
		t.Fatalf("-2^63: %d, %v", got, err)
	}
	if got, err := asInt64("9223372036854775807"); err != nil || got != 1<<63-1 {
		t.Fatalf("string form: %d, %v", got, err)
	}
	if got, err := asUint64("18446744073709551615"); err != nil || got != 1<<64-1 {
		t.Fatalf("string form: %d, %v", got, err)
	}
}
