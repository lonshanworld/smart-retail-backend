class StaffDashboardSummary {
  final String shopName;
  final int salesToday;
  final double totalRevenueToday;

  StaffDashboardSummary({
    required this.shopName,
    required this.salesToday,
    required this.totalRevenueToday,
  });

  factory StaffDashboardSummary.fromJson(Map<String, dynamic> json) {
    return StaffDashboardSummary(
      shopName: json['shopName'] ?? 'N/A',
      salesToday: json['salesToday'] ?? 0,
      totalRevenueToday: (json['totalRevenueToday'] ?? 0.0).toDouble(),
    );
  }
}
