package fixture

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Stable diagnostic codes. New codes may be added; existing codes must not change meaning.
const (
	CodeSchema           = "E_SCHEMA"
	CodeIO               = "E_IO"
	CodeIDFormat         = "E_ID_FORMAT"
	CodeDuplicateArea    = "E_DUPLICATE_AREA"
	CodeInvalidDBNumber  = "E_INVALID_DB_NUMBER"
	CodeDBNumberOnNonDB  = "E_DB_NUMBER_ON_NON_DB"
	CodeMissingDBNumber  = "E_MISSING_DB_NUMBER"
	CodeBounds           = "E_BOUNDS"
	CodeOverlap          = "E_OVERLAP"
	CodeTypeRange        = "E_TYPE_RANGE"
	CodeTypeEncoding     = "E_TYPE_ENCODING"
	CodeAllocationLimit  = "E_ALLOCATION_LIMIT"
	CodeHexLength        = "E_HEX_LENGTH"
	CodeBulkInitializer  = "E_BULK_INITIALIZER"
	CodeUnknownField     = "E_UNKNOWN_FIELD"
)

// Diagnostic is a structured validation finding.
type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("%s: %s", d.Code, d.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", d.Code, d.Message, d.Path)
}

// Result aggregates validation outcomes for one fixture file.
type Result struct {
	File         string        `json:"file"`
	Valid        bool          `json:"valid"`
	Diagnostics  []Diagnostic  `json:"diagnostics,omitempty"`
}

func (r Result) Error() string {
	if r.Valid {
		return ""
	}
	parts := make([]string, 0, len(r.Diagnostics)+1)
	parts = append(parts, fmt.Sprintf("%s: invalid fixture", r.File))
	for _, d := range r.Diagnostics {
		parts = append(parts, d.String())
	}
	return strings.Join(parts, "\n")
}

// WriteText writes human-readable diagnostics.
func WriteText(w io.Writer, results []Result) error {
	for _, r := range results {
		if r.Valid {
			if _, err := fmt.Fprintf(w, "OK  %s\n", r.File); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "FAIL %s\n", r.File); err != nil {
			return err
		}
		for _, d := range r.Diagnostics {
			if _, err := fmt.Fprintf(w, "  %s\n", d.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteJSON writes machine-readable diagnostics.
func WriteJSON(w io.Writer, results []Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
