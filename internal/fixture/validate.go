package fixture

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var fixtureIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Validator validates source fixtures against schema and semantic rules.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator loads the committed JSON Schema from schemaPath.
func NewValidator(schemaPath string) (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	f, err := os.Open(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("open schema: %w", err)
	}
	defer func() { _ = f.Close() }()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return nil, fmt.Errorf("parse schema json: %w", err)
	}
	if err := compiler.AddResource(schemaPath, doc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Validator{schema: schema}, nil
}

// ValidateFile validates one fixture file.
func (v *Validator) ValidateFile(path string) Result {
	result := Result{File: path, Valid: true}

	raw, err := os.ReadFile(path)
	if err != nil {
		return invalid(result, Diagnostic{Code: CodeIO, Message: err.Error()})
	}

	var instance any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&instance); err != nil {
		return invalid(result, Diagnostic{
			Code:    CodeSchema,
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
	}

	// Re-decode without UseNumber for schema lib compatibility, then semantic with numbers as float64.
	var schemaInstance any
	if err := json.Unmarshal(raw, &schemaInstance); err != nil {
		return invalid(result, Diagnostic{
			Code:    CodeSchema,
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
	}

	if err := v.schema.Validate(schemaInstance); err != nil {
		for _, d := range schemaDiagnostics(err) {
			result = invalid(result, d)
		}
		return result
	}

	var src SourceFixture
	if err := json.Unmarshal(raw, &src); err != nil {
		return invalid(result, Diagnostic{
			Code:    CodeSchema,
			Message: fmt.Sprintf("decode fixture model: %v", err),
		})
	}

	for _, d := range semanticValidate(src) {
		result = invalid(result, d)
	}
	return result
}

func invalid(r Result, d Diagnostic) Result {
	r.Valid = false
	r.Diagnostics = append(r.Diagnostics, d)
	return r
}

func schemaDiagnostics(err error) []Diagnostic {
	var out []Diagnostic
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		collectSchemaErrors(ve, &out)
		if len(out) > 0 {
			return out
		}
	}
	return []Diagnostic{{
		Code:    CodeSchema,
		Message: err.Error(),
	}}
}

func collectSchemaErrors(ve *jsonschema.ValidationError, out *[]Diagnostic) {
	if len(ve.Causes) == 0 {
		path := "/"
		if len(ve.InstanceLocation) > 0 {
			path = "/" + strings.Join(ve.InstanceLocation, "/")
		}
		msg := ve.Error()
		code := CodeSchema
		if strings.Contains(strings.ToLower(msg), "additional properties") {
			code = CodeUnknownField
		}
		*out = append(*out, Diagnostic{Code: code, Path: path, Message: msg})
		return
	}
	for _, c := range ve.Causes {
		collectSchemaErrors(c, out)
	}
}

func semanticValidate(src SourceFixture) []Diagnostic {
	var diags []Diagnostic

	if !fixtureIDPattern.MatchString(src.ID) {
		diags = append(diags, Diagnostic{
			Code:    CodeIDFormat,
			Path:    "/id",
			Message: fmt.Sprintf("fixture id %q must match kebab-case pattern", src.ID),
		})
	}

	if len(src.Areas) > MaxAreas {
		diags = append(diags, Diagnostic{
			Code:    CodeAllocationLimit,
			Path:    "/areas",
			Message: fmt.Sprintf("too many areas: %d > %d", len(src.Areas), MaxAreas),
		})
	}

	totalBytes := 0
	seen := map[string]int{}
	for i, area := range src.Areas {
		base := fmt.Sprintf("/areas/%d", i)
		key := areaKey(area)
		if prev, ok := seen[key]; ok {
			diags = append(diags, Diagnostic{
				Code:    CodeDuplicateArea,
				Path:    base,
				Message: fmt.Sprintf("duplicate area %s (also at /areas/%d)", key, prev),
			})
		} else {
			seen[key] = i
		}

		if area.Type == "db" {
			if area.Number == nil {
				diags = append(diags, Diagnostic{
					Code:    CodeMissingDBNumber,
					Path:    base + "/number",
					Message: "db area requires number",
				})
			} else if *area.Number < 1 || *area.Number > 65535 {
				diags = append(diags, Diagnostic{
					Code:    CodeInvalidDBNumber,
					Path:    base + "/number",
					Message: fmt.Sprintf("invalid db number %d", *area.Number),
				})
			}
		} else if area.Number != nil {
			diags = append(diags, Diagnostic{
				Code:    CodeDBNumberOnNonDB,
				Path:    base + "/number",
				Message: fmt.Sprintf("non-db area type %q must not define number", area.Type),
			})
		}

		if area.Size < 1 || area.Size > MaxAreaBytes {
			diags = append(diags, Diagnostic{
				Code:    CodeAllocationLimit,
				Path:    base + "/size",
				Message: fmt.Sprintf("area size %d outside 1..%d", area.Size, MaxAreaBytes),
			})
		}
		totalBytes += area.Size

		if len(area.Values) > MaxValuePatches {
			diags = append(diags, Diagnostic{
				Code:    CodeAllocationLimit,
				Path:    base + "/values",
				Message: fmt.Sprintf("too many value patches: %d > %d", len(area.Values), MaxValuePatches),
			})
		}

		diags = append(diags, validateAreaContent(area, base)...)
	}

	if totalBytes > MaxTotalAreaBytes {
		diags = append(diags, Diagnostic{
			Code:    CodeAllocationLimit,
			Path:    "/areas",
			Message: fmt.Sprintf("total area bytes %d exceeds limit %d", totalBytes, MaxTotalAreaBytes),
		})
	}

	if len(src.Expectations) > MaxExpectations {
		diags = append(diags, Diagnostic{
			Code:    CodeAllocationLimit,
			Path:    "/expectations",
			Message: fmt.Sprintf("too many expectations: %d > %d", len(src.Expectations), MaxExpectations),
		})
	}

	return diags
}

func areaKey(a Area) string {
	if a.Type == "db" && a.Number != nil {
		return fmt.Sprintf("db:%d", *a.Number)
	}
	return a.Type
}

func validateAreaContent(area Area, base string) []Diagnostic {
	var diags []Diagnostic
	var occupied []occupiedRange

	if area.Initial != nil {
		n, d := bulkLength(area.Initial, base+"/initial")
		if d != nil {
			diags = append(diags, *d)
		} else if n > area.Size {
			diags = append(diags, Diagnostic{
				Code:    CodeBounds,
				Path:    base + "/initial",
				Message: fmt.Sprintf("bulk initializer length %d exceeds area size %d", n, area.Size),
			})
		} else if n > 0 {
			occupied = append(occupied, occupiedRange{start: 0, end: n, path: base + "/initial"})
		}
	}

	for i, patch := range area.Values {
		ppath := fmt.Sprintf("%s/values/%d", base, i)
		size, err := dataTypeSize(patch)
		if err != nil {
			diags = append(diags, Diagnostic{
				Code:    CodeTypeEncoding,
				Path:    ppath,
				Message: err.Error(),
			})
			continue
		}
		if err := validateTypedValue(patch); err != nil {
			code := CodeTypeRange
			if strings.Contains(err.Error(), "requires") || strings.Contains(err.Error(), "must be") ||
				strings.Contains(err.Error(), "mismatch") || strings.Contains(err.Error(), "unsupported") ||
				strings.Contains(err.Error(), "invalid byteOrder") {
				code = CodeTypeEncoding
			}
			if strings.Contains(err.Error(), "out of range") {
				code = CodeTypeRange
			}
			diags = append(diags, Diagnostic{
				Code:    code,
				Path:    ppath,
				Message: err.Error(),
			})
			continue
		}
		if patch.Offset < 0 || patch.Offset+size > area.Size {
			diags = append(diags, Diagnostic{
				Code:    CodeBounds,
				Path:    ppath,
				Message: fmt.Sprintf("patch [%d,%d) outside area size %d", patch.Offset, patch.Offset+size, area.Size),
			})
			continue
		}

		cur := occupiedRange{start: patch.Offset, end: patch.Offset + size, path: ppath}
		if !patch.Replace {
			for _, prev := range occupied {
				if rangesOverlap(prev, cur) {
					// bool bit patches in the same byte are allowed only with replace, or same-byte different bits.
					if patch.DataType == "bool" && sameByteBitPatch(prev, cur) {
						continue
					}
					diags = append(diags, Diagnostic{
						Code: CodeOverlap,
						Path: ppath,
						Message: fmt.Sprintf(
							"overlaps %s without replace=true",
							prev.path,
						),
					})
					break
				}
			}
		}
		occupied = append(occupied, cur)
	}

	return diags
}

func sameByteBitPatch(a, b occupiedRange) bool {
	return a.start == b.start && a.end == b.end && a.end-a.start == 1
}

func bulkLength(init *BulkInitializer, path string) (int, *Diagnostic) {
	kinds := 0
	if init.Hex != "" {
		kinds++
	}
	if init.Base64 != "" {
		kinds++
	}
	if init.Bytes != nil {
		kinds++
	}
	if kinds != 1 {
		return 0, &Diagnostic{
			Code:    CodeBulkInitializer,
			Path:    path,
			Message: "initial must define exactly one of hex, base64, or bytes",
		}
	}

	switch {
	case init.Hex != "":
		if len(init.Hex)%2 != 0 {
			return 0, &Diagnostic{
				Code:    CodeHexLength,
				Path:    path + "/hex",
				Message: "hex string must have even length",
			}
		}
		for i := 0; i < len(init.Hex); i++ {
			c := init.Hex[i]
			if !isHex(c) {
				return 0, &Diagnostic{
					Code:    CodeHexLength,
					Path:    path + "/hex",
					Message: fmt.Sprintf("invalid hex character %q at %d", c, i),
				}
			}
		}
		return len(init.Hex) / 2, nil
	case init.Base64 != "":
		b, err := base64.StdEncoding.DecodeString(init.Base64)
		if err != nil {
			return 0, &Diagnostic{
				Code:    CodeBulkInitializer,
				Path:    path + "/base64",
				Message: fmt.Sprintf("invalid base64: %v", err),
			}
		}
		return len(b), nil
	default:
		for i, v := range init.Bytes {
			if v < 0 || v > 255 {
				return 0, &Diagnostic{
					Code:    CodeBulkInitializer,
					Path:    fmt.Sprintf("%s/bytes/%d", path, i),
					Message: "byte values must be 0..255",
				}
			}
		}
		return len(init.Bytes), nil
	}
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// DefaultSchemaPath returns the schema path relative to the repository root.
func DefaultSchemaPath(repoRoot string) string {
	return filepath.Join(repoRoot, "fixtures", "schema", "fixture.schema.json")
}
