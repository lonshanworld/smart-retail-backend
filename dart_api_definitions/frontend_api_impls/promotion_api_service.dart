import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/promotion_model.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class PromotionApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _promotionsUrl => '${ApiConstants.baseUrl}/merchant/promotions';
  String get _shopsUrl => '${ApiConstants.baseUrl}/merchant/shops';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {'Authorization': 'Bearer $token', 'Content-Type': 'application/json'};
  }

  /// Fetches shops for the merchant.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/shops`
  Future<List<Shop>> getShops() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 400));
      return List.generate(3, (i) => Shop(id: 'shop-$i', name: 'Shop Branch $i', merchantId: 'merch-1', createdAt: DateTime.now(), updatedAt: DateTime.now()));
    }
    final response = await _connect.get(_shopsUrl, headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((json) => Shop.fromJson(json)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load shops');
    }
  }

  /// Fetches inventory items for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/shops/{shopId}/products`
  Future<List<InventoryItem>> getProductsForShop(String shopId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 600));
      return List.generate(10, (i) => InventoryItem(id: 'prod-$i', name: 'Product $i from Shop $shopId', sku: 'SKU$i', merchantId: 'merch-1', sellingPrice: 10.0 + i, originalPrice: 7.0 + i, createdAt: DateTime.now(), updatedAt: DateTime.now()));
    }
    final response = await _connect.get('$_shopsUrl/$shopId/products', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((json) => InventoryItem.fromJson(json)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load products for shop');
    }
  }

  /// Fetches a paginated list of promotions.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/promotions`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int`
  ///   - `pageSize`: `int`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A paginated response of promotion objects)
  Future<PaginatedPromotionsResponse> getPromotions({int page = 1, int pageSize = 10}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final items = [
        Promotion(id: 'promo-1', name: '20% Off Laptops', description: 'Discount on all laptops', type: 'percentage', value: 20, startDate: DateTime.now(), endDate: DateTime.now().add(const Duration(days: 10)), shopId: 'shop-0', merchantId: 'merch-1', minSpend: 0, conditions: {}, isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        Promotion(id: 'promo-2', name: 'Buy One Get One Free', description: 'On select T-shirts', type: 'bogo', value: 1, startDate: DateTime.now(), endDate: DateTime.now().add(const Duration(days: 5)), shopId: 'shop-1', merchantId: 'merch-1', minSpend: 0, conditions: {}, isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      ];
      return PaginatedPromotionsResponse(items: items, totalItems: items.length, currentPage: 1, totalPages: 1);
    }
    final response = await _connect.get(
      _promotionsUrl,
      headers: await _getHeaders(),
      query: {'page': page.toString(), 'pageSize': pageSize.toString()},
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return PaginatedPromotionsResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load promotions');
    }
  }

  /// Creates a new promotion.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/promotions`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "10% Off",
  ///     "description": "10% off on all items",
  ///     "type": "percentage",
  ///     "value": 10,
  ///     "startDate": "2023-11-01T00:00:00Z",
  ///     "endDate": "2023-11-30T23:59:59Z",
  ///     "shopId": "uuid-shop-1",
  ///     "productId": "uuid-product-1"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created promotion object)
  Future<Promotion> createPromotion(Map<String, dynamic> data) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Promotion.fromJson(data..['id'] = 'new-promo-id'..['merchantId'] = '1'..['createdAt'] = DateTime.now().toIso8601String()..['updatedAt'] = DateTime.now().toIso8601String());
    }
    final response = await _connect.post(
      _promotionsUrl,
      data,
      headers: await _getHeaders(),
    );

    if (response.statusCode == 201 && response.body['success'] == true) {
      return Promotion.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create promotion');
    }
  }

  /// Updates an existing promotion.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/promotions/{id}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Fields to be updated)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated promotion object)
  Future<Promotion> updatePromotion(String id, Map<String, dynamic> data) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Promotion.fromJson(data..['id'] = id..['merchantId'] = '1'..['createdAt'] = DateTime.now().toIso8601String()..['updatedAt'] = DateTime.now().toIso8601String());
    }
    final response = await _connect.put(
      '$_promotionsUrl/$id',
      data,
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return Promotion.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to update promotion');
    }
  }
}
