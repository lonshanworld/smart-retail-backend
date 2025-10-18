import 'dart:convert';

class StockInfo {
  final int quantity;
  final String shopId;

  StockInfo({required this.quantity, required this.shopId});

  factory StockInfo.fromJson(Map<String, dynamic> json) {
    return StockInfo(
      quantity: json['quantity'] as int,
      shopId: json['shop_id'] as String,
    );
  }
}

class InventoryItem {
  String? id;
  String merchantId;
  String name;
  String? description;
  String? sku;
  double sellingPrice;
  double? originalPrice;
  int? lowStockThreshold;
  String? category;
  String? supplier;
  bool isArchived;
  DateTime createdAt;
  DateTime updatedAt;
  
  bool isSynced;
  bool needsUpdate;
  bool needsCreate;

  StockInfo? stock;

  // RESTORED: Fields for combined stock view
  String? shopName;
  String? shopId;

  InventoryItem({
    this.id,
    required this.merchantId,
    required this.name,
    this.description,
    this.sku,
    required this.sellingPrice,
    this.originalPrice,
    this.lowStockThreshold,
    this.category,
    this.supplier,
    this.isArchived = false,
    required this.createdAt,
    required this.updatedAt,
    this.isSynced = false,
    this.needsUpdate = false,
    this.needsCreate = false,
    this.stock,
    this.shopName, // RESTORED
    this.shopId,   // RESTORED
  });

  InventoryItem copyWith({
    String? id,
    String? merchantId,
    String? name,
    String? description,
    String? sku, 
    double? sellingPrice,
    double? originalPrice,
    int? lowStockThreshold,
    String? category,
    String? supplier,
    bool? isArchived,
    DateTime? createdAt,
    DateTime? updatedAt,
    bool? isSynced,
    bool? needsUpdate,
    bool? needsCreate,
    StockInfo? stock,
    String? shopName,
    String? shopId,
  }) {
    return InventoryItem(
      id: id ?? this.id,
      merchantId: merchantId ?? this.merchantId,
      name: name ?? this.name,
      description: description ?? this.description,
      sku: sku ?? this.sku,
      sellingPrice: sellingPrice ?? this.sellingPrice,
      originalPrice: originalPrice ?? this.originalPrice,
      lowStockThreshold: lowStockThreshold ?? this.lowStockThreshold,
      category: category ?? this.category,
      supplier: supplier ?? this.supplier,
      isArchived: isArchived ?? this.isArchived,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      isSynced: isSynced ?? this.isSynced,
      needsUpdate: needsUpdate ?? this.needsUpdate,
      needsCreate: needsCreate ?? this.needsCreate,
      stock: stock ?? this.stock,
      shopName: shopName ?? this.shopName,
      shopId: shopId ?? this.shopId,
    );
  }

  factory InventoryItem.fromJson(Map<String, dynamic> json) {
    return InventoryItem(
      id: json['id'] as String?,
      merchantId: json['merchantId'] as String,
      name: json['name'] as String,
      description: json['description'] as String?,
      sku: json['sku'] as String?,
      sellingPrice: (json['sellingPrice'] as num).toDouble(),
      originalPrice: (json['originalPrice'] as num?)?.toDouble(),
      lowStockThreshold: (json['lowStockThreshold'] as num?)?.toInt(),
      category: json['category'] as String?,
      supplier: json['supplier'] as String?,
      isArchived: json['isArchived'] as bool? ?? false,
      createdAt: DateTime.parse(json['createdAt'] as String),
      updatedAt: DateTime.parse(json['updatedAt'] as String),
      isSynced: true,
      stock: json.containsKey('stock') && json['stock'] != null
          ? StockInfo.fromJson(json['stock'] as Map<String, dynamic>)
          : null,
      shopName: json['shopName'] as String?,
      shopId: json['shopId'] as String?,
    );
  }

  factory InventoryItem.fromDbMap(Map<String, dynamic> map) {
    return InventoryItem(
      id: map['id'] as String?,
      merchantId: map['merchantId'] as String,
      name: map['name'] as String,
      description: map['description'] as String?,
      sku: map['sku'] as String?,
      sellingPrice: (map['sellingPrice'] as num).toDouble(),
      originalPrice: map['originalPrice'] as double?,
      lowStockThreshold: map['lowStockThreshold'] as int?,
      category: map['category'] as String?,
      supplier: map['supplier'] as String?,
      isArchived: map['isArchived'] == 1,
      createdAt: DateTime.parse(map['createdAt'] as String),
      updatedAt: DateTime.parse(map['updatedAt'] as String),
      isSynced: map['isSynced'] == 1,
      needsUpdate: map['needsUpdate'] == 1,
      needsCreate: map['needsCreate'] == 1,
    );
  }

  Map<String, dynamic> toDbMap() {
    return {
      'id': id,
      'merchantId': merchantId,
      'name': name,
      'description': description,
      'sku': sku,
      'sellingPrice': sellingPrice,
      'originalPrice': originalPrice,
      'lowStockThreshold': lowStockThreshold,
      'category': category,
      'supplier': supplier,
      'isArchived': isArchived ? 1 : 0,
      'createdAt': createdAt.toIso8601String(),
      'updatedAt': updatedAt.toIso8601String(),
      'isSynced': isSynced ? 1 : 0,
      'needsUpdate': needsUpdate ? 1 : 0,
      'needsCreate': needsCreate ? 1 : 0,
    };
  }

  Map<String, dynamic> toJsonForCreate() {
    return {
      'name': name,
      'description': description,
      'sku': sku,
      'sellingPrice': sellingPrice,
      'originalPrice': originalPrice,
      'lowStockThreshold': lowStockThreshold,
      'category': category,
      'supplier': supplier,
    };
  }

  Map<String, dynamic> toJsonForUpdate() {
    final Map<String, dynamic> data = {};
    data['name'] = name;
    if (description != null) data['description'] = description;
    if (sku != null) data['sku'] = sku;
    data['sellingPrice'] = sellingPrice;
    if (originalPrice != null) data['originalPrice'] = originalPrice;
    if (lowStockThreshold != null) data['lowStockThreshold'] = lowStockThreshold;
    if (category != null) data['category'] = category;
    if (supplier != null) data['supplier'] = supplier;
    data['isArchived'] = isArchived;
    return data;
  }
}

class PaginatedInventoryResponse {
  final List<InventoryItem> items;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedInventoryResponse({
    required this.items,
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedInventoryResponse.fromJson(Map<String, dynamic> json) {
    var itemsList = json['items'] as List;
    List<InventoryItem> items = itemsList.map((i) => InventoryItem.fromJson(i)).toList();

    return PaginatedInventoryResponse(
      items: items,
      totalItems: (json['totalItems'] as num).toInt(),
      currentPage: (json['currentPage'] as num).toInt(),
      pageSize: (json['pageSize'] as num).toInt(),
      totalPages: (json['totalPages'] as num).toInt(),
    );
  }
}
