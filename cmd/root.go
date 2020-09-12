package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrflynn/air-alert/internal/database"
	"github.com/mrflynn/air-alert/internal/purpleapi"
	"github.com/mrflynn/air-alert/internal/router"
	"github.com/mrflynn/air-alert/internal/task"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string
	db         *database.Controller
	taskRunner *task.Runner
	server     *router.Router

	rootCmd = &cobra.Command{
		Use:   "air-alert",
		Short: "A server for alerting people to air quality changes",
		Long: `Air Alert is a web application for alerting people to changes in air quality through 
web push notifications`,
		RunE: run,
	}
)

func init() {
	cobra.OnInitialize(initConfig, initApp, initTasks)

	rootCmd.Flags().StringVarP(
		&configFile, "config", "c", "", "configuration file (default is $PWD/config.toml)",
	)
	rootCmd.Flags().BoolP("skip-startup", "s", false, "skip startup tasks")
	rootCmd.Flags().MarkHidden("skip-startup") // The above should not be in help menus.

	// Program information.
	viper.SetDefault("author", "Nick Pleatsikas <nick@pleatsikas.me>")

	// Default settings.
	viper.SetDefault("database.redis.addr", ":6379")
	viper.SetDefault("database.redis.password", "")
	viper.SetDefault("database.redis.id", 0)
	viper.SetDefault("timezone", "UTC")
	viper.SetDefault("web.port", 3000)

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("toml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf(`could not find configuration file: "%s"`, viper.ConfigFileUsed())
		} else {
			log.Fatal(err)
		}
	}
}

func initApp() {
	var err error

	db, err = database.NewController()
	if err != nil {
		log.Fatal(err)
	}

	taskRunner, err = task.NewRunner()
	if err != nil {
		log.Fatal(err)
	}

	server = router.NewRouter(db)
}

func initTasks() {
	// Air quality refresh task.
	taskRunner.AddTask(task.MinuteTask{
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
	taskRunner.AddTask(task.DailyTask{
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

func run(cmd *cobra.Command, args []string) error {
	// External signal receiver.
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, os.Kill)

	if err := taskRunner.Start(); err != nil {
		return err
	}

	if err := server.Run(); err != nil {
		return err
	}

	<-stopSignal
	fmt.Printf("\n")

	if err := server.Shutdown(); err != nil {
		return err
	}

	taskRunner.Stop()

	return nil
}

// Execute runs the main application and/or any child subcommands.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
