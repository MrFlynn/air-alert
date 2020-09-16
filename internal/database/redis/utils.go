package redis

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var (
	splitCmdRegex = regexp.MustCompile(`[\s:]`)
	keyRegex      = regexp.MustCompile(`^[0-9]+$`)
)

func addAQIRequestToPipe(ctx context.Context, pipe redis.Pipeliner, id int, count ...int64) error {
	var numResults int64 = -1

	if len(count) > 0 {
		numResults = count[0]
	}

	if err := pipe.ZRevRangeWithScores(ctx, createRedisKey(id, "data", "pm25"), 0, numResults).Err(); err != nil {
		return err
	}

	if err := pipe.ZRevRangeWithScores(ctx, createRedisKey(id, "data", "aqi"), 0, numResults).Err(); err != nil {
		return err
	}

	return nil
}

// UnionKey is a tuple of an ID and a timestamp.
// Compared to structs as keys in maps, using a fixed-size array is about 1.5x faster.
type UnionKey [2]int

// ID returns the ID field from the union.
func (u UnionKey) ID() (int, error) {
	if len(u) < 1 {
		return 0, errors.New("id not present in union")
	}

	return u[0], nil
}

// Timestamp returns the timestamp field from the union.
func (u UnionKey) Timestamp() (int, error) {
	if len(u) < 2 {
		return 0, errors.New("timestamp not present in union")
	}

	return u[1], nil
}

func serializeSensorData(cmds []redis.Cmder) (map[UnionKey]*RawQualityData, error) {
	// The key for this map is a union type. Since PM25 and AQI data is stored in separate sorted sets,
	// we get each value in different Z slices. In order to properly place these values into the correct
	// structs, we need the ID of the sensor and the recorded timestamp. We need to be able to extract
	// the ID separately when assigning the results of this function to the proper RawSensorData structs
	// but there exists no reversible, non-associative (this is key since id + time could equal id2 + time2)
	// function to accomplish this.
	resultLookup := make(map[UnionKey]*RawQualityData, len(cmds)-1)

	for _, c := range cmds {
		switch cmd := c.(type) {
		case *redis.ZSliceCmd:
			id, path := getFullIDFromRedisKey(cmd)
			if id < 0 {
				log.Debugf("got id %d less than 0", id)
				continue
			}

			set, err := cmd.Result()
			if err != nil {
				log.Debugf("could not get result for %#v : %d : %s", path, id, err)
				continue
			}

			for _, item := range set {
				value, err := strconv.ParseFloat(item.Member.(string), 64)
				if err != nil {
					log.Debugf("could not convert field %#v : %d to float: %s", path, id, err)
					continue
				}

				key := UnionKey{id, int(item.Score)}

				// Allocate the struct if it doesn't exist.
				if _, ok := resultLookup[key]; !ok {
					resultLookup[key] = &RawQualityData{Time: int(item.Score)}
				}

				if path[len(path)-1] == "pm25" {
					resultLookup[key].PM25 = value
				} else if path[len(path)-1] == "aqi" {
					resultLookup[key].AQI = value
				}
			}
		// These will sometimes show up in pipeline results. Just skip them.
		case *redis.StatusCmd:
			continue
		default:
			return resultLookup, fmt.Errorf("could not find valid conversion for %T", cmd)
		}
	}

	return resultLookup, nil
}

func getForecastsFromStream(cmds []redis.Cmder) ([]AQIStreamData, error) {
	streamData := make([]AQIStreamData, 0, len(cmds)-1)

	for _, c := range cmds {
		switch cmd := c.(type) {
		case *redis.XMessageSliceCmd:
			id := getIDFromRedisKey(cmd)
			if id < 0 {
				log.Debugf("got id %d less than 0", id)
				continue
			}

			stream, err := cmd.Result()
			if err != nil {
				log.Debugf("could not get result for %d : %s", id, err)
				continue
			}

			for _, s := range stream {
				var aqi float64
				var forecast int
				var err error

				aqi, err = strconv.ParseFloat(s.Values["aqi"].(string), 64)
				if err != nil {
					log.Debugf("could not conver aqi field from stream: %s", err)
					continue
				}

				forecast, err = strconv.Atoi(s.Values["forecast"].(string))
				if err != nil {
					log.Debugf("could not convert forecast field from stream %s", err)
					continue
				}

				streamData = append(streamData, AQIStreamData{
					ID:       id,
					AQI:      aqi,
					Forecast: AQIForecast(forecast),
				})
			}
		case *redis.StatusCmd:
			continue
		default:
			return nil, fmt.Errorf("could not find valid conversion for %T", cmd)
		}
	}

	return streamData, nil
}

func createRedisKey(id int, path ...string) string {
	builder := strings.Builder{}
	builder.Grow((len(path) + 1) * 10) // Assumes that there will be n+1 subkeys each 10 chars long.

	for _, s := range path {
		builder.WriteString(s)
		builder.WriteString(":")
	}

	builder.WriteString(strconv.Itoa(id))

	return builder.String()
}

func getIDFromRedisKey(cmd redis.Cmder) int {
	id, _ := getFullIDFromRedisKey(cmd)
	return id
}

func getFullIDFromRedisKey(cmd redis.Cmder) (int, []string) {
	if len(cmd.Args()) < 2 {
		return -1, nil
	}

	// The key should always be the second argument.
	if key, ok := cmd.Args()[1].(string); ok {
		path := strings.Split(key, ":")
		for i, element := range path {
			if keyRegex.MatchString(element) {
				if id, err := strconv.Atoi(element); err == nil {
					return id, append(path[:i], path[i+1:]...)
				}
			}
		}
	}

	return -1, nil
}
