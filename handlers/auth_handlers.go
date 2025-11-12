package handlers

import (
	"database/sql"
	"log"
	"time"

	"app/config"
	"app/database"
	"app/models"
	"app/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// HandleLogin authenticates a user and returns a JWT token.
func HandleLogin(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	// Validate and normalize role
	normalizedRole, valid := utils.ValidateAndNormalizeRole(req.UserType)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid user type. Must be one of: admin, merchant, or staff",
		})
	}
	req.UserType = normalizedRole

	var user models.User
	var passwordHash string
	var phone, assignedShopID, merchantID sql.NullString

	query := `
		SELECT id, name, email, password_hash, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
		FROM users
		WHERE email = $1 AND role = $2`

	err := database.GetDB().QueryRow(c.Context(), query, req.Email, req.UserType).Scan(
		&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials or user role"})
		}
		log.Printf("Database error during login for email %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "User account is inactive"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials"})
	}

	// If merchant and shopId is provided, verify ownership
	if user.Role == "merchant" && req.ShopID != nil && *req.ShopID != "" {
		var shopMerchantID string
		shopQuery := "SELECT merchant_id FROM shops WHERE id = $1"
		err := database.GetDB().QueryRow(c.Context(), shopQuery, *req.ShopID).Scan(&shopMerchantID)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
			}
			log.Printf("Error verifying shop ownership: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
		}
		if shopMerchantID != user.ID {
			log.Printf("Access denied: Merchant %s attempted to access shop %s owned by %s", user.ID, *req.ShopID, shopMerchantID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied to this shop"})
		}
		log.Printf("✅ Merchant %s verified as owner of shop %s", user.ID, *req.ShopID)
	}

	token, err := createJWT(user.ID, user.Role)
	if err != nil {
		log.Printf("Error creating JWT for user %s: %v", user.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not sign token"})
	}

	if phone.Valid {
		user.Phone = &phone.String
	}
	if assignedShopID.Valid {
		user.AssignedShopID = &assignedShopID.String
	}
	if merchantID.Valid {
		user.MerchantID = &merchantID.String
	}

	return c.JSON(fiber.Map{"accessToken": token, "user": user})
}

// HandleShopLogin authenticates a staff member for a specific shop.
func HandleShopLogin(c *fiber.Ctx) error {
	var req models.ShopLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	var user models.User
	var passwordHash string
	var phone, merchantID, assignedShopID sql.NullString

	query := `
		SELECT id, name, email, password_hash, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
		FROM users
		WHERE email = $1 AND assigned_shop_id = $2 AND role = 'staff'`

	err := database.GetDB().QueryRow(c.Context(), query, req.Email, req.ShopID).Scan(
		&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials or not a staff for this shop"})
		}
		log.Printf("Database error during shop login for email %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Staff account is inactive"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials"})
	}

	token, err := createJWT(user.ID, user.Role)
	if err != nil {
		log.Printf("Error creating JWT for staff %s: %v", user.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not sign token"})
	}

	if phone.Valid {
		user.Phone = &phone.String
	}
	if assignedShopID.Valid {
		user.AssignedShopID = &assignedShopID.String
	}
	if merchantID.Valid {
		user.MerchantID = &merchantID.String
	}

	shop, err := fetchShop(c, req.ShopID)
	if err != nil {
		log.Printf("Error fetching shop details for shop %s: %v", req.ShopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch shop details"})
	}

	return c.JSON(fiber.Map{"accessToken": token, "user": user, "shop": shop})
}

// --- Helper Functions ---

func createJWT(userID, role string) (string, error) {
	claims := models.JwtClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

func fetchShop(c *fiber.Ctx, shopID string) (*models.Shop, error) {
	var shop models.Shop
	var address, phone sql.NullString
	query := `SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE id = $1`
	err := database.GetDB().QueryRow(c.Context(), query, shopID).Scan(
		&shop.ID, &shop.Name, &shop.MerchantID, &address, &phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if address.Valid {
		shop.Address = &address.String
	}
	if phone.Valid {
		shop.Phone = &phone.String
	}
	return &shop, nil
}
