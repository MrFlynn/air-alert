package router

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gofiber/fiber"
	"github.com/mrflynn/air-alert/internal/database"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Router is the main application HTTP router.
type Router struct {
	Port int

	app *fiber.App
	db  *database.Controller
}

// NewRouter creates a new Router struct from the given context.
func NewRouter(ctx *cli.Context, db *database.Controller) *Router {
	return &Router{
		Port: ctx.Int("port"),
		app:  fiber.New(),
		db:   db,
	}
}

func (r *Router) addRoutes() {
	r.app.Get("/api/data", func(ctx *fiber.Ctx) {
		getAQIReadings(ctx, r.db)
	})
}

// Run starts the router and handles all shutdown operations if an external shutdown signal is
// received.
func (r *Router) Run() error {
	var err error

	extSignalChan := make(chan os.Signal, 1)
	signal.Notify(extSignalChan, os.Interrupt, os.Kill)

	r.addRoutes()

	go func() {
		log.Infof("router is now listening at %d", r.Port)
		err = r.app.Listen(r.Port)
	}()

	if err != nil {
		return err
	}

	<-extSignalChan

	fmt.Printf("\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Signal to indicate that the router has shutdown properly.
	stopChan := make(chan bool, 1)

	// Run shutdown asynchronously so we can properly utilize the context timeout.
	go func() {
		log.Info("attempting to shutdown router...")
		err = r.app.Shutdown()
		stopChan <- true
	}()

	select {
	case <-ctx.Done():
		log.Error("forced shutdown of router")
		err = ctx.Err()
	case <-stopChan:
		log.Info("router has shutdown")
	}

	return err
}
