package fixture

// Limits bound fixture allocations for safety (REQUIREMENTS §15).
const (
	MaxAreaBytes       = 1 << 20 // 1 MiB per area
	MaxAreas           = 64
	MaxTotalAreaBytes  = 8 << 20 // 8 MiB across all areas
	MaxValuePatches    = 4096
	MaxExpectations    = 4096
)

// SourceFixture is the decoded source document used for semantic validation.
type SourceFixture struct {
	SchemaVersion string         `json:"schemaVersion"`
	ID            string         `json:"id"`
	Description   string         `json:"description"`
	Revision      int            `json:"revision,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	Requires      []string       `json:"requires,omitempty"`
	References    []string       `json:"references,omitempty"`
	Notes         string         `json:"notes,omitempty"`
	Server        ServerSettings `json:"server"`
	Areas         []Area         `json:"areas"`
	Expectations  []Expectation  `json:"expectations,omitempty"`
}

type ServerSettings struct {
	Rack     int             `json:"rack"`
	Slot     int             `json:"slot"`
	PDUSize  int             `json:"pduSize,omitempty"`
	CPUState string          `json:"cpuState,omitempty"`
	Identity *ServerIdentity `json:"identity,omitempty"`
}

type ServerIdentity struct {
	OrderCode    string `json:"orderCode,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
	ModuleName   string `json:"moduleName,omitempty"`
}

type Area struct {
	Type    string           `json:"type"`
	Number  *int             `json:"number,omitempty"`
	Size    int              `json:"size"`
	Access  string           `json:"access"`
	Initial *BulkInitializer `json:"initial,omitempty"`
	Values  []ValuePatch     `json:"values,omitempty"`
}

type BulkInitializer struct {
	Hex     string `json:"hex,omitempty"`
	Base64  string `json:"base64,omitempty"`
	Bytes   []int  `json:"bytes,omitempty"`
	Replace bool   `json:"replace,omitempty"`
}

type ValuePatch struct {
	Offset    int     `json:"offset"`
	Bit       *int    `json:"bit,omitempty"`
	DataType  string  `json:"dataType"`
	ByteOrder string  `json:"byteOrder,omitempty"`
	Length    int     `json:"length,omitempty"`
	Value     any     `json:"value"`
	Replace   bool    `json:"replace,omitempty"`
}

type Expectation struct {
	Operation string `json:"operation"`
	Area      AreaRef `json:"area"`
	Offset    *int   `json:"offset,omitempty"`
	Bit       *int   `json:"bit,omitempty"`
	Length    *int   `json:"length,omitempty"`
	Data      *BytePayload `json:"data,omitempty"`
	Result    any    `json:"result,omitempty"`
}

type AreaRef struct {
	Type   string `json:"type"`
	Number *int   `json:"number,omitempty"`
}

type BytePayload struct {
	Hex    string `json:"hex,omitempty"`
	Base64 string `json:"base64,omitempty"`
	Bytes  []int  `json:"bytes,omitempty"`
}
