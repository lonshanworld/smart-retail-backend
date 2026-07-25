package handlers

import (
	"app/database"
	"context"
	"github.com/gofiber/fiber/v2"
)

func HandleHealth(c *fiber.Ctx) error {
	db := database.GetDB()
	if db == nil || db.Ping(context.Background()) != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "unhealthy", "database": "unavailable"})
	}
	return c.JSON(fiber.Map{"status": "ok", "database": "ok"})
}
