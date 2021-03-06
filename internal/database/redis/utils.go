package redis

import (
	"context"
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
func (u UnionKey) ID() int {
	return u[0]
}

// Timestamp returns the timestamp field from the union.
func (u UnionKey) Timestamp() int {
	return u[1]
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

func getNotificationsFromStream(c redis.Cmder, count int64) ([]NotificationStream, error) {
	streamData := make([]NotificationStream, 0, count)

	switch cmd := c.(type) {
	case *redis.XStreamSliceCmd:
		var err error

		stream, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				// If we get nothing, return nothing.
				return nil, nil
			}

			log.Debugf("could not get result: %s", err)
			return nil, err
		}

		for _, s := range stream {
			for _, m := range s.Messages {
				uid, err := strconv.Atoi(m.Values["uid"].(string))
				if err != nil {
					log.Debugf("could not convert %s to uid, skipping...", m.Values["uid"].(string))
					continue
				}

				var aqi float64
				var forecast int

				aqi, err = strconv.ParseFloat(m.Values["aqi"].(string), 64)
				if err != nil {
					log.Debugf("could not conver aqi field from stream: %s", err)
					continue
				}

				forecast, err = strconv.Atoi(m.Values["forecast"].(string))
				if err != nil {
					log.Debugf("could not convert forecast field from stream %s", err)
					continue
				}

				streamData = append(streamData, NotificationStream{
					MessageID: m.ID,
					UID:       uid,
					AQI:       aqi,
					Forecast:  AQIForecast(forecast),
				})
			}
		}
	default:
		return nil, fmt.Errorf("could not find valid conversion for %T", cmd)
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
