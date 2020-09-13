package redis

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type exampleStruct struct {
	ID       int    `db:"primary_key"`
	ParentID int    `db:"append_only"`
	First    string `db:"key,first"`
	Second   string `db:"key,second"`
}

type badStruct struct {
	ID    int    `db:"primary_key"`
	First string `db:"primary_key"`
}

var (
	testStruct = exampleStruct{
		ID:     1,
		First:  "hello",
		Second: "world",
	}
	testStructValue = reflect.ValueOf(testStruct)
)

func TestAppendOnly(t *testing.T) {
	testAltStruct := reflect.ValueOf(exampleStruct{
		ID:       1,
		ParentID: 2,
	})

	ok, v, err := appendOnly(testAltStruct)

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	if !ok {
		t.Errorf(`Expected result to be true`)
	}

	expected := valuesMatrix{
		"parentid": 2,
	}

	if !cmp.Equal(expected, v) {
		t.Errorf("Expected %+v\nGot: %+v", expected, v)
	}
}

func TestAppendOnlyWithoutParentID(t *testing.T) {
	ok, v, err := appendOnly(testStructValue)

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	if ok {
		t.Errorf(`Expected result to be false`)
	}

	expected := valuesMatrix{
		"parentid": 0,
	}

	if !cmp.Equal(expected, v) {
		t.Errorf("Expected %+v\nGot: %+v", expected, v)
	}
}

func TestGetPrimaryKey(t *testing.T) {
	id, err := getPrimaryKey(testStructValue)

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	if id != 1 {
		t.Errorf(`Expected id to be 1, but got %d`, id)
	}
}

func TestGetPrimaryKeyBad(t *testing.T) {
	testBadStructValue := reflect.ValueOf(badStruct{
		ID:    2,
		First: "test",
	})

	id, err := getPrimaryKey(testBadStructValue)

	if err == nil {
		t.Errorf(`Expected error`)
	}

	if id != 0 {
		t.Errorf(`Expected id to be 0, got %d`, id)
	}
}

func TestGetPrimaryKeyFromAlternate(t *testing.T) {
	testWithAltValue := reflect.ValueOf(exampleStruct{
		ID:       2,
		ParentID: 1,
	})

	id, err := getPrimaryKey(testWithAltValue)

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	if id != 1 {
		t.Errorf(`Expected ID to be 1, got %d`, id)
	}
}

func TestGetFirstKey(t *testing.T) {
	v, err := matchValuesWithFlags(testStructValue, "key", "first")

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	expected := valuesMatrix{
		"first": "hello",
	}

	if !cmp.Equal(expected, v) {
		t.Errorf("Expected %+v\nGot: %+v", expected, v)
	}
}

func TestGetAllKeys(t *testing.T) {
	v, err := matchValuesWithFlags(testStructValue, "key")

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	expected := valuesMatrix{
		"first":  "hello",
		"second": "world",
	}

	if !cmp.Equal(expected, v) {
		t.Errorf("Expected %+v\nGot: %+v", expected, v)
	}
}

func TestPanicHandling(t *testing.T) {
	_, err := matchValuesWithFlags(reflect.ValueOf(1), "key")

	if err == nil {
		t.Errorf(`Received no error`)
	}

	if msg := err.Error(); msg != "expected struct type, got int" {
		t.Errorf(`Got incorrect error message: %s`, msg)
	}
}

func TestNoFlagsProvided(t *testing.T) {
	_, err := matchValuesWithFlags(testStructValue)

	if err == nil {
		t.Errorf(`Received no error`)
	}

	if msg := err.Error(); msg != "at least 1 flag must be provided" {
		t.Errorf(`Got incorrect error message: %s`, msg)
	}
}

func TestNoMatchingFlags(t *testing.T) {
	v, err := matchValuesWithFlags(testStructValue, "none")

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	if !cmp.Equal(valuesMatrix{}, v) {
		t.Errorf(`Expected result to be empty, got %+v`, v)
	}
}

func TestNilValueInStruct(t *testing.T) {
	nilValueStructValue := reflect.ValueOf(exampleStruct{
		ID: 3,
	})

	v, err := matchValuesWithFlags(nilValueStructValue, "first")

	if err != nil {
		t.Errorf(`Got error: %s`, err)
	}

	expected := valuesMatrix{
		"first": "",
	}

	if !cmp.Equal(expected, v) {
		t.Errorf(`Expected result to be empty, got %+v`, v)
	}
}
