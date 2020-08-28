package internal

import "reflect"

// IsIsSubsetSlice checks if the first slice is a subset of the
// second one.
func IsSubsetSlice(first, second []string) bool {
	set := make(map[string]int, len(second))
	for _, v := range second {
		set[v] += 1
	}

	for _, v := range first {
		count, exists := set[v]
		if !exists {
			return false
		} else if count < 1 {
			return false
		} else {
			set[v] -= 1
		}
	}

	return true
}

// IsNil checks is the provided interface is nil-valued or not.
func IsNil(v interface{}) bool {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Int:
		return v == 0
	case reflect.Float32, reflect.Float64:
		return v == 0.0
	case reflect.String:
		return v == ""
	case reflect.Bool:
		return !v.(bool)
	case reflect.Ptr, reflect.Array, reflect.Map, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(v).IsNil()
	default:
		return false
	}
}
