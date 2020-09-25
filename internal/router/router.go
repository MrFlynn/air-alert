package router

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
	"github.com/mrflynn/air-alert/internal/database/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Router is the main application HTTP router.
type Router struct {
	Address string

	app       *fiber.App
	datastore *redis.Controller
}

type errorInfo struct {
	err *fiber.Error
	why string
}

func (e errorInfo) Error() string {
	return fmt.Sprintf("%s: %s", e.err, e.why)
}

// NewRouter creates a new Router struct from the given context.
func NewRouter(datastore *redis.Controller) *Router {
	router := &Router{
		Address: viper.GetString("web.addr"),
		app: fiber.New(fiber.Config{
			DisableStartupMessage: true,
			ErrorHandler: func(ctx *fiber.Ctx, err error) error {
				if info, ok := err.(errorInfo); ok {
					return ctx.Status(info.err.Code).SendString(info.why)
				}

				return ctx.SendStatus(fiber.StatusInternalServerError)
			},
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  30 * time.Second,
			Views:        html.New(viper.GetString("web.template_dir"), ".html").Reload(true),
		}),
		datastore: datastore,
	}

	router.app.Static("/static", viper.GetString("web.static_dir"))

	return router
}

func (r *Router) addRoutes() {
	api := r.app.Group("/api/v0")

	api.Get("/sensors", func(ctx *fiber.Ctx) error {
		return getAQIReadings(ctx, r.datastore)
	})

	r.app.Get("/aqi/current", func(ctx *fiber.Ctx) error {
		return getAverageAQI(ctx, r.datastore)
	})

	r.app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.Render("index", fiber.Map{})
	})
}

// Run starts the router and handles all shutdown operations if an external shutdown signal is
// received.
func (r *Router) Run() error {
	var err error

	r.addRoutes()

	go func() {
		log.Infof("router is now listening at %s", r.Address)
		err = r.app.Listen(r.Address)
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
