
package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetShopProfile godoc
// @Summary Get shop user profile
// @Description Fetches the profile of the currently authenticated staff user, including their assigned shop details.
// @Tags Shop
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {object} models.User
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 404 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/shop/profile [get]
func HandleGetShopProfile(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// Get user from JWT claims
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	var user models.User
	var shop models.Shop

	// 1. Fetch user details
	userQuery := `SELECT id, name, email, role, is_active, assigned_shop_id, merchant_id, created_at, updated_at FROM users WHERE id = $1`
	err := db.QueryRow(ctx, userQuery, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive,
		&user.AssignedShopID, &user.MerchantID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error fetching user profile: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	// 2. If user is assigned to a shop, fetch shop details
	if user.AssignedShopID != nil && *user.AssignedShopID != "" {
		shopQuery := `SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE id = $1`
		err := db.QueryRow(ctx, shopQuery, *user.AssignedShopID).Scan(
			&shop.ID, &shop.Name, &shop.MerchantID, &shop.Address, &shop.Phone,
			&shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error fetching assigned shop details: %v", err)
			// Not a fatal error, we can still return the user profile without the shop
		} else {
			user.Shop = &shop
		}
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

