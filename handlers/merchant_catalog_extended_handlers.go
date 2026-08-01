package handlers

import (
	"app/database"
	"app/models"
	"app/storage"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

var variantSortFields = map[string]string{"name": "name", "sku": "sku", "createdAt": "created_at", "updatedAt": "updated_at"}
var imageSortFields = map[string]string{"position": "position", "createdAt": "created_at"}
var attributeSortFields = map[string]string{"code": "code", "name": "name", "createdAt": "created_at", "updatedAt": "updated_at"}
var optionSortFields = map[string]string{"position": "position", "value": "value", "label": "label"}
var assignmentSortFields = map[string]string{"createdAt": "created_at", "updatedAt": "updated_at"}

func validJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("attributes must be valid JSON")
	}
	return raw, nil
}

func duplicateResponse(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "success": false, "message": message})
}

func isUniqueViolation(err error) bool {
	pgErr, ok := err.(*pgconn.PgError)
	return ok && pgErr.Code == "23505"
}

func HandleListMerchantVariants(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	productID := strings.TrimSpace(c.Params("productId"))
	q := getCatalogListQuery(c, "createdAt", variantSortFields)
	where := " WHERE merchant_id=$1 AND product_id=$2 AND deleted_at IS NULL"
	args := []interface{}{merchantID, productID}
	if q.Search != "" {
		where += " AND (name ILIKE $3 OR sku ILIKE $3 OR COALESCE(barcode,'') ILIKE $3)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	var total int64
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM product_variants"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count variants")
	}
	rows, err := db.Query(context.Background(), "SELECT id,merchant_id,product_id,name,sku,barcode,attributes,is_active,deleted_at,created_at,updated_at FROM product_variants"+where+q.orderBy()+" LIMIT $4 OFFSET $5", append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list variants")
	}
	defer rows.Close()
	variants := make([]models.ProductVariant, 0)
	for rows.Next() {
		var v models.ProductVariant
		var attrs []byte
		if err := rows.Scan(&v.ID, &v.MerchantID, &v.ProductID, &v.Name, &v.SKU, &v.Barcode, &attrs, &v.IsActive, &v.DeletedAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read variant")
		}
		v.Attributes = json.RawMessage(attrs)
		variants = append(variants, v)
	}
	return c.JSON(paginatedResponse(variants, total, q))
}

func HandleCreateMerchantVariant(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	productID := strings.TrimSpace(c.Params("productId"))
	var req models.ProductVariantRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SKU) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name and sku are required")
	}
	attrs, err := validJSON(req.Attributes)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	db := database.GetDB()
	ctx := context.Background()
	var productExists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND merchant_id=$2)", productID, merchantID).Scan(&productExists); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to validate product")
	}
	if !productExists {
		return fiber.NewError(fiber.StatusNotFound, "product not found")
	}
	var v models.ProductVariant
	if err := db.QueryRow(ctx, `INSERT INTO product_variants (merchant_id,product_id,name,sku,barcode,attributes,is_active) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,merchant_id,product_id,name,sku,barcode,attributes,is_active,deleted_at,created_at,updated_at`, merchantID, productID, strings.TrimSpace(req.Name), strings.TrimSpace(req.SKU), nullableStringValue(req.Barcode), attrs, active).Scan(&v.ID, &v.MerchantID, &v.ProductID, &v.Name, &v.SKU, &v.Barcode, &attrs, &v.IsActive, &v.DeletedAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "variant SKU already exists for this product")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create variant")
	}
	v.Attributes = json.RawMessage(attrs)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": v})
}

func nullableStringValue(value *string) interface{} {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func HandleGetMerchantVariant(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var v models.ProductVariant
	var attrs []byte
	err = database.GetDB().QueryRow(context.Background(), `SELECT id,merchant_id,product_id,name,sku,barcode,attributes,is_active,deleted_at,created_at,updated_at FROM product_variants WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`, c.Params("variantId"), merchantID).Scan(&v.ID, &v.MerchantID, &v.ProductID, &v.Name, &v.SKU, &v.Barcode, &attrs, &v.IsActive, &v.DeletedAt, &v.CreatedAt, &v.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "variant not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to retrieve variant")
	}
	v.Attributes = json.RawMessage(attrs)
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": v})
}

func HandleUpdateMerchantVariant(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.ProductVariantRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	attrs, err := validJSON(req.Attributes)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if len(req.Attributes) == 0 {
		attrs = nil
	}
	active := req.IsActive
	var v models.ProductVariant
	var raw []byte
	query := `UPDATE product_variants SET name=COALESCE(NULLIF($1,''),name),sku=COALESCE(NULLIF($2,''),sku),barcode=$3,attributes=COALESCE($4,attributes),is_active=COALESCE($5,is_active),updated_at=NOW() WHERE id=$6 AND merchant_id=$7 AND deleted_at IS NULL RETURNING id,merchant_id,product_id,name,sku,barcode,attributes,is_active,deleted_at,created_at,updated_at`
	if err := database.GetDB().QueryRow(context.Background(), query, strings.TrimSpace(req.Name), strings.TrimSpace(req.SKU), nullableStringValue(req.Barcode), attrs, active, c.Params("variantId"), merchantID).Scan(&v.ID, &v.MerchantID, &v.ProductID, &v.Name, &v.SKU, &v.Barcode, &raw, &v.IsActive, &v.DeletedAt, &v.CreatedAt, &v.UpdatedAt); err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "variant not found")
	} else if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "variant SKU already exists for this product")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update variant")
	}
	v.Attributes = json.RawMessage(raw)
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": v})
}

func HandleArchiveMerchantVariant(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `UPDATE product_variants SET is_active=FALSE,deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`, c.Params("variantId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to archive variant")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "variant not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func HandleRestoreMerchantVariant(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `UPDATE product_variants SET is_active=TRUE,deleted_at=NULL,updated_at=NOW() WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NOT NULL`, c.Params("variantId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to restore variant")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "archived variant not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func HandleListMerchantProductImages(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "position", imageSortFields)
	where := " WHERE p.id=$1 AND p.merchant_id=$2"
	args := []interface{}{c.Params("productId"), merchantID}
	if q.Search != "" {
		where += " AND i.url ILIKE $3"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	var total int64
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM product_images i JOIN products p ON p.id=i.product_id"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count product images")
	}
	rows, err := db.Query(context.Background(), "SELECT i.id,i.product_id,i.url,i.source_type,i.original_url,i.storage_provider,i.storage_public_id,i.storage_object_name,i.metadata,i.position,i.created_at FROM product_images i JOIN products p ON p.id=i.product_id"+where+q.orderBy()+" LIMIT $4 OFFSET $5", append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list product images")
	}
	defer rows.Close()
	images := make([]models.ProductImage, 0)
	for rows.Next() {
		var image models.ProductImage
		var originalURL, provider, publicID, objectName sql.NullString
		var metadata []byte
		if err := rows.Scan(&image.ID, &image.ProductID, &image.URL, &image.SourceType, &originalURL, &provider, &publicID, &objectName, &metadata, &image.Position, &image.CreatedAt); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read product image")
		}
		if originalURL.Valid {
			image.OriginalURL = &originalURL.String
		}
		if provider.Valid {
			image.StorageProvider = &provider.String
		}
		if publicID.Valid {
			image.StoragePublicID = &publicID.String
		}
		if objectName.Valid {
			image.StorageObjectName = &objectName.String
		}
		image.Metadata = map[string]interface{}{}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &image.Metadata)
		}
		images = append(images, image)
	}
	return c.JSON(paginatedResponse(images, total, q))
}

func HandleCreateMerchantProductImage(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.ProductImageRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "image URL is required")
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	if position < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "position cannot be negative")
	}
	source, err := storage.ResolveImageSource(req.URL)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	validated, err := storage.ValidateRemoteImage(context.Background(), source, storage.LoadConfig().MaxUploadBytes)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if _, ok := metadata["contentType"]; !ok {
		metadata["contentType"] = validated.ContentType
	}
	if validated.Size >= 0 {
		if _, ok := metadata["size"]; !ok {
			metadata["size"] = validated.Size
		}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "metadata must be valid JSON")
	}
	var image models.ProductImage
	var originalURL, provider, publicID, objectName sql.NullString
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO product_images(product_id,url,source_type,original_url,metadata,position) SELECT $1,$2,$3,$4,$5,$6 WHERE EXISTS(SELECT 1 FROM products WHERE id=$1 AND merchant_id=$7) RETURNING id,product_id,url,source_type,original_url,storage_provider,storage_public_id,storage_object_name,metadata,position,created_at`, c.Params("productId"), source.ResolvedURL, source.Kind, source.OriginalURL, metadataJSON, position, merchantID).Scan(&image.ID, &image.ProductID, &image.URL, &image.SourceType, &originalURL, &provider, &publicID, &objectName, &metadataJSON, &image.Position, &image.CreatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "product not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create product image")
	}
	if originalURL.Valid {
		image.OriginalURL = &originalURL.String
	}
	if provider.Valid {
		image.StorageProvider = &provider.String
	}
	if publicID.Valid {
		image.StoragePublicID = &publicID.String
	}
	if objectName.Valid {
		image.StorageObjectName = &objectName.String
	}
	image.Metadata = metadata
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": image})
}

func HandleUploadMerchantProductImage(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	productID := strings.TrimSpace(c.Params("productId"))
	var productExists bool
	if err := database.GetDB().QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND merchant_id=$2)", productID, merchantID).Scan(&productExists); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to validate product")
	}
	if !productExists {
		return fiber.NewError(fiber.StatusNotFound, "product not found")
	}
	file, err := c.FormFile("image")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "multipart image field is required")
	}
	maxSize := storage.LoadConfig().MaxUploadBytes
	if file.Size <= 0 || file.Size > maxSize {
		return fiber.NewError(fiber.StatusBadRequest, "image size is invalid or exceeds the configured limit")
	}
	reader, err := file.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "failed to open uploaded image")
	}
	defer reader.Close()
	header := make([]byte, 512)
	n, readErr := reader.Read(header)
	if readErr != nil && readErr != io.EOF {
		return fiber.NewError(fiber.StatusBadRequest, "failed to inspect uploaded image")
	}
	contentType := http.DetectContentType(header[:n])
	if !strings.HasPrefix(contentType, "image/") {
		return fiber.NewError(fiber.StatusBadRequest, "uploaded file must be an image")
	}
	if seeker, ok := reader.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "failed to read uploaded image")
		}
	} else {
		return fiber.NewError(fiber.StatusBadRequest, "uploaded image cannot be rewound")
	}
	provider, err := storage.NewFromEnv()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	}
	object, err := provider.Upload(context.Background(), storage.UploadInput{Reader: reader, Size: file.Size, Filename: file.Filename, ContentType: contentType, Folder: "products/" + productID})
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "image storage upload failed")
	}
	metadata := map[string]interface{}{"originalFilename": file.Filename, "contentType": contentType, "size": file.Size}
	metadataJSON, _ := json.Marshal(metadata)
	position := c.QueryInt("position", 0)
	var image models.ProductImage
	var originalURL, storageProvider, publicID, objectName sql.NullString
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO product_images(product_id,url,source_type,original_url,storage_provider,storage_public_id,storage_object_name,metadata,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,product_id,url,source_type,original_url,storage_provider,storage_public_id,storage_object_name,metadata,position,created_at`, productID, object.PublicURL, object.Provider, nil, object.Provider, nullableStringValue(&object.PublicID), nullableStringValue(&object.ObjectName), metadataJSON, position).Scan(&image.ID, &image.ProductID, &image.URL, &image.SourceType, &originalURL, &storageProvider, &publicID, &objectName, &metadataJSON, &image.Position, &image.CreatedAt)
	if err != nil {
		_ = provider.Delete(context.Background(), object)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to save product image")
	}
	if storageProvider.Valid {
		image.StorageProvider = &storageProvider.String
	}
	if publicID.Valid {
		image.StoragePublicID = &publicID.String
	}
	if objectName.Valid {
		image.StorageObjectName = &objectName.String
	}
	image.Metadata = metadata
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": image})
}

func HandleDeleteMerchantProductImage(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM product_images i USING products p WHERE i.id=$1 AND i.product_id=p.id AND p.merchant_id=$2`, c.Params("imageId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete product image")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "product image not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

var validAttributeValueTypes = map[string]bool{"TEXT": true, "NUMBER": true, "BOOLEAN": true, "SELECT": true, "DATE": true, "JSON": true}

func scanAttributeDefinition(scan func(...interface{}) error) (models.AttributeDefinition, error) {
	var d models.AttributeDefinition
	var description sql.NullString
	err := scan(&d.ID, &d.MerchantID, &d.Code, &d.Name, &d.ValueType, &description, &d.CreatedAt, &d.UpdatedAt)
	if description.Valid {
		d.Description = &description.String
	}
	return d, err
}

func HandleListMerchantAttributeDefinitions(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "code", attributeSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	if q.Search != "" {
		where += " AND (code ILIKE $2 OR name ILIKE $2 OR COALESCE(description,'') ILIKE $2)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	var total int64
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM attribute_definitions"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count attribute definitions")
	}
	rows, err := db.Query(context.Background(), "SELECT id,merchant_id,code,name,value_type,description,created_at,updated_at FROM attribute_definitions"+where+q.orderBy()+" LIMIT $3 OFFSET $4", append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list attribute definitions")
	}
	defer rows.Close()
	items := make([]models.AttributeDefinition, 0)
	for rows.Next() {
		d, scanErr := scanAttributeDefinition(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read attribute definition")
		}
		items = append(items, d)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateMerchantAttributeDefinition(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.AttributeDefinitionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.ValueType = strings.ToUpper(strings.TrimSpace(req.ValueType))
	if req.Code == "" || strings.TrimSpace(req.Name) == "" || !validAttributeValueTypes[req.ValueType] {
		return fiber.NewError(fiber.StatusBadRequest, "code, name, and a valid valueType are required")
	}
	var d models.AttributeDefinition
	db := database.GetDB()
	err = db.QueryRow(context.Background(), `INSERT INTO attribute_definitions(merchant_id,code,name,value_type,description) VALUES($1,$2,$3,$4,$5) RETURNING id,merchant_id,code,name,value_type,description,created_at,updated_at`, merchantID, req.Code, strings.TrimSpace(req.Name), req.ValueType, req.Description).Scan(&d.ID, &d.MerchantID, &d.Code, &d.Name, &d.ValueType, &d.Description, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "attribute code already exists for this merchant")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create attribute definition")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": d})
}

func HandleGetMerchantAttributeDefinition(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	d, err := scanAttributeDefinition(database.GetDB().QueryRow(context.Background(), `SELECT id,merchant_id,code,name,value_type,description,created_at,updated_at FROM attribute_definitions WHERE id=$1 AND merchant_id=$2`, c.Params("definitionId"), merchantID).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "attribute definition not found")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to retrieve attribute definition")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": d})
}

func HandleUpdateMerchantAttributeDefinition(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.AttributeDefinitionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	req.ValueType = strings.ToUpper(strings.TrimSpace(req.ValueType))
	if req.Code == "" || strings.TrimSpace(req.Name) == "" || !validAttributeValueTypes[req.ValueType] {
		return fiber.NewError(fiber.StatusBadRequest, "code, name, and a valid valueType are required")
	}
	var d models.AttributeDefinition
	err = database.GetDB().QueryRow(context.Background(), `UPDATE attribute_definitions SET code=$1,name=$2,value_type=$3,description=$4,updated_at=NOW() WHERE id=$5 AND merchant_id=$6 RETURNING id,merchant_id,code,name,value_type,description,created_at,updated_at`, req.Code, strings.TrimSpace(req.Name), req.ValueType, req.Description, c.Params("definitionId"), merchantID).Scan(&d.ID, &d.MerchantID, &d.Code, &d.Name, &d.ValueType, &d.Description, &d.CreatedAt, &d.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "attribute definition not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "attribute code already exists for this merchant")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update attribute definition")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": d})
}

func HandleDeleteMerchantAttributeDefinition(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM attribute_definitions WHERE id=$1 AND merchant_id=$2`, c.Params("definitionId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusConflict, "attribute definition cannot be deleted because it is in use")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "attribute definition not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func HandleListMerchantAttributeOptions(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "position", optionSortFields)
	where := " WHERE o.definition_id=$1 AND d.merchant_id=$2"
	args := []interface{}{c.Params("definitionId"), merchantID}
	if q.Search != "" {
		where += " AND (o.value ILIKE $3 OR o.label ILIKE $3)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	var total int64
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM attribute_definition_options o JOIN attribute_definitions d ON d.id=o.definition_id"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count attribute options")
	}
	rows, err := db.Query(context.Background(), "SELECT o.id,o.definition_id,o.value,o.label,o.position FROM attribute_definition_options o JOIN attribute_definitions d ON d.id=o.definition_id"+where+q.orderBy()+" LIMIT $4 OFFSET $5", append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list attribute options")
	}
	defer rows.Close()
	items := make([]models.AttributeOption, 0)
	for rows.Next() {
		var item models.AttributeOption
		if err := rows.Scan(&item.ID, &item.DefinitionID, &item.Value, &item.Label, &item.Position); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read attribute option")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateMerchantAttributeOption(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.AttributeOptionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Value) == "" || strings.TrimSpace(req.Label) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "value and label are required")
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	var item models.AttributeOption
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO attribute_definition_options(definition_id,value,label,position) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM attribute_definitions WHERE id=$1 AND merchant_id=$5) RETURNING id,definition_id,value,label,position`, c.Params("definitionId"), strings.TrimSpace(req.Value), strings.TrimSpace(req.Label), position, merchantID).Scan(&item.ID, &item.DefinitionID, &item.Value, &item.Label, &item.Position)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "attribute definition not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "attribute option value already exists")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create attribute option")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateMerchantAttributeOption(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.AttributeOptionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Value) == "" || strings.TrimSpace(req.Label) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "value and label are required")
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	}
	var item models.AttributeOption
	err = database.GetDB().QueryRow(context.Background(), `UPDATE attribute_definition_options o SET value=$1,label=$2,position=$3 WHERE o.id=$4 AND EXISTS(SELECT 1 FROM attribute_definitions d WHERE d.id=o.definition_id AND d.merchant_id=$5) RETURNING o.id,o.definition_id,o.value,o.label,o.position`, strings.TrimSpace(req.Value), strings.TrimSpace(req.Label), position, c.Params("optionId"), merchantID).Scan(&item.ID, &item.DefinitionID, &item.Value, &item.Label, &item.Position)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "attribute option not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "attribute option value already exists")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update attribute option")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteMerchantAttributeOption(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM attribute_definition_options o WHERE o.id=$1 AND EXISTS(SELECT 1 FROM attribute_definitions d WHERE d.id=o.definition_id AND d.merchant_id=$2)`, c.Params("optionId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusConflict, "attribute option cannot be deleted because it is in use")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "attribute option not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func scanProductAttributeAssignment(scan func(...interface{}) error) (models.ProductAttributeAssignment, error) {
	var item models.ProductAttributeAssignment
	var variantID, valueText sql.NullString
	var valueNumber sql.NullFloat64
	var valueBoolean sql.NullBool
	var valueJSON []byte
	err := scan(&item.ID, &item.DefinitionID, &item.ProductID, &variantID, &valueText, &valueNumber, &valueBoolean, &valueJSON, &item.CreatedAt, &item.UpdatedAt)
	if variantID.Valid {
		item.VariantID = &variantID.String
	}
	if valueText.Valid {
		item.ValueText = &valueText.String
	}
	if valueNumber.Valid {
		item.ValueNumber = &valueNumber.Float64
	}
	if valueBoolean.Valid {
		item.ValueBoolean = &valueBoolean.Bool
	}
	if len(valueJSON) > 0 {
		item.ValueJSON = json.RawMessage(valueJSON)
	}
	return item, err
}

func validateAssignment(req models.ProductAttributeAssignmentRequest) error {
	if strings.TrimSpace(req.DefinitionID) == "" {
		return errors.New("definitionId is required")
	}
	if len(req.ValueJSON) > 0 && !json.Valid(req.ValueJSON) {
		return errors.New("valueJson must be valid JSON")
	}
	if req.ValueText == nil && req.ValueNumber == nil && req.ValueBoolean == nil && len(req.ValueJSON) == 0 {
		return errors.New("one attribute value is required")
	}
	return nil
}

func assignmentArgs(req models.ProductAttributeAssignmentRequest) []interface{} {
	return []interface{}{req.DefinitionID, nullableStringValue(req.VariantID), req.ValueText, req.ValueNumber, req.ValueBoolean, nullableJSON(req.ValueJSON)}
}

func nullableJSON(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func HandleListMerchantProductAttributeAssignments(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	productID := strings.TrimSpace(c.Params("productId"))
	q := getCatalogListQuery(c, "createdAt", assignmentSortFields)
	where := " WHERE a.product_id=$1 AND d.merchant_id=$2"
	args := []interface{}{productID, merchantID}
	if variantID := strings.TrimSpace(c.Query("variantId")); variantID != "" {
		where += " AND a.variant_id=$3"
		args = append(args, variantID)
	}
	db := database.GetDB()
	var total int64
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM product_attribute_assignments a JOIN attribute_definitions d ON d.id=a.definition_id"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count attribute assignments")
	}
	rows, err := db.Query(context.Background(), "SELECT a.id,a.definition_id,a.product_id,a.variant_id,a.value_text,a.value_number,a.value_boolean,a.value_json,a.created_at,a.updated_at FROM product_attribute_assignments a JOIN attribute_definitions d ON d.id=a.definition_id"+where+q.orderBy()+" LIMIT $"+strconv.Itoa(len(args)+1)+" OFFSET $"+strconv.Itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list attribute assignments")
	}
	defer rows.Close()
	items := make([]models.ProductAttributeAssignment, 0)
	for rows.Next() {
		item, scanErr := scanProductAttributeAssignment(rows.Scan)
		if scanErr != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read attribute assignment")
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateMerchantProductAttributeAssignment(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.ProductAttributeAssignmentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := validateAssignment(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	productID := strings.TrimSpace(c.Params("productId"))
	args := assignmentArgs(req)
	args = append(args, productID, merchantID)
	var item models.ProductAttributeAssignment
	item, err = scanProductAttributeAssignment(database.GetDB().QueryRow(context.Background(), `INSERT INTO product_attribute_assignments(definition_id,variant_id,product_id,value_text,value_number,value_boolean,value_json) SELECT $1,$2,$7,$3,$4,$5,$6 WHERE EXISTS(SELECT 1 FROM attribute_definitions WHERE id=$1 AND merchant_id=$8) AND EXISTS(SELECT 1 FROM products WHERE id=$7 AND merchant_id=$8) AND ($2::uuid IS NULL OR EXISTS(SELECT 1 FROM product_variants WHERE id=$2 AND product_id=$7 AND merchant_id=$8)) RETURNING id,definition_id,product_id,variant_id,value_text,value_number,value_boolean,value_json,created_at,updated_at`, args...).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusBadRequest, "invalid product, definition, or variant ownership")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create attribute assignment")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateMerchantProductAttributeAssignment(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.ProductAttributeAssignmentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := validateAssignment(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	args := assignmentArgs(req)
	args = append(args, c.Params("assignmentId"), merchantID)
	item, err := scanProductAttributeAssignment(database.GetDB().QueryRow(context.Background(), `UPDATE product_attribute_assignments a SET definition_id=$1,variant_id=$2,value_text=$3,value_number=$4,value_boolean=$5,value_json=$6,updated_at=NOW() WHERE a.id=$7 AND EXISTS(SELECT 1 FROM attribute_definitions d WHERE d.id=$1 AND d.merchant_id=$8) AND EXISTS(SELECT 1 FROM products p WHERE p.id=a.product_id AND p.merchant_id=$8) AND ($2::uuid IS NULL OR EXISTS(SELECT 1 FROM product_variants v WHERE v.id=$2 AND v.product_id=a.product_id AND v.merchant_id=$8)) RETURNING a.id,a.definition_id,a.product_id,a.variant_id,a.value_text,a.value_number,a.value_boolean,a.value_json,a.created_at,a.updated_at`, args...).Scan)
	if err == pgx.ErrNoRows {
		return fiber.NewError(fiber.StatusNotFound, "attribute assignment not found or ownership is invalid")
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update attribute assignment")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteMerchantProductAttributeAssignment(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM product_attribute_assignments a USING products p WHERE a.id=$1 AND a.product_id=p.id AND p.merchant_id=$2`, c.Params("assignmentId"), merchantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete attribute assignment")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(fiber.StatusNotFound, "attribute assignment not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
