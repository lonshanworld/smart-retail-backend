class ShopCustomer {
  final String id;
  final String shopId;
  final String merchantId;
  final String name;
  final String? email;
  final String? phone;
  final DateTime createdAt;
  final DateTime updatedAt;

  ShopCustomer({
    required this.id,
    required this.shopId,
    required this.merchantId,
    required this.name,
    this.email,
    this.phone,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ShopCustomer.fromJson(Map<String, dynamic> json) {
    return ShopCustomer(
      id: json['id'],
      shopId: json['shopId'],
      merchantId: json['merchantId'],
      name: json['name'],
      email: json['email'],
      phone: json['phone'],
      createdAt: DateTime.parse(json['createdAt']),
      updatedAt: DateTime.parse(json['updatedAt']),
    );
  }
}
