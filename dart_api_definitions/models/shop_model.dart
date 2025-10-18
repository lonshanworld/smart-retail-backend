import 'dart:convert';

class Shop {
  String? id;
  String merchantId;
  String name;
  String? address;
  String? phone;
  bool? isActive;
  bool? isPrimary;
  DateTime createdAt;
  DateTime updatedAt;

  Shop({
    this.id,
    required this.merchantId,
    required this.name,
    this.address,
    this.phone,
    this.isActive,
    this.isPrimary,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Shop.fromJson(Map<String, dynamic> json) {
    return Shop(
      id: json['id'] as String?,
      merchantId: json['merchantId'] as String? ?? json['merchant_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      address: json['address'] as String?,
      phone: json['phone'] as String?,
      isActive: json['isActive'] as bool? ?? json['is_active'] as bool?,
      isPrimary: json['isPrimary'] as bool? ?? json['is_primary'] as bool?,
      createdAt: DateTime.parse(json['createdAt'] as String? ?? json['created_at'] as String? ?? DateTime.now().toIso8601String()),
      updatedAt: DateTime.parse(json['updatedAt'] as String? ?? json['updated_at'] as String? ?? DateTime.now().toIso8601String()),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'merchantId': merchantId,
      'name': name,
      'address': address,
      'phone': phone,
      'isActive': isActive,
      'isPrimary': isPrimary,
      'createdAt': createdAt.toIso8601String(),
      'updatedAt': updatedAt.toIso8601String(),
    };
  }

  Map<String, dynamic> toJsonForCreate() {
    return {
      'name': name,
      if (address != null && address!.isNotEmpty) 'address': address,
      if (phone != null && phone!.isNotEmpty) 'phone': phone,
    };
  }

  Map<String, dynamic> toJsonForAdminCreate() {
    final Map<String, dynamic> data = {
      'merchantId': merchantId,
      'name': name,
    };
    if (address != null && address!.isNotEmpty) {
      data['address'] = address;
    }
    if (phone != null && phone!.isNotEmpty) {
      data['phone'] = phone;
    }
    if (isActive != null) {
      data['isActive'] = isActive;
    }
    if (isPrimary != null) {
      data['isPrimary'] = isPrimary;
    }
    return data;
  }

  Map<String, dynamic> toJsonForUpdate() {
    final Map<String, dynamic> data = {};
    data['name'] = name;
    if (address != null) {
      data['address'] = address;
    } else {
    }
    if (phone != null) {
      data['phone'] = phone;
    } else {
    }
    return data;
  }

  Map<String, dynamic> toJsonForAdminUpdate({
    String? newMerchantId,
    String? newName,
    String? newAddress, 
    String? newPhone, 
    bool? newIsActive,
    bool? newIsPrimary,
  }) {
    final Map<String, dynamic> data = {};
    if (newMerchantId != null) {
      data['merchantId'] = newMerchantId;
    }
    if (newName != null) {
      data['name'] = newName;
    }
    if (newAddress != null) { 
      data['address'] = newAddress;
    }
    if (newPhone != null) {
      data['phone'] = newPhone;
    }
    if (newIsActive != null) {
      data['isActive'] = newIsActive;
    }
    if (newIsPrimary != null) {
      data['isPrimary'] = newIsPrimary;
    }
    return data;
  }

  factory Shop.fromDbMap(Map<String, dynamic> map) {
    return Shop(
      id: map['id'] as String?,
      merchantId: map['merchantId'] as String,
      name: map['name'] as String,
      address: map['address'] as String?,
      phone: map['phone'] as String?,
      isActive: map['isActive'] == 1,
      isPrimary: map['isPrimary'] == 1,
      createdAt: DateTime.parse(map['createdAt'] as String),
      updatedAt: DateTime.parse(map['updatedAt'] as String),
    );
  }

  Map<String, dynamic> toDbMap() {
    return {
      'id': id,
      'merchantId': merchantId,
      'name': name,
      'address': address,
      'phone': phone,
      'isActive': isActive == true ? 1 : 0,
      'isPrimary': isPrimary == true ? 1 : 0,
      'createdAt': createdAt.toIso8601String(),
      'updatedAt': updatedAt.toIso8601String(),
    };
  }

  Shop copyWith({
    String? id,
    String? merchantId,
    String? name,
    String? address, 
    String? phone, 
    bool? isActive,  
    bool? isPrimary,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return Shop(
      id: id ?? this.id,
      merchantId: merchantId ?? this.merchantId,
      name: name ?? this.name,
      address: address ?? this.address, 
      phone: phone ?? this.phone, 
      isActive: isActive ?? this.isActive, 
      isPrimary: isPrimary ?? this.isPrimary,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  String toString() {
    return 'Shop{id: $id, name: $name, merchantId: $merchantId, isActive: $isActive, isPrimary: $isPrimary}';
  }
}

class PaginationInfo {
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;
  final String? nextPageUrl; 
  final String? prevPageUrl; 

  PaginationInfo({
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
    this.nextPageUrl,
    this.prevPageUrl,
  });

  factory PaginationInfo.fromJson(Map<String, dynamic> json) {
    return PaginationInfo(
      totalItems: (json['total_items'] as num?)?.toInt() ?? 0,
      currentPage: (json['current_page'] as num?)?.toInt() ?? 1,
      pageSize: (json['page_size'] as num?)?.toInt() ?? 10,
      totalPages: (json['total_pages'] as num?)?.toInt() ?? 0,
      nextPageUrl: json['next_page'] as String?,
      prevPageUrl: json['prev_page'] as String?,
    );
  }
}

class PaginatedAdminShopsResponse {
  final List<Shop> shops;
  final PaginationInfo pagination;

  PaginatedAdminShopsResponse({
    required this.shops,
    required this.pagination,
  });

  factory PaginatedAdminShopsResponse.fromJson(Map<String, dynamic> json) {
    var shopsListFromJson = json['data'] as List? ?? [];
    List<Shop> shopsList = shopsListFromJson
        .map((s) => Shop.fromJson(s as Map<String, dynamic>))
        .toList();
    
    var paginationJson = json['pagination'] as Map<String, dynamic>?;
    PaginationInfo paginationInfo;
    if (paginationJson != null) {
        paginationInfo = PaginationInfo.fromJson(paginationJson);
    } else {
        paginationInfo = PaginationInfo(totalItems: 0, currentPage: 1, pageSize: 10, totalPages: 0);
    }

    return PaginatedAdminShopsResponse(
      shops: shopsList,
      pagination: paginationInfo,
    );
  }
}

class PaginatedShopResponse {
  final List<Shop> shops;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedShopResponse({
    required this.shops,
    required this.totalItems,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedShopResponse.fromJson(Map<String, dynamic> json) {
    var dataField = json['data'];
    List<dynamic> shopsListJson;
    int totalItemsJson, currentPageJson, pageSizeJson, totalPagesJson;

    if (dataField is Map<String, dynamic>) { 
        shopsListJson = dataField['shops'] as List? ?? dataField['data'] as List? ?? [];
        totalItemsJson = (dataField['totalItems'] as num?)?.toInt() ?? (dataField['total_items'] as num?)?.toInt() ?? shopsListJson.length;
        currentPageJson = (dataField['currentPage'] as num?)?.toInt() ?? (dataField['current_page'] as num?)?.toInt() ?? 1;
        pageSizeJson = (dataField['pageSize'] as num?)?.toInt() ?? (dataField['page_size'] as num?)?.toInt() ?? (shopsListJson.isNotEmpty ? shopsListJson.length : 10) ;
        totalPagesJson = (dataField['totalPages'] as num?)?.toInt() ?? (dataField['total_pages'] as num?)?.toInt() ?? ((totalItemsJson + pageSizeJson -1) ~/ pageSizeJson);
    } else if (dataField is List) { 
        shopsListJson = dataField;
        totalItemsJson = shopsListJson.length;
        currentPageJson = 1;
        pageSizeJson = shopsListJson.length > 0 ? shopsListJson.length : 10;
        totalPagesJson = 1;
    } else { 
        shopsListJson = [];
        totalItemsJson = 0;
        currentPageJson = 1;
        pageSizeJson = 10;
        totalPagesJson = 0;
    }
    
    List<Shop> shops = shopsListJson.map((s) => Shop.fromJson(s as Map<String, dynamic>)).toList();
    
    return PaginatedShopResponse(
      shops: shops,
      totalItems: totalItemsJson,
      currentPage: currentPageJson,
      pageSize: pageSizeJson,
      totalPages: totalPagesJson,
    );
  }
}
