package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGenerateText(t *testing.T) {
	app := fiber.New()
	app.Post("/api/v1/gemini/generate", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	req := httptest.NewRequest("POST", "/api/v1/gemini/generate", nil)

	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	assert.Equal(t, 200, resp.StatusCode)
}
