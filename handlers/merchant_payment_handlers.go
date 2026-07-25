package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
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
	ShopID            string          `json:"shopId"`
	Items             []SaleItemInput `json:"items"`
	CustomerID        *string         `json:"customerId,omitempty"` // Optional customer ID
	ClientOperationID string          `json:"clientOperationId"`
}

// HandleCreatePaymentIntent creates a Stripe Payment Intent.
func HandleCreatePaymentIntent(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	var req CreatePaymentIntentRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot create a payment for an empty cart."})
	}
	if req.ShopID == "" || len(req.Items) > 100 || len(req.ClientOperationID) < 8 || len(req.ClientOperationID) > 255 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "shopId, clientOperationId, and no more than 100 items are required"})
	}
	var shopOwned bool
	if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shops WHERE id=$1 AND merchant_id=$2)`, req.ShopID, merchantID).Scan(&shopOwned); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to verify shop"})
	}
	if !shopOwned {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "Shop access denied"})
	}
	if req.CustomerID != nil && *req.CustomerID != "" {
		var customerOwned bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shop_customers WHERE id=$1 AND shop_id=$2 AND merchant_id=$3)`, *req.CustomerID, req.ShopID, merchantID).Scan(&customerOwned); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to verify customer"})
		}
		if !customerOwned {
			return c.Status(403).JSON(fiber.Map{"success": false, "message": "Customer does not belong to this shop"})
		}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to start database transaction"})
	}
	defer tx.Rollback(ctx) // Rollback in case of errors

	var totalAmount int64 = 0
	var descriptionItems []string

	for _, item := range req.Items {
		if item.InventoryItemID == "" || item.QuantitySold <= 0 {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid payment item"})
		}
		var sellingPrice float64
		var currentStock int
		var itemName string

		// 1. Resolve the canonical stock item and its current merchant price.
		queryItem := `SELECT si.name, COALESCE(pp.selling_price,0) FROM stock_items si
			JOIN products p ON p.id=si.product_id
			LEFT JOIN LATERAL (SELECT selling_price FROM product_prices WHERE product_id=si.product_id AND merchant_id=$2 AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1) pp ON TRUE
			WHERE si.id=$1 AND si.merchant_id=$2`
		err := tx.QueryRow(ctx, queryItem, item.InventoryItemID, merchantID).Scan(&itemName, &sellingPrice)
		if err != nil {
			log.Printf("Error fetching item details for %s: %v", item.InventoryItemID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid item in cart."})
		}

		// 2. Check stock availability in the specific shop.
		queryStock := "SELECT quantity_on_hand FROM inventory_items WHERE shop_id = $1 AND stock_item_id = $2 AND merchant_id = $3 FOR UPDATE"
		err = tx.QueryRow(ctx, queryStock, req.ShopID, item.InventoryItemID, merchantID).Scan(&currentStock)
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
	params.SetIdempotencyKey(req.ClientOperationID)
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
