package main

import (
	"context"
	"time"

	"github.com/mrflynn/air-alert/internal/purpleapi"
	"github.com/mrflynn/air-alert/internal/task"
	log "github.com/sirupsen/logrus"
)

func initializeTasks(runner *task.Runner) {
	// Air quality refresh task.
	runner.AddTask(task.MinuteTask{
		Rate:     5,
		Priority: 2,
		RunFunc: func() error {
			err := createTask("aqi data", db.SetAirQuality)
			if err != nil {
				log.Errorf(`aqi data refresh failed: %s`, err)
			}

			return err
		},
	})

	// Sensor location refresh task.
	runner.AddTask(task.DailyTask{
		TimeOfDay: "03:30",
		Priority:  1,
		RunFunc: func() error {
			err := createTask("sensor locations", db.SetSensorLocationData)
			if err != nil {
				log.Errorf(`aqi data refresh failed: %s`, err)
			}

			return err
		},
	})
}

func createTask(name string, dbFunc func(context.Context, []purpleapi.Response) error) error {
	log.Infof(`starting %s refresh`, name)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := purpleapi.Get(ctx)
	if err != nil {
		return err
	}

	err = dbFunc(ctx, resp)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			return ctx.Err()
		}
	default:
		log.Infof(`completed %s refresh`, name)
	}

	return nil
}
