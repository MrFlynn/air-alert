package internal

import (
	"math/rand"
	"reflect"
	"strings"
	"time"
)

const (
	letters    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterBits = 6
	letterMask = 1<<letterBits - 1
	letterMax  = 63 / letterBits
)

var source = rand.NewSource(time.Now().UnixNano())

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

// CreateRandomString creates a random string of `size`.
func CreateRandomString(size int) string {
	builder := strings.Builder{}
	builder.Grow(size)

	for i, cache, remain := size-1, source.Int63(), letterMax; i >= 0; {
		if remain == 0 {
			cache = source.Int63()
			remain = letterMax
		}

		if idx := int(cache & letterMask); idx < len(letters) {
			builder.WriteByte(letters[idx])
			i--
		}

		cache >>= letterBits
		remain--
	}

	return builder.String()
}

// Above credit goes the following post for inspiration:
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
