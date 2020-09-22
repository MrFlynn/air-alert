package cmd

import (
	"context"
	"math"
	"sort"
	"time"

	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/database/redis"
	"github.com/mrflynn/air-alert/internal/purpleapi"
	log "github.com/sirupsen/logrus"
)

func updateAQITask(ctx context.Context) error {
	log.Info("starting AQI data refresh")

	resp, err := purpleapi.Get(ctx)
	if err != nil {
		return err
	}

	err = datastore.SetAirQuality(ctx, resp)
	if err != nil {
		return err
	}

	log.Info("completed AQI data refresh")
	return nil
}

func updateSensorsTask(ctx context.Context) error {
	log.Info("starting sensor location refresh")

	resp, err := purpleapi.Get(ctx)
	if err != nil {
		return err
	}

	err = datastore.SetSensorLocationData(ctx, resp)
	if err != nil {
		return err
	}

	log.Info("completed sensor location refresh")
	return nil
}

type coordinatePair [2]float64

type forecastCacheItem struct {
	aqi       float64
	forecast  redis.AQIForecast
	crossover time.Time
}

// This function finds the point (if it exists) where the measured AQI passes the user's
// selected threshold.
func findCrossover(data []*redis.RawQualityData, threshold float64) (bool, time.Time) {
	if len(data) < 1 {
		return false, time.Time{}
	}

	if currAQI := data[0].AQI; !utils.IsNil(currAQI) {
		prevSign := math.Signbit(currAQI - threshold)

		for _, d := range data[1:] {
			if !utils.IsNil(d.AQI) {
				newSign := math.Signbit(d.AQI - threshold)

				// If the new and old signs do not match or the difference is zero, then
				// the crossover time has been discovered.
				if newSign != prevSign || (d.AQI-threshold) == 0 {
					return true, time.Unix(int64(d.Time), 0)
				}

				prevSign = newSign
			}
		}
	}

	return false, time.Time{}
}

func generateNotifications(ctx context.Context) error {
	log.Info("starting notification generator task")

	users, err := database.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	cache := make(map[coordinatePair]forecastCacheItem, len(users))
	for _, user := range users {
		var (
			// aqi and aqiDiff are running averages of all AQI readings and the absolute difference in
			// readings across 1 hour.
			aqi       float64
			aqiDiff   float64
			forecast  redis.AQIForecast
			crossover time.Time
		)

		// If we've already calculated the AQI, forecast, and crossover time for the given coordinates,
		// then we can reuse the previous results to speed up certain calculations. This will speed up
		// notification delivery where multiple users are in the same geographic area.
		if res, ok := cache[coordinatePair{user.Longitude, user.Latitude}]; ok {
			aqi = res.aqi
			forecast = res.forecast
			crossover = res.crossover
		} else {
			sensors, err := datastore.GetAQIFromSensorsInRange(ctx, user.Longitude, user.Latitude, 2000)
			if err != nil {
				log.Errorf("could not get sensors: %s", err)

				continue
			}

			var prevCount int
			for _, sensor := range sensors {
				// Want to sort in descending order.
				sort.Slice(sensor.Data, func(i, j int) bool {
					return sensor.Data[i].Time > sensor.Data[j].Time
				})

				if ok, newCrossover := findCrossover(sensor.Data, user.AQIThreshold); ok {
					aqi = utils.RecalculateAverage(sensor.Data[0].AQI, aqi, prevCount)
					aqiDiff = utils.RecalculateAverage(
						sensor.Data[0].AQI-sensor.Data[len(sensor.Data)-1].AQI, aqiDiff, prevCount,
					)

					// Take the newest time.
					if newCrossover.After(crossover) {
						crossover = newCrossover
					}

					prevCount++
				}
			}

			// We only want to consider a forecast as changing if the difference between AQI
			// values over 1 hour is > 10.
			if aqiDiff > 10 {
				forecast = redis.AQIIncreasing
			} else if aqiDiff < -10 {
				forecast = redis.AQIDecreasing
			} else {
				forecast = redis.AQIStatic
			}

			cache[coordinatePair{user.Longitude, user.Latitude}] = forecastCacheItem{
				aqi, forecast, crossover,
			}
		}

		var oldCrossover time.Time
		if user.LastCrossover.Valid {
			oldCrossover = user.LastCrossover.Time
		}

		// Only send a notification if forecast is changing and a new crossover point has been found.
		if forecast != redis.AQIStatic && crossover.After(oldCrossover) {
			// Store new computed crossover time.
			if err := database.UpdateCrossoverTime(ctx, user.ID, crossover); err != nil {
				log.Errorf("could not update crossover time: %s", err)
			}

			if err := datastore.AddToNotificationStream(ctx, redis.NotificationStream{
				UID:      user.ID,
				AQI:      aqi,
				Forecast: forecast,
			}); err != nil {
				log.Errorf("could not push notification: %s", err)
			}

			log.Debugf("updated crossover time and created notification for user: %d", user.ID)
		}
	}

	log.Info("stopping notification generator task")
	return nil
}
