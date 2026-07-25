package handlers

import "testing"

func TestValidateSQLQueryEnforcesMerchantScopeAndBounds(t *testing.T) {
	merchantID := "11111111-1111-1111-1111-111111111111"
	valid := "SELECT shop_id, SUM(total_amount) FROM sales WHERE merchant_id = '" + merchantID + "' GROUP BY shop_id LIMIT 10"
	if err := validateSQLQuery(valid, merchantID); err != nil {
		t.Fatalf("valid scoped query rejected: %v", err)
	}
	joined := "SELECT s.id, c.name FROM sales s LEFT JOIN shop_customers c ON c.id = s.customer_id AND c.merchant_id = s.merchant_id WHERE s.merchant_id = '" + merchantID + "' ORDER BY s.sale_date DESC LIMIT 100;"
	if err := validateSQLQuery(joined, merchantID); err != nil {
		t.Fatalf("valid joined query rejected: %v", err)
	}
	joinedReport := "SELECT s.id, sh.name, sc.name, u.name, COUNT(si.id) FROM sales s LEFT JOIN shops sh ON sh.id = s.shop_id AND sh.merchant_id = s.merchant_id LEFT JOIN shop_customers sc ON sc.id = s.customer_id AND sc.merchant_id = s.merchant_id LEFT JOIN users u ON u.id = s.staff_id AND u.merchant_id = s.merchant_id LEFT JOIN sale_items si ON si.sale_id = s.id WHERE s.merchant_id = '" + merchantID + "' GROUP BY s.id, sh.name, sc.name, u.name LIMIT 100;"
	if err := validateSQLQuery(joinedReport, merchantID); err != nil {
		t.Fatalf("valid report join query rejected: %v", err)
	}
	cases := []string{
		"UPDATE sales SET total_amount=0 WHERE merchant_id = '" + merchantID + "' LIMIT 1",
		"SELECT * FROM sales WHERE merchant_id = '" + merchantID + "'; LIMIT 1",
		"SELECT * FROM sales WHERE merchant_id = 'other' LIMIT 10",
		"SELECT * FROM sales WHERE merchant_id = '" + merchantID + "' LIMIT 101",
		"SELECT * FROM sales WHERE merchant_id = '" + merchantID + "' OR 1=1 LIMIT 10",
	}
	for _, query := range cases {
		if err := validateSQLQuery(query, merchantID); err == nil {
			t.Errorf("unsafe query was accepted: %s", query)
		}
	}
}
