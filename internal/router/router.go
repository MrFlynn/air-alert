package router

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/database/redis"
	"github.com/mrflynn/air-alert/internal/database/sql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/crypto/acme/autocert"
)

// Router is the main application HTTP router.
type Router struct {
	Address string

	app       *fiber.App
	datastore *redis.Controller
	database  *sql.Controller
}

type errorInfo struct {
	err *fiber.Error
	why string
}

func (e errorInfo) Error() string {
	return fmt.Sprintf("%s: %s", e.err, e.why)
}

// NewRouter creates a new Router struct from the given context.
func NewRouter(datastore *redis.Controller, database *sql.Controller) *Router {
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
		database:  database,
	}

	router.app.Static("/", viper.GetString("web.static_dir"))

	return router
}

func (r *Router) addRoutes() {
	r.app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.Render("components/home", fiber.Map{}, "index")
	})

	r.app.Post("/subscribe", func(ctx *fiber.Ctx) error {
		return subscribeToNotifications(ctx, r.database)
	})

	r.app.Get("/subscribe/key", func(ctx *fiber.Ctx) error {
		key, err := base64.RawURLEncoding.DecodeString(viper.GetString("web.notifications.public_key"))
		if err != nil {
			log.Errorf("could not decode vapid public key: %s", err)

			return errorInfo{
				err: fiber.ErrInternalServerError,
				why: "could not render home page",
			}
		}

		return ctx.Send(key)
	})

	r.app.Delete("/unsubscribe", func(ctx *fiber.Ctx) error {
		return unsubscribeFromNofications(ctx, r.database)
	})

	r.app.Get("/aqi/current", func(ctx *fiber.Ctx) error {
		return getAverageAQI(ctx, r.datastore)
	})

	api := r.app.Group("/api/v0")

	api.Get("/sensors", func(ctx *fiber.Ctx) error {
		return getAQIReadings(ctx, r.datastore)
	})
}

// Run starts the router and handles all shutdown operations if an external shutdown signal is
// received.
func (r *Router) Run() error {
	var err error

	r.addRoutes()

	var ln net.Listener
	if viper.GetBool("web.ssl.enable") {
		cache, err := utils.NewCache()
		if err != nil {
			return err
		}

		domains := viper.GetStringSlice("web.ssl.domains")
		if len(domains) < 1 {
			return errors.New("No domains provided. You must provide a list of domains when SSL is enabled")
		}

		manager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      cache,
			HostPolicy: autocert.HostWhitelist(domains...),
			Email:      viper.GetString("web.ssl.email"),
		}

		config := manager.TLSConfig()
		config.MinVersion = tls.VersionTLS12
		config.PreferServerCipherSuites = true

		ln, err = tls.Listen("tcp4", r.Address, config)
		if err != nil {
			return err
		}
	} else {
		ln, err = net.Listen("tcp4", r.Address)
		if err != nil {
			return err
		}
	}

	go func() {
		log.Infof("router is now listening at %s", r.Address)
		err = r.app.Listener(ln)
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
