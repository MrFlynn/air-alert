package redis

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-redis/redis/v8"
	"github.com/google/go-cmp/cmp"
)

// This method is so I can set the result of an existing ZSliceCmd. Currently
// there are no convenience functions that allow you to set the result and the
// command arguments.
func addZSlice(r *redis.ZSliceCmd, s []redis.Z) {
	rPtr := reflect.Indirect(reflect.ValueOf(r))

	resultSlice := rPtr.FieldByName("val")
	ptrToVal := unsafe.Pointer(resultSlice.UnsafeAddr())

	realPtrToVal := (*[]redis.Z)(ptrToVal)
	*realPtrToVal = s
}

// This function is similar to the one above, but with XMessages instead.
func addXMessageSlice(r *redis.XMessageSliceCmd, s []redis.XMessage) {
	rPtr := reflect.Indirect(reflect.ValueOf(r))

	resultSlice := rPtr.FieldByName("val")
	ptrToVal := unsafe.Pointer(resultSlice.UnsafeAddr())

	realPtrToVal := (*[]redis.XMessage)(ptrToVal)
	*realPtrToVal = s
}

func TestGetFullIDFromRedisKey(t *testing.T) {
	cmd := redis.NewZSliceCmd(
		context.Background(), "zrevrange", "data:pm25:1", 0, 1, "withscores",
	)

	id, path := getFullIDFromRedisKey(cmd)
	if id != 1 {
		t.Errorf("got id %d, expected id to be 1", id)
	}

	if !cmp.Equal(path, []string{"data", "pm25"}) {
		t.Errorf("got path %#v, expected %#v", path, []string{"data", "pm25"})
	}
}

func TestGetIDFromRedisKey(t *testing.T) {
	cmd := redis.NewZSliceCmd(
		context.Background(), "zrevrange", "data:pm25:1", 0, 1, "withscores",
	)

	id := getIDFromRedisKey(cmd)
	if id != 1 {
		t.Errorf("got id %d, expected id to be 1", id)
	}
}

func TestCreateRedisKey(t *testing.T) {
	key := createRedisKey(12, "some", "data")

	if key != "some:data:12" {
		t.Errorf(`expected "some:data:12, got %s`, key)
	}
}

func TestMarshalRawDataSingle(t *testing.T) {
	cmd := redis.NewZSliceCmd(
		context.Background(), "zrevrange", "data:pm25:1", 0, 1, "withscores",
	)
	results := []redis.Z{
		{
			Score:  1.0,
			Member: "3.0",
		},
	}

	addZSlice(cmd, results)

	data, err := marshalRawDataFromResult([]redis.Cmder{cmd})
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	expected := map[UnionKey]*RawQualityData{
		{1, 1}: {
			Time: 1,
			PM25: 3.0,
		},
	}

	if !cmp.Equal(data, expected) {
		t.Errorf("\nexpected %#v\ngot %#v", expected, data)
	}
}

func TestMarshalRawDataMulti(t *testing.T) {
	first := redis.NewZSliceCmd(context.Background(), "zrevrange", "data:pm25:1", 0, 1, "withscores")
	addZSlice(first, []redis.Z{
		{
			Score:  2.0,
			Member: "1.0",
		},
		{
			Score:  1.0,
			Member: "2.0",
		},
	})

	second := redis.NewZSliceCmd(context.Background(), "zrevrange", "data:aqi:1", 0, 1, "withscores")
	addZSlice(second, []redis.Z{
		{
			Score:  2.0,
			Member: "3.0",
		},
	})

	third := redis.NewZSliceCmd(context.Background(), "zrevrange", "data:pm25:2", 0, 1, "withscores")
	addZSlice(third, []redis.Z{
		{
			Score:  2.0,
			Member: "4.0",
		},
	})

	cmds := []redis.Cmder{first, second, third}

	data, err := marshalRawDataFromResult(cmds)
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	expected := map[UnionKey]*RawQualityData{
		{1, 1}: {
			Time: 1,
			PM25: 2.0,
		},
		{1, 2}: {
			Time: 2,
			PM25: 1.0,
			AQI:  3.0,
		},
		{2, 2}: {
			Time: 2,
			PM25: 4.0,
		},
	}

	if !cmp.Equal(data, expected) {
		t.Errorf("\nexpected %#v\ngot %#v", expected, data)
	}
}

func TestGetForecastsFromStream(t *testing.T) {
	cmd := redis.NewXMessageSliceCmd(context.Background(), "xrange", "1", "+", "-", "count", 1)
	addXMessageSlice(cmd, []redis.XMessage{
		{
			ID: "0",
			Values: map[string]interface{}{
				"aqi":      "3.0",
				"forecast": "0",
			},
		},
	})

	data, err := getForecastsFromStream([]redis.Cmder{cmd})
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	expected := []AQIStreamData{
		{
			ID:       1,
			AQI:      3.0,
			Forecast: AQIStatic,
		},
	}

	if !cmp.Equal(data, expected) {
		t.Errorf("\nexpected %#v\ngot %#v", expected, data)
	}
}
