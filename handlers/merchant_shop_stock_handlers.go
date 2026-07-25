package handlers

import (
	"app/middleware"
	"github.com/gofiber/fiber/v2"
)

// HandleShopStockIn receives stock into a shop selected by the merchant.
func HandleShopStockIn(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	shopID := c.Params("shopId")
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	return handleBulkStockIn(c, shopID, claims.UserID, "merchant_shop_stock_in")
}
