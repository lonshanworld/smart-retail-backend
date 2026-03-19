package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

type catalogCreateRequest struct {
	MerchantID        string  `json:"merchantId"`
	CategoryID        string  `json:"categoryId"`
	ClientOperationID string  `json:"clientOperationId"`
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	ImageUrl          *string `json:"imageUrl"`
}

type categoryWithChildren struct {
	models.Category
	Subcategories []models.Subcategory `json:"subcategories"`
}

func getMerchantIDFromClaims(c *fiber.Ctx) (string, error) {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func getRequiredMerchantIDForAdmin(c *fiber.Ctx, reqMerchantID string) (string, error) {
	if reqMerchantID != "" {
		return reqMerchantID, nil
	}
	merchantID := c.Query("merchantId")
	if merchantID == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "merchantId is required")
	}
	return merchantID, nil
}

func getCatalogClientOperationID(c *fiber.Ctx, reqClientOperationID string) string {
	if reqClientOperationID != "" {
		return reqClientOperationID
	}
	if headerValue := c.Get("X-Client-Operation-Id"); headerValue != "" {
		return headerValue
	}
	return c.Query("clientOperationId")
}

func HandleGetMerchantCatalogOptions(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}

	categories := make([]categoryWithChildren, 0)
	categoryByID := map[string]*categoryWithChildren{}

	categoryQuery := `
		SELECT c.id, c.merchant_id, c.name, c.description, c.created_at, c.updated_at,
			sc.id, sc.name, sc.description, sc.created_at, sc.updated_at
		FROM categories c
		LEFT JOIN subcategories sc ON c.id = sc.category_id
		WHERE c.merchant_id = $1
		ORDER BY c.name, sc.name
	`

	rows, err := db.Query(ctx, categoryQuery, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load categories"})
	}
	defer rows.Close()

	for rows.Next() {
		var cat models.Category
		var catDesc sql.NullString
		var subID, subName, subDesc sql.NullString
		var subCreatedAt, subUpdatedAt sql.NullTime

		if err := rows.Scan(&cat.ID, &cat.MerchantID, &cat.Name, &catDesc, &cat.CreatedAt, &cat.UpdatedAt, &subID, &subName, &subDesc, &subCreatedAt, &subUpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan categories"})
		}
		if catDesc.Valid {
			cat.Description = &catDesc.String
		}

		target, ok := categoryByID[cat.ID]
		if !ok {
			categories = append(categories, categoryWithChildren{Category: cat, Subcategories: []models.Subcategory{}})
			target = &categories[len(categories)-1]
			categoryByID[cat.ID] = target
		}

		if subID.Valid {
			sub := models.Subcategory{
				ID:         subID.String,
				CategoryID: cat.ID,
				Name:       subName.String,
			}
			if subDesc.Valid {
				sub.Description = &subDesc.String
			}
			if subCreatedAt.Valid {
				sub.CreatedAt = subCreatedAt.Time
			}
			if subUpdatedAt.Valid {
				sub.UpdatedAt = subUpdatedAt.Time
			}
			target.Subcategories = append(target.Subcategories, sub)
		}
	}

	// Try to select image_url if the column exists; otherwise fall back to a query without it.
	brands := make([]models.Brand, 0)
	queryWithImg := `SELECT id, merchant_id, name, description, image_url, created_at, updated_at FROM brands WHERE merchant_id = $1 ORDER BY name`
	brandRows, err := db.Query(ctx, queryWithImg, merchantID)
	if err != nil {
		// if the DB doesn't have image_url column, retry without it
		if strings.Contains(strings.ToLower(err.Error()), "image_url") {
			brandRows, err = db.Query(ctx, `SELECT id, merchant_id, name, description, created_at, updated_at FROM brands WHERE merchant_id = $1 ORDER BY name`, merchantID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load brands"})
			}
			defer brandRows.Close()
			for brandRows.Next() {
				var b models.Brand
				var bDesc sql.NullString
				if err := brandRows.Scan(&b.ID, &b.MerchantID, &b.Name, &bDesc, &b.CreatedAt, &b.UpdatedAt); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan brands"})
				}
				if bDesc.Valid {
					b.Description = &bDesc.String
				}
				brands = append(brands, b)
			}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to load brands"})
		}
	} else {
		defer brandRows.Close()
		for brandRows.Next() {
			var b models.Brand
			var bDesc sql.NullString
			var bImg sql.NullString
			if err := brandRows.Scan(&b.ID, &b.MerchantID, &b.Name, &bDesc, &bImg, &b.CreatedAt, &b.UpdatedAt); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan brands"})
			}
			if bDesc.Valid {
				b.Description = &bDesc.String
			}
			if bImg.Valid {
				b.ImageUrl = &bImg.String
			}
			brands = append(brands, b)
		}
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"success": true,
		"data": fiber.Map{
			"categories": categories,
			"brands":     brands,
		},
	})
}

func HandleListMerchantCategories(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return listCategoriesByMerchant(c, merchantID)
}

func HandleCreateMerchantCategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return createCategory(c, merchantID)
}

func HandleUpdateMerchantCategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return updateCategory(c, c.Params("categoryId"), merchantID)
}

func HandleDeleteMerchantCategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return deleteCategory(c, c.Params("categoryId"), merchantID)
}

func HandleListMerchantSubcategories(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return listSubcategories(c, merchantID, c.Query("categoryId"))
}

func HandleCreateMerchantSubcategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return createSubcategory(c, merchantID)
}

func HandleUpdateMerchantSubcategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return updateSubcategory(c, c.Params("subcategoryId"), merchantID)
}

func HandleDeleteMerchantSubcategory(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return deleteSubcategory(c, c.Params("subcategoryId"), merchantID)
}

func HandleListMerchantBrands(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return listBrands(c, merchantID)
}

func HandleCreateMerchantBrand(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return createBrand(c, merchantID)
}

func HandleUpdateMerchantBrand(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return updateBrand(c, c.Params("brandId"), merchantID)
}

func HandleDeleteMerchantBrand(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	return deleteBrand(c, c.Params("brandId"), merchantID)
}

func HandleListAdminCategories(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return listCategoriesByMerchant(c, merchantID)
}

func HandleCreateAdminCategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return createCategory(c, merchantID)
}

func HandleUpdateAdminCategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return updateCategory(c, c.Params("categoryId"), merchantID)
}

func HandleDeleteAdminCategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return deleteCategory(c, c.Params("categoryId"), merchantID)
}

func HandleListAdminSubcategories(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return listSubcategories(c, merchantID, c.Query("categoryId"))
}

func HandleCreateAdminSubcategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return createSubcategory(c, merchantID)
}

func HandleUpdateAdminSubcategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return updateSubcategory(c, c.Params("subcategoryId"), merchantID)
}

func HandleDeleteAdminSubcategory(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return deleteSubcategory(c, c.Params("subcategoryId"), merchantID)
}

func HandleListAdminBrands(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return listBrands(c, merchantID)
}

func HandleCreateAdminBrand(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return createBrand(c, merchantID)
}

func HandleUpdateAdminBrand(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return updateBrand(c, c.Params("brandId"), merchantID)
}

func HandleDeleteAdminBrand(c *fiber.Ctx) error {
	merchantID, err := getRequiredMerchantIDForAdmin(c, "")
	if err != nil {
		return err
	}
	return deleteBrand(c, c.Params("brandId"), merchantID)
}

func listCategoriesByMerchant(c *fiber.Ctx, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	rows, err := db.Query(ctx, `SELECT id, merchant_id, name, description, created_at, updated_at FROM categories WHERE merchant_id = $1 ORDER BY name`, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to list categories"})
	}
	defer rows.Close()

	categories := make([]models.Category, 0)
	for rows.Next() {
		var cat models.Category
		var desc sql.NullString
		if err := rows.Scan(&cat.ID, &cat.MerchantID, &cat.Name, &desc, &cat.CreatedAt, &cat.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan categories"})
		}
		if desc.Valid {
			cat.Description = &desc.String
		}
		categories = append(categories, cat)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": categories})
}

func createCategory(c *fiber.Ctx, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "name is required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting category create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create category"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_category_create", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming category create operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create category"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "success": true, "message": "Category already processed"})
	}

	var cat models.Category
	var desc sql.NullString
	err = tx.QueryRow(ctx, `INSERT INTO categories (merchant_id, name, description) VALUES ($1, $2, $3) RETURNING id, merchant_id, name, description, created_at, updated_at`, merchantID, req.Name, req.Description).Scan(&cat.ID, &cat.MerchantID, &cat.Name, &desc, &cat.CreatedAt, &cat.UpdatedAt)
	if err != nil {
		log.Printf("Error creating category: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create category"})
	}
	if desc.Valid {
		cat.Description = &desc.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing category create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create category"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": cat})
}

func updateCategory(c *fiber.Ctx, categoryID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "name is required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting category update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update category"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_category_update", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming category update operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update category"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "success": true, "message": "Category already processed"})
	}
	var cat models.Category
	var desc sql.NullString
	err = tx.QueryRow(ctx, `UPDATE categories SET name = $1, description = $2, updated_at = NOW() WHERE id = $3 AND merchant_id = $4 RETURNING id, merchant_id, name, description, created_at, updated_at`, req.Name, req.Description, categoryID, merchantID).Scan(&cat.ID, &cat.MerchantID, &cat.Name, &desc, &cat.CreatedAt, &cat.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Category not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update category"})
	}
	if desc.Valid {
		cat.Description = &desc.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing category update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update category"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": cat})
}

func deleteCategory(c *fiber.Ctx, categoryID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	clientOperationID := getCatalogClientOperationID(c, "")
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting category delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete category"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_category_delete", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming category delete operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete category"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusNoContent)
	}

	res, err := tx.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND merchant_id = $2`, categoryID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete category"})
	}
	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Category not found"})
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing category delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete category"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func listSubcategories(c *fiber.Ctx, merchantID, categoryID string) error {
	db := database.GetDB()
	ctx := context.Background()
	query := `SELECT sc.id, sc.category_id, sc.name, sc.description, sc.created_at, sc.updated_at FROM subcategories sc JOIN categories c ON sc.category_id = c.id WHERE c.merchant_id = $1`
	args := []interface{}{merchantID}
	if categoryID != "" {
		query += " AND sc.category_id = $2"
		args = append(args, categoryID)
	}
	query += " ORDER BY sc.name"
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to list subcategories"})
	}
	defer rows.Close()
	subs := make([]models.Subcategory, 0)
	for rows.Next() {
		var s models.Subcategory
		var desc sql.NullString
		if err := rows.Scan(&s.ID, &s.CategoryID, &s.Name, &desc, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan subcategories"})
		}
		if desc.Valid {
			s.Description = &desc.String
		}
		subs = append(subs, s)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": subs})
}

func createSubcategory(c *fiber.Ctx, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}
	if req.CategoryID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "categoryId and name are required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting subcategory create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create subcategory"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_subcategory_create", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming subcategory create operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create subcategory"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Subcategory already processed"})
	}

	var ownerCheck int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE id = $1 AND merchant_id = $2`, req.CategoryID, merchantID).Scan(&ownerCheck); err != nil || ownerCheck == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid categoryId for merchant"})
	}
	var sub models.Subcategory
	var desc sql.NullString
	err = tx.QueryRow(ctx, `INSERT INTO subcategories (category_id, name, description) VALUES ($1, $2, $3) RETURNING id, category_id, name, description, created_at, updated_at`, req.CategoryID, req.Name, req.Description).Scan(&sub.ID, &sub.CategoryID, &sub.Name, &desc, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create subcategory"})
	}
	if desc.Valid {
		sub.Description = &desc.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing subcategory create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create subcategory"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": sub})
}

func updateSubcategory(c *fiber.Ctx, subcategoryID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	if req.CategoryID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "categoryId and name are required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting subcategory update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update subcategory"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_subcategory_update", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming subcategory update operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update subcategory"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Subcategory already processed"})
	}

	var sub models.Subcategory
	var desc sql.NullString
	err = tx.QueryRow(ctx, `
		UPDATE subcategories sc
		SET category_id = $1, name = $2, description = $3, updated_at = NOW()
		FROM categories c
		WHERE sc.id = $4 AND c.id = sc.category_id AND c.merchant_id = $5
		RETURNING sc.id, sc.category_id, sc.name, sc.description, sc.created_at, sc.updated_at
	`, req.CategoryID, req.Name, req.Description, subcategoryID, merchantID).Scan(&sub.ID, &sub.CategoryID, &sub.Name, &desc, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Subcategory not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update subcategory"})
	}
	if desc.Valid {
		sub.Description = &desc.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing subcategory update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update subcategory"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": sub})
}

func deleteSubcategory(c *fiber.Ctx, subcategoryID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	clientOperationID := getCatalogClientOperationID(c, "")
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting subcategory delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete subcategory"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_subcategory_delete", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming subcategory delete operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete subcategory"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusNoContent)
	}

	res, err := tx.Exec(ctx, `DELETE FROM subcategories sc USING categories c WHERE sc.id = $1 AND c.id = sc.category_id AND c.merchant_id = $2`, subcategoryID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete subcategory"})
	}
	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Subcategory not found"})
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing subcategory delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete subcategory"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func listBrands(c *fiber.Ctx, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	// Try selecting image_url first; fall back if column missing in older schemas
	brands := make([]models.Brand, 0)
	rows, err := db.Query(ctx, `SELECT id, merchant_id, name, description, image_url, created_at, updated_at FROM brands WHERE merchant_id = $1 ORDER BY name`, merchantID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "image_url") {
			rows, err = db.Query(ctx, `SELECT id, merchant_id, name, description, created_at, updated_at FROM brands WHERE merchant_id = $1 ORDER BY name`, merchantID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to list brands"})
			}
			defer rows.Close()
			for rows.Next() {
				var b models.Brand
				var desc sql.NullString
				if err := rows.Scan(&b.ID, &b.MerchantID, &b.Name, &desc, &b.CreatedAt, &b.UpdatedAt); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan brands"})
				}
				if desc.Valid {
					b.Description = &desc.String
				}
				brands = append(brands, b)
			}
			return c.JSON(fiber.Map{"status": "success", "success": true, "data": brands})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to list brands"})
	}
	defer rows.Close()
	for rows.Next() {
		var b models.Brand
		var desc sql.NullString
		var img sql.NullString
		if err := rows.Scan(&b.ID, &b.MerchantID, &b.Name, &desc, &img, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to scan brands"})
		}
		if desc.Valid {
			b.Description = &desc.String
		}
		if img.Valid {
			b.ImageUrl = &img.String
		}
		brands = append(brands, b)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": brands})
}

func createBrand(c *fiber.Ctx, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "name is required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting brand create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create brand"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_brand_create", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming brand create operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create brand"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "success": true, "message": "Brand already processed"})
	}
	// Pre-check for duplicate brand name for this merchant to return a clean 409
	var exists int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM brands WHERE merchant_id = $1 AND lower(name) = lower($2) LIMIT 1`, merchantID, req.Name).Scan(&exists); err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "success": false, "message": "Brand already exists"})
	} else if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking existing brand: %v", err)
		// fallthrough to attempt insert and let insert handle unexpected errors
	}
	var b models.Brand
	var desc sql.NullString
	var img sql.NullString
	err = tx.QueryRow(ctx, `INSERT INTO brands (merchant_id, name, description, image_url) VALUES ($1, $2, $3, $4) RETURNING id, merchant_id, name, description, image_url, created_at, updated_at`, merchantID, req.Name, req.Description, req.ImageUrl).Scan(&b.ID, &b.MerchantID, &b.Name, &desc, &img, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		// If a race caused a unique constraint violation, still map it to 409
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				log.Printf("Duplicate brand for merchant %s: %v", merchantID, pgErr.Message)
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "success": false, "message": "Brand already exists"})
			}
		}
		log.Printf("Error creating brand: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create brand"})
	}
	if desc.Valid {
		b.Description = &desc.String
	}
	if img.Valid {
		b.ImageUrl = &img.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing brand create transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to create brand"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": b})
}

func updateBrand(c *fiber.Ctx, brandID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	var req catalogCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "name is required"})
	}
	clientOperationID := getCatalogClientOperationID(c, req.ClientOperationID)
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting brand update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update brand"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_brand_update", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming brand update operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update brand"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "success": true, "message": "Brand already processed"})
	}
	var b models.Brand
	var desc sql.NullString
	var img sql.NullString
	err = tx.QueryRow(ctx, `UPDATE brands SET name = $1, description = $2, image_url = $3, updated_at = NOW() WHERE id = $4 AND merchant_id = $5 RETURNING id, merchant_id, name, description, image_url, created_at, updated_at`, req.Name, req.Description, req.ImageUrl, brandID, merchantID).Scan(&b.ID, &b.MerchantID, &b.Name, &desc, &img, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Brand not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update brand"})
	}
	if desc.Valid {
		b.Description = &desc.String
	}
	if img.Valid {
		b.ImageUrl = &img.String
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing brand update transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to update brand"})
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": b})
}

func deleteBrand(c *fiber.Ctx, brandID, merchantID string) error {
	db := database.GetDB()
	ctx := context.Background()
	clientOperationID := getCatalogClientOperationID(c, "")
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "success": false, "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting brand delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete brand"})
	}
	defer tx.Rollback(ctx)

	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "catalog_brand_delete", merchantID, nil)
	if err != nil {
		log.Printf("Error claiming brand delete operation %s: %v", clientOperationID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete brand"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusNoContent)
	}

	res, err := tx.Exec(ctx, `DELETE FROM brands WHERE id = $1 AND merchant_id = $2`, brandID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete brand"})
	}
	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "success": false, "message": "Brand not found"})
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing brand delete transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "success": false, "message": "Failed to delete brand"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
