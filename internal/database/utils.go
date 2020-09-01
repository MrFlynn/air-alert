package database

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

func addAQIRequestToPipe(ctx context.Context, pipe redis.Pipeliner, id int) error {
	key := createRedisKey(id, "data", "pm25")
	return pipe.ZRevRangeWithScores(ctx, key, 0, -1).Err()
}

func getTimeIndexFromSortedSet(cmd redis.Cmder) ([]RawQualityData, error) {
	switch c := cmd.(type) {
	case *redis.ZSliceCmd:
		if set, err := c.Result(); err == nil {
			timeIndex := make([]RawQualityData, 0, len(set))

			for _, item := range set {
				if value, err := strconv.ParseFloat(item.Member.(string), 32); err == nil {
					timeIndex = append(timeIndex, RawQualityData{
						Time: int64(item.Score),
						PM25: value,
					})
				}
			}

			return timeIndex, nil
		} else {
			return nil, err
		}
	case *redis.StatusCmd:
		return nil, nil
	default:
		return nil, fmt.Errorf("could not find valid conversion for %T", c)
	}
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
	for _, arg := range cmd.Args() {
		if argString, ok := arg.(string); ok {
			for _, item := range splitCmdRegex.Split(argString, -1) {
				if keyRegex.MatchString(item) {
					id, _ := strconv.Atoi(item)
					return id
				}
			}
		}
	}

	return -1
}
