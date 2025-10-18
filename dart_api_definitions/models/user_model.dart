import 'package:smart_retail/app/data/enums/user_role.dart';
import 'package:smart_retail/app/data/models/shop_model.dart'; // Import the Shop model

class User {
  final String id;
  final String name;
  final String email;
  final String role;
  final bool isActive;
  final String? phone;
  final String? assignedShopId;
  final String? merchantId;
  final DateTime? createdAt;
  final DateTime? updatedAt;
  final Shop? shop;

  User({
    required this.id,
    required this.name,
    required this.email,
    required this.role,
    this.isActive = true,
    this.phone,
    this.assignedShopId,
    this.merchantId,
    this.createdAt,
    this.updatedAt,
    this.shop,
  });

  UserRole get roleAsEnum {
    return role.toUserRole();
  }

  String get roleDisplay {
    return roleAsEnum.toDisplayString();
  }

  String? get shopName => shop?.name;

  String? get merchantName {
    if (roleAsEnum == UserRole.merchant) {
      return shopName ?? name;
    }
    return null;
  }

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['ID'] ?? json['id'],
      name: json['name'],
      email: json['email'],
      role: json['role'] ?? UserRole.unknown.name,
      isActive: json['is_active'] ?? true,
      phone: json['phone'],
      assignedShopId: json['assigned_shop_id'],
      merchantId: json['merchant_id'],
      createdAt: json['created_at'] != null ? DateTime.tryParse(json['created_at']) : null,
      updatedAt: json['updated_at'] != null ? DateTime.tryParse(json['updated_at']) : null,
      shop: json['Shop'] != null ? Shop.fromJson(json['Shop']) : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'email': email,
      'role': role,
      'is_active': isActive,
      'phone': phone,
      'assigned_shop_id': assignedShopId,
      'merchant_id': merchantId,
    };
  }

  // ADDED: copyWith method
  User copyWith({
    String? id,
    String? name,
    String? email,
    String? role,
    bool? isActive,
    String? phone,
    String? assignedShopId,
    String? merchantId,
    DateTime? createdAt,
    DateTime? updatedAt,
    Shop? shop,
  }) {
    return User(
      id: id ?? this.id,
      name: name ?? this.name,
      email: email ?? this.email,
      role: role ?? this.role,
      isActive: isActive ?? this.isActive,
      phone: phone ?? this.phone,
      assignedShopId: assignedShopId ?? this.assignedShopId,
      merchantId: merchantId ?? this.merchantId,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      shop: shop ?? this.shop,
    );
  }
}
