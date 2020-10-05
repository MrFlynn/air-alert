// +build unit

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

	data, err := serializeSensorData([]redis.Cmder{cmd})
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

	status := redis.NewStatusCmd(context.Background())

	cmds := []redis.Cmder{first, second, third, status}

	data, err := serializeSensorData(cmds)
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

func TestMarshalRawDataBadType(t *testing.T) {
	cmd := redis.NewBoolCmd(context.Background(), "exists", "1")

	_, err := serializeSensorData([]redis.Cmder{cmd})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetNotifcationStream(t *testing.T) {
	cmd := redis.NewXStreamSliceCmdResult([]redis.XStream{
		{
			Stream: "test",
			Messages: []redis.XMessage{
				{
					ID: "0",
					Values: map[string]interface{}{
						"uid":      "1",
						"aqi":      "3.0",
						"forecast": "0",
					},
				},
				{
					ID: "1",
					Values: map[string]interface{}{
						"uid":      "1",
						"aqi":      "2.5",
						"forecast": "2",
					},
				},
			},
		},
	}, nil)

	data, err := getNotificationsFromStream(cmd, 2)
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	expected := []NotificationStream{
		{
			MessageID: "0",
			UID:       1,
			AQI:       3.0,
			Forecast:  AQIStatic,
		},
		{
			MessageID: "1",
			UID:       1,
			AQI:       2.5,
			Forecast:  AQIDecreasing,
		},
	}

	if !cmp.Equal(data, expected) {
		t.Errorf("\nexpected %#v\ngot %#v", expected, data)
	}
}

func TestReceivedNil(t *testing.T) {
	cmd := redis.NewXStreamSliceCmd(context.Background(), "xrange", "1", "+", "-", "count", 1)
	cmd.SetErr(redis.Nil)

	data, err := getNotificationsFromStream(cmd, 1)
	if err != nil {
		t.Errorf("got unexpected error: %s", err)
	}

	if data != nil {
		t.Errorf("expected data to be nil, got %#v", data)
	}
}

func TestGetForecastsFromStreamBadType(t *testing.T) {
	cmd := redis.NewBoolCmd(context.Background(), "exists", "1")

	_, err := getNotificationsFromStream(cmd, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestUnionKey(t *testing.T) {
	u := UnionKey{1, 2}

	id := u.ID()

	if id != 1 {
		t.Errorf("expected id to be 1, got %d", id)
	}

	ts := u.Timestamp()

	if ts != 2 {
		t.Errorf("expected timestamp to be 2, got %d", id)
	}
}
