// lib/app/data/models/merchant_dashboard_summary_model.dart

class MerchantDashboardSummaryModel {
  final KpiData totalSalesRevenue;
  final KpiData numberOfTransactions;
  final KpiData averageOrderValue;
  final List<ProductSummaryModel> topSellingProducts;

  MerchantDashboardSummaryModel({
    required this.totalSalesRevenue,
    required this.numberOfTransactions,
    required this.averageOrderValue,
    required this.topSellingProducts,
  });

  factory MerchantDashboardSummaryModel.fromJson(Map<String, dynamic> json) {
    var topSellingProductsList = json['top_selling_products'] as List? ?? [];
    List<ProductSummaryModel> products = topSellingProductsList
        .map((i) => ProductSummaryModel.fromJson(i as Map<String, dynamic>))
        .toList();

    return MerchantDashboardSummaryModel(
      totalSalesRevenue: KpiData.fromJson(json['total_sales_revenue'] as Map<String, dynamic>? ?? {}),
      numberOfTransactions: KpiData.fromJson(json['number_of_transactions'] as Map<String, dynamic>? ?? {}),
      averageOrderValue: KpiData.fromJson(json['average_order_value'] as Map<String, dynamic>? ?? {}),
      topSellingProducts: products,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'total_sales_revenue': totalSalesRevenue.toJson(),
      'number_of_transactions': numberOfTransactions.toJson(),
      'average_order_value': averageOrderValue.toJson(),
      'top_selling_products': topSellingProducts.map((p) => p.toJson()).toList(),
    };
  }
}

class KpiData {
  final double value;

  KpiData({
    required this.value,
  });

  factory KpiData.fromJson(Map<String, dynamic> json) {
    return KpiData(
      value: (json['value'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'value': value,
    };
  }
}

class ProductSummaryModel {
  final String productId;
  final String productName;
  final int? quantitySold;
  final double? revenue;

  ProductSummaryModel({
    required this.productId,
    required this.productName,
    this.quantitySold,
    this.revenue,
  });

  factory ProductSummaryModel.fromJson(Map<String, dynamic> json) {
    return ProductSummaryModel(
      productId: json['product_id'] as String? ?? '',
      productName: json['product_name'] as String? ?? 'Unknown Product',
      quantitySold: json['quantity_sold'] as int?,
      revenue: (json['revenue'] as num?)?.toDouble(),
    );
  }

   Map<String, dynamic> toJson() {
    return {
      'product_id': productId,
      'product_name': productName,
      'quantity_sold': quantitySold,
      'revenue': revenue,
    };
  }
}
