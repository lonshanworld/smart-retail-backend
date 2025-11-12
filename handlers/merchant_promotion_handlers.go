package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// PromotionCreateRequest defines the structure for creating a promotion.
type PromotionCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PromoType   string   `json:"type"`
	PromoValue  float64  `json:"value"`
	MinSpend    float64  `json:"minSpend"`
	StartDate   *string  `json:"startDate"` // Pointer to allow null
	EndDate     *string  `json:"endDate"`   // Pointer to allow null
	ShopID      string   `json:"shopId"`
	ProductIDs  []string `json:"productIds"` // IDs of products this promotion applies to
}

// HandleCreatePromotion creates a new promotion for the merchant.
func HandleCreatePromotion(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	var req PromotionCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// First check if shop belongs to merchant
	checkQuery := `SELECT COUNT(*) FROM shops WHERE id = $1 AND merchant_id = $2`
	var count int
	err = db.QueryRow(ctx, checkQuery, req.ShopID, merchantID).Scan(&count)
	if err != nil {
		log.Printf("Error checking shop ownership: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to verify shop ownership"})
	}
	if count == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Shop does not belong to this merchant"})
	}

	// Insert the promotion
	promoQuery := `
        INSERT INTO promotions (merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, start_date, end_date)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
    `
	var promotionID string
	err = db.QueryRow(ctx, promoQuery, merchantID, req.ShopID, req.Name, req.Description, req.PromoType, req.PromoValue, req.MinSpend, req.StartDate, req.EndDate).Scan(&promotionID)
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

	// Fetch the created promotion to return complete object
	var promotion models.Promotion
	fetchQuery := `
		SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend,
		       start_date, end_date, is_active, created_at, updated_at
		FROM promotions
		WHERE id = $1
	`
	err = db.QueryRow(ctx, fetchQuery, promotionID).Scan(
		&promotion.ID, &promotion.MerchantID, &promotion.ShopID, &promotion.Name,
		&promotion.Description, &promotion.PromoType, &promotion.PromoValue, &promotion.MinSpend,
		&promotion.StartDate, &promotion.EndDate, &promotion.IsActive,
		&promotion.CreatedAt, &promotion.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error fetching created promotion: %v", err)
		// Still return success since promotion was created
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Promotion created successfully", "data": fiber.Map{"id": promotionID}})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "message": "Promotion created successfully", "data": promotion})
}

// HandleListPromotions lists all promotions for the authenticated merchant.
func HandleListPromotions(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

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

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	promotionID := c.Params("id")

	// Parse the raw body to check if it's just an isActive toggle
	var rawBody map[string]interface{}
	if err := c.BodyParser(&rawBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	// If only isActive is provided, do a simple toggle
	if len(rawBody) == 1 {
		if isActive, ok := rawBody["isActive"].(bool); ok {
			updateQuery := `UPDATE promotions SET is_active = $1 WHERE id = $2 AND merchant_id = $3`
			_, err = db.Exec(ctx, updateQuery, isActive, promotionID, merchantID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update promotion status"})
			}

			// Fetch and return the updated promotion
			var promotion models.Promotion
			fetchQuery := `
				SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend,
				       start_date, end_date, is_active, created_at, updated_at
				FROM promotions
				WHERE id = $1
			`
			err = db.QueryRow(ctx, fetchQuery, promotionID).Scan(
				&promotion.ID, &promotion.MerchantID, &promotion.ShopID, &promotion.Name,
				&promotion.Description, &promotion.PromoType, &promotion.PromoValue, &promotion.MinSpend,
				&promotion.StartDate, &promotion.EndDate, &promotion.IsActive,
				&promotion.CreatedAt, &promotion.UpdatedAt,
			)
			if err != nil {
				return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Promotion status updated successfully"})
			}
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": promotion})
		}
	}

	// Otherwise, parse as full update request
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
        SET name = $1, description = $2, promo_type = $3, promo_value = $4, min_spend = $5, start_date = $6, end_date = $7, shop_id = $8
        WHERE id = $9 AND merchant_id = $10
    `
	_, err = tx.Exec(ctx, promoQuery, req.Name, req.Description, req.PromoType, req.PromoValue, req.MinSpend, req.StartDate, req.EndDate, req.ShopID, promotionID, merchantID)
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

	// Fetch the updated promotion to return complete object
	var promotion models.Promotion
	fetchQuery := `
		SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend,
		       start_date, end_date, is_active, created_at, updated_at
		FROM promotions
		WHERE id = $1
	`
	err = db.QueryRow(ctx, fetchQuery, promotionID).Scan(
		&promotion.ID, &promotion.MerchantID, &promotion.ShopID, &promotion.Name,
		&promotion.Description, &promotion.PromoType, &promotion.PromoValue, &promotion.MinSpend,
		&promotion.StartDate, &promotion.EndDate, &promotion.IsActive,
		&promotion.CreatedAt, &promotion.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error fetching updated promotion: %v", err)
		// Still return success since promotion was updated
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Promotion updated successfully"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Promotion updated successfully", "data": promotion})
}

// HandleDeletePromotion deletes a promotion.
func HandleDeletePromotion(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

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
