class SaleItemInput {
  final String inventoryItemId;
  final int quantitySold;

  SaleItemInput({
    required this.inventoryItemId,
    required this.quantitySold,
  });

  Map<String, dynamic> toJson() => {
        'inventoryItemId': inventoryItemId,
        'quantitySold': quantitySold,
      };
}

class CreateSaleInput {
  final String shopId;
  final String paymentType;
  final String? paymentStatus;
  final String? stripePaymentIntentId;
  final String? notes;
  final List<SaleItemInput> items;
  final String? customerId;

  CreateSaleInput({
    required this.shopId,
    required this.paymentType,
    this.paymentStatus,
    this.stripePaymentIntentId,
    this.notes,
    required this.items,
    this.customerId,
  });

  Map<String, dynamic> toJson() => {
        'shopId': shopId,
        'paymentType': paymentType,
        if (paymentStatus != null) 'paymentStatus': paymentStatus,
        if (stripePaymentIntentId != null) 'stripePaymentIntentId': stripePaymentIntentId,
        if (notes != null && notes!.isNotEmpty) 'notes': notes,
        'items': items.map((item) => item.toJson()).toList(),
        if (customerId != null) 'customerId': customerId,
      };
}

// --- Response Models for Fetching Sales ---

class SaleItem {
  final String id;
  final String saleId;
  final String inventoryItemId;
  final int quantitySold;
  final double sellingPriceAtSale; // RENAMED
  final double? originalPriceAtSale; // ADDED
  final double subtotal;
  final DateTime createdAt;
  final DateTime updatedAt;
  final String? itemName;
  final String? itemSku;

  // Calculated property for profit
  double get profit => (sellingPriceAtSale - (originalPriceAtSale ?? 0.0)) * quantitySold;

  SaleItem({
    required this.id,
    required this.saleId,
    required this.inventoryItemId,
    required this.quantitySold,
    required this.sellingPriceAtSale, // RENAMED
    this.originalPriceAtSale, // ADDED
    required this.subtotal,
    required this.createdAt,
    required this.updatedAt,
    this.itemName,
    this.itemSku,
  });

  factory SaleItem.fromJson(Map<String, dynamic> json) {
    return SaleItem(
      id: json['id'],
      saleId: json['saleId'],
      inventoryItemId: json['inventoryItemId'],
      quantitySold: json['quantitySold'],
      sellingPriceAtSale: (json['sellingPriceAtSale'] as num).toDouble(), // RENAMED
      originalPriceAtSale: (json['originalPriceAtSale'] as num?)?.toDouble(), // ADDED
      subtotal: (json['subtotal'] as num).toDouble(),
      createdAt: DateTime.parse(json['createdAt']),
      updatedAt: DateTime.parse(json['updatedAt']),
      itemName: json['itemName'],
      itemSku: json['itemSku'],
    );
  }
}

class Sale {
  final String id;
  final String shopId;
  final String merchantId;
  final DateTime saleDate;
  final double totalAmount;
  final String? appliedPromotionId;
  final double? discountAmount;
  final String paymentType;
  final String paymentStatus;
  final String? stripePaymentIntentId;
  final String? notes;
  final DateTime createdAt;
  final DateTime updatedAt;
  final List<SaleItem> items;

  // Calculated property for total profit of the sale
  double get totalProfit => items.fold(0.0, (sum, item) => sum + item.profit);

  Sale({
    required this.id,
    required this.shopId,
    required this.merchantId,
    required this.saleDate,
    required this.totalAmount,
    this.appliedPromotionId,
    this.discountAmount,
    required this.paymentType,
    required this.paymentStatus,
    this.stripePaymentIntentId,
    this.notes,
    required this.createdAt,
    required this.updatedAt,
    required this.items,
  });

  factory Sale.fromJson(Map<String, dynamic> json) {
    var itemsList = json['items'] as List? ?? [];
    List<SaleItem> saleItems = itemsList.map((i) => SaleItem.fromJson(i as Map<String, dynamic>)).toList();

    double discount = (json['discountAmount'] as num? ?? 0.0).toDouble();
    double finalAmount = (json['totalAmount'] as num).toDouble();
    double originalTotal = finalAmount + discount;

    return Sale(
      id: json['id'],
      shopId: json['shopId'],
      merchantId: json['merchantId'],
      saleDate: DateTime.parse(json['saleDate']),
      totalAmount: originalTotal,
      appliedPromotionId: json['appliedPromotionId'],
      discountAmount: discount,
      paymentType: json['paymentType'],
      paymentStatus: json['paymentStatus'] ?? 'succeeded',
      stripePaymentIntentId: json['stripePaymentIntentId'],
      notes: json['notes'],
      createdAt: DateTime.parse(json['createdAt']),
      updatedAt: DateTime.parse(json['updatedAt']),
      items: saleItems,
    );
  }
}

class PaginatedSalesResponse {
  final List<Sale> items;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedSalesResponse({
    required this.items,
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedSalesResponse.fromJson(Map<String, dynamic> json) {
    var data = json['data'] ?? {};
    var salesListJson = data['items'] as List? ?? [];
    List<Sale> salesList = salesListJson
        .map((i) => Sale.fromJson(i as Map<String, dynamic>))
        .toList();

    var meta = data['meta'] ?? {};
    return PaginatedSalesResponse(
      items: salesList,
      totalItems: meta['totalItems'] as int? ?? 0,
      currentPage: meta['currentPage'] as int? ?? 1,
      pageSize: meta['pageSize'] as int? ?? 0,
      totalPages: meta['totalPages'] as int? ?? 0,
    );
  }
}
