package handlers

import (
	"context"
	"fmt"
	"strconv"

	"app/database"
	"app/models"

	"github.com/gofiber/fiber/v2"
)

func HandleCreateShop(c *fiber.Ctx) error {
	req := new(models.CreateShopRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot parse JSON"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Shop name is required"})
	}

	userID, ok := c.Locals("userID").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid user ID in token"})
	}
	merchantID, err := strconv.Atoi(userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid user ID format"})
	}

	ctx := context.Background()
	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	var newShop models.Shop
	var newShopID int
	newShop.MerchantID = userID
	newShop.Name = req.Name

	shopQuery := `INSERT INTO shops (name, merchant_id) VALUES ($1, $2) RETURNING id, created_at`
	err = tx.QueryRow(ctx, shopQuery, req.Name, merchantID).Scan(&newShopID, &newShop.CreatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create shop"})
	}
	newShop.ID = fmt.Sprintf("%d", newShopID)

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to commit transaction"})
	}

	newShop.UpdatedAt = newShop.CreatedAt

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": newShop})
}
