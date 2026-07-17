package fixture

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
)

const (
	CompiledFormat        = "snap-interop-compiled-fixture"
	CompiledFormatVersion = "1.0.0"
	CompilerVersion       = "0.1.0"
	SchemaVersion         = "1.0.0"
)

// CompiledFixture is the byte-stable artifact consumed by server adapters.
type CompiledFixture struct {
	Format        string           `json:"format"`
	FormatVersion string           `json:"formatVersion"`
	Fixture       CompiledMeta     `json:"fixture"`
	Server        CompiledServer   `json:"server"`
	Areas         []CompiledArea   `json:"areas"`
	Expectations  []map[string]any `json:"expectations"`
	Requires      []string         `json:"requires"`
}

type CompiledMeta struct {
	ID              string `json:"id"`
	Revision        int    `json:"revision"`
	SchemaVersion   string `json:"schemaVersion"`
	CompilerVersion string `json:"compilerVersion"`
	SourceSha256    string `json:"sourceSha256"`
	CompiledSha256  string `json:"compiledSha256"`
	Description     string `json:"description"`
}

type CompiledServer struct {
	Rack    int `json:"rack"`
	Slot    int `json:"slot"`
	PDUSize int `json:"pduSize"`
}

type CompiledArea struct {
	Type       string `json:"type"`
	Number     *int   `json:"number,omitempty"`
	Size       int    `json:"size"`
	Access     string `json:"access"`
	DataBase64 string `json:"dataBase64"`
}

// CompileResult is the outcome of compiling one source fixture.
type CompileResult struct {
	SourcePath string
	Compiled   *CompiledFixture
	Bytes      []byte // canonical JSON including compiledSha256
}

// CompileFile validates and compiles a source fixture into canonical JSON bytes.
func (v *Validator) CompileFile(path string) (*CompileResult, error) {
	vr := v.ValidateFile(path)
	if !vr.Valid {
		return nil, fmt.Errorf("%w", vr)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sourceDigest := sha256.Sum256(raw)

	var src SourceFixture
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, err
	}

	revision := src.Revision
	if revision == 0 {
		revision = 1
	}
	pdu := src.Server.PDUSize
	if pdu == 0 {
		pdu = 480
	}

	areas := make([]CompiledArea, 0, len(src.Areas))
	for i, area := range src.Areas {
		data, err := materializeArea(area)
		if err != nil {
			return nil, fmt.Errorf("/areas/%d: %w", i, err)
		}
		ca := CompiledArea{
			Type:       area.Type,
			Size:       area.Size,
			Access:     area.Access,
			DataBase64: base64.StdEncoding.EncodeToString(data),
		}
		if area.Type == "db" && area.Number != nil {
			n := *area.Number
			ca.Number = &n
		}
		areas = append(areas, ca)
	}

	requires := append([]string(nil), src.Requires...)
	sort.Strings(requires)

	expectations, err := normalizeExpectations(raw)
	if err != nil {
		return nil, err
	}

	compiled := &CompiledFixture{
		Format:        CompiledFormat,
		FormatVersion: CompiledFormatVersion,
		Fixture: CompiledMeta{
			ID:              src.ID,
			Revision:        revision,
			SchemaVersion:   SchemaVersion,
			CompilerVersion: CompilerVersion,
			SourceSha256:    hex.EncodeToString(sourceDigest[:]),
			CompiledSha256:  "", // filled after canonical encoding without this field
			Description:     src.Description,
		},
		Server: CompiledServer{
			Rack:    src.Server.Rack,
			Slot:    src.Server.Slot,
			PDUSize: pdu,
		},
		Areas:        areas,
		Expectations: expectations,
		Requires:     requires,
	}

	canonical, err := marshalCanonicalWithDigest(compiled)
	if err != nil {
		return nil, err
	}

	return &CompileResult{
		SourcePath: path,
		Compiled:   compiled,
		Bytes:      canonical,
	}, nil
}

func materializeArea(area Area) ([]byte, error) {
	buf := make([]byte, area.Size)

	if area.Initial != nil {
		payload, err := decodeBulk(area.Initial)
		if err != nil {
			return nil, err
		}
		if len(payload) > len(buf) {
			return nil, fmt.Errorf("bulk initializer exceeds area size")
		}
		copy(buf, payload)
	}

	for i, patch := range area.Values {
		if err := applyPatch(buf, patch); err != nil {
			return nil, fmt.Errorf("values[%d]: %w", i, err)
		}
	}
	return buf, nil
}

func decodeBulk(init *BulkInitializer) ([]byte, error) {
	switch {
	case init.Hex != "":
		return hex.DecodeString(init.Hex)
	case init.Base64 != "":
		return base64.StdEncoding.DecodeString(init.Base64)
	case init.Bytes != nil:
		out := make([]byte, len(init.Bytes))
		for i, v := range init.Bytes {
			out[i] = byte(v)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("empty bulk initializer")
	}
}

func applyPatch(buf []byte, patch ValuePatch) error {
	size, err := dataTypeSize(patch)
	if err != nil {
		return err
	}
	if patch.Offset < 0 || patch.Offset+size > len(buf) {
		return fmt.Errorf("patch out of bounds")
	}

	var order binary.ByteOrder = binary.BigEndian
	if patch.ByteOrder == "little-endian" {
		order = binary.LittleEndian
	}

	switch patch.DataType {
	case "bool":
		v, ok := patch.Value.(bool)
		if !ok {
			return fmt.Errorf("bool value type")
		}
		if patch.Bit == nil {
			return fmt.Errorf("bool requires bit")
		}
		bit := *patch.Bit
		if v {
			buf[patch.Offset] |= 1 << bit
		} else {
			buf[patch.Offset] &^= 1 << bit
		}
	case "int8":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		buf[patch.Offset] = byte(int8(n))
	case "uint8":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		buf[patch.Offset] = byte(n)
	case "int16":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint16(buf[patch.Offset:], uint16(int16(n)))
	case "uint16":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint16(buf[patch.Offset:], uint16(n))
	case "int32":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint32(buf[patch.Offset:], uint32(int32(n)))
	case "uint32":
		n, err := asUint64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint32(buf[patch.Offset:], uint32(n))
	case "int64":
		n, err := asInt64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint64(buf[patch.Offset:], uint64(n))
	case "uint64":
		n, err := asUint64(patch.Value)
		if err != nil {
			return err
		}
		order.PutUint64(buf[patch.Offset:], n)
	case "float32":
		f, ok := patch.Value.(float64)
		if !ok {
			return fmt.Errorf("float32 value type")
		}
		order.PutUint32(buf[patch.Offset:], math.Float32bits(float32(f)))
	case "float64":
		f, ok := patch.Value.(float64)
		if !ok {
			return fmt.Errorf("float64 value type")
		}
		order.PutUint64(buf[patch.Offset:], math.Float64bits(f))
	case "bytes":
		arr, ok := patch.Value.([]any)
		if !ok {
			return fmt.Errorf("bytes value type")
		}
		for i, v := range arr {
			n, err := asInt64(v)
			if err != nil {
				return err
			}
			buf[patch.Offset+i] = byte(n)
		}
	case "ascii":
		s, ok := patch.Value.(string)
		if !ok {
			return fmt.Errorf("ascii value type")
		}
		copy(buf[patch.Offset:], []byte(s))
	default:
		return fmt.Errorf("unsupported dataType %q", patch.DataType)
	}
	return nil
}

func normalizeExpectations(raw []byte) ([]map[string]any, error) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	rawExp, ok := top["expectations"]
	if !ok || rawExp == nil {
		return []map[string]any{}, nil
	}
	arr, ok := rawExp.([]any)
	if !ok {
		return nil, fmt.Errorf("expectations must be an array")
	}
	out := make([]map[string]any, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expectations[%d] must be an object", i)
		}
		out = append(out, canonicalizeJSONValue(m).(map[string]any))
	}
	return out, nil
}

// marshalCanonicalWithDigest encodes compiled JSON deterministically and sets compiledSha256.
func marshalCanonicalWithDigest(c *CompiledFixture) ([]byte, error) {
	c.Fixture.CompiledSha256 = ""
	withoutDigest, err := marshalCanonical(c)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(withoutDigest)
	c.Fixture.CompiledSha256 = hex.EncodeToString(sum[:])
	return marshalCanonical(c)
}

func marshalCanonical(v any) ([]byte, error) {
	normalized := canonicalizeJSONValue(toJSONValue(v))
	b, err := encodeCanonical(normalized)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func toJSONValue(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func canonicalizeJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = canonicalizeJSONValue(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = canonicalizeJSONValue(t[i])
		}
		return out
	default:
		return t
	}
}

func encodeCanonical(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf = append(buf, kb...)
			buf = append(buf, ':')
			vb, err := encodeCanonical(t[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, '}')
		return buf, nil
	case []any:
		buf := []byte{'['}
		for i, item := range t {
			if i > 0 {
				buf = append(buf, ',')
			}
			vb, err := encodeCanonical(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, ']')
		return buf, nil
	case json.Number:
		return []byte(t.String()), nil
	case float64:
		// Prefer integer encoding when exact.
		if t == math.Trunc(t) && !math.IsInf(t, 0) && t >= -1e15 && t <= 1e15 {
			return []byte(strconv.FormatInt(int64(t), 10)), nil
		}
		return json.Marshal(t)
	case nil:
		return []byte("null"), nil
	default:
		return json.Marshal(t)
	}
}
