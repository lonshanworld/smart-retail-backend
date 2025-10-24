package models

// AIAssistantRequest defines the structure for requests to the AI assistant.
type AIAssistantRequest struct {
	Prompt string `json:"prompt"`
}

// BestSeller represents a best-selling product.
type BestSeller struct {
	ProductName string `json:"product_name"`
	TotalSold   int    `json:"total_sold"`
}

// ShopSales represents the total sales for a shop.
type ShopSales struct {
	ShopName   string  `json:"shop_name"`
	TotalSales float64 `json:"total_sales"`
}

// PeakHour represents a peak sales hour.
type PeakHour struct {
	Hour      int `json:"hour"`
	TotalSales int `json:"total_sales"`
}
