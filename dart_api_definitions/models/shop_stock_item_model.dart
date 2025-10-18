class ShopStockItem {
  final String itemId;
  final String itemName;
  final String? itemDescription;
  final String? sku;
  final String? barcode;
  final String? imageUrl;
  int quantity; // Current stock in the specific shop
  final String shopId;
  // Add other relevant fields from your InventoryItem model if needed
  // e.g., category, brand, supplierPrice, sellingPrice

  ShopStockItem({
    required this.itemId,
    required this.itemName,
    this.itemDescription,
    this.sku,
    this.barcode,
    this.imageUrl,
    required this.quantity,
    required this.shopId,
  });

  factory ShopStockItem.fromJson(Map<String, dynamic> json) {
    return ShopStockItem(
      itemId: json['item_id'] as String, // Ensure your API returns 'item_id'
      itemName: json['item_name'] as String,
      itemDescription: json['item_description'] as String?,
      sku: json['sku'] as String?,
      barcode: json['barcode'] as String?,
      imageUrl: json['image_url'] as String?,
      quantity: json['quantity'] as int, // Stock quantity from shop_stocks
      shopId: json['shop_id'] as String, // The shop this stock pertains to
      // Map other fields as necessary
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'item_id': itemId,
      'item_name': itemName,
      'item_description': itemDescription,
      'sku': sku,
      'barcode': barcode,
      'image_url': imageUrl,
      'quantity': quantity,
      'shop_id': shopId,
      // Map other fields
    };
  }
}
