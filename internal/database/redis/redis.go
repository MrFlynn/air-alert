package redis

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/purpleapi"
	"github.com/mrflynn/go-aqi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	sensorMapKey      = "sensors"
	forecastStreamKey = "forecast"
	streamSize        = 10
)

// Controller is a container for a Redis client.
type Controller struct {
	db *redis.Client
}

// NewController creates a new Redis client.
func NewController() (*Controller, error) {
	db := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("database.redis.addr"),
		Password: viper.GetString("database.redis.password"),
		DB:       viper.GetInt("database.redis.id"),
	})

	if err := db.Ping(context.Background()).Err(); err != nil {
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

				pm25Key := createRedisKey(id, "data", "pm25")
				aqiKey := createRedisKey(id, "data", "aqi")

				// Add pm2.5 value with score being equal to reading capture time.
				pipe.ZAddNX(ctx, pm25Key, &redis.Z{
					Score:  float64(resp.LastUpdated),
					Member: resp.PM25,
				})

				// Add calculated AQI if the result is valid.
				if aqi, err := aqi.Calculate(aqi.PM25{Concentration: resp.PM25}); err == nil {
					pipe.ZAddNX(ctx, aqiKey, &redis.Z{
						Score:  float64(resp.LastUpdated),
						Member: aqi.AQI,
					})
				}

				// This removes all measurements older than 60 minutes.
				pipe.ZRemRangeByScore(ctx, pm25Key, "0", cutoffTime)
				pipe.ZRemRangeByScore(ctx, aqiKey, "0", cutoffTime)
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
	Time int     `json:"time"`
	PM25 float64 `json:"pm25"`
	AQI  float64 `json:"aqi,omitempty"`
}

// GetTimeSeriesData takes a list of sensor IDs and returns the time-series sensor and computed data
// for each sensor.
func (c *Controller) GetTimeSeriesData(ctx context.Context, ids []int, count ...int64) (map[UnionKey]*RawQualityData, error) {
	pipelineResults, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, id := range ids {
			err := addAQIRequestToPipe(ctx, pipe, id, count...)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	compositeDataMap, err := serializeSensorData(pipelineResults)
	if err != nil {
		return nil, err
	}

	return compositeDataMap, nil
}

// GetAirQuality gets 10 most recent PM2.5 AQI readings from a specific sensor.
func (c *Controller) GetAirQuality(ctx context.Context, id int) (*RawSensorData, error) {
	data, err := c.GetTimeSeriesData(ctx, []int{id})
	if err != nil {
		return nil, err
	}

	for key, item := range data {
		if itemKey, err := key.ID(); err == nil {
			if id == itemKey {
				return &RawSensorData{
					ID:   itemKey,
					Data: []RawQualityData{*item},
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("could not get sensor data for sensor id: %d", id)
}

// GetAQIFromSensorsInRange returns raw sensor data from all sensors within the specified
// radius around the given coordinates.
func (c *Controller) GetAQIFromSensorsInRange(ctx context.Context, longitude, latitude, radius float64) ([]*RawSensorData, error) {
	ids, err := c.GetSensorsInRange(ctx, longitude, latitude, radius)
	if err != nil {
		return nil, err
	}

	compositeDataMap, err := c.GetTimeSeriesData(ctx, ids)
	if err != nil {
		return nil, err
	}

	sensorResultMap := make(map[int]*RawSensorData, len(compositeDataMap))
	for key, item := range compositeDataMap {
		if id, err := key.ID(); err == nil {
			if _, ok := sensorResultMap[id]; !ok {
				sensorResultMap[id] = &RawSensorData{
					ID:   id,
					Data: make([]RawQualityData, 0, 10), // There will only every be a maximum of 10 results.
				}
			}

			sensorResultMap[id].Data = append(sensorResultMap[id].Data, *item)
		}
	}

	rawSensorSlice := make([]*RawSensorData, 0, len(compositeDataMap))
	for _, sensor := range sensorResultMap {
		rawSensorSlice = append(rawSensorSlice, sensor)
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

// AQIForecast indicates changes in the direction of AQI values. In other words it indicates whether
// or not the AQI is increasing, decreasing, or remaining the same.
type AQIForecast int

const (
	// AQIStatic indicates that the AQI is not changing.
	AQIStatic AQIForecast = iota
	// AQIIncreasing indicates that the AQI is increasing.
	AQIIncreasing
	// AQIDecreasing indicates that the AQI is decreasing.
	AQIDecreasing
)

// AQIStreamData contains data to insert into Redis stream that contains changing AQI information.
type AQIStreamData struct {
	ID       int
	AQI      float64
	Forecast AQIForecast
}

func (a AQIStreamData) getStreamArgs() map[string]interface{} {
	return map[string]interface{}{
		"aqi":      a.AQI,
		"forecast": a.Forecast,
	}
}

// AddToForecastStream adds one or more AQIStreamData items into the forecast stream.
func (c *Controller) AddToForecastStream(ctx context.Context, data ...AQIStreamData) error {
	_, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, d := range data {
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: createRedisKey(d.ID, forecastStreamKey),
				MaxLen: streamSize,
				ID:     "*",
				Values: d.getStreamArgs(),
			})
		}

		return nil
	})

	return err
}

// GetForecastStreamForIDs ...
func (c *Controller) GetForecastStreamForIDs(ctx context.Context, ids ...int) ([]AQIStreamData, error) {
	pipelineResults, err := c.db.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, id := range ids {
			pipe.XRangeN(ctx, createRedisKey(id, forecastStreamKey), "+", "-", 1)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return getForecastsFromStream(pipelineResults)
}
