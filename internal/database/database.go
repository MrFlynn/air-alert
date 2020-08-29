package database

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-redis/redis/v8"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/purpleapi"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	measurementStart = 0
	numMeasurements  = 10
	sensorMapKey     = "sensors"
)

// Controller is a container for a Redis client.
type Controller struct {
	db *redis.Client
}

// NewController creates a new Redis client.
func NewController(ctx *cli.Context) (Controller, error) {
	db := redis.NewClient(&redis.Options{
		Addr:     ctx.String("database"),
		Password: ctx.String("database-password"),
		DB:       ctx.Int("database-id"),
	})

	if _, err := db.Ping(ctx.Context).Result(); err != nil {
		return Controller{}, err
	}

	return Controller{
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
	_, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, resp := range data {
			id, err := getPrimaryKey(reflect.ValueOf(resp))
			if err != nil {
				return err
			}

			aqi := resp.PM25
			key := "measurements:" + strconv.Itoa(id) + ":quality"

			// Only keep the last 10 measurements (previous 50 minutes of data)
			pipe.LPush(ctx, key, aqi)
			pipe.LTrim(ctx, key, measurementStart, numMeasurements-1)
		}

		return nil
	})

	return err
}

// GetAirQuality gets 10 most recent PM2.5 AQI readings from a specific sensor.
func (c *Controller) GetAirQuality(ctx context.Context, id int) ([]float64, error) {
	key := "measurements:" + strconv.Itoa(id) + ":quality"
	qualityList := make([]float64, 0, numMeasurements)

	measurements, err := c.db.LRange(ctx, key, measurementStart, numMeasurements-1).Result()
	if err == redis.Nil {
		return qualityList, fmt.Errorf(`could not find data for sensor %d`, id)
	} else if err != nil {
		return qualityList, err
	}

	for i, stringValue := range measurements {
		if aqi, err := strconv.ParseFloat(stringValue, 32); err == nil {
			qualityList = append(qualityList, aqi)
		} else {
			log.Errorf(`ID:%d:%d conversion err: %s`, id, i, err)
		}
	}

	return qualityList, nil
}

// SetSensorLocationData takes an array of Purple Air API response structs and creates a map of all
// sensors in the network.
func (c *Controller) SetSensorLocationData(ctx context.Context, data []purpleapi.Response) error {
	temporaryKey := sensorMapKey + utils.CreateRandomString(20)

	_, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, resp := range data {
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
