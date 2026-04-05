package utils

import "fmt"

const CurrencySymbol = "MMK"

func FormatCurrency(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	return fmt.Sprintf("%s%s %.2f", sign, CurrencySymbol, amount)
}
