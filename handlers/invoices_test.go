package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestInvoicesRouteNotFound(t *testing.T) {
	app := fiber.New()
	// we don't register invoices route here; expect 404
	req := httptest.NewRequest("GET", "/api/v1/merchant/invoices", nil)
	resp, _ := app.Test(req)
	assert.Equal(t, 404, resp.StatusCode)
}
