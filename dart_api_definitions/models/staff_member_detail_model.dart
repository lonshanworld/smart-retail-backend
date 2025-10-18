import 'dart:convert';

class StaffMemberDetail {
  final String id;
  final String name;
  final String email;
  final String role;
  final String shopId;
  final String shopName;
  final bool isActive;
  final DateTime createdAt;
  final DateTime updatedAt;

  StaffMemberDetail({
    required this.id,
    required this.name,
    required this.email,
    required this.role,
    required this.shopId,
    required this.shopName,
    required this.isActive,
    required this.createdAt,
    required this.updatedAt,
  });

  factory StaffMemberDetail.fromJson(Map<String, dynamic> json) {
    return StaffMemberDetail(
      id: json['id'],
      name: json['name'],
      email: json['email'],
      role: json['role'],
      shopId: json['shopId'],
      shopName: json['shopName'],
      isActive: json['isActive'],
      createdAt: DateTime.parse(json['createdAt']),
      updatedAt: DateTime.parse(json['updatedAt']),
    );
  }

  static StaffMemberDetail fromJsonString(String str) =>
      StaffMemberDetail.fromJson(json.decode(str));
}
