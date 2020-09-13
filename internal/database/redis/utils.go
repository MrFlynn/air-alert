package redis

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
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

func marshalRawDataFromResult(cmds []redis.Cmder) (map[int]*RawQualityData, error) {
	// The the key for this map is made by adding the time index + the sensor id. Since each
	// sensor ID is unique and there is only one measurement per time index, this will yield
	// unique, reversible keys.
	resultLookup := make(map[int]*RawQualityData, len(cmds)-1)

	for _, c := range cmds {
		switch cmd := c.(type) {
		case *redis.ZSliceCmd:
			id, path := getFullIDFromRedisKey(cmd)

			set, err := cmd.Result()
			if err != nil {
				continue
			}

			for _, item := range set {
				value, err := strconv.ParseFloat(item.Member.(string), 64)
				if err != nil {
					continue
				}

				key := id + int(item.Score)

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
