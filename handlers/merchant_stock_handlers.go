package handlers

import (
	"app/middleware"
	"github.com/gofiber/fiber/v2"
)

// HandleMerchantStockIn receives stock into one merchant-owned shop.
func HandleMerchantStockIn(c *fiber.Ctx) error {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		ShopID string `json:"shopId"`
	}
	_ = c.BodyParser(&body)
	shopID := c.Query("shopId")
	if shopID == "" {
		shopID = body.ShopID
	}
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	return handleBulkStockIn(c, shopID, claims.UserID, "merchant_stock_in")
}
