import 'dart:convert';

class Supplier {
  final String? id;
  final String merchantId;
  final String name;
  final String? contactName;
  final String? contactEmail;
  final String? contactPhone;
  final String? address;
  final String? notes;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  Supplier({
    this.id,
    required this.merchantId,
    required this.name,
    this.contactName,
    this.contactEmail,
    this.contactPhone,
    this.address,
    this.notes,
    this.createdAt,
    this.updatedAt,
  });

  factory Supplier.fromJson(Map<String, dynamic> json) => Supplier(
        id: json["id"],
        merchantId: json["merchantId"],
        name: json["name"],
        contactName: json["contactName"],
        contactEmail: json["contactEmail"],
        contactPhone: json["contactPhone"],
        address: json["address"],
        notes: json["notes"],
        createdAt: json["createdAt"] == null ? null : DateTime.parse(json["createdAt"]),
        updatedAt: json["updatedAt"] == null ? null : DateTime.parse(json["updatedAt"]),
      );

  Map<String, dynamic> toJson() => {
        "id": id,
        "merchantId": merchantId,
        "name": name,
        "contactName": contactName,
        "contactEmail": contactEmail,
        "contactPhone": contactPhone,
        "address": address,
        "notes": notes,
        "createdAt": createdAt?.toIso8601String(),
        "updatedAt": updatedAt?.toIso8601String(),
      };
}

class PaginatedSuppliersResponse {
  final List<Supplier> suppliers;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedSuppliersResponse({
    required this.suppliers,
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedSuppliersResponse.fromJson(Map<String, dynamic> json) {
    return PaginatedSuppliersResponse(
      suppliers: (json['data'] as List).map((i) => Supplier.fromJson(i)).toList(),
      totalItems: json['pagination']['totalItems'],
      currentPage: json['pagination']['currentPage'],
      pageSize: json['pagination']['pageSize'],
      totalPages: json['pagination']['totalPages'],
    );
  }
}
