package middleware

import (
	"errors"
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
	authHeader := c.Get("Authorization")

	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Missing authorization token",
		})
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid authorization format",
		})
	}

	claims := &models.JwtClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			log.Printf("Unexpected signing method: %v", token.Header["alg"])
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid token",
		})
	}

	c.Locals("user", token)
	c.Locals("userID", claims.UserID)
	c.Locals("userRole", claims.Role)

	return c.Next()
}

// AdminRequired checks if the authenticated user has admin role
func AdminRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	if !ok || role != "admin" {
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
	if !ok || role != "merchant" {
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
	if !ok || role != "staff" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Staff access required",
		})
	}
	return c.Next()
}

// ExtractClaims extracts the JWT claims from the context
func ExtractClaims(c *fiber.Ctx) (*models.JwtClaims, error) {
	user, ok := c.Locals("user").(*jwt.Token)
	if !ok || user == nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated user")
	}
	claims, ok := user.Claims.(*models.JwtClaims)
	if !ok {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Invalid token claims")
	}
	return claims, nil
}
