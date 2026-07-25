package main

import (
	"app/config"
	"app/routes"
	"fmt"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// This verifies that every registered HTTP method/path is actually reachable;
// business-flow tests cover authenticated success paths separately.
func TestRegisteredRouteSurfaceDoesNotReturnNotFound(t *testing.T) {
	config.AppConfig.JWTSecret = "route-audit-secret"
	app := fiber.New()
	routes.SetupRoutes(app)
	param := regexp.MustCompile(`:[^/]+`)
	tested := 0
	for _, methodRoutes := range app.Stack() {
		for _, route := range methodRoutes {
			if route.Method == fiber.MethodHead || route.Method == fiber.MethodOptions || route.Path == "*" {
				continue
			}
			path := param.ReplaceAllString(route.Path, "00000000-0000-0000-0000-000000000000")
			resp, err := app.Test(httptest.NewRequest(route.Method, path, nil), int(2*time.Second/time.Millisecond))
			if err != nil {
				t.Fatalf("route %s %s returned transport error: %v", route.Method, route.Path, err)
			}
			if resp.StatusCode == 404 && len(route.Handlers) == 1 {
				continue // middleware-only group entry exposed by Fiber's Stack().
			}
			if resp.StatusCode == 404 {
				t.Fatalf("registered route was not matched: %s %s", route.Method, route.Path)
			}
			tested++
		}
	}
	if tested < 140 {
		t.Fatalf("route audit covered only %d routes; expected at least 140", tested)
	}
	fmt.Printf("route surface audit: tested=%d\n", tested)
}
