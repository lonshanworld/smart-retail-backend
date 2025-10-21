package models

import "time"

// SalesReportData holds aggregated sales data for a specific period.
type SalesReportData struct {
	Period     time.Time `json:"period"`
	TotalSales float64   `json:"totalSales"`
}

// HistoricalSale represents a single past sale of an item, used for forecasting.
type HistoricalSale struct {
	SaleDate     time.Time `json:"saleDate"`
	QuantitySold int       `json:"quantitySold"`
}

// ForecastPeriod defines the start and end dates for a forecast.
type ForecastPeriod struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

// DailyForecast represents the predicted sales for a single day.
type DailyForecast struct {
	Date           string `json:"date"`
	PredictedSales int    `json:"predicted_sales"`
}

// AiAnalysis contains the qualitative insights from the Gemini model.
type AiAnalysis struct {
	Summary         string   `json:"summary"`
	PositiveFactors []string `json:"positive_factors"`
	NegativeFactors []string `json:"negative_factors"`
}

// SalesForecastResponse is the complete structure for the sales forecast API response.
type SalesForecastResponse struct {
	ReportName     string         `json:"reportName"`
	GeneratedAt    time.Time      `json:"generatedAt"`
	ProductName    string         `json:"productName"`
	ShopName       string         `json:"shopName"`
	CurrentStock   int            `json:"currentStock"`
	ForecastPeriod ForecastPeriod `json:"forecastPeriod"`
	DailyForecast  []DailyForecast `json:"dailyForecast"`
	AiAnalysis     AiAnalysis     `json:"aiAnalysis"`
}
