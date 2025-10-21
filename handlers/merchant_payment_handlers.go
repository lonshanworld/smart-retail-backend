package handlers

import (
	"app/database"
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
)

func init() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
}

// SaleItemInput defines the structure for an item in the payment request.
type SaleItemInput struct {
	InventoryItemID string `json:"inventoryItemId"`
	QuantitySold    int    `json:"quantitySold"`
}

// CreatePaymentIntentRequest defines the request body for creating a payment intent.
type CreatePaymentIntentRequest struct {
	ShopID     string          `json:"shopId"`
	Items      []SaleItemInput `json:"items"`
	CustomerID *string         `json:"customerId,omitempty"` // Optional customer ID
}

// HandleCreatePaymentIntent creates a Stripe Payment Intent.
func HandleCreatePaymentIntent(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	var req CreatePaymentIntentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot create a payment for an empty cart."})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start database transaction"})
	}
	defer tx.Rollback(ctx) // Rollback in case of errors

	var totalAmount int64 = 0
	var descriptionItems []string

	for _, item := range req.Items {
		var sellingPrice float64
		var currentStock int
		var itemName string

		// 1. Get item price and name from inventory_items
		queryItem := "SELECT name, selling_price FROM inventory_items WHERE id = $1 AND merchant_id = $2"
		err := tx.QueryRow(ctx, queryItem, item.InventoryItemID, merchantID).Scan(&itemName, &sellingPrice)
		if err != nil {
			log.Printf("Error fetching item details for %s: %v", item.InventoryItemID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid item in cart."})
		}

		// 2. Check stock availability in the specific shop
		queryStock := "SELECT quantity FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2 FOR UPDATE"
		err = tx.QueryRow(ctx, queryStock, req.ShopID, item.InventoryItemID).Scan(&currentStock)
		if err != nil {
			log.Printf("Error fetching stock for item %s in shop %s: %v", item.InventoryItemID, req.ShopID, err)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "Item not found in this shop's inventory."})
		}

		if currentStock < item.QuantitySold {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false, 
				"message": "Insufficient stock for item: " + itemName,
			})
		}

		totalAmount += int64(sellingPrice * float64(item.QuantitySold) * 100) // Convert to cents
		descriptionItems = append(descriptionItems, itemName)
	}

	// If we reach here, all items are valid and stock is sufficient. No need to commit yet.

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(totalAmount),
		Currency: stripe.String(string(stripe.CurrencyUSD)), // Or get from config
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	params.AddMetadata("shopId", req.ShopID)
	params.AddMetadata("merchantId", merchantID)

	pi, err := paymentintent.New(params)
	if err != nil {
		log.Printf("Stripe payment intent creation failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create payment intent with provider."})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"clientSecret": pi.ClientSecret},
	})
}
