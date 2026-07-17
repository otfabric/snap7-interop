package interop

// DefaultCapabilities are the shared capabilities of both M2/M3 adapters.
var DefaultCapabilities = map[string]bool{
	"area.db":         true,
	"area.markers":    true,
	"area.inputs":     true,
	"area.outputs":    true,
	"operation.read":  true,
	"operation.write": true,
}

// MissingCapabilities returns required capabilities not present in caps.
func MissingCapabilities(requires []string, caps map[string]bool) []string {
	var missing []string
	for _, r := range requires {
		if !caps[r] {
			missing = append(missing, r)
		}
	}
	return missing
}
