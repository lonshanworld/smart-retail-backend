package handlers

import (
	"app/config"
	"app/database"
	"app/middleware"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

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

func ptrString(value string) *string { return &value }

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
	if config.AppConfig.LocalStorageOnly {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":  "error",
			"message": "Cloud sync is disabled in LOCAL_STORAGE_ONLY mode",
		})
	}

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

		if _, err := tx.Exec(ctx, "SAVEPOINT offline_sale_sync"); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to prepare sale sync", "results": results})
		}
		result := processSaleSync(ctx, tx, merchantID, offlineSale)
		results = append(results, result)

		if result.Status == "synced" {
			if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT offline_sale_sync"); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to finalize sale sync", "results": results})
			}
			successCount++
			log.Printf("✅ [SYNC] Sale %s synced successfully as %s", offlineSale.ID, *result.ServerID)
		} else {
			if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT offline_sale_sync"); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to roll back failed sale sync", "results": results})
			}
			_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT offline_sale_sync")
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
		"status":  "success",
		"success": true,
		"data":    response,
	})
}

// processSaleSync processes a single offline sale and returns the result
func processSaleSync(ctx context.Context, tx pgx.Tx, merchantID string, offlineSale OfflineSaleData) SyncResult {
	// Wrap pgx transaction to DBTx and call testable function.
	adapter := pgxTxAdapter{tx: tx}
	return processSaleSyncWithDB(ctx, adapter, merchantID, offlineSale)
}

// processSaleSyncWithDB contains the core sync logic over a minimal DBTx interface
// so unit tests can run without a real database transaction.
func processSaleSyncWithDB(ctx context.Context, tx DBTx, merchantID string, offlineSale OfflineSaleData) SyncResult {
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
		errMsg := "Access denied - shop belongs to different merchant"
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Access denied for sale %s", offlineSale.ID)
		return result
	}

	// Check for duplicate sale using local_id (prevents re-syncing same sale)
	var existingSaleID string
	if offlineSale.ID == "" {
		errMsg := "Missing client sale id"
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Missing client sale id for sale")
		return result
	}
	duplicateCheckQuery := "SELECT id FROM sales WHERE client_sale_id = $1 AND merchant_id = $2"
	if err := tx.QueryRow(ctx, duplicateCheckQuery, offlineSale.ID, merchantID).Scan(&existingSaleID); err == nil {
		// Sale already exists - return success with existing server ID
		log.Printf("⚠️  [SYNC ITEM] Duplicate detected - Sale %s already synced as %s", offlineSale.ID, existingSaleID)
		result.Status = "synced"
		result.ServerID = &existingSaleID
		now := time.Now()
		result.ServerTimestamp = &now
		return result
	} else if !isNoRows(err) {
		// Real error (not just "no rows")
		errMsg := fmt.Sprintf("Error checking for duplicates: %v", err)
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Duplicate check failed: %v", err)
		return result
	}
	if offlineSale.ShopID == "" || len(offlineSale.Items) == 0 || len(offlineSale.Items) > 100 || offlineSale.TotalAmount < 0 {
		errMsg := "Invalid offline sale payload"
		result.Error = &errMsg
		return result
	}
	seenProducts := make(map[string]struct{}, len(offlineSale.Items))
	var calculatedTotal float64
	for _, item := range offlineSale.Items {
		if item.ProductID == "" || item.Quantity <= 0 || item.SellingPriceAtSale < 0 {
			errMsg := "Invalid offline sale item"
			result.Error = &errMsg
			return result
		}
		if _, exists := seenProducts[item.ProductID]; exists {
			errMsg := "Duplicate product lines are not allowed in an offline sale"
			result.Error = &errMsg
			return result
		}
		seenProducts[item.ProductID] = struct{}{}
		calculatedTotal += float64(item.Quantity) * item.SellingPriceAtSale
	}
	if calculatedTotal < offlineSale.TotalAmount-0.01 || calculatedTotal > offlineSale.TotalAmount+0.01 {
		errMsg := "Offline sale total does not match its items"
		result.Error = &errMsg
		return result
	}

	// Validate items exist and get prices
	itemPrices := make(map[string]float64)
	itemInventoryIDs := make(map[string]string)
	itemProductIDs := make(map[string]string)
	itemStockItemIDs := make(map[string]string)
	itemNames := make(map[string]string)
	itemSKUs := make(map[string]*string)
	for _, item := range offlineSale.Items {
		var currentPrice float64
		var inventoryID, productID, stockItemID, itemName string
		var itemSKU *string
		priceQuery := `SELECT ii.id,si.product_id,si.id,si.name,si.sku,COALESCE(pp.selling_price,0) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id LEFT JOIN LATERAL(SELECT selling_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE WHERE ii.shop_id=$1 AND (ii.stock_item_id=$2 OR ii.product_id=$2) AND ii.merchant_id=$3 AND ii.quantity_on_hand >= $4 FOR UPDATE OF ii,si`
		if err := tx.QueryRow(ctx, priceQuery, offlineSale.ShopID, item.ProductID, merchantID, item.Quantity).Scan(&inventoryID, &productID, &stockItemID, &itemName, &itemSKU, &currentPrice); err != nil {
			if isNoRows(err) {
				errMsg := fmt.Sprintf("Item not found or unavailable: %s", item.ProductID)
				result.Error = &errMsg
				log.Printf("❌ [SYNC ITEM] Item validation failed: %v", err)
				return result
			}
			errMsg := fmt.Sprintf("Error fetching price for item %s: %v", item.ProductID, err)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Item price query failed: %v", err)
			return result
		}
		itemPrices[item.ProductID] = currentPrice
		itemInventoryIDs[item.ProductID] = inventoryID
		itemProductIDs[item.ProductID] = productID
		itemStockItemIDs[item.ProductID] = stockItemID
		itemNames[item.ProductID] = itemName
		itemSKUs[item.ProductID] = itemSKU
	}

	// Create sale
	saleID := generateUUID()
	now := time.Now()

	// Store local_id in notes for audit/history, but use client_sale_id for real idempotency.
	notesWithLocalID := fmt.Sprintf("offline_local_id:%s", offlineSale.ID)
	if offlineSale.Notes != nil && *offlineSale.Notes != "" {
		notesWithLocalID = fmt.Sprintf("%s | %s", notesWithLocalID, *offlineSale.Notes)
	}

	createSaleQuery := `
		INSERT INTO sales (id, client_sale_id, shop_id, merchant_id, sale_date, total_amount, payment_type, payment_status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	if _, err := tx.Exec(ctx, createSaleQuery,
		saleID, offlineSale.ID, offlineSale.ShopID, merchantID, offlineSale.Timestamp,
		offlineSale.TotalAmount, offlineSale.PaymentType, "succeeded",
		notesWithLocalID, now, now,
	); err != nil {
		errMsg := fmt.Sprintf("Failed to create sale: %v", err)
		result.Error = &errMsg
		log.Printf("❌ [SYNC ITEM] Sale creation failed: %v", err)
		return result
	}

	// Create sale items
	for _, item := range offlineSale.Items {
		itemID := generateUUID()
		createItemQuery := `
			INSERT INTO sale_items (id, sale_id, inventory_item_id, product_id, stock_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`

		subtotal := float64(item.Quantity) * item.SellingPriceAtSale

		if _, err := tx.Exec(ctx, createItemQuery,
			itemID, saleID, itemInventoryIDs[item.ProductID], itemProductIDs[item.ProductID], itemStockItemIDs[item.ProductID], itemNames[item.ProductID], itemSKUs[item.ProductID], item.Quantity,
			item.SellingPriceAtSale, item.OriginalPriceAtSale, subtotal, now, now,
		); err != nil {
			errMsg := fmt.Sprintf("Failed to create sale item: %v", err)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Item creation failed: %v", err)
			return result
		}
	}

	// Update inventory (deduct quantities)
	for _, item := range offlineSale.Items {
		updateStockQuery := `UPDATE inventory_items SET quantity_on_hand=quantity_on_hand-$1,updated_at=$2 WHERE id=$3 AND merchant_id=$4 AND quantity_on_hand >= $1`

		updateResult, err := tx.Exec(ctx, updateStockQuery,
			item.Quantity, now, itemInventoryIDs[item.ProductID], merchantID,
		)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to update inventory: %v", err)
			result.Error = &errMsg
			log.Printf("❌ [SYNC ITEM] Inventory update failed: %v", err)
			return result
		}
		if updateResult != 1 {
			errMsg := fmt.Sprintf("Insufficient stock for item: %s", item.ProductID)
			result.Error = &errMsg
			return result
		}
		if _, err := tx.Exec(ctx, `INSERT INTO inventory_movements(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes) VALUES($1,$2,$3,$4,$5,'OUT',$6,$6,'OFFLINE_SALE',$7,$8,'Offline sale sync')`, merchantID, offlineSale.ShopID, itemInventoryIDs[item.ProductID], itemProductIDs[item.ProductID], itemStockItemIDs[item.ProductID], item.Quantity, saleID, fmt.Sprintf("%s:%s", offlineSale.ID, item.ProductID)); err != nil {
			result.Error = ptrString(fmt.Sprintf("Failed to record inventory movement: %v", err))
			return result
		}
	}

	// Success
	result.Status = "synced"
	result.ServerID = &saleID
	now = time.Now()
	result.ServerTimestamp = &now
	log.Printf("✅ [SYNC ITEM] Sale %s synced as %s", offlineSale.ID, saleID)

	return result
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return err.Error() == "no rows in result set"
}

// logSyncOperation logs the sync operation for audit trail
func logSyncOperation(ctx context.Context, db *pgxpool.Pool, merchantID, deviceID string, total, success, failed int) error {
	query := `
		INSERT INTO sync_logs (id, merchant_id, device_id, total_sales, successful_syncs, failed_syncs, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	syncLogID := generateUUID()
	_, err := db.Exec(ctx, query,
		syncLogID, merchantID, deviceID, total, success, failed, time.Now(),
	)

	return err
}

// generateUUID generates a UUID (simplified for this example)
// Uses google/uuid for collision-safe IDs.
func generateUUID() string {
	return uuid.NewString()
}
