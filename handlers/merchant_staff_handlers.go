package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"golang.org/x/crypto/bcrypt"
)

// HandleListMerchantStaff handles listing all staff for the current merchant.
func HandleListMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantId := claims.UserID

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'staff' AND merchant_id = $1"
	if err = db.QueryRow(ctx, countQuery, merchantId).Scan(&totalCount); err != nil {
		log.Printf("Error counting merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// Get paginated staff
	query := `
		SELECT u.id, u.name, u.email, u.role, u.is_active, u.merchant_id, u.assigned_shop_id, u.created_at, u.updated_at
		FROM users u
		WHERE u.role = 'staff' AND u.merchant_id = $1
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := db.Query(ctx, query, merchantId, pageSize, offset)
	if err != nil {
		log.Printf("Error listing merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	staffList := make([]models.User, 0)
	for rows.Next() {
		var staff models.User
		if err := rows.Scan(&staff.ID, &staff.Name, &staff.Email, &staff.Role, &staff.IsActive, &staff.MerchantID, &staff.AssignedShopID, &staff.CreatedAt, &staff.UpdatedAt); err != nil {
			log.Printf("Error scanning staff row: %v", err)
			continue
		}
		staffList = append(staffList, staff)
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"users":       staffList,
			"totalCount":  totalCount,
			"currentPage": page,
			"pageSize":    pageSize,
			"totalPages":  totalPages,
		},
	})
}

// HandleCreateMerchantStaff handles creating a new staff member for the current merchant.
func HandleCreateMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantId := claims.UserID

	type CreateStaffRequest struct {
		Name           string  `json:"name"`
		Email          string  `json:"email"`
		Password       string  `json:"password"`
		AssignedShopID *string `json:"assignedShopId,omitempty"`
	}

	var req CreateStaffRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	log.Printf("Parsed request: name=%s, email=%s, assignedShopId=%v", req.Name, req.Email, req.AssignedShopID)
	if req.AssignedShopID != nil {
		log.Printf("AssignedShopID value: '%s'", *req.AssignedShopID)
	}

	// Check for existing email
	var existingId string
	emailCheckQuery := "SELECT id FROM users WHERE email = $1"
	checkErr := db.QueryRow(ctx, emailCheckQuery, req.Email).Scan(&existingId)
	if checkErr != nil && checkErr != pgx.ErrNoRows {
		log.Printf("Error checking for existing email: %v", checkErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	if checkErr == nil {
		// Email exists
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Email is already in use"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to hash password"})
	}

	// Convert empty string to nil for assigned_shop_id
	var assignedShopID interface{}
	if req.AssignedShopID != nil && *req.AssignedShopID != "" {
		assignedShopID = *req.AssignedShopID
	} else {
		assignedShopID = nil
	}

	query := `
		INSERT INTO users (name, email, password_hash, role, merchant_id, assigned_shop_id)
		VALUES ($1, $2, $3, 'staff', $4, $5)
		RETURNING id, name, email, role, is_active, merchant_id, assigned_shop_id, created_at, updated_at
	`

	var newStaff models.User
	err = db.QueryRow(ctx, query, req.Name, req.Email, string(hashedPassword), merchantId, assignedShopID).Scan(
		&newStaff.ID, &newStaff.Name, &newStaff.Email, &newStaff.Role, &newStaff.IsActive, &newStaff.MerchantID, &newStaff.AssignedShopID, &newStaff.CreatedAt, &newStaff.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating merchant staff: %v", err)
		log.Printf("merchantId: %s, assignedShopId: %v", merchantId, assignedShopID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create staff member."})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": newStaff})
}

// HandleUpdateMerchantStaff handles updating a staff member for the current merchant.
func HandleUpdateMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantId := claims.UserID
	staffId := c.Params("staffId")

	type UpdateStaffRequest struct {
		Name           *string `json:"name,omitempty"`
		Password       *string `json:"password,omitempty"`
		IsActive       *bool   `json:"isActive,omitempty"`
		AssignedShopID *string `json:"assignedShopId,omitempty"`
	}

	var req UpdateStaffRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	var setClauses []string
	args := make([]interface{}, 0)
	paramIndex := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", paramIndex))
		args = append(args, *req.Name)
		paramIndex++
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to hash password"})
		}
		setClauses = append(setClauses, fmt.Sprintf("password_hash = $%d", paramIndex))
		args = append(args, hashedPassword)
		paramIndex++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", paramIndex))
		args = append(args, *req.IsActive)
		paramIndex++
	}
	if req.AssignedShopID != nil {
		setClauses = append(setClauses, fmt.Sprintf("assigned_shop_id = $%d", paramIndex))
		args = append(args, *req.AssignedShopID)
		paramIndex++
	}

	if len(setClauses) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Nothing to update"})
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND merchant_id = $%d AND role = 'staff' RETURNING id, name, email, role, is_active, merchant_id, assigned_shop_id, created_at, updated_at", strings.Join(setClauses, ", "), paramIndex, paramIndex+1)
	args = append(args, staffId, merchantId)

	var updatedStaff models.User
	if err = db.QueryRow(ctx, query, args...).Scan(
		&updatedStaff.ID, &updatedStaff.Name, &updatedStaff.Email, &updatedStaff.Role, &updatedStaff.IsActive, &updatedStaff.MerchantID, &updatedStaff.AssignedShopID, &updatedStaff.CreatedAt, &updatedStaff.UpdatedAt,
	); err != nil {
		log.Printf("Error updating merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update staff member."})
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedStaff})
}

// HandleDeleteMerchantStaff handles deleting a staff member for the current merchant.
func HandleDeleteMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantId := claims.UserID
	staffId := c.Params("staffId")

	query := "DELETE FROM users WHERE id = $1 AND merchant_id = $2 AND role = 'staff'"

	res, err := db.Exec(ctx, query, staffId, merchantId)
	if err != nil {
		log.Printf("Error deleting merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Staff member not found or you do not have permission to delete it."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success"})
}
