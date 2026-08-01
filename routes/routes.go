package routes

import (
	"app/handlers"
	"app/middleware"

	"github.com/gofiber/fiber/v2"
	"time"
)

// SetupRoutes defines all the routes for the application.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")
	api.Get("/health", handlers.HandleHealth)

	// --- System Initialization ---
	api.Post("/init/admin", handlers.HandleInitializeAdmin)

	// --- Authentication Routes ---
	auth := api.Group("/auth", middleware.RateLimit(30, time.Minute))
	auth.Post("/login", handlers.HandleLogin)
	auth.Post("/shop-login", handlers.HandleShopLogin)
	auth.Post("/refresh", handlers.HandleRefresh)
	// Public merchant signup (no JWT required)
	auth.Post("/signup", handlers.HandleMerchantSignup)

	// --- Admin Routes ---
	admin := api.Group("/admin", middleware.JWTMiddleware, middleware.AdminRequired)

	// Admin Profile
	admin.Get("/profile", handlers.HandleGetAdminProfile)
	admin.Put("/profile", handlers.HandleUpdateAdminProfile)

	// Dashboard
	admin.Get("/dashboard/summary", handlers.HandleGetAdminDashboardSummary)

	// User Management (Staff, Admins)
	adminUsers := admin.Group("/users")
	adminUsers.Get("/merchants-for-selection", handlers.HandleGetMerchantsForSelection) // Must be before /:userId
	adminUsers.Post("/", handlers.HandleCreateUser)
	adminUsers.Get("/", handlers.HandleListUsers)
	adminUsers.Get("/:userId", handlers.HandleGetUserByID)
	adminUsers.Put("/:userId", handlers.HandleUpdateUser)
	adminUsers.Delete("/:userId", handlers.HandleHardDeleteUser) // Corrected path

	// Specific Admin-related routes
	admin.Get("/admins", handlers.HandleGetAdmins)
	admin.Get("/staff", handlers.HandleGetAllStaff)

	// Merchant Management (as seen by Admin)
	admin.Get("/merchants", handlers.HandleListMerchants)
	admin.Get("/merchants/:merchantIdOrUserId", handlers.HandleGetMerchantByID)

	// Admin catalog management
	adminCatalog := admin.Group("/catalog")
	adminCatalog.Get("/categories", handlers.HandleListAdminCategories)
	adminCatalog.Post("/categories", handlers.HandleCreateAdminCategory)
	adminCatalog.Put("/categories/:categoryId", handlers.HandleUpdateAdminCategory)
	adminCatalog.Delete("/categories/:categoryId", handlers.HandleDeleteAdminCategory)
	adminCatalog.Get("/subcategories", handlers.HandleListAdminSubcategories)
	adminCatalog.Post("/subcategories", handlers.HandleCreateAdminSubcategory)
	adminCatalog.Put("/subcategories/:subcategoryId", handlers.HandleUpdateAdminSubcategory)
	adminCatalog.Delete("/subcategories/:subcategoryId", handlers.HandleDeleteAdminSubcategory)
	adminCatalog.Get("/brands", handlers.HandleListAdminBrands)
	adminCatalog.Post("/brands", handlers.HandleCreateAdminBrand)
	adminCatalog.Put("/brands/:brandId", handlers.HandleUpdateAdminBrand)
	adminCatalog.Delete("/brands/:brandId", handlers.HandleDeleteAdminBrand)

	// Shop Management (Admin)
	adminShops := admin.Group("/shops")
	adminShops.Get("/", handlers.HandleListShops)
	adminShops.Get("/:shopId", handlers.HandleGetShopByID)
	adminShops.Post("/", handlers.HandleCreateShop)
	adminShops.Put("/:shopId", handlers.HandleUpdateShop)
	adminShops.Put("/:shopId/status", handlers.HandleSetShopActiveStatus)
	adminShops.Delete("/:shopId", handlers.HandleDeleteShop)

	// --- Merchant Routes ---
	merchant := api.Group("/merchant", middleware.JWTMiddleware, middleware.MerchantRequired)
	merchant.Get("/dashboard/summary", handlers.HandleGetMerchantDashboardSummary)

	// AI Assistant
	merchant.Post("/ai-analysis", handlers.HandleAIAssistant)

	// Merchant Profile
	merchant.Get("/profile", handlers.HandleGetMerchantProfile)
	merchant.Put("/profile", handlers.HandleUpdateMerchantProfile)

	// Merchant Shops
	merchantShops := merchant.Group("/shops")
	merchantShops.Get("/", handlers.HandleListMerchantShops)
	merchantShops.Post("/", handlers.HandleCreateShop) // This was already correct
	merchantShops.Put("/:shopId", handlers.HandleUpdateMerchantShop)
	merchantShops.Delete("/:shopId", handlers.HandleDeleteMerchantShop)
	merchantShops.Get("/:shopId/products", handlers.HandleListProductsForShop)
	// Deletion preflight check (does not perform delete) - returns blockers and deletable flag
	merchantShops.Get("/:shopId/delete-check", handlers.HandleCheckDeleteMerchantShop)
	merchantShops.Patch("/:shopId/set-primary", handlers.HandleSetPrimaryShop)
	merchantShops.Get("/:shopId/inventory", handlers.HandleListInventoryForShop)
	merchantShops.Post("/:shopId/stock-in", handlers.HandleShopStockIn)
	merchantShops.Post("/:shopId/inventory/:inventoryItemId/stock-in", handlers.HandleStockInItem)
	merchantShops.Patch("/:shopId/inventory/:inventoryItemId/adjust-stock", handlers.HandleAdjustStockItem)
	merchantShops.Get("/:shopId/sales", handlers.HandleListSalesForShop)

	// New routes for stock adjustment and history
	merchantShops.Post("/:shopId/inventory/:itemId/adjust", handlers.HandleAdjustStock)
	merchantShops.Get("/:shopId/inventory/:itemId/history", handlers.HandleGetStockMovementHistory)

	// Merchant Sales
	merchantSales := merchant.Group("/sales")
	merchantSales.Post("/", handlers.HandleCreateSale)
	merchantSales.Get("/:saleId", handlers.HandleGetSaleByID)
	merchantSales.Get("/:saleId/receipt", handlers.HandleGetReceipt)

	// Merchant Promotions
	promotions := merchant.Group("/promotions")
	promotions.Get("/", handlers.HandleListPromotions)
	promotions.Post("/", handlers.HandleCreatePromotion)
	promotions.Put("/:id", handlers.HandleUpdatePromotion)
	promotions.Delete("/:id", handlers.HandleDeletePromotion)

	// Merchant Reports
	reports := merchant.Group("/reports")
	reports.Get("/sales", handlers.HandleGetSalesReport)
	reports.Get("/sales-forecast", handlers.HandleGetSalesForecast)

	// Merchant Staff
	merchantStaff := merchant.Group("/staff")
	merchantStaff.Get("/", handlers.HandleListMerchantStaff)
	merchantStaff.Post("/", handlers.HandleCreateMerchantStaff)
	merchantStaff.Put("/:staffId", handlers.HandleUpdateMerchantStaff)
	merchantStaff.Get("/:staffId/delete-check", handlers.HandleCheckDeleteMerchantStaff)
	merchantStaff.Delete("/:staffId", handlers.HandleDeleteMerchantStaff)

	// Merchant Stocks
	merchant.Get("/stocks", handlers.HandleGetCombinedStocks)

	// Merchant Notifications
	notifications := merchant.Group("/notifications")
	notifications.Get("/", handlers.HandleGetNotifications)
	notifications.Get("/unread-count", handlers.HandleGetUnreadNotificationCount)
	notifications.Patch("/:notificationId/read", handlers.HandleMarkNotificationAsRead)

	// Merchant Payments
	payments := merchant.Group("/payments")
	payments.Post("/create-intent", handlers.HandleCreatePaymentIntent)

	// Merchant POS
	pos := merchant.Group("/pos")
	pos.Get("/products", handlers.HandleSearchProductsForPOS)
	pos.Get("/promotions", handlers.HandleGetActivePromotionsForPOS)
	pos.Get("/sessions", handlers.HandleListPOSSessions)
	pos.Post("/sessions", handlers.HandleOpenPOSSession)
	pos.Post("/sessions/:sessionId/close", handlers.HandleClosePOSSession)
	pos.Post("/checkout", handlers.HandleCheckout)
	pos.Post("/sync", handlers.HandleSyncOfflineSales)

	customers := merchant.Group("/customers")
	customers.Get("/search", handlers.HandleSearchCustomers)
	customers.Post("/", handlers.HandleCreateCustomer)
	customers.Get("/tags", handlers.HandleListCustomerTags)
	customers.Post("/tags", handlers.HandleCreateCustomerTag)
	customers.Put("/tags/:tagId", handlers.HandleUpdateCustomerTag)
	customers.Delete("/tags/:tagId", handlers.HandleDeleteCustomerTag)
	customers.Get("/:customerId/tags", handlers.HandleListCustomerTagAssignments)
	customers.Post("/:customerId/tags/:tagId", handlers.HandleAssignCustomerTag)
	customers.Delete("/:customerId/tags/:tagId", handlers.HandleUnassignCustomerTag)
	customers.Get("/:customerId/notes", handlers.HandleListCustomerNotes)
	customers.Post("/:customerId/notes", handlers.HandleCreateCustomerNote)
	customers.Delete("/:customerId/notes/:noteId", handlers.HandleDeleteCustomerNote)
	customers.Get("/:customerId/activities", handlers.HandleListCustomerActivities)
	customers.Post("/:customerId/activities", handlers.HandleCreateCustomerActivity)

	suppliers := merchant.Group("/suppliers")
	procurement := merchant.Group("/purchasing")
	procurement.Get("/orders", handlers.HandleListPurchaseOrders)
	procurement.Post("/orders", handlers.HandleCreatePurchaseOrder)
	procurement.Post("/orders/:orderId/receive", handlers.HandleReceivePurchaseOrder)
	accounting := merchant.Group("/accounting")
	accounting.Get("/accounts", handlers.HandleListAccounts)
	accounting.Post("/accounts", handlers.HandleCreateAccount)
	accounting.Post("/journal-entries", handlers.HandleCreateJournalEntry)
	suppliers.Get("/", handlers.HandleListMerchantSuppliers)
	suppliers.Post("/", handlers.HandleCreateNewSupplier)
	suppliers.Get("/:supplierId", handlers.HandleGetSupplierDetails)
	suppliers.Put("/:supplierId", handlers.HandleUpdateExistingSupplier)
	suppliers.Delete("/:supplierId", handlers.HandleDeleteExistingSupplier)

	inventory := merchant.Group("/inventory")
	inventory.Get("/", handlers.HandleListInventoryItems)
	inventory.Post("/", handlers.HandleCreateInventoryItem)
	inventory.Post("/stock-in", handlers.HandleMerchantStockIn)
	inventory.Post("/move-stock", handlers.HandleMoveStock)
	inventory.Get("/measurement-groups", handlers.HandleListMeasurementGroups)
	inventory.Post("/measurement-groups", handlers.HandleCreateMeasurementGroup)
	inventory.Put("/measurement-groups/:groupId", handlers.HandleUpdateMeasurementGroup)
	inventory.Delete("/measurement-groups/:groupId", handlers.HandleDeleteMeasurementGroup)
	inventory.Get("/units", handlers.HandleListUnitDefinitions)
	inventory.Post("/units", handlers.HandleCreateUnitDefinition)
	inventory.Put("/units/:unitId", handlers.HandleUpdateUnitDefinition)
	inventory.Delete("/units/:unitId", handlers.HandleDeleteUnitDefinition)
	inventory.Get("/stock-items/:stockItemId/configuration", handlers.HandleGetStockItemConfiguration)
	inventory.Put("/stock-items/:stockItemId/configuration", handlers.HandleUpsertStockItemConfiguration)
	inventory.Get("/stock-items/:stockItemId/units", handlers.HandleListStockItemUnits)
	inventory.Post("/stock-items/:stockItemId/units", handlers.HandleCreateStockItemUnit)
	inventory.Put("/stock-units/:stockUnitId", handlers.HandleUpdateStockItemUnit)
	inventory.Delete("/stock-units/:stockUnitId", handlers.HandleDeleteStockItemUnit)
	inventory.Get("/stock-items/:stockItemId/unit-conversions", handlers.HandleListStockItemUnitConversions)
	inventory.Post("/stock-items/:stockItemId/unit-conversions", handlers.HandleCreateStockItemUnitConversion)
	inventory.Delete("/unit-conversions/:conversionId", handlers.HandleDeleteStockItemUnitConversion)
	inventory.Get("/identifier-types", handlers.HandleListInventoryIdentifierTypes)
	inventory.Post("/identifier-types", handlers.HandleCreateInventoryIdentifierType)
	inventory.Put("/identifier-types/:identifierTypeId", handlers.HandleUpdateInventoryIdentifierType)
	inventory.Delete("/identifier-types/:identifierTypeId", handlers.HandleDeleteInventoryIdentifierType)
	inventory.Get("/barcodes", handlers.HandleListMerchantBarcodes)
	inventory.Post("/barcodes", handlers.HandleCreateMerchantBarcode)
	inventory.Put("/barcodes/:barcodeId", handlers.HandleUpdateMerchantBarcode)
	inventory.Delete("/barcodes/:barcodeId", handlers.HandleDeleteMerchantBarcode)
	inventory.Get("/batches", handlers.HandleListInventoryBatches)
	inventory.Post("/:inventoryItemId/batches", handlers.HandleCreateInventoryBatch)
	inventory.Put("/batches/:batchId", handlers.HandleUpdateInventoryBatch)
	inventory.Delete("/batches/:batchId", handlers.HandleDeleteInventoryBatch)
	inventory.Get("/reservations", handlers.HandleListInventoryReservations)
	inventory.Post("/:inventoryItemId/reservations", handlers.HandleCreateInventoryReservation)
	inventory.Patch("/reservations/:reservationId/release", handlers.HandleReleaseInventoryReservation)
	inventory.Get("/serials", handlers.HandleListInventorySerials)
	inventory.Post("/:inventoryItemId/serials", handlers.HandleCreateInventorySerial)
	inventory.Put("/serials/:serialId", handlers.HandleUpdateInventorySerial)
	inventory.Delete("/serials/:serialId", handlers.HandleDeleteInventorySerial)
	inventory.Get("/assets", handlers.HandleListInventoryAssets)
	inventory.Post("/:inventoryItemId/assets", handlers.HandleCreateInventoryAsset)
	inventory.Put("/assets/:assetId", handlers.HandleUpdateInventoryAsset)
	inventory.Delete("/assets/:assetId", handlers.HandleDeleteInventoryAsset)
	inventory.Get("/assets/:assetId/identifiers", handlers.HandleListInventoryAssetIdentifiers)
	inventory.Post("/assets/:assetId/identifiers", handlers.HandleCreateInventoryAssetIdentifier)
	inventory.Delete("/asset-identifiers/:identifierId", handlers.HandleDeleteInventoryAssetIdentifier)
	inventory.Get("/:itemId", handlers.HandleGetInventoryItemByID)
	inventory.Put("/:itemId", handlers.HandleUpdateInventoryItem)
	inventory.Delete("/:itemId", handlers.HandleDeleteInventoryItem)
	inventory.Get("/:itemId/delete-check", handlers.HandleCheckDeleteInventoryItem)
	inventory.Patch("/:itemId/archive", handlers.HandleArchiveInventoryItem)
	inventory.Patch("/:itemId/unarchive", handlers.HandleUnarchiveInventoryItem)

	// Merchant catalog management (categories, subcategories, brands)
	catalog := merchant.Group("/catalog")
	catalog.Get("/options", handlers.HandleGetMerchantCatalogOptions)
	catalog.Get("/categories", handlers.HandleListMerchantCategories)
	catalog.Post("/categories", handlers.HandleCreateMerchantCategory)
	catalog.Put("/categories/:categoryId", handlers.HandleUpdateMerchantCategory)
	catalog.Delete("/categories/:categoryId", handlers.HandleDeleteMerchantCategory)
	catalog.Get("/products/:productId/variants", handlers.HandleListMerchantVariants)
	catalog.Post("/products/:productId/variants", handlers.HandleCreateMerchantVariant)
	catalog.Get("/variants/:variantId", handlers.HandleGetMerchantVariant)
	catalog.Put("/variants/:variantId", handlers.HandleUpdateMerchantVariant)
	catalog.Patch("/variants/:variantId/archive", handlers.HandleArchiveMerchantVariant)
	catalog.Patch("/variants/:variantId/restore", handlers.HandleRestoreMerchantVariant)
	catalog.Get("/products/:productId/images", handlers.HandleListMerchantProductImages)
	catalog.Post("/products/:productId/images", handlers.HandleCreateMerchantProductImage)
	catalog.Post("/products/:productId/images/upload", handlers.HandleUploadMerchantProductImage)
	catalog.Delete("/images/:imageId", handlers.HandleDeleteMerchantProductImage)
	catalog.Get("/attributes", handlers.HandleListMerchantAttributeDefinitions)
	catalog.Post("/attributes", handlers.HandleCreateMerchantAttributeDefinition)
	catalog.Get("/attributes/:definitionId", handlers.HandleGetMerchantAttributeDefinition)
	catalog.Put("/attributes/:definitionId", handlers.HandleUpdateMerchantAttributeDefinition)
	catalog.Delete("/attributes/:definitionId", handlers.HandleDeleteMerchantAttributeDefinition)
	catalog.Get("/attributes/:definitionId/options", handlers.HandleListMerchantAttributeOptions)
	catalog.Post("/attributes/:definitionId/options", handlers.HandleCreateMerchantAttributeOption)
	catalog.Put("/attribute-options/:optionId", handlers.HandleUpdateMerchantAttributeOption)
	catalog.Delete("/attribute-options/:optionId", handlers.HandleDeleteMerchantAttributeOption)
	catalog.Get("/products/:productId/attributes", handlers.HandleListMerchantProductAttributeAssignments)
	catalog.Post("/products/:productId/attributes", handlers.HandleCreateMerchantProductAttributeAssignment)
	catalog.Put("/attribute-assignments/:assignmentId", handlers.HandleUpdateMerchantProductAttributeAssignment)
	catalog.Delete("/attribute-assignments/:assignmentId", handlers.HandleDeleteMerchantProductAttributeAssignment)
	catalog.Get("/subcategories", handlers.HandleListMerchantSubcategories)
	catalog.Post("/subcategories", handlers.HandleCreateMerchantSubcategory)
	catalog.Put("/subcategories/:subcategoryId", handlers.HandleUpdateMerchantSubcategory)
	catalog.Delete("/subcategories/:subcategoryId", handlers.HandleDeleteMerchantSubcategory)
	catalog.Get("/brands", handlers.HandleListMerchantBrands)
	catalog.Post("/brands", handlers.HandleCreateMerchantBrand)
	catalog.Put("/brands/:brandId", handlers.HandleUpdateMerchantBrand)
	catalog.Delete("/brands/:brandId", handlers.HandleDeleteMerchantBrand)

	// Merchant Invoices
	invoices := merchant.Group("/invoices")
	invoices.Get("/", handlers.HandleListInvoices)
	invoices.Get("/:invoiceId", handlers.HandleGetInvoiceByID)
	invoices.Get("/sale/:saleId", handlers.HandleGetInvoiceBySaleID)

	// --- Staff Routes ---
	staff := api.Group("/staff", middleware.JWTMiddleware, middleware.StaffRequired)
	staff.Get("/dashboard/summary", handlers.HandleGetStaffDashboardSummary)
	staff.Get("/assigned-shop", handlers.HandleGetAssignedShop)
	staff.Get("/profile", handlers.HandleGetStaffProfile)
	staff.Get("/salary", handlers.HandleGetSalaryHistory)

	// --- Staff POS Routes ---
	staffPOS := staff.Group("/pos")
	staffPOS.Get("/products", handlers.HandleSearchProductsForStaff)
	staffPOS.Get("/promotions", handlers.HandleGetActivePromotionsForStaff)
	staffPOS.Post("/checkout", handlers.HandleStaffCheckout)

	// --- Staff Items Routes ---
	staffItems := staff.Group("/items")
	staffItems.Get("/", handlers.HandleGetStaffItems)
	staffInvoices := staff.Group("/invoices")
	staffInvoices.Get("/", handlers.HandleListStaffInvoices)
	staffInvoices.Get("/:invoiceId", handlers.HandleGetStaffInvoiceByID)

	// --- Shop Routes ---
	// Shop routes are accessible by both merchants (with shopId param) and staff (with assigned shop)
	shop := api.Group("/shop", middleware.JWTMiddleware)
	shop.Get("/dashboard/summary", handlers.HandleGetShopDashboardSummary)
	shop.Get("/profile", handlers.HandleGetShopProfile) // Corrected route

	// New routes for shop inventory management
	shop.Get("/items", handlers.HandleGetShopItems)
	shop.Put("/items/:itemId/stock", handlers.HandleUpdateShopItemStock)
	shop.Get("/inventory", handlers.HandleGetShopInventory)
	shop.Post("/inventory/stock-in", handlers.HandleStockIn)

	// Shop customers routes (accessible by both merchant and staff)
	shopCustomers := shop.Group("/customers")
	shopCustomers.Get("/search", handlers.HandleSearchCustomers)
	shopCustomers.Post("/", handlers.HandleCreateCustomer)

	// Shop sales routes (accessible by both merchant and staff)
	shopSales := shop.Group("/shops/:shopId/sales")
	shopSales.Get("/", handlers.HandleListSalesForShop)

	// Shop invoices (accessible to merchant owners and staff assigned to the shop)
	shopInvoices := shop.Group("/shops/:shopId/invoices")
	shopInvoices.Get("/", handlers.HandleListShopInvoices)
	shopInvoices.Get("/:invoiceId", handlers.HandleGetShopInvoiceByID)

	// Shop support tickets (accessible by merchant owners and assigned staff)
	shopSupport := shop.Group("/support")
	shopSupport.Get("/tickets", handlers.HandleListShopSupportTickets)
	shopSupport.Post("/tickets", handlers.HandleCreateShopSupportTicket)
	shopSupport.Get("/tickets/:ticketId", handlers.HandleGetShopSupportTicketByID)
	shopSupport.Post("/tickets/:ticketId/replies", handlers.HandleReplyShopSupportTicket)
	shopSupport.Patch("/tickets/:ticketId/status", handlers.HandleUpdateShopSupportTicketStatus)

	// --- Dashboard-only shop endpoints (infer shop from auth for staff dashboard) ---
	dashboard := api.Group("/dashboard", middleware.JWTMiddleware)
	dashboardShop := dashboard.Group("/shop")
	dashboardShop.Get("/sales", handlers.HandleDashboardListSales)
	dashboardShop.Get("/items", handlers.HandleDashboardGetItems)
	dashboardShop.Get("/customers/search", handlers.HandleDashboardSearchCustomers)

	// --- Shop POS Routes ---
	shopPOS := shop.Group("/pos")
	shopPOS.Get("/:shopId/products", handlers.HandleSearchShopProducts)
	shopPOS.Get("/promotions", handlers.HandleGetActivePromotionsForShop)
	shopPOS.Post("/:shopId/checkout", handlers.HandleShopCheckout)

	// --- Gemini Routes ---
	gemini := api.Group("/gemini", middleware.JWTMiddleware, middleware.MerchantRequired, middleware.RateLimit(20, time.Minute))
	gemini.Post("/generate", handlers.HandleGenerateText)
}
