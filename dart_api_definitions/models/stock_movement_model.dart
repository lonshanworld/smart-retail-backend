import 'package:intl/intl.dart'; // For date formatting

class StockMovement {
  final String id;
  final String itemId;
  final String shopId;
  final String movementType; // e.g., "stock_in", "sale", "adjustment_damage", "adjustment_theft", "initial_stock"
  final int quantityChanged; // Can be positive (stock in) or negative (stock out)
  final int newQuantity; // Stock quantity after this movement
  final String? reason; // Optional reason for adjustment
  final String userId; // User who performed the action
  final DateTime movementDate;
  final String? notes; // Additional notes if any

  // Potentially include item details if the API provides them and they are needed in the UI
  // final String? itemName;
  // final String? itemSku;

  StockMovement({
    required this.id,
    required this.itemId,
    required this.shopId,
    required this.movementType,
    required this.quantityChanged,
    required this.newQuantity,
    this.reason,
    required this.userId,
    required this.movementDate,
    this.notes,
    // this.itemName,
    // this.itemSku,
  });

  factory StockMovement.fromJson(Map<String, dynamic> json) {
    return StockMovement(
      id: json['id'] as String,
      itemId: json['item_id'] as String,
      shopId: json['shop_id'] as String,
      movementType: json['movement_type'] as String,
      quantityChanged: json['quantity_changed'] as int,
      newQuantity: json['new_quantity'] as int,
      reason: json['reason'] as String?,
      userId: json['user_id'] as String,
      movementDate: DateTime.parse(json['movement_date'] as String), // Ensure API returns ISO 8601
      notes: json['notes'] as String?,
      // itemName: json['item_name'] as String?, // If API includes this
      // itemSku: json['item_sku'] as String?,   // If API includes this
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'item_id': itemId,
      'shop_id': shopId,
      'movement_type': movementType,
      'quantity_changed': quantityChanged,
      'new_quantity': newQuantity,
      'reason': reason,
      'user_id': userId,
      'movement_date': movementDate.toIso8601String(),
      'notes': notes,
      // 'item_name': itemName,
      // 'item_sku': itemSku,
    };
  }

  String get formattedMovementDate {
    return DateFormat('yyyy-MM-dd HH:mm').format(movementDate);
  }
}
