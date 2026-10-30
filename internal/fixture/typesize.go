package fixture

import (
	"fmt"
	"math"
	"strconv"
)

func dataTypeSize(p ValuePatch) (int, error) {
	switch p.DataType {
	case "bool":
		return 1, nil
	case "int8", "uint8":
		return 1, nil
	case "int16", "uint16":
		return 2, nil
	case "int32", "uint32", "float32":
		return 4, nil
	case "int64", "uint64", "float64":
		return 8, nil
	case "bytes", "ascii":
		if p.Length < 1 {
			return 0, fmt.Errorf("dataType %s requires length >= 1", p.DataType)
		}
		return p.Length, nil
	default:
		return 0, fmt.Errorf("unsupported dataType %q", p.DataType)
	}
}

func needsByteOrder(dataType string) bool {
	switch dataType {
	case "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64":
		return true
	default:
		return false
	}
}

func validateTypedValue(p ValuePatch) error {
	switch p.DataType {
	case "bool":
		if _, ok := p.Value.(bool); !ok {
			return fmt.Errorf("bool value must be a JSON boolean")
		}
		if p.Bit == nil {
			return fmt.Errorf("bool requires bit index")
		}
	case "int8":
		n, err := asInt64(p.Value)
		if err != nil {
			return err
		}
		if n < math.MinInt8 || n > math.MaxInt8 {
			return fmt.Errorf("int8 out of range: %d", n)
		}
	case "uint8":
		n, err := asInt64(p.Value)
		if err != nil {
			return err
		}
		if n < 0 || n > math.MaxUint8 {
			return fmt.Errorf("uint8 out of range: %d", n)
		}
	case "int16":
		n, err := asInt64(p.Value)
		if err != nil {
			return err
		}
		if n < math.MinInt16 || n > math.MaxInt16 {
			return fmt.Errorf("int16 out of range: %d", n)
		}
	case "uint16":
		n, err := asInt64(p.Value)
		if err != nil {
			return err
		}
		if n < 0 || n > math.MaxUint16 {
			return fmt.Errorf("uint16 out of range: %d", n)
		}
	case "int32":
		n, err := asInt64(p.Value)
		if err != nil {
			return err
		}
		if n < math.MinInt32 || n > math.MaxInt32 {
			return fmt.Errorf("int32 out of range: %d", n)
		}
	case "uint32":
		n, err := asUint64(p.Value)
		if err != nil {
			return err
		}
		if n > math.MaxUint32 {
			return fmt.Errorf("uint32 out of range: %d", n)
		}
	case "int64":
		if _, err := asInt64(p.Value); err != nil {
			return err
		}
	case "uint64":
		if _, err := asUint64(p.Value); err != nil {
			return err
		}
	case "float32", "float64":
		if _, ok := p.Value.(float64); !ok {
			return fmt.Errorf("%s value must be a JSON number", p.DataType)
		}
	case "bytes":
		arr, ok := p.Value.([]any)
		if !ok {
			return fmt.Errorf("bytes value must be an array of integers")
		}
		if len(arr) != p.Length {
			return fmt.Errorf("bytes length mismatch: declared %d, got %d", p.Length, len(arr))
		}
		for i, v := range arr {
			n, err := asInt64(v)
			if err != nil || n < 0 || n > 255 {
				return fmt.Errorf("bytes[%d] must be 0..255", i)
			}
		}
	case "ascii":
		s, ok := p.Value.(string)
		if !ok {
			return fmt.Errorf("ascii value must be a string")
		}
		if len(s) != p.Length {
			return fmt.Errorf("ascii length mismatch: declared %d, got %d", p.Length, len(s))
		}
		for i := 0; i < len(s); i++ {
			if s[i] > 0x7f {
				return fmt.Errorf("ascii[%d] is not 7-bit ASCII", i)
			}
		}
	default:
		return fmt.Errorf("unsupported dataType %q", p.DataType)
	}

	if needsByteOrder(p.DataType) && p.ByteOrder == "" {
		return fmt.Errorf("dataType %s requires byteOrder", p.DataType)
	}
	if p.ByteOrder != "" && p.ByteOrder != "big-endian" && p.ByteOrder != "little-endian" {
		return fmt.Errorf("invalid byteOrder %q", p.ByteOrder)
	}
	return nil
}

func asInt64(v any) (int64, error) {
	switch n := v.(type) {
	case float64:
		if n != math.Trunc(n) {
			return 0, fmt.Errorf("expected integer, got %v", n)
		}
		if n >= float64(math.MaxInt64) || n < float64(math.MinInt64) { // float64(MaxInt64) is 2^63
			return 0, fmt.Errorf("integer out of int64 range")
		}
		return int64(n), nil
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("expected integer, got %T", v)
	}
}

func asUint64(v any) (uint64, error) {
	switch n := v.(type) {
	case float64:
		if n < 0 || n != math.Trunc(n) {
			return 0, fmt.Errorf("expected unsigned integer, got %v", n)
		}
		if n >= float64(math.MaxUint64) { // float64(MaxUint64) is 2^64
			return 0, fmt.Errorf("integer out of uint64 range")
		}
		return uint64(n), nil
	case string:
		return strconv.ParseUint(n, 10, 64)
	default:
		return 0, fmt.Errorf("expected unsigned integer, got %T", v)
	}
}

// occupiedRange describes a half-open byte interval [start, end).
type occupiedRange struct {
	start int
	end   int
	path  string
}

func rangesOverlap(a, b occupiedRange) bool {
	return a.start < b.end && b.start < a.end
}
