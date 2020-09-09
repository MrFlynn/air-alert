package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mrflynn/air-alert/internal/database"
	"github.com/mrflynn/air-alert/internal/router"
	"github.com/mrflynn/air-alert/internal/task"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	db     *database.Controller
	runner *task.Runner
	server *router.Router
)

func initializeApp(ctx *cli.Context) error {
	var err error

	db, err = database.NewController(ctx)
	if err != nil {
		return err
	}

	runner, err = task.NewRunner(ctx)
	if err != nil {
		return err
	}

	server = router.NewRouter(ctx, db)

	initializeTasks(runner)

	return nil
}

func run(ctx *cli.Context) error {
	// External signal receiver.
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, os.Kill)

	initializeApp(ctx)

	if !ctx.Bool("skip-loading") {
		if err := runner.Start(); err != nil {
			return err
		}
	}

	if err := server.Run(); err != nil {
		return err
	}

	<-stopSignal
	fmt.Printf("\n")

	if err := server.Shutdown(); err != nil {
		return err
	}

	runner.Stop()

	return nil
}

func main() {
	app := cli.App{
		Name:  "air-alert",
		Usage: "A service to alert people of changes in air quality",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "database",
				Aliases: []string{"d"},
				Usage:   "Address of database",
				Value:   ":6379",
			},
			&cli.StringFlag{
				Name:    "database-password",
				Aliases: []string{"pass"},
				Usage:   "Password for database",
				Value:   "",
			},
			&cli.IntFlag{
				Name:    "database-id",
				Aliases: []string{"i"},
				Usage:   "ID of database",
				Value:   0,
			},
			&cli.StringFlag{
				Name:    "timezone",
				Aliases: []string{"tz"},
				Usage:   "Default timezone",
				Value:   "UTC",
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Port for web server",
				Value:   3000,
			},
			&cli.BoolFlag{
				Name:  "skip-loading",
				Usage: "Skip loading data into datastore",
				Value: false,
			},
		},
		Action: run,
		ExitErrHandler: func(ctx *cli.Context, err error) {
			if err != nil {
				log.Fatalf(`got fatal error: %s`, err)
			}
		},
		Authors: []*cli.Author{
			{
				Name:  "Nick Pleatsikas",
				Email: "nick@pleatsikas.me",
			},
		},
	}

	app.Run(os.Args)
}
