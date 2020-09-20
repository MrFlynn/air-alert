package router

import (
	"github.com/gofiber/fiber"
	"github.com/mrflynn/air-alert/internal/database/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Router is the main application HTTP router.
type Router struct {
	Port int

	app       *fiber.App
	datastore *redis.Controller
}

// NewRouter creates a new Router struct from the given context.
func NewRouter(datastore *redis.Controller) *Router {
	return &Router{
		Port: viper.GetInt("web.port"),
		app: fiber.New(&fiber.Settings{
			DisableStartupMessage: true,
		}),
		datastore: datastore,
	}
}

func (r *Router) addRoutes() {
	r.app.Get("/api/data", func(ctx *fiber.Ctx) {
		getAQIReadings(ctx, r.datastore)
	})
}

// Run starts the router and handles all shutdown operations if an external shutdown signal is
// received.
func (r *Router) Run() error {
	var err error

	r.addRoutes()

	go func() {
		log.Infof("router is now listening at %d", r.Port)
		err = r.app.Listen(r.Port)
	}()

	return err
}

// Shutdown attempts to safely shutdown the router.
func (r *Router) Shutdown() error {
	log.Debug("attempting to shutdown router...")
	err := r.app.Shutdown()
	log.Debug("router has shutdown")
	return err
}
