import 'dart:convert';

class Promotion {
  final String id;
  final String merchantId;
  final String? shopId;
  final String name;
  final String description;
  final String type;
  final double value;
  final double minSpend; // ADDED
  final Map<String, dynamic> conditions;
  final DateTime startDate;
  final DateTime endDate;
  final bool isActive;
  final DateTime createdAt;
  final DateTime updatedAt;

  Promotion({
    required this.id,
    required this.merchantId,
    this.shopId,
    required this.name,
    required this.description,
    required this.type,
    required this.value,
    required this.minSpend, // ADDED
    required this.conditions,
    required this.startDate,
    required this.endDate,
    required this.isActive,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Promotion.fromJson(Map<String, dynamic> json) {
    return Promotion(
      id: json['id'],
      merchantId: json['merchantId'],
      shopId: json['shopId'],
      name: json['name'],
      description: json['description'] ?? '',
      type: json['type'],
      value: (json['value'] as num).toDouble(),
      minSpend: (json['minSpend'] as num?)?.toDouble() ?? 0.0, // ADDED
      conditions: json['conditions'] != null ? Map<String, dynamic>.from(jsonDecode(json['conditions'] ?? '{}')) : {},
      startDate: DateTime.parse(json['startDate']),
      endDate: DateTime.parse(json['endDate']),
      isActive: json['isActive'],
      createdAt: DateTime.parse(json['createdAt']),
      updatedAt: DateTime.parse(json['updatedAt']),
    );
  }
}

class PaginatedPromotionsResponse {
  final List<Promotion> items;
  final int totalItems;
  final int currentPage;
  final int totalPages;

  PaginatedPromotionsResponse({
    required this.items,
    required this.totalItems,
    required this.currentPage,
    required this.totalPages,
  });

  factory PaginatedPromotionsResponse.fromJson(Map<String, dynamic> json) {
    var itemsList = json['items'] as List;
    List<Promotion> promotions = itemsList.map((i) => Promotion.fromJson(i)).toList();

    return PaginatedPromotionsResponse(
      items: promotions,
      totalItems: json['totalItems'],
      currentPage: json['currentPage'],
      totalPages: json['totalPages'],
    );
  }
}
