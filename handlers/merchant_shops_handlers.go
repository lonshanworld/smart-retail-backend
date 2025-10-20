package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleListMerchantShops handles listing all shops for the current merchant.
func HandleListMerchantShops(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	query := "SELECT id, name, address, merchant_id, is_active, created_at, updated_at FROM shops WHERE merchant_id = $1 ORDER BY created_at DESC"

	rows, err := db.Query(ctx, query, merchantId)
	if err != nil {
		log.Printf("Error listing merchant shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	shops := make([]models.Shop, 0)
	for rows.Next() {
		var shop models.Shop
		if err := rows.Scan(&shop.ID, &shop.Name, &shop.Address, &shop.MerchantID, &shop.IsActive, &shop.CreatedAt, &shop.UpdatedAt); err != nil {
			log.Printf("Error scanning shop row: %v", err)
			continue
		}
		shops = append(shops, shop)
	}

	return c.JSON(fiber.Map{"status": "success", "data": shops})
}

// HandleUpdateMerchantShop handles updating a shop owned by the current merchant.
func HandleUpdateMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	shopId := c.Params("shopId")

	type UpdateShopRequest struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}

	var req UpdateShopRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := "UPDATE shops SET name = $1, address = $2, updated_at = NOW() WHERE id = $3 AND merchant_id = $4 RETURNING id, name, address, merchant_id, is_active, created_at, updated_at"

	var shop models.Shop
	err := db.QueryRow(ctx, query, req.Name, req.Address, shopId, merchantId).Scan(&shop.ID, &shop.Name, &shop.Address, &shop.MerchantID, &shop.IsActive, &shop.CreatedAt, &shop.UpdatedAt)

	if err != nil {
		log.Printf("Error updating merchant shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update shop. It might not exist or you may not have permission."})
	}

	return c.JSON(fiber.Map{"status": "success", "data": shop})
}

// HandleDeleteMerchantShop handles deleting a shop owned by the current merchant.
func HandleDeleteMerchantShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	shopId := c.Params("shopId")

	query := "DELETE FROM shops WHERE id = $1 AND merchant_id = $2"

	res, err := db.Exec(ctx, query, shopId, merchantId)
	if err != nil {
		log.Printf("Error deleting merchant shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found or you do not have permission to delete it."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success"})
}
