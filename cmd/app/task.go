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
		TTL:      60 * time.Second,
		RunFunc: func(ctx context.Context) error {
			log.Info("starting AQI data refresh")

			resp, err := purpleapi.Get(ctx)
			if err != nil {
				return err
			}

			err = db.SetAirQuality(ctx, resp)
			if err != nil {
				return err
			}

			log.Info("completed AQI data refresh")
			return nil
		},
	})

	// Sensor location refresh task.
	runner.AddTask(task.DailyTask{
		TimeOfDay: "03:30",
		Priority:  1,
		TTL:       60 * time.Second,
		RunFunc: func(ctx context.Context) error {
			log.Info("starting sensor location refresh")

			resp, err := purpleapi.Get(ctx)
			if err != nil {
				return err
			}

			err = db.SetSensorLocationData(ctx, resp)
			if err != nil {
				return err
			}

			log.Info("completed sensor location refresh")
			return nil
		},
	})
}
