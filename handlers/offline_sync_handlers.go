package handlers

import (
	"app/database"
	"app/middleware"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// SyncRequest represents a batch of offline sales to sync
type SyncRequest struct {
	BatchID   string            `json:"batchId"`
	Timestamp time.Time         `json:"timestamp"`
	Sales     []OfflineSaleData `json:"sales"`
	DeviceID  string            `json:"deviceId"`
	UserID    string            `json:"userId"`
}

// OfflineSaleData represents a single offline sale to sync
type OfflineSaleData struct {
	ID            string            `json:"id"`
	ShopID        string            `json:"shopId"`
	TotalAmount   float64           `json:"totalAmount"`
	Items         []OfflineSaleItem `json:"items"`
	PaymentType   string            `json:"paymentType"`
	PaymentStatus string            `json:"paymentStatus"`
	Timestamp     time.Time         `json:"timestamp"`
	Notes         *string           `json:"notes"`
}

// OfflineSaleItem represents an item in an offline sale
type OfflineSaleItem struct {
	ProductID           string   `json:"productId"`
	Quantity            int      `json:"quantity"`
	SellingPriceAtSale  float64  `json:"sellingPriceAtSale"`
	OriginalPriceAtSale *float64 `json:"originalPriceAtSale"`
	DiscountAmount      *float64 `json:"discountAmount"`
}

// SyncResult represents the result of syncing a single sale
type SyncResult struct {
	LocalID         string     `json:"localId"`
	ServerID        *string    `json:"serverId"`
	Status          string     `json:"status"` // "synced" or "failed"
	Error           *string    `json:"error"`
	ServerTimestamp *time.Time `json:"serverTimestamp"`
}

// BatchSyncResponse represents the response for a batch sync
type BatchSyncResponse struct {
	Status      string       `json:"status"` // "success", "partial", "failed"
	SyncBatchID string       `json:"syncBatchId"`
	Results     []SyncResult `json:"results"`
	SyncedCount int          `json:"syncedCount"`
	FailedCount int          `json:"failedCount"`
}

// HandleSyncOfflineSales handles batch syncing of offline sales
func HandleSyncOfflineSales(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	// Parse request body
	var syncReq SyncRequest
	if err := c.BodyParser(&syncReq); err != nil {
		log.Printf("❌ [SYNC] Error parsing sync request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid sync request format",
		})
	}

	log.Printf("🔄 [SYNC] Batch sync started - Batch: %s, Sales: %d, Merchant: %s, User: %s",
		syncReq.BatchID, len(syncReq.Sales), merchantID, syncReq.UserID)

	// Validate request
	if len(syncReq.Sales) == 0 {
		log.Printf("⚠️  [SYNC] Empty sales batch")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "No sales to sync",
		})
	}

	// Process each sale
	results := make([]SyncResult, 0, len(syncReq.Sales))
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("❌ [SYNC] Transaction begin error: %v", err)
		errorMsg := "Transaction error"
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": errorMsg,
			"results": results,
		})
	}
	defer tx.Rollback(ctx)

	successCount := 0
	failureCount := 0

	for _, offlineSale := range syncReq.Sales {
		log.Printf("📝 [SYNC] Processing sale: %s, Shop: %s, Amount: %.2f",
			offlineSale.ID, offlineSale.ShopID, offlineSale.TotalAmount)

		result := processSaleSync(ctx, tx, merchantID, offlineSale)
		results = append(results, result)

		if result.Status == "synced" {
			successCount++
			log.Printf("✅ [SYNC] Sale %s synced successfully as %s", offlineSale.ID, *result.ServerID)
		} else {
			failureCount++
			errorMsg := ""
			if result.Error != nil {
				errorMsg = *result.Error
			}
			log.Printf("❌ [SYNC] Sale %s failed: %s", offlineSale.ID, errorMsg)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("❌ [SYNC] Transaction commit error: %v", err)
		errorMsg := "Failed to commit sync transaction"
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": errorMsg,
			"results": results,
		})
	}

	// Log sync operation
	if err := logSyncOperation(ctx, db, merchantID, syncReq.DeviceID, len(syncReq.Sales), successCount, failureCount); err != nil {
		log.Printf("⚠️  [SYNC] Warning - Failed to log sync: %v", err)
		// Don't fail the response, just log warning
	}

	response := BatchSyncResponse{
		Status:      "success",
		SyncBatchID: syncReq.BatchID,
		Results:     results,
		SyncedCount: successCount,
		FailedCount: failureCount,
	}

	if failureCount > 0 && successCount == 0 {
		response.Status = "failed"
	} else if failureCount > 0 && successCount > 0 {
		response.Status = "partial"
	}

	log.Printf("✅ [SYNC] Batch sync completed - Total: %d, Success: %d, Failed: %d",
		len(syncReq.Sales), successCount, failureCount)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "success",
		"data":   response,
	})
}

// processSaleSync processes a single offline sale and returns the result
func processSaleSync(ctx context.Context, tx pgx.Tx, merchantID string, offlineSale OfflineSaleData) SyncResult {
	result := SyncResult{
		LocalID: offlineSale.ID,
		Status:  "failed",
	}

	// Validate shop ownership
	var foundMerchantID string
	shopCheckQuery := "SELECT merchant_id FROM shops WHERE id = $1"
	if err := tx.QueryRow(ctx, shopCheckQuery, offlineSale.ShopID).Scan(&foundMerchantID); err != nil {
		errMsg := fmt.Sprintf("Shop not found: %s", offlineSale.ShopID)
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Shop validation failed: %v", err)
		return result
	}

	if foundMerchantID != merchantID {
		errMsg := fmt.Sprintf("Access denied - shop belongs to different merchant")
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Access denied for sale %s", offlineSale.ID)
		return result
	}

	// Check for duplicate sale using local_id (prevents re-syncing same sale)
	var existingSaleID string
	duplicateCheckQuery := "SELECT id FROM sales WHERE notes LIKE $1 AND merchant_id = $2"
	localIDPattern := fmt.Sprintf("offline_local_id:%s%%", offlineSale.ID)
	err := tx.QueryRow(ctx, duplicateCheckQuery, localIDPattern, merchantID).Scan(&existingSaleID)

	if err == nil {
		// Sale already exists - return success with existing server ID
		log.Printf("⚠️  [SYNC ITEM] Duplicate detected - Sale %s already synced as %s", offlineSale.ID, existingSaleID)
		result.Status = "synced"
		result.ServerID = &existingSaleID
		now := time.Now()
		result.ServerTimestamp = &now
		return result
	} else if err != pgx.ErrNoRows {
		// Real error (not just "no rows")
		errMsg := fmt.Sprintf("Error checking for duplicates: %v", err)
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Duplicate check failed: %v", err)
		return result
	}

	// Validate items exist and get prices
	itemPrices := make(map[string]float64)
	for _, item := range offlineSale.Items {
		var currentPrice float64
		priceQuery := "SELECT selling_price FROM inventory_items WHERE id = $1 AND merchant_id = $2"
		if err := tx.QueryRow(ctx, priceQuery, item.ProductID, merchantID).Scan(&currentPrice); err != nil {
			errMsg := fmt.Sprintf("Item not found or unavailable: %s", item.ProductID)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Item validation failed: %v", err)
			return result
		}
		itemPrices[item.ProductID] = currentPrice
	}

	// Create sale
	saleID := generateUUID()
	now := time.Now()

	// Store local_id in notes for duplicate tracking
	notesWithLocalID := fmt.Sprintf("offline_local_id:%s", offlineSale.ID)
	if offlineSale.Notes != nil && *offlineSale.Notes != "" {
		notesWithLocalID = fmt.Sprintf("%s | %s", notesWithLocalID, *offlineSale.Notes)
	}

	createSaleQuery := `
		INSERT INTO sales (id, shop_id, merchant_id, sale_date, total_amount, payment_type, payment_status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	var returnedID string
	if err := tx.QueryRow(ctx, createSaleQuery,
		saleID, offlineSale.ShopID, merchantID, offlineSale.Timestamp,
		offlineSale.TotalAmount, offlineSale.PaymentType, "succeeded",
		notesWithLocalID, now, now,
	).Scan(&returnedID); err != nil {
		errMsg := fmt.Sprintf("Failed to create sale: %v", err)
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Sale creation failed: %v", err)
		return result
	}

	// Create sale items
	for _, item := range offlineSale.Items {
		itemID := generateUUID()
		createItemQuery := `
			INSERT INTO sale_items (id, sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, subtotal, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`

		subtotal := float64(item.Quantity) * item.SellingPriceAtSale

		if err := tx.QueryRow(ctx, createItemQuery,
			itemID, returnedID, item.ProductID, item.Quantity,
			item.SellingPriceAtSale, subtotal, now, now,
		).Scan(); err != nil && err != pgx.ErrNoRows {
			errMsg := fmt.Sprintf("Failed to create sale item: %v", err)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Item creation failed: %v", err)
			return result
		}
	}

	// Update inventory (deduct quantities)
	for _, item := range offlineSale.Items {
		updateStockQuery := `
			UPDATE inventory_items
			SET quantity_available = quantity_available - $1,
			    updated_at = $2
			WHERE id = $3 AND merchant_id = $4
		`

		if err := tx.QueryRow(ctx, updateStockQuery,
			item.Quantity, now, item.ProductID, merchantID,
		).Scan(); err != nil && err != pgx.ErrNoRows {
			errMsg := fmt.Sprintf("Failed to update inventory: %v", err)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Inventory update failed: %v", err)
			return result
		}
	}

	// Success
	result.Status = "synced"
	result.ServerID = &returnedID
	now = time.Now()
	result.ServerTimestamp = &now
	log.Printf("✅ [SYNC ITEM] Sale %s synced as %s", offlineSale.ID, returnedID)

	return result
}

// logSyncOperation logs the sync operation for audit trail
func logSyncOperation(ctx context.Context, db *pgxpool.Pool, merchantID, deviceID string, total, success, failed int) error {
	query := `
		INSERT INTO sync_logs (id, merchant_id, device_id, total_sales, successful_syncs, failed_syncs, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	syncLogID := generateUUID()
	_, err := db.Exec(ctx, query,
		syncLogID, merchantID, deviceID, total, success, failed, time.Now(),
	)

	return err
}

// generateUUID generates a UUID (simplified for this example)
// In production, use a proper UUID library
func generateUUID() string {
	return fmt.Sprintf("uuid_%d", time.Now().UnixNano())
}
