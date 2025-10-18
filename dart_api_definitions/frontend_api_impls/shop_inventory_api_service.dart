import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/models/stock_movement_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

// This service handles inventory operations from the MERCHANT'S perspective,
// allowing them to manage stock across different shops.
class ShopInventoryApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    if (token == null) throw Exception('Authentication token not found');
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a list of all shops for the current merchant.
  Future<List<Shop>> getShops() async {
    // In development, return mock data
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      return [
        Shop(id: '1', name: 'Main Street Branch', address: '123 Main St', merchantId: 'mock-merchant-1', createdAt: DateTime.now(), updatedAt: DateTime.now()), 
        Shop(id: '2', name: 'Second Ave Store', address: '456 Second Ave', merchantId: 'mock-merchant-1', createdAt: DateTime.now(), updatedAt: DateTime.now())
      ];
    }

    final response = await _connect.get('${ApiConstants.baseUrl}/merchant/shops', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      final List<dynamic> shopList = response.body['data'];
      return shopList.map((json) => Shop.fromJson(json)).toList();
    }
    throw Exception('Failed to load shops');
  }

  /// Fetches the inventory for a specific shop, including stock quantities.
  Future<List<InventoryItem>> getInventoryForShop(String shopId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return List.generate(5, (i) => InventoryItem(id: 'item-$i', merchantId: 'merch-1', name: 'Product $i from Shop $shopId', sellingPrice: 10.0 + i, createdAt: DateTime.now(), updatedAt: DateTime.now(), stock: StockInfo(quantity: 20 + i * 5, shopId: shopId)));
    }
    
    final response = await _connect.get('${ApiConstants.baseUrl}/merchant/shops/$shopId/inventory', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      final List<dynamic> inventoryList = response.body['data'];
      return inventoryList.map((json) => InventoryItem.fromJson(json)).toList();
    }
    throw Exception('Failed to load inventory for shop');
  }

  /// Adjusts the stock of a specific item in a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/shops/{shopId}/inventory/{itemId}/adjust`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "quantity": -5,
  ///     "reason": "Damaged Goods"
  ///   }
  ///   ```
  Future<void> adjustStock({
    required String shopId,
    required String itemId,
    required int quantity,
    required String reason,
  }) async {
    if (_appConfig.isDevelopment) {
       await Future.delayed(const Duration(seconds: 1));
       print('Mock stock adjustment: shop=$shopId, item=$itemId, qty=$quantity, reason=$reason');
       return;
    }

    final response = await _connect.post(
      '${ApiConstants.baseUrl}/merchant/shops/$shopId/inventory/$itemId/adjust',
      {
        'quantity': quantity,
        'reason': reason,
      },
      headers: await _getHeaders(),
    );
    if (!response.isOk) {
      throw Exception('Failed to adjust stock: ${response.body?['message'] ?? response.statusText}');
    }
  }

  /// Fetches the stock movement history for a specific item in a specific shop.
  Future<List<StockMovement>> getMovementHistory(String shopId, String itemId) async {
     if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return [StockMovement(id: '1', shopId: shopId, itemId: itemId, userId: 'user-1', movementType: 'inventory_correction', quantityChanged: -2, newQuantity: 18, reason: 'Damaged', notes: 'Box was wet', movementDate: DateTime.now())];
    }

    final response = await _connect.get('${ApiConstants.baseUrl}/merchant/shops/$shopId/inventory/$itemId/history', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      final List<dynamic> historyList = response.body['data'];
      return historyList.map((json) => StockMovement.fromJson(json)).toList();
    }
    throw Exception('Failed to load stock movement history');
  }
}
