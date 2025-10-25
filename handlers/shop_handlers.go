package handlers

// import (
// 	"app/database"
// 	"context"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/golang-jwt/jwt/v4"
// )

/*
// HandleGetShopProfile fetches the profile for a user in the context of a specific shop.
func HandleGetShopProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)
	userRole := claims["role"].(string)

	shopID := c.Get("X-Shop-ID")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "X-Shop-ID header is required"})
	}

	var name, email, shopName string

	// Get user details
	userQuery := "SELECT name, email FROM users WHERE id = $1"
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&name, &email); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve user profile"})
	}

	// Get shop name
	shopQuery := "SELECT name FROM shops WHERE id = $1"
	if err := db.QueryRow(ctx, shopQuery, shopID).Scan(&shopName); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop name"})
	}

	return c.JSON(fiber.Map{
		"name":     name,
		"email":    email,
		"role":     userRole,
		"shopName": shopName,
	})
}
*/
