class StaffDashboardSummaryResponse {
  final String assignedShopName;
  final double salesToday;
  final int transactionsToday;
  final List<ActivityItemDTO> recentActivities;

  StaffDashboardSummaryResponse({
    required this.assignedShopName,
    required this.salesToday,
    required this.transactionsToday,
    required this.recentActivities,
  });

  factory StaffDashboardSummaryResponse.fromJson(Map<String, dynamic> json) {
    return StaffDashboardSummaryResponse(
      assignedShopName: json['assignedShopName'] as String,
      salesToday: (json['salesToday'] as num).toDouble(),
      transactionsToday: json['transactionsToday'] as int,
      recentActivities: (json['recentActivities'] as List)
          .map((i) => ActivityItemDTO.fromJson(i as Map<String, dynamic>))
          .toList(),
    );
  }
}

class ActivityItemDTO {
  final String type;
  final String details;
  final DateTime timestamp;
  final String? relatedId;

  ActivityItemDTO({
    required this.type,
    required this.details,
    required this.timestamp,
    this.relatedId,
  });

  factory ActivityItemDTO.fromJson(Map<String, dynamic> json) {
    return ActivityItemDTO(
      type: json['type'] as String,
      details: json['details'] as String,
      timestamp: DateTime.parse(json['timestamp'] as String),
      relatedId: json['relatedId'] as String?,
    );
  }
}
