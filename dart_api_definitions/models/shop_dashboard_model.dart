class ShopDashboardSummary {
  final String shopId;
  final String shopName;
  final String userName;
  final double salesToday;
  final int transactionsToday;

  ShopDashboardSummary({
    required this.shopId,
    required this.shopName,
    required this.userName,
    required this.salesToday,
    required this.transactionsToday,
  });

  factory ShopDashboardSummary.fromJson(Map<String, dynamic> json) {
    return ShopDashboardSummary(
      shopId: json['shopId'],
      shopName: json['shopName'],
      userName: json['userName'],
      salesToday: (json['salesToday'] as num).toDouble(),
      transactionsToday: json['transactionsToday'],
    );
  }
}
