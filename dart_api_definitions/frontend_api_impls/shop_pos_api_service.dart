import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/models/sale_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class ShopPosApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/shop/pos';

  final List<InventoryItem> _mockProducts = [
    InventoryItem(id: 'prod_001', merchantId: 'mock-merchant', name: 'Espresso', sku: 'BEV-001', sellingPrice: 2.50, originalPrice: 1.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_002', merchantId: 'mock-merchant', name: 'Latte', sku: 'BEV-002', sellingPrice: 3.50, originalPrice: 1.5, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_003', merchantId: 'mock-merchant', name: 'Cappuccino', sku: 'BEV-003', sellingPrice: 3.50, originalPrice: 1.5, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_004', merchantId: 'mock-merchant', name: 'Americano', sku: 'BEV-004', sellingPrice: 3.00, originalPrice: 1.2, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_005', merchantId: 'mock-merchant', name: 'Iced Coffee', sku: 'BEV-005', sellingPrice: 3.75, originalPrice: 1.6, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_006', merchantId: 'mock-merchant', name: 'Herbal Tea', sku: 'BEV-006', sellingPrice: 2.75, originalPrice: 1.1, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_007', merchantId: 'mock-merchant', name: 'Orange Juice', sku: 'BEV-007', sellingPrice: 4.00, originalPrice: 2.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),

    InventoryItem(id: 'prod_101', merchantId: 'mock-merchant', name: 'Croissant', sku: 'PST-001', sellingPrice: 2.95, originalPrice: 1.2, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_102', merchantId: 'mock-merchant', name: 'Chocolate Muffin', sku: 'PST-002', sellingPrice: 3.25, originalPrice: 1.4, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_103', merchantId: 'mock-merchant', name: 'Blueberry Scone', sku: 'PST-003', sellingPrice: 3.50, originalPrice: 1.5, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_104', merchantId: 'mock-merchant', name: 'Banana Bread Slice', sku: 'PST-004', sellingPrice: 2.85, originalPrice: 1.3, createdAt: DateTime.now(), updatedAt: DateTime.now()),

    InventoryItem(id: 'prod_201', merchantId: 'mock-merchant', name: 'Ham & Cheese Sandwich', sku: 'SND-001', sellingPrice: 7.50, originalPrice: 3.5, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_202', merchantId: 'mock-merchant', name: 'Turkey Club Sandwich', sku: 'SND-002', sellingPrice: 8.50, originalPrice: 4.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_203', merchantId: 'mock-merchant', name: 'Veggie Wrap', sku: 'SND-003', sellingPrice: 6.95, originalPrice: 3.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),

    InventoryItem(id: 'prod_301', merchantId: 'mock-merchant', name: 'Bag of Coffee Beans (12oz)', sku: 'MER-001', sellingPrice: 14.99, originalPrice: 8.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_302', merchantId: 'mock-merchant', name: 'Travel Mug', sku: 'MER-002', sellingPrice: 18.00, originalPrice: 10.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_303', merchantId: 'mock-merchant', name: 'Gift Card', sku: 'MER-003', sellingPrice: 25.00, originalPrice: 25.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
  ];

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Searches for products available for sale in the current shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/pos/products`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>` (shop-level token)
  /// - __Query Parameters:__
  ///   - `searchTerm`: `String` (The search query for product name or SKU)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of `InventoryItem` objects matching the search)
  Future<List<InventoryItem>> searchProducts(String searchTerm) async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 300));
      
      if (searchTerm.isEmpty) {
        return _mockProducts;
      }
      
      final lowerCaseSearchTerm = searchTerm.toLowerCase();
      return _mockProducts.where((product) {
        final nameMatch = product.name.toLowerCase().contains(lowerCaseSearchTerm);
        final skuMatch = product.sku?.toLowerCase().contains(lowerCaseSearchTerm) ?? false;
        return nameMatch || skuMatch;
      }).toList();
    }
    // =========================================================================

    final response = await _connect.get(
      '$_baseUrl/products',
      headers: await _getHeaders(),
      query: {'searchTerm': searchTerm},
    );

    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => InventoryItem.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to search products');
    }
  }

  /// Processes a new sale for the current shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/shop/pos/checkout`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>` (shop-level token)
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "items": [
  ///       { "productId": "uuid-item-1", "quantity": 2, "sellingPriceAtSale": 15.0 },
  ///       { "productId": "uuid-item-2", "quantity": 1, "sellingPriceAtSale": 25.0 }
  ///     ],
  ///     "totalAmount": 55.0,
  ///     "paymentType": "cash",
  ///     "customerId": "cust-uuid-1" // Optional
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created `Sale` object)
  Future<Sale> checkout(Map<String, dynamic> saleData) async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final saleId = 'sale-${DateTime.now().millisecondsSinceEpoch}';
      final now = DateTime.now();

      final saleItems = (saleData['items'] as List).map((item) {
        final product = _mockProducts.firstWhere((p) => p.id == item['productId'], orElse: () => throw Exception('Mock product not found: ${item['productId']}'));
        final quantity = item['quantity'] as int;
        final price = item['sellingPriceAtSale'] as double;
        return SaleItem(
          id: 'sale-item-${item['productId']}-${now.microsecondsSinceEpoch}',
          saleId: saleId,
          inventoryItemId: item['productId'] as String,
          quantitySold: quantity,
          sellingPriceAtSale: price,
          originalPriceAtSale: product.originalPrice,
          subtotal: quantity * price,
          createdAt: now,
          updatedAt: now,
          itemName: product.name,
          itemSku: product.sku,
        );
      }).toList();

      return Sale(
        id: saleId,
        merchantId: 'mock-merchant',
        shopId: 'shop-1', // Assuming a mock shop ID
        saleDate: now,
        totalAmount: saleData['totalAmount'],
        items: saleItems,
        paymentType: saleData['paymentType'],
        paymentStatus: 'succeeded',
        createdAt: now,
        updatedAt: now,
      );
    }
    // =========================================================================

    final response = await _connect.post('$_baseUrl/checkout', saleData, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return Sale.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Checkout failed');
    }
  }
}
