package database

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/purpleapi"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const sensorMapKey = "sensors"

// Controller is a container for a Redis client.
type Controller struct {
	db *redis.Client
}

// NewController creates a new Redis client.
func NewController(ctx *cli.Context) (*Controller, error) {
	db := redis.NewClient(&redis.Options{
		Addr:     ctx.String("database"),
		Password: ctx.String("database-password"),
		DB:       ctx.Int("database-id"),
	})

	if _, err := db.Ping(ctx.Context).Result(); err != nil {
		return &Controller{}, err
	}

	return &Controller{
		db: db,
	}, nil
}

// This method tries to safely exchange keys and discards the old one.
func (c *Controller) exchangeAndRemove(ctx context.Context, originalKey, newKey string) error {
	res, err := c.db.Exists(ctx, originalKey).Result()
	if err != nil {
		if err == redis.Nil {
			c.db.Rename(ctx, newKey, originalKey)
			return nil
		}

		return err
	} else if res == 0 {
		c.db.Rename(ctx, newKey, originalKey)
		return nil
	}

	originalMoved := originalKey + utils.CreateRandomString(20)

	err = c.db.Rename(ctx, originalKey, originalMoved).Err()
	if err != nil {
		return err
	}

	err = c.db.Rename(ctx, newKey, originalKey).Err()
	if err != nil {
		moveBackErr := c.db.Rename(ctx, originalMoved, originalKey).Err()
		if moveBackErr != nil {
			return moveBackErr
		}
	}

	c.db.Del(ctx, originalMoved)

	return nil
}

// SetAirQuality takes an array of Purple Air API response structs and stores the most recent
// PM2.5 AQI readings from those sensors.
func (c *Controller) SetAirQuality(ctx context.Context, data []purpleapi.Response) error {
	cutoffTime := strconv.FormatInt(time.Now().Add(-60*time.Minute).Unix(), 10)

	_, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, resp := range data {
			// We only want sensors that are outside.
			if resp.Location == purpleapi.Outside {
				id, err := getPrimaryKey(reflect.ValueOf(resp))
				if err != nil {
					return err
				}

				aqi := resp.PM25
				key := createRedisKey(id, "data", "pm25")

				// Add aqi with score being equal to reading capture time.
				pipe.ZAddNX(ctx, key, &redis.Z{
					Score:  float64(resp.LastUpdated),
					Member: aqi,
				})

				// This removes all measurements older than 60 minutes.
				pipe.ZRemRangeByScore(ctx, key, "0", cutoffTime)
			}
		}

		return nil
	})

	return err
}

// RawSensorData contains raw sensor from the Redis datastore.
type RawSensorData struct {
	ID   int              `json:"sensor_id"`
	Data []RawQualityData `json:"measurements"`
}

// RawQualityData contains a time stamp the corresponding pm2.5 measurement.
type RawQualityData struct {
	Time int64   `json:"time"`
	PM25 float64 `json:"pm25"`
}

// GetAirQuality gets 10 most recent PM2.5 AQI readings from a specific sensor.
func (c *Controller) GetAirQuality(ctx context.Context, id int) (RawSensorData, error) {
	results, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		return addAQIRequestToPipe(ctx, pipe, id)
	})

	if err == redis.Nil {
		return RawSensorData{}, fmt.Errorf("could not find data for sensor %d", id)
	} else if err != nil {
		return RawSensorData{}, err
	}

	if len(results) < 1 {
		return RawSensorData{}, fmt.Errorf("could not find data for sensor %d", id)
	}

	// Only take first as there should only be one result.
	data, err := getTimeIndexFromSortedSet(results[0])
	if err != nil {
		return RawSensorData{}, err
	}

	if data == nil {
		return RawSensorData{}, errors.New("could not get time indexed data from query")
	}

	return RawSensorData{
		ID:   id,
		Data: data,
	}, nil
}

// GetAQIFromSensorsInRange returns raw sensor data from all sensors within the specified
// radius around the given coordinates.
func (c *Controller) GetAQIFromSensorsInRange(ctx context.Context, longitude, latitude, radius float64) ([]RawSensorData, error) {
	ids, err := c.GetSensorsInRange(ctx, longitude, latitude, radius)
	if err != nil {
		return nil, err
	}

	results, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, id := range ids {
			err := addAQIRequestToPipe(ctx, pipe, id)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	rawSensorSlice := make([]RawSensorData, 0, len(ids))
	for _, result := range results {
		data, err := getTimeIndexFromSortedSet(result)
		if err != nil {
			return nil, err
		}

		id := getIDFromRedisKey(result)
		if data != nil && id > 0 {
			rawSensorSlice = append(rawSensorSlice, RawSensorData{
				ID:   id,
				Data: data,
			})
		}
	}

	return rawSensorSlice, nil
}

// SetSensorLocationData takes an array of Purple Air API response structs and creates a map of all
// sensors in the network.
func (c *Controller) SetSensorLocationData(ctx context.Context, data []purpleapi.Response) error {
	temporaryKey := sensorMapKey + utils.CreateRandomString(20)

	_, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, resp := range data {
			// We only want primary, outside sensors.
			if resp.Location == purpleapi.Outside && resp.ParentID == 0 {
				id, err := getPrimaryKey(reflect.ValueOf(resp))
				if err != nil {
					return err
				}

				pipe.GeoAdd(ctx, temporaryKey, &redis.GeoLocation{
					Name:      strconv.Itoa(id),
					Longitude: resp.Longitude,
					Latitude:  resp.Latitude,
				})
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	err = c.exchangeAndRemove(ctx, sensorMapKey, temporaryKey)
	if err != nil {
		return err
	}

	return nil
}

// GetSensorsInRange takes a pair of coordinates and a radius (in meters) and returns a list of sensor IDs within that
// circle (if any exist).
func (c *Controller) GetSensorsInRange(ctx context.Context, longitude, latitude, radius float64) ([]int, error) {
	ids := make([]int, 0, 10)

	results, err := c.db.GeoRadius(ctx, sensorMapKey, longitude, latitude, &redis.GeoRadiusQuery{
		Radius: radius,
		Unit:   "m",
		Sort:   "ASC",
	}).Result()

	if err == redis.Nil {
		return ids, fmt.Errorf(`No sensors at %.4f, %.4f (%.1f m radius)`, longitude, latitude, radius)
	} else if err != nil {
		return ids, err
	}

	for i, sensor := range results {
		if id, err := strconv.Atoi(sensor.Name); err == nil {
			ids = append(ids, id)
		} else {
			log.Errorf(`ID:%d:%d conversion err: %s`, id, i, err)
		}
	}

	return ids, nil
}
