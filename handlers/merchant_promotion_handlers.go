package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v4"
)

// PromotionCreateRequest defines the structure for creating a promotion.
type PromotionCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PromoType   string   `json:"type"`
	PromoValue  float64  `json:"value"`
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	ShopID      string   `json:"shopId"`
	ProductIDs  []string `json:"productIds"` // IDs of products this promotion applies to
}

// HandleCreatePromotion creates a new promotion for the merchant.
func HandleCreatePromotion(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	var req PromotionCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Insert the promotion
	promoQuery := `
        INSERT INTO promotions (merchant_id, shop_id, name, description, promo_type, promo_value, start_date, end_date)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
    `
	var promotionID string
	err = tx.QueryRow(ctx, promoQuery, merchantID, req.ShopID, req.Name, req.Description, req.PromoType, req.PromoValue, req.StartDate, req.EndDate).Scan(&promotionID)
	if err != nil {
		log.Printf("Error creating promotion: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create promotion"})
	}

	// Link products to the promotion
	if len(req.ProductIDs) > 0 {
		stmt, err := tx.Prepare(ctx, "insert_promo_product", "INSERT INTO promotion_products (promotion_id, inventory_item_id) VALUES ($1, $2)")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to prepare product link statement"})
		}
		for _, productID := range req.ProductIDs {
			if _, err := tx.Exec(ctx, stmt.Name, promotionID, productID); err != nil {
				log.Printf("Error linking product %s to promotion: %v", productID, err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to link product to promotion"})
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Promotion created successfully", "data": promotionID})
}

// HandleListPromotions lists all promotions for the authenticated merchant.
func HandleListPromotions(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	query := `
        SELECT id, name, description, promo_type, promo_value, start_date, end_date, shop_id, is_active, created_at, updated_at
        FROM promotions
        WHERE merchant_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := db.Query(ctx, query, merchantID, pageSize, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to retrieve promotions"})
	}
	defer rows.Close()

	promotions := make([]models.Promotion, 0)
	for rows.Next() {
		var p models.Promotion
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PromoType, &p.PromoValue, &p.StartDate, &p.EndDate, &p.ShopID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			log.Printf("Error scanning promotion: %v", err)
			continue
		}
		promotions = append(promotions, p)
	}

	// Get total count for pagination
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1"
	if err := db.QueryRow(ctx, countQuery, merchantID).Scan(&totalItems); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count promotions"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"items":       promotions,
			"totalItems":  totalItems,
			"currentPage": page,
			"totalPages":  (totalItems + pageSize - 1) / pageSize,
		},
	})
}

// HandleUpdatePromotion updates an existing promotion.
func HandleUpdatePromotion(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	promotionID := c.Params("id")
	var req PromotionCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Update the promotion
	promoQuery := `
        UPDATE promotions
        SET name = $1, description = $2, promo_type = $3, promo_value = $4, start_date = $5, end_date = $6, shop_id = $7
        WHERE id = $8 AND merchant_id = $9
    `
	_, err = tx.Exec(ctx, promoQuery, req.Name, req.Description, req.PromoType, req.PromoValue, req.StartDate, req.EndDate, req.ShopID, promotionID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update promotion"})
	}

	// Delete existing product links
	if _, err := tx.Exec(ctx, "DELETE FROM promotion_products WHERE promotion_id = $1", promotionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update product links"})
	}

	// Add new product links
	if len(req.ProductIDs) > 0 {
		stmt, err := tx.Prepare(ctx, "update_promo_product", "INSERT INTO promotion_products (promotion_id, inventory_item_id) VALUES ($1, $2)")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to prepare product link statement"})
		}
		for _, productID := range req.ProductIDs {
			if _, err := tx.Exec(ctx, stmt.Name, promotionID, productID); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to link product to promotion"})
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Promotion updated successfully"})
}

// HandleDeletePromotion deletes a promotion.
func HandleDeletePromotion(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	promotionID := c.Params("id")

	query := "DELETE FROM promotions WHERE id = $1 AND merchant_id = $2"
	commandTag, err := db.Exec(ctx, query, promotionID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete promotion"})
	}
	if commandTag.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Promotion not found or you do not have permission to delete it"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Promotion deleted successfully"})
}
