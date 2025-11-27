package main

import (
	"net/http/httptest"
	"testing"

	"app/middleware"

	"github.com/gofiber/fiber/v2"
)

// Helper to create an app with a pre-local middleware that sets userRole
func makeAppWithRole(role string, check func(*fiber.Ctx) error) *fiber.App {
	app := fiber.New()

	// Insert a middleware to set role before the requirement middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userRole", role)
		return c.Next()
	})

	app.Use(check)

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("ok")
	})

	return app
}

func TestAdminRequired_AllowsAdmin(t *testing.T) {
	app := makeAppWithRole("admin", middleware.AdminRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for admin role, got %d", resp.StatusCode)
	}
}

func TestAdminRequired_DeniesNonAdmin(t *testing.T) {
	app := makeAppWithRole("staff", middleware.AdminRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for non-admin role, got %d", resp.StatusCode)
	}
}

func TestMerchantRequired_AllowsMerchant(t *testing.T) {
	app := makeAppWithRole("merchant", middleware.MerchantRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for merchant role, got %d", resp.StatusCode)
	}
}

func TestMerchantRequired_DeniesNonMerchant(t *testing.T) {
	app := makeAppWithRole("admin", middleware.MerchantRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for non-merchant role, got %d", resp.StatusCode)
	}
}

func TestStaffRequired_AllowsStaff(t *testing.T) {
	app := makeAppWithRole("staff", middleware.StaffRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for staff role, got %d", resp.StatusCode)
	}
}

func TestStaffRequired_DeniesNonStaff(t *testing.T) {
	app := makeAppWithRole("merchant", middleware.StaffRequired)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for non-staff role, got %d", resp.StatusCode)
	}
}
