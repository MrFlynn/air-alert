package router

import (
	"bytes"
	"strconv"

	"github.com/gofiber/fiber"
	jsoniter "github.com/json-iterator/go"
	"github.com/mrflynn/air-alert/internal/database/redis"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func getAQIReadings(ctx *fiber.Ctx, datastore *redis.Controller) {
	long, err := strconv.ParseFloat(ctx.Query("long"), 32)
	if err != nil {
		log.Error("longitude parameter has invalid format")
		ctx.Status(fiber.StatusBadRequest).SendString("invald or missing longitude parameter")

		return
	}

	lat, err := strconv.ParseFloat(ctx.Query("lat"), 32)
	if err != nil {
		log.Error("latitude parameter has invalid format")
		ctx.Status(fiber.StatusBadRequest).SendString("invalid or missing latitude parameter")

		return
	}

	results, err := datastore.GetAQIFromSensorsInRange(ctx.Context(), long, lat, 2000)
	if err != nil {
		log.Errorf("database error: %s", err)
		ctx.Status(
			fiber.StatusInternalServerError,
		).SendString(
			"could not get sensor data from database",
		)

		return
	}

	buff := bytes.NewBuffer([]byte{})
	err = json.NewEncoder(buff).Encode(results)

	if err != nil {
		log.Errorf("error in marshalling API response data: %s", err)
		ctx.Status(fiber.StatusInternalServerError).SendString("error marshalling json object")

		return
	}

	ctx.Type("json", "utf-8")
	ctx.SendStream(buff, buff.Len())
}
