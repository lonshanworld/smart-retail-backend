/// A simplified model representing an item within a specific shop's inventory.
class ShopInventoryItem {
  final String id; // This is the ShopInventory unique ID
  final String productId; // This links to the master InventoryItem
  final String name;
  final String? sku;
  final int quantity;
  final double sellingPrice;

  ShopInventoryItem({
    required this.id,
    required this.productId,
    required this.name,
    this.sku,
    required this.quantity,
    required this.sellingPrice,
  });

  factory ShopInventoryItem.fromJson(Map<String, dynamic> json) {
    return ShopInventoryItem(
      id: json['id'],
      productId: json['productId'],
      name: json['name'],
      sku: json['sku'] as String?,
      quantity: (json['quantity'] as num).toInt(),
      // Staff and shop-level users only see the selling price.
      sellingPrice: (json['sellingPrice'] as num).toDouble(),
    );
  }
}
