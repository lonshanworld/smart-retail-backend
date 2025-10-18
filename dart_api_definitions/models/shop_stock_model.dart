import 'dart:convert';

// Represents the stock of a specific inventory item within a specific shop.
// This often includes details from the master InventoryItem for display purposes.
class ShopStockItem {
  String id; // ID of the shop_stock entry itself
  String shopId;
  String inventoryItemId;
  int quantity;
  DateTime lastStockedInAt;
  DateTime createdAt; // ShopStock entry creation
  DateTime updatedAt; // ShopStock entry update

  // Enriched data from the master InventoryItem
  String itemName;
  String? itemSku;
  double itemUnitPrice; // Selling price from master item
  bool itemIsArchived; // Could be useful to know if master item is archived

  ShopStockItem({
    required this.id,
    required this.shopId,
    required this.inventoryItemId,
    required this.quantity,
    required this.lastStockedInAt,
    required this.createdAt,
    required this.updatedAt,
    required this.itemName,
    this.itemSku,
    required this.itemUnitPrice,
    this.itemIsArchived = false,
  });

  factory ShopStockItem.fromJson(Map<String, dynamic> json) {
    return ShopStockItem(
      id: json['id'] as String,
      shopId: json['shopId'] as String,
      inventoryItemId: json['inventoryItemId'] as String,
      quantity: (json['quantity'] as num).toInt(),
      lastStockedInAt: DateTime.parse(json['lastStockedInAt'] as String),
      createdAt: DateTime.parse(json['createdAt'] as String),
      updatedAt: DateTime.parse(json['updatedAt'] as String),
      itemName: json['itemName'] as String,
      itemSku: json['itemSku'] as String?,
      itemUnitPrice: (json['itemUnitPrice'] as num).toDouble(),
      itemIsArchived: json['itemIsArchived'] as bool? ?? false, // If included from backend
    );
  }

  // No toJson needed here typically, as ShopStockItem is usually for display.
  // Stock-in operations will have their own request DTOs.

  // copyWith if needed for state management
  ShopStockItem copyWith({
    String? id,
    String? shopId,
    String? inventoryItemId,
    int? quantity,
    DateTime? lastStockedInAt,
    DateTime? createdAt,
    DateTime? updatedAt,
    String? itemName,
    String? itemSku,
    double? itemUnitPrice,
    bool? itemIsArchived,
  }) {
    return ShopStockItem(
      id: id ?? this.id,
      shopId: shopId ?? this.shopId,
      inventoryItemId: inventoryItemId ?? this.inventoryItemId,
      quantity: quantity ?? this.quantity,
      lastStockedInAt: lastStockedInAt ?? this.lastStockedInAt,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      itemName: itemName ?? this.itemName,
      itemSku: itemSku ?? this.itemSku,
      itemUnitPrice: itemUnitPrice ?? this.itemUnitPrice,
      itemIsArchived: itemIsArchived ?? this.itemIsArchived,
    );
  }

  @override
  String toString() {
    return 'ShopStockItem{itemId: $inventoryItemId, shopId: $shopId, name: $itemName, quantity: $quantity}';
  }
}

class PaginatedShopStockResponse {
  final List<ShopStockItem> items;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedShopStockResponse({
    required this.items,
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedShopStockResponse.fromJson(Map<String, dynamic> json) {
    var itemsList = json['items'] as List;
    List<ShopStockItem> items = itemsList.map((i) => ShopStockItem.fromJson(i)).toList();

    return PaginatedShopStockResponse(
      items: items,
      totalItems: (json['totalItems'] as num).toInt(),
      currentPage: (json['currentPage'] as num).toInt(),
      pageSize: (json['pageSize'] as num).toInt(),
      totalPages: (json['totalPages'] as num).toInt(),
    );
  }
}
