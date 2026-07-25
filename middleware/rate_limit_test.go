package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimit(t *testing.T) {
	app := fiber.New()
	app.Use(RateLimit(1, time.Minute))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	first, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || first.StatusCode != fiber.StatusNoContent {
		t.Fatalf("first request should pass, status=%v err=%v", first.StatusCode, err)
	}
	second, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || second.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second request should be limited, status=%v err=%v", second.StatusCode, err)
	}
}
