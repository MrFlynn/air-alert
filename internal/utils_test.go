// +build unit

package internal

import (
	"math"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var floatComparer = cmp.Comparer(func(x, y float64) bool {
	delta := math.Abs(x - y)
	mean := math.Abs(x+y) / 2.0
	return delta/mean < 0.00001
})

func TestIsSubSetSlice(t *testing.T) {
	if !IsSubsetSlice([]string{"a"}, []string{"a", "ba", "b"}) {
		t.Error("Expected result to be true")
	}
}

func TestIsSubsetSliceExact(t *testing.T) {
	if !IsSubsetSlice([]string{"a"}, []string{"a"}) {
		t.Error("Expected result to be true")
	}
}

func TestIsSubSetSliceNotSubset(t *testing.T) {
	if IsSubsetSlice([]string{"a", "a"}, []string{"a", "b"}) {
		t.Error("Expected result to be false")
	}
}

func TestIsNilInt(t *testing.T) {
	if !IsNil(0) {
		t.Error("Expected result to be true")
	}
}

func TestIsNilIntPositive(t *testing.T) {
	if IsNil(1) {
		t.Error("Expected result to be false")
	}
}

func TestIsNilFloat(t *testing.T) {
	if !IsNil(0.0) {
		t.Error("Expected result to be true")
	}
}

func TestIsNilFloatPositive(t *testing.T) {
	if IsNil(1.0) {
		t.Error("Expected result to be false")
	}
}

func TestIsNilEmptyString(t *testing.T) {
	if !IsNil("") {
		t.Error("Expected result to be true")
	}
}

func TestIsNilString(t *testing.T) {
	if IsNil("a") {
		t.Error("Expected result to be false")
	}
}

func TestIsNilBoolFalse(t *testing.T) {
	if !IsNil(false) {
		t.Error("Expected result to be true")
	}
}

func TestIsNilBoolTrue(t *testing.T) {
	if IsNil(true) {
		t.Error("Expected result to be false")
	}
}

func TestIsNilSliceNil(t *testing.T) {
	var s []int
	if !IsNil(s) {
		t.Error("Expected result to be true")
	}
}

func TestIsNilSliceWithValues(t *testing.T) {
	if IsNil([]int{1, 2}) {
		t.Error("Expected result to be false")
	}
}

func TestCreateRandomString(t *testing.T) {
	str := CreateRandomString(10)

	if size := len(str); size != 10 {
		t.Errorf("Expected string to be 10 bytes, got %d bytes", size)
	}
}

func TestRecalculateAverage(t *testing.T) {
	numbers := make([]float64, 10)
	for i := 0; i < len(numbers); i++ {
		numbers[i] = rand.Float64()
	}

	var avgExpected float64
	for _, n := range numbers {
		avgExpected += n
	}
	avgExpected /= 10

	var avgComputed float64
	for i, n := range numbers {
		avgComputed = RecalculateAverage(n, avgComputed, i)
	}

	if !cmp.Equal(avgExpected, avgComputed, floatComparer) {
		t.Errorf("expected average to be %f, got %f", avgExpected, avgComputed)
	}
}
