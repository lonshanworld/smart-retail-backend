import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/models/sale_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class StaffPosApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/staff/pos';

  final List<InventoryItem> _mockProducts = [
    InventoryItem(id: 'prod_001', merchantId: 'mock-merchant', name: 'Espresso', sku: 'BEV-001', sellingPrice: 2.50, originalPrice: 1.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_002', merchantId: 'mock-merchant', name: 'Latte', sku: 'BEV-002', sellingPrice: 3.50, originalPrice: 1.5, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    InventoryItem(id: 'prod_101', merchantId: 'mock-merchant', name: 'Croissant', sku: 'PST-001', sellingPrice: 2.95, originalPrice: 1.2, createdAt: DateTime.now(), updatedAt: DateTime.now()),
  ];

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Searches for products available for sale in the staff member's assigned shop.
  /// The backend uses the token to identify the shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/pos/products`
  /// - __Query Parameters:__
  ///   - `searchTerm`: `String`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A list of `InventoryItem` objects.
  Future<List<InventoryItem>> searchProducts(String searchTerm) async {
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

  /// Processes a new sale for the staff member's assigned shop.
  /// The backend uses the token to identify the shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/staff/pos/checkout`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "items": [
  ///       { "productId": "uuid-item-1", "quantity": 2, "sellingPriceAtSale": 15.0 },
  ///       { "productId": "uuid-item-2", "quantity": 1, "sellingPriceAtSale": 25.0 }
  ///     ],
  ///     "totalAmount": 55.0,
  ///     "paymentType": "cash"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ The newly created `Sale` object.
  Future<Sale> checkout(Map<String, dynamic> saleData) async {
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

    final response = await _connect.post('$_baseUrl/checkout', saleData, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return Sale.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Checkout failed');
    }
  }
}
