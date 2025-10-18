import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/models/sale_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class MerchantPosApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  final Map<String, List<InventoryItem>> _mockProductsByShop = {
    'shop-0': [
      InventoryItem(id: 'prod_001', merchantId: 'mock-merchant', name: 'Espresso', sku: 'BEV-001', sellingPrice: 2.50, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      InventoryItem(id: 'prod_002', merchantId: 'mock-merchant', name: 'Latte', sku: 'BEV-002', sellingPrice: 3.50, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      InventoryItem(id: 'prod_101', merchantId: 'mock-merchant', name: 'Croissant', sku: 'PST-001', sellingPrice: 2.95, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      InventoryItem(id: 'prod_201', merchantId: 'mock-merchant', name: 'Ham & Cheese Sandwich', sku: 'SND-001', sellingPrice: 7.50, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    ],
    'shop-1': [
      InventoryItem(id: 'prod_102', merchantId: 'mock-merchant', name: 'Chocolate Muffin', sku: 'PST-002', sellingPrice: 3.25, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      InventoryItem(id: 'prod_103', merchantId: 'mock-merchant', name: 'Blueberry Scone', sku: 'PST-003', sellingPrice: 3.50, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      InventoryItem(id: 'prod_301', merchantId: 'mock-merchant', name: 'Bag of Coffee Beans (12oz)', sku: 'MER-001', sellingPrice: 14.99, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    ],
    'shop-2': [], // An empty shop for testing
  };

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/pos';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Searches for products available for sale in a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/pos/products`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `shopId`: `String` (The ID of the shop to search in)
  ///   - `searchTerm`: `String` (The search query for product name or SKU)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of `InventoryItem` objects matching the search)
  Future<List<InventoryItem>> searchProducts(String shopId, String searchTerm) async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 400));
      final productsForShop = _mockProductsByShop[shopId] ?? [];
      
      if (searchTerm.isEmpty) {
        return productsForShop;
      }
      
      return productsForShop
          .where((p) => 
              p.name.toLowerCase().contains(searchTerm.toLowerCase()) || 
              (p.sku?.toLowerCase().contains(searchTerm.toLowerCase()) ?? false))
          .toList();
    }
    // =========================================================================

    final response = await _connect.get(
      '$_baseUrl/products',
      headers: await _getHeaders(),
      query: {'shopId': shopId, 'searchTerm': searchTerm},
    );

    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => InventoryItem.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to search products');
    }
  }

  /// Processes a new sale for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/pos/checkout`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   '''json
  ///   {
  ///     "shopId": "uuid-shop-1",
  ///     "items": [
  ///       { "productId": "uuid-item-1", "quantity": 2, "sellingPriceAtSale": 15.0 },
  ///       { "productId": "uuid-item-2", "quantity": 1, "sellingPriceAtSale": 25.0 }
  ///     ],
  ///     "totalAmount": 55.0,
  ///     "paymentType": "cash"
  ///   }
  ///   '''
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created `Sale` object)
  Future<Sale> checkout(String shopId, Map<String, dynamic> saleData) async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));

      final saleId = 'sale-${DateTime.now().millisecondsSinceEpoch}';
      final now = DateTime.now();
      
      final saleItems = (saleData['items'] as List).map((item) {
        final quantity = item['quantity'] as int;
        final price = item['sellingPriceAtSale'] as double;
        return SaleItem(
          id: 'sale-item-${item['productId']}-${now.microsecondsSinceEpoch}',
          saleId: saleId,
          inventoryItemId: item['productId'] as String,
          quantitySold: quantity,
          sellingPriceAtSale: price,
          subtotal: quantity * price,
          createdAt: now,
          updatedAt: now,
        );
      }).toList();

      return Sale(
        id: saleId,
        merchantId: 'mock-merchant',
        shopId: shopId,
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

    final response = await _connect.post('$_baseUrl/checkout', saleData..['shopId'] = shopId, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return Sale.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Checkout failed');
    }
  }
}
