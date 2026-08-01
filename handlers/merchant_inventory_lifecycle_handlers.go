package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

var batchSortFields = map[string]string{"batchCode": "batch_code", "quantityRemaining": "quantity_remaining", "expiryDate": "expiry_date", "createdAt": "created_at"}
var reservationSortFields = map[string]string{"quantity": "quantity", "status": "status", "createdAt": "created_at", "updatedAt": "updated_at"}

func scanInventoryBatch(scan func(...interface{}) error) (models.InventoryBatch, error) {
	var item models.InventoryBatch
	var stock sql.NullString
	if err := scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &item.BatchCode, &item.QuantityReceived, &item.QuantityRemaining, &item.UnitCost, &item.ManufactureDate, &item.ExpiryDate, &item.CreatedAt); err != nil {
		return item, err
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	return item, nil
}

func HandleListInventoryBatches(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "createdAt", batchSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	if v := strings.TrimSpace(c.Query("shopId")); v != "" {
		where += " AND shop_id=$" + itoa(len(args)+1)
		args = append(args, v)
	}
	if v := strings.TrimSpace(c.Query("inventoryItemId")); v != "" {
		where += " AND inventory_item_id=$" + itoa(len(args)+1)
		args = append(args, v)
	}
	if q.Search != "" {
		where += " AND batch_code ILIKE $" + itoa(len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_batches"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count inventory batches")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,batch_code,quantity_received,quantity_remaining,unit_cost,manufacture_date,expiry_date,created_at FROM inventory_batches"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list inventory batches")
	}
	defer rows.Close()
	items := make([]models.InventoryBatch, 0)
	for rows.Next() {
		item, scanErr := scanInventoryBatch(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(500, "failed to read inventory batch")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateInventoryBatch(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryBatchRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.BatchCode) == "" || req.QuantityReceived < 0 || req.UnitCost < 0 {
		return fiber.NewError(400, "batchCode, non-negative quantityReceived, and non-negative unitCost are required")
	}
	item, err := scanInventoryBatch(database.GetDB().QueryRow(context.Background(), `INSERT INTO inventory_batches(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,batch_code,quantity_received,quantity_remaining,unit_cost,manufacture_date,expiry_date) SELECT ii.merchant_id,ii.shop_id,ii.id,ii.product_id,ii.stock_item_id,$2,$3,$3,$4,$5,$6 FROM inventory_items ii WHERE ii.id=$1 AND ii.shop_id=$7 AND ii.merchant_id=$8 RETURNING id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,batch_code,quantity_received,quantity_remaining,unit_cost,manufacture_date,expiry_date,created_at`, c.Params("inventoryItemId"), strings.TrimSpace(req.BatchCode), req.QuantityReceived, req.UnitCost, req.ManufactureDate, req.ExpiryDate, c.Query("shopId"), merchantID).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory item not found for this shop")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "batch code already exists for this stock item")
		}
		return fiber.NewError(500, "failed to create inventory batch")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventoryBatch(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryBatchRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.BatchCode) == "" || req.QuantityReceived < 0 || req.UnitCost < 0 {
		return fiber.NewError(400, "batchCode, non-negative quantityReceived, and non-negative unitCost are required")
	}
	var item models.InventoryBatch
	var stock sql.NullString
	err = database.GetDB().QueryRow(context.Background(), `UPDATE inventory_batches SET batch_code=$1,quantity_received=$2,quantity_remaining=LEAST(quantity_remaining,$2),unit_cost=$3,manufacture_date=$4,expiry_date=$5 WHERE id=$6 AND merchant_id=$7 RETURNING id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,batch_code,quantity_received,quantity_remaining,unit_cost,manufacture_date,expiry_date,created_at`, strings.TrimSpace(req.BatchCode), req.QuantityReceived, req.UnitCost, req.ManufactureDate, req.ExpiryDate, c.Params("batchId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &item.BatchCode, &item.QuantityReceived, &item.QuantityRemaining, &item.UnitCost, &item.ManufactureDate, &item.ExpiryDate, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory batch not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "batch code already exists for this stock item")
		}
		return fiber.NewError(500, "failed to update inventory batch")
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteInventoryBatch(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM inventory_batches WHERE id=$1 AND merchant_id=$2 AND quantity_remaining=0`, c.Params("batchId"), merchantID)
	if err != nil {
		return fiber.NewError(409, "batch cannot be deleted while quantity remains")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "empty inventory batch not found")
	}
	return c.SendStatus(204)
}

func scanInventoryReservation(scan func(...interface{}) error) (models.InventoryReservation, error) {
	var item models.InventoryReservation
	var stock, unit, ref sql.NullString
	var base sql.NullFloat64
	if err := scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &unit, &ref, &item.ReservationKey, &item.Quantity, &base, &item.Status, &item.ReleasedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	if unit.Valid {
		item.UnitID = &unit.String
	}
	if ref.Valid {
		item.ReferenceID = &ref.String
	}
	if base.Valid {
		item.BaseQuantity = &base.Float64
	}
	return item, nil
}

func HandleListInventoryReservations(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "createdAt", reservationSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	for _, pair := range []struct{ key, col string }{{"shopId", "shop_id"}, {"inventoryItemId", "inventory_item_id"}, {"status", "status"}} {
		if v := strings.TrimSpace(c.Query(pair.key)); v != "" {
			where += " AND " + pair.col + "=$" + itoa(len(args)+1)
			args = append(args, v)
		}
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_reservations"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count inventory reservations")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,unit_id,reference_id,reservation_key,quantity,base_quantity,status,released_at,created_at,updated_at FROM inventory_reservations"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list inventory reservations")
	}
	defer rows.Close()
	items := make([]models.InventoryReservation, 0)
	for rows.Next() {
		item, scanErr := scanInventoryReservation(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(500, "failed to read inventory reservation")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateInventoryReservation(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryReservationRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.ShopID) == "" || strings.TrimSpace(req.ReservationKey) == "" || req.Quantity <= 0 {
		return fiber.NewError(400, "shopId, reservationKey, and positive quantity are required")
	}
	ctx := context.Background()
	tx, err := database.GetDB().Begin(ctx)
	if err != nil {
		return fiber.NewError(500, "failed to start reservation transaction")
	}
	defer tx.Rollback(ctx)
	var productID, stockItemID, inventoryShop string
	var onHand, reserved float64
	err = tx.QueryRow(ctx, `SELECT product_id,stock_item_id,shop_id,quantity_on_hand,reserved_quantity FROM inventory_items WHERE id=$1 AND shop_id=$2 AND merchant_id=$3 FOR UPDATE`, c.Params("inventoryItemId"), req.ShopID, merchantID).Scan(&productID, &stockItemID, &inventoryShop, &onHand, &reserved)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory item not found")
	}
	if err != nil {
		return fiber.NewError(500, "failed to lock inventory item")
	}
	if onHand-reserved < req.Quantity {
		return fiber.NewError(409, "insufficient available inventory")
	}
	var item models.InventoryReservation
	var stock, unit, ref sql.NullString
	var base sql.NullFloat64
	err = tx.QueryRow(ctx, `INSERT INTO inventory_reservations(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,unit_id,reference_id,reservation_key,quantity,base_quantity) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,unit_id,reference_id,reservation_key,quantity,base_quantity,status,released_at,created_at,updated_at`, merchantID, req.ShopID, c.Params("inventoryItemId"), productID, stockItemID, nullableStringValue(req.UnitID), nullableStringValue(req.ReferenceID), req.ReservationKey, req.Quantity, req.BaseQuantity).Scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &unit, &ref, &item.ReservationKey, &item.Quantity, &base, &item.Status, &item.ReleasedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "reservation key already exists")
		}
		return fiber.NewError(500, "failed to create reservation")
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_items SET reserved_quantity=reserved_quantity+$1,updated_at=NOW() WHERE id=$2`, req.Quantity, c.Params("inventoryItemId")); err != nil {
		return fiber.NewError(500, "failed to update reserved inventory")
	}
	if err = tx.Commit(ctx); err != nil {
		return fiber.NewError(500, "failed to commit reservation")
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	if unit.Valid {
		item.UnitID = &unit.String
	}
	if ref.Valid {
		item.ReferenceID = &ref.String
	}
	if base.Valid {
		item.BaseQuantity = &base.Float64
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleReleaseInventoryReservation(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := database.GetDB().Begin(ctx)
	if err != nil {
		return fiber.NewError(500, "failed to start release transaction")
	}
	defer tx.Rollback(ctx)
	var inventoryID string
	var qty float64
	var status string
	err = tx.QueryRow(ctx, `SELECT inventory_item_id,quantity,status FROM inventory_reservations WHERE id=$1 AND merchant_id=$2 FOR UPDATE`, c.Params("reservationId"), merchantID).Scan(&inventoryID, &qty, &status)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "reservation not found")
	}
	if err != nil {
		return fiber.NewError(500, "failed to lock reservation")
	}
	if status == "RELEASED" {
		return c.SendStatus(204)
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_reservations SET status='RELEASED',released_at=NOW(),updated_at=NOW() WHERE id=$1`, c.Params("reservationId")); err != nil {
		return fiber.NewError(500, "failed to release reservation")
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory_items SET reserved_quantity=GREATEST(0,reserved_quantity-$1),updated_at=NOW() WHERE id=$2`, qty, inventoryID); err != nil {
		return fiber.NewError(500, "failed to release reserved inventory")
	}
	if err = tx.Commit(ctx); err != nil {
		return fiber.NewError(500, "failed to commit reservation release")
	}
	return c.SendStatus(204)
}
