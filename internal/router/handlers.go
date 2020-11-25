package router

import (
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	utils "github.com/mrflynn/air-alert/internal"
	"github.com/mrflynn/air-alert/internal/database/redis"
	"github.com/mrflynn/air-alert/internal/database/sql"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func getLocationParameters(ctx *fiber.Ctx) (float64, float64, float64, error) {
	var (
		value             decimal.Decimal
		lat, long, radius float64
		err               error
	)

	value, err = decimal.NewFromString(ctx.Params("longitude"))
	if err != nil {
		err = errorInfo{
			err: fiber.ErrBadRequest,
			why: "invald or missing longitude parameter",
		}

		goto Finish
	}
	long, _ = value.Round(2).Float64()

	value, err = decimal.NewFromString(ctx.Params("latitude"))
	if err != nil {
		err = errorInfo{
			err: fiber.ErrBadRequest,
			why: "invalid or missing latitude parameter",
		}

		goto Finish
	}
	lat, _ = value.Round(2).Float64()

	value, err = decimal.NewFromString(ctx.Query("radius", "2000.0"))
	if err != nil {
		err = errorInfo{
			err: fiber.ErrBadRequest,
			why: "invalid radius parameter",
		}

		goto Finish
	}
	radius, _ = value.Round(2).Float64()

Finish:
	return long, lat, radius, err
}

func getAQIReadings(ctx *fiber.Ctx, datastore *redis.Controller) error {
	long, lat, radius, err := getLocationParameters(ctx)
	if err != nil {
		return err
	}

	results, err := datastore.GetAQIFromSensorsInRange(ctx.Context(), long, lat, radius)
	if err != nil {
		log.Errorf("database error: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not get sensor data from database",
		}
	}

	err = json.NewEncoder(ctx.Type("json", "utf-8").Response().BodyWriter()).Encode(results)

	if err != nil {
		log.Errorf("error in marshalling API response data: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "error marshalling json object",
		}
	}

	return nil
}

func getAverageAQI(ctx *fiber.Ctx, datastore *redis.Controller) error {
	long, lat, radius, err := getLocationParameters(ctx)
	if err != nil {
		return err
	}

	ids, err := datastore.GetSensorsInRange(ctx.Context(), long, lat, radius)
	if err != nil {
		log.Errorf("GetSensorsInRange error: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not get sensor ids from database",
		}
	}

	data, err := datastore.GetTimeSeriesData(ctx.Context(), 1, ids...)
	if err != nil {
		log.Errorf("GetTimeSeriesData error: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not get sensor data from database",
		}
	}

	var (
		aqi       float64
		prevCount int
	)

	for _, d := range data {
		if !utils.IsNil(d.AQI) {
			aqi = utils.RecalculateAverage(d.AQI, aqi, prevCount)
			prevCount++
		}
	}

	return ctx.SendString(decimal.NewFromFloat(aqi).Round(1).String())
}

func subscribeToNotifications(ctx *fiber.Ctx, database *sql.Controller) error {
	var req sql.UserRequest

	err := ctx.BodyParser(&req)
	if err != nil {
		log.Errorf("could not parse subscription request: %s", err)

		return errorInfo{
			err: fiber.ErrBadRequest,
			why: "could not parse subscription request",
		}
	}

	id, err := database.CreateUser(ctx.Context(), req)
	if err != nil {
		log.Errorf("could not create user: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not subscribe user",
		}
	}

	log.Infof("registered user %d", id)

	return ctx.SendStatus(fiber.StatusCreated)
}

func unsubscribeFromNofications(ctx *fiber.Ctx, database *sql.Controller) error {
	var req sql.UserRequest

	err := ctx.BodyParser(&req)
	if err != nil {
		log.Errorf("could not parse unsubscribe request: %s", err)

		return errorInfo{
			err: fiber.ErrBadRequest,
			why: "could not parse unsubscribe request",
		}
	}

	err = database.DeleteUser(ctx.Context(), req)
	if err != nil {
		log.Errorf("could not delete user: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not unsubcribe user",
		}
	}

	log.Info("unsubscribed user")

	return ctx.SendStatus(fiber.StatusOK)
}
