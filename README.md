# Project Overview

This project is a Go backend server designed to power a multi-platform Flutter application. It handles business logic for a system involving admins, merchants, staff, customers, and inventory management.

## Business Logic Rules

### User Roles & Permissions

1.  **Admin**: Can create other Admins, Merchants, and Staff. Can assign Staff to any Merchant.
2.  **Merchant**: Can create multiple Shops. Can create Suppliers. Can assign their own Staff to one of their own Shops.
3.  **Staff**: Can be assigned to only one Merchant and one Shop.
4.  **Supplier**: Created by a Merchant for recording stock-in.
5.  **Customer**: Created automatically during a purchase transaction.

### Entity Relationships

*   **Shops & Inventories**: Each Shop must have its own dedicated Inventory.

---

## Frontend Data Models

These are the Dart models used by the Flutter application. The backend API will be designed to return JSON that conforms to these structures.

### `AdminDashboardSummary`

```dart
class AdminDashboardSummary {
  final int totalMerchants;
  final int activeMerchants;
  final int totalStaff;
  final int activeStaff;
  final int totalShops;
  final double totalSalesValue;
  final double salesToday;
  final int transactionsToday;

  AdminDashboardSummary({
    required this.totalMerchants,
    required this.activeMerchants,
    required this.totalStaff,
    required this.activeStaff,
    required this.totalShops,
    required this.totalSalesValue,
    required this.salesToday,
    required this.transactionsToday,
  });

  factory AdminDashboardSummary.fromJson(Map<String, dynamic> json) {
    return AdminDashboardSummary(
      totalMerchants: json['totalMerchants'] as int,
      activeMerchants: json['activeMerchants'] as int,
      totalStaff: json['totalStaff'] as int,
      activeStaff: json['activeStaff'] as int,
      totalShops: json['totalShops'] as int,
      totalSalesValue: (json['totalSalesValue'] as num).toDouble(),
      salesToday: (json['salesToday'] as num).toDouble(),
      transactionsToday: json['transactionsToday'] as int,
    );
  }
}
```

### `AdminPaginatedUsersResponse`

```dart
import 'package:smart_retail/app/data/models/user_model.dart';

class AdminPaginatedUsersResponse {
  final List<User> users;
  final int currentPage;
  final int totalPages;
  final int pageSize;
  final int totalCount;

  AdminPaginatedUsersResponse({
    required this.users,
    required this.currentPage,
    required this.totalPages,
    required this.pageSize,
    required this.totalCount,
  });

  factory AdminPaginatedUsersResponse.fromJson(Map<String, dynamic> json) {
    var usersList = json['users'] as List? ?? [];
    List<User> users = usersList.map((i) => User.fromJson(i as Map<String, dynamic>)).toList();

    return AdminPaginatedUsersResponse(
      users: users,
      currentPage: json['current_page'] as int? ?? 1,
      totalPages: json['total_pages'] as int? ?? 1,
      pageSize: json['page_size'] as int? ?? users.length,
      totalCount: json['total_count'] as int? ?? users.length,
    );
  }
}
```

### `CartItem`

```dart
import 'package:get/get.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';

class CartItem {
  final InventoryItem product;
  final RxInt quantity;

  CartItem({required this.product, int initialQuantity = 1}) : quantity = initialQuantity.obs;

  double get subtotal => product.sellingPrice * quantity.value;

  void increment() {
    quantity.value++;
  }

  void decrement() {
    if (quantity.value > 0) {
      quantity.value--;
    }
  }
}
```

### `Customer` & `ShopCustomer`

```dart
class Customer {
  final String id;
  final String shopId;
  final String name;
  final String? email;
  final String? phone;
  final DateTime createdAt;

  Customer({
    required this.id,
    required this.shopId,
    required this.name,
    this.email,
    this.phone,
    required this.createdAt,
  });

  factory Customer.fromJson(Map<String, dynamic> json) {
    return Customer(
      id: json['id'] as String,
      shopId: json['shopId'] as String,
      name: json['name'] as String,
      email: json['email'] as String?,
      phone: json['phone'] as String?,
      createdAt: DateTime.parse(json['createdAt'] as String),
    );
  }
}

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
```

### `InventoryItem` & `PaginatedInventoryResponse`

```dart
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
  // ... other fields and methods
}

class PaginatedInventoryResponse {
  final List<InventoryItem> items;
  final int totalItems;
  final int currentPage;
  final int pageSize;
  final int totalPages;
  // ... constructor and fromJson
}
```

### `MasterInventoryItem`

```dart
class MasterInventoryItem {
  final String id;
  final String name;

  MasterInventoryItem({required this.id, required this.name});

  factory MasterInventoryItem.fromJson(Map<String, dynamic> json) {
    return MasterInventoryItem(
      id: json['id'] as String,
      name: json['name'] as String,
    );
  }
}
```

### `Merchant` & Related Models

```dart
class Merchant {
  final String id;
  final String name;
  final String email;
  final String? shopName;
  // ... other fields and methods
}

class PaginatedAdminMerchantsResponse {
  final List<Merchant> merchants;
  final PaginationInfo pagination;
  // ... constructor and fromJson
}

class MerchantDashboardSummaryModel {
  final KpiData totalSalesRevenue;
  final KpiData numberOfTransactions;
  final KpiData averageOrderValue;
  final List<ProductSummaryModel> topSellingProducts;
  // ... constructor and fromJson
}

class KpiData {
  final double value;
  // ... constructor and fromJson
}

class ProductSummaryModel {
  final String productId;
  final String productName;
  final int? quantitySold;
  final double? revenue;
  // ... constructor and fromJson
}
```

### `Sale` & Related Models

```dart
class CreateSaleInput {
  final String shopId;
  final String paymentType;
  final List<SaleItemInput> items;
  // ... other fields and toJson
}

class SaleItemInput {
  final String inventoryItemId;
  final int quantitySold;
  // ... constructor and toJson
}

class Sale {
  final String id;
  final String shopId;
  final String merchantId;
  final DateTime saleDate;
  final double totalAmount;
  final List<SaleItem> items;
  // ... other fields and fromJson
}

class SaleItem {
  final String id;
  final String saleId;
  final String inventoryItemId;
  final int quantitySold;
  final double sellingPriceAtSale;
  final double subtotal;
  // ... other fields and fromJson
}

class PaginatedSalesResponse {
  final List<Sale> items;
  final int totalItems;
  final int currentPage;
  // ... other fields and fromJson
}
```

### `Shop` & Related Models

```dart
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
  // ... constructor, fromJson, toJson, etc.
}

class PaginatedAdminShopsResponse {
  final List<Shop> shops;
  final PaginationInfo pagination;
  // ... constructor and fromJson
}

class PaginatedShopResponse {
  final List<Shop> shops;
  final int totalItems;
  // ... other fields
}

class ShopDashboardSummary {
  final String shopId;
  final String shopName;
  final String userName;
  final double salesToday;
  final int transactionsToday;

  // ... constructor and fromJson
}
```

### `Shop/Stock` Related Models

```dart
class ShopInventoryItem {
  final String id;
  final String productId;
  final String name;
  final String? sku;
  final int quantity;
  final double sellingPrice;
  // ... constructor and fromJson
}

class ShopStockItem {
  String id;
  String shopId;
  String inventoryItemId;
  int quantity;
  DateTime lastStockedInAt;
  // Enriched data
  String itemName;
  double itemUnitPrice;
  // ... other fields, constructor, fromJson
}

class PaginatedShopStockResponse {
  final List<ShopStockItem> items;
  final int totalItems;
  // ... other fields
}

class StockMovement {
  final String id;
  final String itemId;
  final String shopId;
  final String movementType;
  final int quantityChanged;
  final int newQuantity;
  final String userId;
  final DateTime movementDate;
  // ... other fields, constructor, fromJson
}
```

### `Staff` & Related Models

```dart
// 'Staff' is a type alias for the standard 'User' model
typedef Staff = User;

class StaffDashboardSummary {
  final String shopName;
  final int salesToday;
  final double totalRevenueToday;
  // ... constructor and fromJson
}

class StaffMemberDetail {
  final String id;
  final String name;
  final String email;
  final String role;
  final String shopId;
  final String shopName;
  // ... other fields and fromJson
}

class StaffProfile {
  final String id;
  final String name;
  final String email;
  final String role;
  final String? shopName;
  final double salary;
  final String payFrequency;
  // ... constructor and fromJson
}
```

### `Supplier` & Related Models

```dart
class Supplier {
  final String? id;
  final String merchantId;
  final String name;
  final String? contactName;
  final String? contactEmail;
  final String? contactPhone;
  final String? address;
  // ... other fields, constructor, fromJson
}

class PaginatedSuppliersResponse {
  final List<Supplier> suppliers;
  final int totalItems;
  // ... pagination fields and fromJson
}
```

### `User` & Related Models

```dart
class User {
  final String id;
  final String name;
  final String email;
  final String role;
  final bool isActive;
  final String? phone;
  final String? assignedShopId;
  final String? merchantId;
  final Shop? shop;
  // ... constructor, fromJson, copyWith
}

class UserProfile {
  final String name;
  final String email;
  final String role;
  final String shopName;
  // ... constructor and fromJson
}

class UserSelectionItem extends Equatable {
  final String id;
  final String name;
  final String? email;
  final String? role;
  // ... constructor, fromJson, fromUser
}
```

### Other Models

```dart
class MerchantStockInRequest {
  final String itemName;
  // ... other fields
}

class PaginatedNotificationsResponse {
  // ... fields
}

class Promotion {
  // ... fields
}

class Receipt {
  // ... fields
}

class Salary {
  // ... fields
}

class PaginationInfo {
    final int totalItems;
    final int currentPage;
    final int pageSize;
    final int totalPages;
    // ... constructor and fromJson
}
```
