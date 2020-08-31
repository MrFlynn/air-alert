package database

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	utils "github.com/mrflynn/air-alert/internal"
)

var (
	splitCmdRegex = regexp.MustCompile(`[\s:]`)
	keyRegex      = regexp.MustCompile(`^[0-9]+$`)
)

func addAQIRequestToPipe(ctx context.Context, pipe redis.Pipeliner, id int) error {
	key := createRedisKey(id, "data", "aqi")
	return pipe.LRange(ctx, key, measurementStart, numMeasurements-1).Err()
}

func getFloatSliceFromRedisList(result redis.Cmder) ([]float64, error) {
	switch result.(type) {
	case *redis.StringSliceCmd:
		if measurements, err := result.(*redis.StringSliceCmd).Result(); err == nil {
			return utils.StringSliceToFloatSlice(measurements), nil
		} else {
			return nil, err
		}
	case *redis.StatusCmd:
		return nil, nil
	default:
		return nil, errors.New("could not convert result to float slice")
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
