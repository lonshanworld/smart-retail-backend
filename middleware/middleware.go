package middleware

import (
	"log"
	"strings"
	"time"

	"app/config"
	"app/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// CreateToken generates a new JWT token for a user
func CreateToken(userID string, role string) (string, error) {
	claims := models.JwtClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Token expires in 24 hours
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
	if err != nil {
		log.Printf("Error creating token: %v", err)
		return "", err
	}

	return tokenString, nil
}

// JWTMiddleware validates the JWT token provided in the Authorization header
func JWTMiddleware(c *fiber.Ctx) error {
	// Skip JWT check for login and init endpoints
	path := c.Path()
	if path == "/api/auth/login" || path == "/api/init" {
		return c.Next()
	}

	// Log headers for debugging
	log.Printf("Request Headers: %v", c.GetReqHeaders())

	authHeader := c.Get("Authorization")
	log.Printf("Authorization Header: %s", authHeader)

	if authHeader == "" {
		log.Println("No Authorization header found")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Missing authorization token",
		})
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		log.Printf("Invalid auth header format: %s", authHeader)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid authorization format",
		})
	}

	claims := &models.JwtClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("Unexpected signing method: %v", token.Header["alg"])
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil {
		log.Printf("Token parsing error: %v", err)
		if err == jwt.ErrTokenExpired {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "error",
				"message": "Token has expired",
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid token",
		})
	}

	if !token.Valid {
		log.Printf("Token is invalid")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid token",
		})
	}

	log.Printf("Token is valid. UserID: %s, Role: %s", claims.UserID, claims.Role)
	c.Locals("user", token)
	c.Locals("userID", claims.UserID)
	c.Locals("userRole", claims.Role)

	return c.Next()
}

// AdminRequired checks if the authenticated user has admin role
func AdminRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	log.Printf("Checking admin role. Got role: %s, ok: %v", role, ok)

	if !ok || role != "admin" {
		log.Printf("Admin access denied for role: %s", role)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Admin access required",
		})
	}
	return c.Next()
}

// MerchantRequired checks if the authenticated user has merchant role
func MerchantRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	log.Printf("Checking merchant role. Got role: %s, ok: %v", role, ok)

	if !ok || role != "merchant" {
		log.Printf("Merchant access denied for role: %s", role)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Merchant access required",
		})
	}
	return c.Next()
}

// StaffRequired checks if the authenticated user has staff role
func StaffRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	log.Printf("Checking staff role. Got role: %s, ok: %v", role, ok)

	if !ok || role != "staff" {
		log.Printf("Staff access denied for role: %s", role)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Staff access required",
		})
	}
	return c.Next()
}

// ExtractClaims extracts the JWT claims from the context
func ExtractClaims(c *fiber.Ctx) (*models.JwtClaims, error) {
	user := c.Locals("user").(*jwt.Token)
	claims, ok := user.Claims.(*models.JwtClaims)
	if !ok {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Invalid token claims")
	}
	return claims, nil
}
