package database

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	utils "github.com/mrflynn/air-alert/internal"
)

type valuesMatrix map[string]interface{}

func (m valuesMatrix) selectFirst(key string) interface{} {
	for k, v := range m {
		if k == key {
			return v
		}
	}

	return nil
}

func matchValuesWithFlags(resp reflect.Value, flags ...string) (v valuesMatrix, err error) {
	if len(flags) < 1 {
		return valuesMatrix{}, errors.New("at least 1 flag must be provided")
	}

	// Handle panics from any subsequent functions.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("expected struct type, got %s", resp.Type().Name())
		}
	}()

	size := resp.NumField()
	v = make(valuesMatrix, size)

	for i := 0; i < size; i++ {
		field := resp.Type().Field(i)
		tag := field.Tag.Get("db")

		structFlags := strings.Split(tag, ",")
		if utils.IsSubsetSlice(flags, structFlags) {
			v[strings.ToLower(field.Name)] = resp.Field(i).Interface()
		}
	}

	return v, nil
}

func appendOnly(resp reflect.Value) (bool, valuesMatrix, error) {
	matrix, err := matchValuesWithFlags(resp, "append_only")
	if err != nil {
		return false, matrix, err
	}

	if len(matrix) == 0 {
		return false, matrix, nil
	}

	// This checks if any value is non-nil.
	for _, v := range matrix {
		if !utils.IsNil(v) {
			return true, matrix, nil
		}
	}

	return false, matrix, nil
}

func getPrimaryKey(resp reflect.Value) (int, error) {
	primaryKey, err := matchValuesWithFlags(resp, "primary_key")
	if err != nil {
		return 0, err
	}

	// Need to check if we get too few or too many primary keys.
	if len(primaryKey) < 1 {
		return 0, errors.New("no primary key provided in struct")
	} else if len(primaryKey) > 1 {
		return 0, errors.New("too many primary keys in struct")
	}

	// We need to check if the response contains a valid ParentID value.
	// If it does, we can assume all measurements are from the parent sensor
	// and we should set the value of the primary key to the value of the ParentID
	// field.
	shouldAppend, appendValues, err := appendOnly(resp)
	if err != nil {
		return 0, err
	}

	// Default search key and matrix.
	key := "id"
	matrix := primaryKey

	if shouldAppend {
		key = "parentid"
		matrix = appendValues
	}

	if id, ok := matrix.selectFirst(key).(int); ok {
		return id, nil
	} else {
		// Fall back to primary key if type assertion fails.
		if backup, ok := primaryKey.selectFirst("id").(int); ok {
			return backup, nil
		}
	}

	// If all type assertions fail return a default and an error.
	return 0, errors.New("could not convert id value to integer")
}
