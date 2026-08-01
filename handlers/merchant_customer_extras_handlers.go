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

var customerTagSortFields = map[string]string{"name": "name"}

func HandleListCustomerTags(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	q := getCatalogListQuery(c, "name", customerTagSortFields)
	where := " WHERE merchant_id=$1"
	args := []interface{}{merchantID}
	if q.Search != "" {
		where += " AND (name ILIKE $2 OR COALESCE(color,'') ILIKE $2)"
		args = append(args, "%"+q.Search+"%")
	}
	db := database.GetDB()
	ctx := context.Background()
	var total int64
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM customer_tags"+where, args...).Scan(&total); err != nil {
		return fiber.NewError(500, "failed to count customer tags")
	}
	rows, err := db.Query(ctx, "SELECT id,merchant_id,name,color FROM customer_tags"+where+q.orderBy()+" LIMIT $"+itoa(len(args)+1)+" OFFSET $"+itoa(len(args)+2), append(args, q.PageSize, q.Offset)...)
	if err != nil {
		return fiber.NewError(500, "failed to list customer tags")
	}
	defer rows.Close()
	items := make([]models.CustomerTag, 0)
	for rows.Next() {
		var item models.CustomerTag
		var color sql.NullString
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &color); err != nil {
			return fiber.NewError(500, "failed to read customer tag")
		}
		if color.Valid {
			item.Color = &color.String
		}
		items = append(items, item)
	}
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateCustomerTag(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.CustomerTagRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "name is required")
	}
	var item models.CustomerTag
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO customer_tags(merchant_id,name,color) VALUES($1,$2,$3) RETURNING id,merchant_id,name,color`, merchantID, strings.TrimSpace(req.Name), nullableStringValue(req.Color)).Scan(&item.ID, &item.MerchantID, &item.Name, &item.Color)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "customer tag already exists")
		}
		return fiber.NewError(500, "failed to create customer tag")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleUpdateCustomerTag(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	var req models.CustomerTagRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "name is required")
	}
	var item models.CustomerTag
	err = database.GetDB().QueryRow(context.Background(), `UPDATE customer_tags SET name=$1,color=$2 WHERE id=$3 AND merchant_id=$4 RETURNING id,merchant_id,name,color`, strings.TrimSpace(req.Name), nullableStringValue(req.Color), c.Params("tagId"), merchantID).Scan(&item.ID, &item.MerchantID, &item.Name, &item.Color)
	if err == pgx.ErrNoRows {
		return fiber.NewError(404, "customer tag not found")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "customer tag already exists")
		}
		return fiber.NewError(500, "failed to update customer tag")
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteCustomerTag(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM customer_tags WHERE id=$1 AND merchant_id=$2`, c.Params("tagId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete customer tag")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "customer tag not found")
	}
	return c.SendStatus(204)
}

func customerOwned(ctx context.Context, customerID, merchantID string) (bool, error) {
	var ok bool
	err := database.GetDB().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shop_customers WHERE id=$1 AND merchant_id=$2)`, customerID, merchantID).Scan(&ok)
	return ok, err
}

func HandleListCustomerTagAssignments(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	rows, err := database.GetDB().Query(context.Background(), `SELECT t.id,t.merchant_id,t.name,t.color FROM customer_tags t JOIN customer_tag_map m ON m.tag_id=t.id WHERE m.customer_id=$1 AND t.merchant_id=$2 ORDER BY t.name`, c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to list customer tags")
	}
	defer rows.Close()
	items := make([]models.CustomerTag, 0)
	for rows.Next() {
		var item models.CustomerTag
		var color sql.NullString
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &color); err != nil {
			return fiber.NewError(500, "failed to read customer tag")
		}
		if color.Valid {
			item.Color = &color.String
		}
		items = append(items, item)
	}
	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items})
}

func HandleAssignCustomerTag(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	var tagOwned bool
	if err := database.GetDB().QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM customer_tags WHERE id=$1 AND merchant_id=$2)`, c.Params("tagId"), merchantID).Scan(&tagOwned); err != nil {
		return fiber.NewError(500, "failed to validate tag")
	}
	if !tagOwned {
		return fiber.NewError(404, "customer tag not found")
	}
	if _, err := database.GetDB().Exec(context.Background(), `INSERT INTO customer_tag_map(customer_id,tag_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, c.Params("customerId"), c.Params("tagId")); err != nil {
		return fiber.NewError(500, "failed to assign customer tag")
	}
	return c.SendStatus(204)
}

func HandleUnassignCustomerTag(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM customer_tag_map m USING shop_customers c,customer_tags t WHERE m.customer_id=$1 AND m.tag_id=$2 AND c.id=m.customer_id AND t.id=m.tag_id AND c.merchant_id=$3 AND t.merchant_id=$3`, c.Params("customerId"), c.Params("tagId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to remove customer tag")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "customer tag assignment not found")
	}
	return c.SendStatus(204)
}

func HandleListCustomerNotes(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	q := getCatalogListQuery(c, "createdAt", map[string]string{"createdAt": "created_at"})
	rows, err := database.GetDB().Query(context.Background(), `SELECT n.id,n.customer_id,n.created_by,n.content,n.created_at FROM customer_notes n JOIN shop_customers c ON c.id=n.customer_id WHERE n.customer_id=$1 AND c.merchant_id=$2 ORDER BY n.created_at DESC LIMIT $3 OFFSET $4`, c.Params("customerId"), merchantID, q.PageSize, q.Offset)
	if err != nil {
		return fiber.NewError(500, "failed to list customer notes")
	}
	defer rows.Close()
	items := make([]models.CustomerNote, 0)
	for rows.Next() {
		var item models.CustomerNote
		var createdBy sql.NullString
		if err := rows.Scan(&item.ID, &item.CustomerID, &createdBy, &item.Content, &item.CreatedAt); err != nil {
			return fiber.NewError(500, "failed to read customer note")
		}
		if createdBy.Valid {
			item.CreatedBy = &createdBy.String
		}
		items = append(items, item)
	}
	var total int64
	_ = database.GetDB().QueryRow(context.Background(), `SELECT COUNT(*) FROM customer_notes n JOIN shop_customers c ON c.id=n.customer_id WHERE n.customer_id=$1 AND c.merchant_id=$2`, c.Params("customerId"), merchantID).Scan(&total)
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateCustomerNote(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	var req models.CustomerNoteRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		return fiber.NewError(400, "content is required")
	}
	var item models.CustomerNote
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO customer_notes(customer_id,created_by,content) VALUES($1,$2,$3) RETURNING id,customer_id,created_by,content,created_at`, c.Params("customerId"), merchantID, strings.TrimSpace(req.Content)).Scan(&item.ID, &item.CustomerID, &item.CreatedBy, &item.Content, &item.CreatedAt)
	if err != nil {
		return fiber.NewError(500, "failed to create customer note")
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}

func HandleDeleteCustomerNote(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	result, err := database.GetDB().Exec(context.Background(), `DELETE FROM customer_notes n USING shop_customers c WHERE n.id=$1 AND c.id=n.customer_id AND c.merchant_id=$2`, c.Params("noteId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to delete customer note")
	}
	if result.RowsAffected() == 0 {
		return fiber.NewError(404, "customer note not found")
	}
	return c.SendStatus(204)
}

func HandleListCustomerActivities(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	q := getCatalogListQuery(c, "createdAt", map[string]string{"createdAt": "created_at"})
	rows, err := database.GetDB().Query(context.Background(), `SELECT a.id,a.customer_id,a.event_key,a.activity_type,a.description,a.metadata,a.created_at FROM customer_activities a JOIN shop_customers c ON c.id=a.customer_id WHERE a.customer_id=$1 AND c.merchant_id=$2 ORDER BY a.created_at DESC LIMIT $3 OFFSET $4`, c.Params("customerId"), merchantID, q.PageSize, q.Offset)
	if err != nil {
		return fiber.NewError(500, "failed to list customer activities")
	}
	defer rows.Close()
	items := make([]models.CustomerActivity, 0)
	for rows.Next() {
		var item models.CustomerActivity
		var eventKey, description sql.NullString
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.CustomerID, &eventKey, &item.ActivityType, &description, &metadata, &item.CreatedAt); err != nil {
			return fiber.NewError(500, "failed to read customer activity")
		}
		if eventKey.Valid {
			item.EventKey = &eventKey.String
		}
		if description.Valid {
			item.Description = &description.String
		}
		item.Metadata = map[string]interface{}{}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		items = append(items, item)
	}
	var total int64
	_ = database.GetDB().QueryRow(context.Background(), `SELECT COUNT(*) FROM customer_activities a JOIN shop_customers c ON c.id=a.customer_id WHERE a.customer_id=$1 AND c.merchant_id=$2`, c.Params("customerId"), merchantID).Scan(&total)
	return c.JSON(paginatedResponse(items, total, q))
}

func HandleCreateCustomerActivity(c *fiber.Ctx) error {
	merchantID, err := getMerchantIDFromClaims(c)
	if err != nil {
		return err
	}
	ok, err := customerOwned(context.Background(), c.Params("customerId"), merchantID)
	if err != nil {
		return fiber.NewError(500, "failed to validate customer")
	}
	if !ok {
		return fiber.NewError(404, "customer not found")
	}
	var req models.CustomerActivityRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.ActivityType) == "" {
		return fiber.NewError(400, "activityType is required")
	}
	metadata, _ := json.Marshal(req.Metadata)
	var item models.CustomerActivity
	err = database.GetDB().QueryRow(context.Background(), `INSERT INTO customer_activities(customer_id,event_key,activity_type,description,metadata) VALUES($1,$2,$3,$4,$5) RETURNING id,customer_id,event_key,activity_type,description,metadata,created_at`, c.Params("customerId"), nullableStringValue(req.EventKey), strings.TrimSpace(req.ActivityType), req.Description, metadata).Scan(&item.ID, &item.CustomerID, &item.EventKey, &item.ActivityType, &item.Description, &metadata, &item.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return duplicateResponse(c, "eventKey already exists")
		}
		return fiber.NewError(500, "failed to create customer activity")
	}
	item.Metadata = req.Metadata
	return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "data": item})
}
