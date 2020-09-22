package router

import (
	"bytes"
	"strconv"

	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/mrflynn/air-alert/internal/database/redis"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func getAQIReadings(ctx *fiber.Ctx, datastore *redis.Controller) error {
	long, err := strconv.ParseFloat(ctx.Query("long"), 32)
	if err != nil {
		log.Error("longitude parameter has invalid format")

		return errorInfo{
			err: fiber.ErrBadRequest,
			why: "invald or missing longitude parameter",
	}
	}

	lat, err := strconv.ParseFloat(ctx.Query("lat"), 32)
	if err != nil {
		log.Error("latitude parameter has invalid format")

		return errorInfo{
			err: fiber.ErrBadRequest,
			why: "invalid or missing latitude parameter",
	}
	}

	radius, err := strconv.ParseFloat(ctx.Query("radius", "2000.0"), 32)
	if err != nil {
		log.Error("radius parameter has invalid format")

		return errorInfo{
			err: fiber.ErrBadRequest,
			why: "invalid radius parameter",
		}
	}

	results, err := datastore.GetAQIFromSensorsInRange(ctx.Context(), long, lat, radius)
	if err != nil {
		log.Errorf("database error: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "could not get sensor data from database",
	}
	}

	buff := bytes.NewBuffer([]byte{})
	err = json.NewEncoder(buff).Encode(results)

	if err != nil {
		log.Errorf("error in marshalling API response data: %s", err)

		return errorInfo{
			err: fiber.ErrInternalServerError,
			why: "error marshalling json object",
	}
	}

	ctx.Type("json", "utf-8")
	ctx.SendStream(buff, buff.Len())
}
