class AdminDashboardSummary {
  final int totalMerchants;
  final int activeMerchants;
  final int totalStaff;
  final int activeStaff;
  final int totalShops;
  final double totalSalesValue;
  final double salesToday;
  final int transactionsToday;

  AdminDashboardSummary({
    required this.totalMerchants,
    required this.activeMerchants,
    required this.totalStaff,
    required this.activeStaff,
    required this.totalShops,
    required this.totalSalesValue,
    required this.salesToday,
    required this.transactionsToday,
  });

  factory AdminDashboardSummary.fromJson(Map<String, dynamic> json) {
    return AdminDashboardSummary(
      totalMerchants: json['totalMerchants'] as int,
      activeMerchants: json['activeMerchants'] as int,
      totalStaff: json['totalStaff'] as int,
      activeStaff: json['activeStaff'] as int,
      totalShops: json['totalShops'] as int,
      totalSalesValue: (json['totalSalesValue'] as num).toDouble(),
      salesToday: (json['salesToday'] as num).toDouble(),
      transactionsToday: json['transactionsToday'] as int,
    );
  }
}
