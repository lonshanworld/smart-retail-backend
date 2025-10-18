
class Customer {
  final String id;
  final String shopId;
  final String name;
  final String? email;
  final String? phone;
  final DateTime createdAt;

  Customer({
    required this.id,
    required this.shopId,
    required this.name,
    this.email,
    this.phone,
    required this.createdAt,
  });

  factory Customer.fromJson(Map<String, dynamic> json) {
    return Customer(
      id: json['id'] as String,
      shopId: json['shopId'] as String,
      name: json['name'] as String,
      email: json['email'] as String?,
      phone: json['phone'] as String?,
      createdAt: DateTime.parse(json['createdAt'] as String),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'shopId': shopId,
      'name': name,
      'email': email,
      'phone': phone,
      'createdAt': createdAt.toIso8601String(),
    };
  }
}
