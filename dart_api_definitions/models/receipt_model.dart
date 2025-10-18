import 'dart:convert';

// Factory function to decode JSON
Receipt receiptFromJson(String str) => Receipt.fromJson(json.decode(str));

class Receipt {
  final String saleId;
  final DateTime saleDate;
  final String shopName;
  final String shopAddress;
  final String merchantName;
  final String? staffName;
  final double originalTotal;
  final double discountAmount;
  final double finalTotal;
  final String paymentType;
  final String paymentStatus; // This field is now included
  final String? appliedPromotionName;
  final String? notes;
  final List<ReceiptItem> items;

  Receipt({
    required this.saleId,
    required this.saleDate,
    required this.shopName,
    required this.shopAddress,
    required this.merchantName,
    this.staffName,
    required this.originalTotal,
    required this.discountAmount,
    required this.finalTotal,
    required this.paymentType,
    required this.paymentStatus, // This field is now included
    this.appliedPromotionName,
    this.notes,
    required this.items,
  });

  factory Receipt.fromJson(Map<String, dynamic> json) {
    // Helper to safely parse doubles
    double safeDouble(dynamic value) => (value as num? ?? 0.0).toDouble();

    // Calculate original total before discount
    double finalAmount = safeDouble(json['final_total']);
    double discount = safeDouble(json['discount_amount']);
    double originalTotal = finalAmount + discount;

    return Receipt(
      saleId: json['sale_id'] as String,
      saleDate: DateTime.parse(json['sale_date'] as String),
      shopName: json['shop_name'] as String,
      shopAddress: json['shop_address'] as String? ?? '',
      merchantName: json['merchant_name'] as String,
      staffName: json['staff_name'] as String?,
      originalTotal: originalTotal, // Calculated value
      discountAmount: discount,
      finalTotal: finalAmount,
      paymentType: json['payment_type'] as String,
      paymentStatus: json['payment_status'] as String? ?? 'succeeded', // This field is now included
      appliedPromotionName: json['promotion_name'] as String?,
      notes: json['notes'] as String?,
      items: (json['items'] as List<dynamic>)
          .map((item) => ReceiptItem.fromJson(item as Map<String, dynamic>))
          .toList(),
    );
  }
}

class ReceiptItem {
  final String itemName;
  final int quantitySold;
  final double unitPriceAtSale;
  final double subtotal;

  ReceiptItem({
    required this.itemName,
    required this.quantitySold,
    required this.unitPriceAtSale,
    required this.subtotal,
  });

  factory ReceiptItem.fromJson(Map<String, dynamic> json) {
    return ReceiptItem(
      itemName: json['item_name'] as String,
      quantitySold: json['quantity_sold'] as int,
      unitPriceAtSale: (json['unit_price_at_sale'] as num? ?? 0.0).toDouble(),
      subtotal: (json['subtotal'] as num? ?? 0.0).toDouble(),
    );
  }
}
