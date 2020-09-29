package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mrflynn/air-alert/internal/database/redis"
	pg "github.com/mrflynn/air-alert/internal/database/sql"
	"github.com/mrflynn/air-alert/internal/notifications"
	"github.com/mrflynn/air-alert/internal/router"
	"github.com/mrflynn/air-alert/internal/task"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	// pq is required for postgres database connection.
	_ "github.com/lib/pq"
)

var (
	configFile string
	datastore  *redis.Controller
	database   *pg.Controller
	taskRunner *task.Runner
	notifier   *notifications.Sender
	server     *router.Router

	rootCmd = &cobra.Command{
		Use:   "air-alert",
		Short: "A server for alerting people to air quality changes",
		Long: `Air Alert is a web application for alerting people to changes in air quality through 
web push notifications`,
		RunE:    run,
		PostRun: shutdown,
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

	// Default redis settings.
	viper.SetDefault("database.redis.addr", ":6379")
	viper.SetDefault("database.redis.password", "")
	viper.SetDefault("database.redis.id", 0)

	// Default postgres settings.
	viper.SetDefault("database.postgres.host", "localhost")
	viper.SetDefault("database.postgres.port", 5432)
	viper.SetDefault("database.postgres.username", "postgres")
	viper.SetDefault("database.postgres.password", "")
	viper.SetDefault("database.postgres.database", "airalert")

	// Default web server settings.
	viper.SetDefault("web.addr", ":3000")
	viper.SetDefault("web.template_dir", "./templates")
	viper.SetDefault("web.static_dir", "./static")

	// Default notification settings.
	viper.SetDefault("web.notifications.threads", 4)
	viper.SetDefault("web.notifications.group", "notification_delivery")
	viper.SetDefault("web.notifications.public_key", "")
	viper.SetDefault("web.notifications.private_key", "")
	viper.SetDefault("web.notifications.admin_mail", "admin@localhost")

	// Other default settings.
	viper.SetDefault("timezone", "UTC")
	viper.SetDefault("purpleair.url", "https://www.purpleair.com/json")
	viper.SetDefault("purpleair.rate_limit_timeout", 10*time.Second)

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

	viper.SetEnvPrefix("AIR_ALERT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	datastore, err = redis.NewController()
	if err != nil {
		log.Fatal(err)
	}

	taskRunner, err = task.NewRunner()
	if err != nil {
		log.Fatal(err)
	}

	var dbConn *sql.DB
	dbConn, err = sql.Open(
		"postgres",
		fmt.Sprintf(
			"dbname=%s user=%s password=%s host=%s port=%d",
			viper.GetString("database.postgres.database"),
			viper.GetString("database.postgres.username"),
			viper.GetString("database.postgres.password"),
			viper.GetString("database.postgres.host"),
			viper.GetInt("database.postgres.port"),
		),
	)

	if err != nil {
		log.Fatal(err)
	}

	database, err = pg.NewController(dbConn)
	if err != nil {
		log.Fatal(err)
	}

	notifier = notifications.NewSender(datastore, database)

	server = router.NewRouter(datastore)
}

func initTasks() {
	// Air quality refresh task.
	taskRunner.AddTask(task.MinuteTask{
		Rate:     5,
		Priority: 2,
		TTL:      60 * time.Second,
		RunFunc:  updateAQITask,
	})

	// Sensor location refresh task.
	taskRunner.AddTask(task.DailyTask{
		TimeOfDay: "03:30",
		Priority:  1,
		TTL:       60 * time.Second,
		RunFunc:   updateSensorsTask,
	})

	// Notification stream task.
	taskRunner.AddTask(task.MinuteTask{
		Rate:     5,
		Priority: 3,
		TTL:      120 * time.Second,
		RunFunc:  generateNotifications,
	})
}

func run(cmd *cobra.Command, args []string) error {
	// External signal receiver.
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, os.Kill)

	if err := taskRunner.Start(); err != nil {
		return err
	}

	notifier.Run()

	if err := server.Run(); err != nil {
		return err
	}

	<-stopSignal
	fmt.Printf("\n")

	return nil
}

func shutdown(cmd *cobra.Command, args []string) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hasShutdown := make(chan bool, 1)
	errs := make(chan error, 3)

	go func() {
		if err := server.Shutdown(); err != nil {
			errs <- err
		}

		taskRunner.Stop()

		notifier.Shutdown()

		if err := database.Shutdown(); err != nil {
			errs <- err
		}

		if err := datastore.Shutdown(); err != nil {
			errs <- err
		}

		hasShutdown <- true
	}()

	select {
	case <-shutdownCtx.Done():
		log.Error(shutdownCtx.Err())
	case <-hasShutdown:
		log.Info("air-alert has shutdown")
	case err := <-errs:
		log.Error(err)

		// Log any remaining errors.
		for err := range errs {
			log.Error(err)
		}
	}
}

// Execute runs the main application and/or any child subcommands.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
