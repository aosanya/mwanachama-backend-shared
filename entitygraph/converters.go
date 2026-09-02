// converters.go — shared property-map helper functions for entity
// converters. Every domain package built on entitygraph (git, task, …)
// defines entity→domain converters that extract typed values from
// Entity.Properties (a map[string]any). These helpers are identical across
// domains, so they live here once.
package entitygraph

// StringProp returns the string value of key in props, or "" if absent or
// the stored value is not a string.
func StringProp(props map[string]any, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// BoolProp returns the bool value of key in props, or false if absent or
// the stored value is not a bool.
func BoolProp(props map[string]any, key string) bool {
	if v, ok := props[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Int64Prop returns the int64 value of key in props, or 0 if absent.
// Handles int64, int, and float64 (JSON-decoded numbers arrive as
// float64).
func Int64Prop(props map[string]any, key string) int64 {
	if v, ok := props[key]; ok {
		switch vv := v.(type) {
		case int64:
			return vv
		case int:
			return int64(vv)
		case float64:
			return int64(vv)
		}
	}
	return 0
}

// Float64Prop returns the float64 value of key in props, or 0 if absent.
// Handles float64, float32, int, and int64.
func Float64Prop(props map[string]any, key string) float64 {
	if v, ok := props[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return 0
}
