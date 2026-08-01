package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

var serialSortFields = map[string]string{"serialNumber": "serial_number", "status": "status", "createdAt": "created_at"}
var assetSortFields = map[string]string{"assetTag": "asset_tag", "status": "status", "createdAt": "created_at"}

func scanInventorySerial(scan func(...interface{}) error) (models.InventorySerial, error) {
	var item models.InventorySerial
	var stock, ref sql.NullString
	if err := scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &item.SerialNumber, &item.Status, &ref, &item.CreatedAt); err != nil {
		return item, err
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	if ref.Valid {
		item.ReferenceID = &ref.String
	}
	return item, nil
}

func HandleListInventorySerials(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "createdAt", serialSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	for _, pair := range []struct{ key, col string }{{"shopId", "shop_id"}, {"inventoryItemId", "inventory_item_id"}, {"status", "status"}} {
		if v := strings.TrimSpace(c.Query(pair.key)); v != "" {
			where += " AND " + pair.col + "=$" + itoa(len(args)+1)
			args = append(args, v)
		}
	}
	if q.Search != "" {
		where += " AND serial_number ILIKE $" + itoa(len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_serials"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count inventory serials")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,serial_number,status,reference_id,created_at FROM inventory_serials"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list inventory serials")
	}
	defer rows.Close()
	items := make([]models.InventorySerial, 0)
	for rows.Next() {
		item, scanErr := scanInventorySerial(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(500, "failed to read inventory serial")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateInventorySerial(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventorySerialRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.ShopID) == "" || strings.TrimSpace(req.SerialNumber) == "" {
		return fiber.NewError(400, "shopId and serialNumber are required")
	}
	status := "AVAILABLE"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.ToUpper(strings.TrimSpace(*req.Status))
	}
	item, err := scanInventorySerial(database.GetDB().QueryRow(context.Background(), `INSERT INTO inventory_serials(merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,serial_number,status,reference_id) SELECT ii.merchant_id,ii.shop_id,ii.id,ii.product_id,ii.stock_item_id,$2,$3,$4 FROM inventory_items ii WHERE ii.id=$1 AND ii.shop_id=$5 AND ii.merchant_id=$6 RETURNING id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,serial_number,status,reference_id,created_at`, c.Params("inventoryItemId"), strings.TrimSpace(req.SerialNumber), status, nullableStringValue(req.ReferenceID), req.ShopID, merchantID).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory item not found for this shop")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "serial number already exists")
		}
		return fiber.NewError(500, "failed to create inventory serial")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventorySerial(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventorySerialRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.SerialNumber) == "" {
		return fiber.NewError(400, "serialNumber is required")
	}
	status := "AVAILABLE"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.ToUpper(strings.TrimSpace(*req.Status))
	}
	var item models.InventorySerial
	var stock, ref sql.NullString
	err = database.GetDB().QueryRow(context.Background(), `UPDATE inventory_serials SET serial_number=$1,status=$2,reference_id=$3 WHERE id=$4 AND merchant_id=$5 RETURNING id,merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,serial_number,status,reference_id,created_at`, strings.TrimSpace(req.SerialNumber), status, nullableStringValue(req.ReferenceID), c.Params("serialId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &item.ProductID, &stock, &item.SerialNumber, &item.Status, &ref, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory serial not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "serial number already exists")
		}
		return fiber.NewError(500, "failed to update inventory serial")
	}
	if stock.Valid {
		item.StockItemID = &stock.String
	}
	if ref.Valid {
		item.ReferenceID = &ref.String
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteInventorySerial(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM inventory_serials WHERE id=$1 AND merchant_id=$2 AND status<>'SOLD'`, c.Params("serialId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete inventory serial")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(409, "serial not found or cannot be deleted")
	}
	return c.SendStatus(204)
}

func scanInventoryAsset(scan func(...interface{}) error) (models.InventoryAsset, error) {
	var item models.InventoryAsset
	var batch sql.NullString
	var metadata []byte
	if err := scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &batch, &item.AssetTag, &item.Status, &metadata, &item.CreatedAt); err != nil {
		return item, err
	}
	if batch.Valid {
		item.BatchID = &batch.String
	}
	item.Metadata = map[string]interface{}{}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &item.Metadata)
	}
	return item, nil
}

func HandleListInventoryAssets(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "createdAt", assetSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	for _, pair := range []struct{ key, col string }{{"shopId", "shop_id"}, {"inventoryItemId", "inventory_item_id"}, {"status", "status"}} {
		if v := strings.TrimSpace(c.Query(pair.key)); v != "" {
			where += " AND " + pair.col + "=$" + itoa(len(args)+1)
			args = append(args, v)
		}
	}
	if q.Search != "" {
		where += " AND asset_tag ILIKE $" + itoa(len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_assets"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count inventory assets")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,shop_id,inventory_item_id,batch_id,asset_tag,status,metadata,created_at FROM inventory_assets"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list inventory assets")
	}
	defer rows.Close()
	items := make([]models.InventoryAsset, 0)
	for rows.Next() {
		item, scanErr := scanInventoryAsset(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(500, "failed to read inventory asset")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateInventoryAsset(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryAssetRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.ShopID) == "" || strings.TrimSpace(req.AssetTag) == "" {
		return fiber.NewError(400, "shopId and assetTag are required")
	}
	status := "AVAILABLE"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.ToUpper(strings.TrimSpace(*req.Status))
	}
	metadata, _ := json.Marshal(req.Metadata)
	item, err := scanInventoryAsset(database.GetDB().QueryRow(context.Background(), `INSERT INTO inventory_assets(merchant_id,shop_id,inventory_item_id,batch_id,asset_tag,status,metadata) SELECT ii.merchant_id,ii.shop_id,ii.id,$2,$3,$4,$5 FROM inventory_items ii WHERE ii.id=$1 AND ii.shop_id=$6 AND ii.merchant_id=$7 RETURNING id,merchant_id,shop_id,inventory_item_id,batch_id,asset_tag,status,metadata,created_at`, c.Params("inventoryItemId"), nullableStringValue(req.BatchID), strings.TrimSpace(req.AssetTag), status, metadata, req.ShopID, merchantID).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory item not found for this shop")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "asset tag already exists")
		}
		return fiber.NewError(500, "failed to create inventory asset")
	}
	item.Metadata = req.Metadata
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateInventoryAsset(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryAssetRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.AssetTag) == "" {
		return fiber.NewError(400, "assetTag is required")
	}
	status := "AVAILABLE"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		status = strings.ToUpper(strings.TrimSpace(*req.Status))
	}
	metadata, _ := json.Marshal(req.Metadata)
	var item models.InventoryAsset
	var batch sql.NullString
	var raw []byte
	err = database.GetDB().QueryRow(context.Background(), `UPDATE inventory_assets SET asset_tag=$1,batch_id=$2,status=$3,metadata=$4 WHERE id=$5 AND merchant_id=$6 RETURNING id,merchant_id,shop_id,inventory_item_id,batch_id,asset_tag,status,metadata,created_at`, strings.TrimSpace(req.AssetTag), nullableStringValue(req.BatchID), status, metadata, c.Params("assetId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.ShopID, &item.InventoryItemID, &batch, &item.AssetTag, &item.Status, &raw, &item.CreatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "inventory asset not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "asset tag already exists")
		}
		return fiber.NewError(500, "failed to update inventory asset")
	}
	if batch.Valid {
		item.BatchID = &batch.String
	}
	item.Metadata = req.Metadata
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteInventoryAsset(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM inventory_assets WHERE id=$1 AND merchant_id=$2 AND status NOT IN ('SOLD','RESERVED')`, c.Params("assetId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete inventory asset")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(409, "asset not found or cannot be deleted")
	}
	return c.SendStatus(204)
}

func HandleListInventoryAssetIdentifiers(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	rows, err := database.GetDB().Query(context.Background(), `SELECT ai.id,ai.asset_id,ai.identifier_type_id,ai.value,ai.normalized_value,ai.is_primary FROM inventory_asset_identifiers ai JOIN inventory_assets a ON a.id=ai.asset_id WHERE ai.asset_id=$1 AND a.merchant_id=$2 ORDER BY ai.is_primary DESC,ai.id`, c.Params("assetId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to list asset identifiers")
	}
	defer rows.Close()
	items := make([]models.InventoryAssetIdentifier, 0)
	for rows.Next() {
		var item models.InventoryAssetIdentifier
		if err := rows.Scan(&item.ID, &item.AssetID, &item.IdentifierTypeID, &item.Value, &item.NormalizedValue, &item.IsPrimary); err != nil {
			return fiber.NewError(500, "failed to read asset identifier")
		}
		items = append(items, item)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items})
}

func HandleCreateInventoryAssetIdentifier(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.InventoryAssetIdentifierRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.IdentifierTypeID) == "" || strings.TrimSpace(req.Value) == "" {
		return fiber.NewError(400, "identifierTypeId and value are required")
	}
	var valid bool
	if err := database.GetDB().QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM inventory_assets a JOIN inventory_identifier_types t ON t.merchant_id=a.merchant_id WHERE a.id=$1 AND a.merchant_id=$2 AND t.id=$3)`, c.Params("assetId"), merchantID, req.IdentifierTypeID).Scan(&valid); err != nil {
		return fiber.NewError(500, "failed to validate asset identifier")
	}
	if !valid {
		return fiber.NewError(400, "asset or identifier type is invalid")
	}
	var item models.InventoryAssetIdentifier
	normalized := strings.ToUpper(strings.TrimSpace(req.Value))
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO inventory_asset_identifiers(asset_id,identifier_type_id,value,normalized_value,is_primary) VALUES($1,$2,$3,$4,$5) RETURNING id,asset_id,identifier_type_id,value,normalized_value,is_primary`, c.Params("assetId"), req.IdentifierTypeID, strings.TrimSpace(req.Value), normalized, req.IsPrimary).Scan(&item.ID, &item.AssetID, &item.IdentifierTypeID, &item.Value, &item.NormalizedValue, &item.IsPrimary)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "asset identifier already exists")
		}
		return fiber.NewError(500, "failed to create asset identifier")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteInventoryAssetIdentifier(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM inventory_asset_identifiers ai USING inventory_assets a WHERE ai.id=$1 AND a.id=ai.asset_id AND a.merchant_id=$2`, c.Params("identifierId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete asset identifier")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "asset identifier not found")
	}
	return c.SendStatus(204)
}
