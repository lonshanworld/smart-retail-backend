class MerchantStockInRequest {
  // Based on the new logic, we define the item directly.
  final String itemName;
  final String? sku;
  final double unitPrice; 

  // Core stock-in details
  final int quantityReceived;
  final double? costPrice;
  final String? supplierName;
  final String? notes;

  MerchantStockInRequest({
    required this.itemName,
    this.sku,
    required this.unitPrice,
    required this.quantityReceived,
    this.costPrice,
    this.supplierName,
    this.notes,
  });

  Map<String, dynamic> toJson() {
    return {
      'itemName': itemName,
      'sku': sku,
      'unitPrice': unitPrice,
      'quantityReceived': quantityReceived,
      'costPrice': costPrice,
      'supplierName': supplierName,
      'notes': notes,
    }..removeWhere((key, value) => value == null);
  }
}
