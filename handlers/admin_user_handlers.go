package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"app/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// HandleListUsers lists all users, with an optional role filter.
func HandleListUsers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	roleFilter := c.Query("role")
	search := strings.TrimSpace(c.Query("search"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	where := " WHERE 1=1"
	var args []interface{}
	if roleFilter != "" {
		where += " AND role = $1"
		args = append(args, roleFilter)
	}
	if search != "" {
		where += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count users"})
	}

	query := fmt.Sprintf("SELECT id, name, email, role, is_active, created_at, updated_at FROM users%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, len(args)+1, len(args)+2)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve users"})
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			log.Printf("Error scanning user row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing user data"})
		}
		users = append(users, user)
	}

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"users": users}, "pagination": fiber.Map{"totalItems": total, "totalPages": int(math.Ceil(float64(total) / float64(pageSize))), "currentPage": page, "pageSize": pageSize}})
}

// HandleGetUserByID fetches a single user by their ID.
func HandleGetUserByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	userID := c.Params("userId")

	var user models.User
	query := "SELECT id, name, email, role, is_active, created_at, updated_at FROM users WHERE id = $1"
	err := db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		log.Printf("Error fetching user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "User not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleCreateUser creates a new user.
func HandleCreateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	actorID := claims.UserID

	// Define a flexible request structure that accepts both camelCase and snake_case
	var rawBody map[string]interface{}
	if err := c.BodyParser(&rawBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	// Helper function to get value from either camelCase or snake_case key
	getValue := func(camelKey, snakeKey string) interface{} {
		if val, ok := rawBody[camelKey]; ok {
			return val
		}
		if val, ok := rawBody[snakeKey]; ok {
			return val
		}
		return nil
	}

	getString := func(camelKey, snakeKey string) string {
		if val := getValue(camelKey, snakeKey); val != nil {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	getBool := func(camelKey, snakeKey string) bool {
		if val := getValue(camelKey, snakeKey); val != nil {
			if b, ok := val.(bool); ok {
				return b
			}
		}
		return false
	}

	getStringPtr := func(camelKey, snakeKey string) *string {
		str := getString(camelKey, snakeKey)
		if str == "" {
			return nil
		}
		return &str
	}

	// Build the request from the raw body
	type createUserRequest struct {
		Name            string
		Email           string
		Password        string
		Role            string
		ShopName        string
		IsActive        bool
		Phone           *string
		BusinessAddress *string
		MerchantID      string
	}

	req := createUserRequest{
		Name:            getString("name", "name"),
		Email:           getString("email", "email"),
		Password:        getString("password", "password"),
		Role:            getString("role", "role"),
		ShopName:        getString("shopName", "shop_name"),
		IsActive:        getBool("isActive", "is_active"),
		Phone:           getStringPtr("phone", "phone"),
		BusinessAddress: getStringPtr("businessAddress", "business_address"),
		MerchantID:      getString("merchantId", "merchant_id"),
	}
	clientOperationID := getString("clientOperationId", "client_operation_id")
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 255 || strings.TrimSpace(req.Email) == "" || len(req.Email) > 255 || len(req.Password) < 8 || len(req.Password) > 128 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Valid name, email, and password are required"})
	}

	log.Printf("Received create user request: role=%s, merchantId=%s", req.Role, req.MerchantID)

	// Validate and normalize role
	normalizedRole, valid := utils.ValidateAndNormalizeRole(req.Role)
	if !valid {
		log.Printf("Invalid role provided: %s", req.Role)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid role. Must be one of: admin, merchant, or staff",
		})
	}
	req.Role = normalizedRole

	// Start a transaction since we might need to create both user and shop
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}
	defer tx.Rollback(ctx) // Will rollback if not committed
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "admin_create_user", actorID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to create user",
		})
	}

	var user models.User
	userQuery := `
		INSERT INTO users (name, email, password_hash, role, is_active, phone) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id, name, email, role, is_active, phone, created_at, updated_at`

	// Convert any nil string pointers to sql.NullString for proper database handling
	var phoneSQL sql.NullString
	if req.Phone != nil {
		phoneSQL.String = *req.Phone
		phoneSQL.Valid = true
	}

	err = tx.QueryRow(ctx, userQuery,
		req.Name,
		req.Email,
		string(hashedPassword),
		req.Role,
		req.IsActive,
		phoneSQL,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Role,
		&user.IsActive,
		&user.Phone,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to create user",
		})
	}

	// If this is a staff user and merchant_id is provided, verify and link the staff to the merchant
	if normalizedRole == "staff" && req.MerchantID != "" {
		// First verify that the merchant exists
		var merchantExists bool
		err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'merchant')",
			req.MerchantID).Scan(&merchantExists)
		if err != nil {
			log.Printf("Error checking merchant existence: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Database error",
			})
		}
		if !merchantExists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "Invalid merchant_id: Merchant not found",
			})
		}

		// Update the staff's merchant_id
		_, err = tx.Exec(ctx,
			"UPDATE users SET merchant_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
			req.MerchantID, user.ID)
		if err != nil {
			log.Printf("Error linking staff to merchant: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to link staff to merchant",
			})
		}

		// Also update our local user object to include the merchant_id
		user.MerchantID = &req.MerchantID
	} // If this is a merchant and a shop name is provided, create the shop
	var shop *models.Shop
	if normalizedRole == "merchant" && req.ShopName != "" {
		shop = &models.Shop{}
		shopQuery := `
			INSERT INTO shops (name, merchant_id, address, is_active, is_primary)
			VALUES ($1, $2, $3, true, true)
			RETURNING id, name, merchant_id, address, is_active, is_primary, created_at, updated_at`

		// Convert any nil string pointers to sql.NullString for proper database handling
		var addressSQL sql.NullString
		if req.BusinessAddress != nil {
			addressSQL.String = *req.BusinessAddress
			addressSQL.Valid = true
		}

		err = tx.QueryRow(ctx, shopQuery,
			req.ShopName,
			user.ID,
			addressSQL,
		).Scan(
			&shop.ID,
			&shop.Name,
			&shop.MerchantID,
			&shop.Address,
			&shop.IsActive,
			&shop.IsPrimary,
			&shop.CreatedAt,
			&shop.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error creating shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to create shop",
			})
		}
	}

	// If this is a staff user, fetch the merchant information
	var merchantInfo *models.User
	if normalizedRole == "staff" && req.MerchantID != "" {
		merchantInfo = &models.User{}
		err = tx.QueryRow(ctx, `
			SELECT id, name, email, role, is_active, created_at, updated_at 
			FROM users WHERE id = $1`,
			req.MerchantID,
		).Scan(
			&merchantInfo.ID,
			&merchantInfo.Name,
			&merchantInfo.Email,
			&merchantInfo.Role,
			&merchantInfo.IsActive,
			&merchantInfo.CreatedAt,
			&merchantInfo.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error fetching merchant info: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to fetch merchant information",
			})
		}
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
		})
	}

	response := fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"user": user,
		},
	}
	if shop != nil {
		response["data"].(fiber.Map)["shop"] = shop
	}
	if merchantInfo != nil {
		response["data"].(fiber.Map)["merchant"] = fiber.Map{
			"id":   merchantInfo.ID,
			"name": merchantInfo.Name,
		}
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// HandleUpdateUser updates an existing user.
func HandleUpdateUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	actorID := claims.UserID

	userID := c.Params("userId")

	// First, get the current user data
	var currentUser models.User
	err = db.QueryRow(ctx, `
		SELECT id, name, email, role, is_active, created_at, updated_at 
		FROM users WHERE id = $1`,
		userID,
	).Scan(
		&currentUser.ID, &currentUser.Name, &currentUser.Email,
		&currentUser.Role, &currentUser.IsActive,
		&currentUser.CreatedAt, &currentUser.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error fetching user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "error",
			"message": "User not found",
		})
	}

	// Parse the update request
	var rawReq map[string]interface{}
	if err := c.BodyParser(&rawReq); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}
	clientOperationID := ""
	if v, ok := rawReq["clientOperationId"].(string); ok {
		clientOperationID = v
	}
	if clientOperationID == "" {
		if v, ok := rawReq["client_operation_id"].(string); ok {
			clientOperationID = v
		}
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}

	// Build structured request, handling both snake_case and camelCase
	var req struct {
		Name     *string
		Email    *string
		Role     *string
		IsActive *bool
	}

	if name, ok := rawReq["name"].(string); ok {
		req.Name = &name
	}
	if email, ok := rawReq["email"].(string); ok {
		req.Email = &email
	}
	if role, ok := rawReq["role"].(string); ok {
		req.Role = &role
	}
	// Handle both is_active and isActive
	if isActiveVal, ok := rawReq["is_active"].(bool); ok {
		req.IsActive = &isActiveVal
	} else if isActiveVal2, ok2 := rawReq["isActive"].(bool); ok2 {
		req.IsActive = &isActiveVal2
	}

	log.Printf("Received update user request for ID %s", userID)

	// Build the update query dynamically based on which fields are provided
	setValues := make([]string, 0)
	args := make([]interface{}, 0)
	argPosition := 1

	if req.Name != nil {
		setValues = append(setValues, fmt.Sprintf("name = $%d", argPosition))
		args = append(args, *req.Name)
		argPosition++
	}

	if req.Email != nil {
		setValues = append(setValues, fmt.Sprintf("email = $%d", argPosition))
		args = append(args, *req.Email)
		argPosition++
	}

	if req.Role != nil {
		normalizedRole, valid := utils.ValidateAndNormalizeRole(*req.Role)
		if !valid {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "Invalid role. Must be one of: admin, merchant, or staff",
			})
		}
		setValues = append(setValues, fmt.Sprintf("role = $%d", argPosition))
		args = append(args, normalizedRole)
		argPosition++
	}

	if req.IsActive != nil {
		setValues = append(setValues, fmt.Sprintf("is_active = $%d", argPosition))
		args = append(args, *req.IsActive)
		argPosition++
	}

	// If no fields to update were provided
	if len(setValues) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "No fields to update were provided",
		})
	}

	// Add updated_at
	setValues = append(setValues, "updated_at = NOW()")

	// Build and execute the query
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "admin_update_user", actorID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}
	query := fmt.Sprintf(`
		UPDATE users 
		SET %s 
		WHERE id = $%d 
		RETURNING id, name, email, role, is_active, created_at, updated_at`,
		strings.Join(setValues, ", "),
		argPosition,
	)
	args = append(args, userID)

	var user models.User
	err = tx.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.Name, &user.Email,
		&user.Role, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		log.Printf("Error updating user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to update user",
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit operation"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": user})
}

// HandleHardDeleteUser permanently deletes a user.
func HandleHardDeleteUser(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	actorID := claims.UserID

	userID := c.Params("userId")

	clientOperationID := c.Get("X-Client-Operation-Id")
	if clientOperationID == "" {
		clientOperationID = c.Query("clientOperationId")
	}
	if clientOperationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientOperationId is required"})
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientOperationID, "admin_hard_delete_user", actorID, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		return c.SendStatus(fiber.StatusOK)
	}
	query := "DELETE FROM users WHERE id = $1"
	_, err = tx.Exec(ctx, query, userID)
	if err != nil {
		log.Printf("Error deleting user with ID %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete user"})
	}
	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit operation"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// HandleGetMerchantsForSelection fetches a list of merchants for selection.
func HandleGetMerchantsForSelection(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 50)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	search := strings.TrimSpace(c.Query("search"))
	where := " WHERE role='merchant' AND is_active=TRUE"
	args := []interface{}{}
	if search != "" {
		where += " AND (name ILIKE $1 OR email ILIKE $1)"
		args = append(args, "%"+search+"%")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count merchants"})
	}
	query := fmt.Sprintf("SELECT id, name FROM users%s ORDER BY name LIMIT $%d OFFSET $%d", where, len(args)+1, len(args)+2)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying merchants for selection: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve merchants"})
	}
	defer rows.Close()

	var merchants []models.UserSelectionItem
	for rows.Next() {
		var merchant models.UserSelectionItem
		if err := rows.Scan(&merchant.ID, &merchant.Name); err != nil {
			log.Printf("Error scanning merchant for selection row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing merchant data"})
		}
		merchants = append(merchants, merchant)
	}

	return c.JSON(fiber.Map{"status": "success", "data": merchants, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize}})
}
