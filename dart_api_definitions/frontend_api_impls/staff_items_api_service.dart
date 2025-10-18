import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class StaffItemsApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/shop/items';

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

  /// Fetches a list of all available product items for the assigned shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/items`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A list of `InventoryItem` objects.
  Future<List<InventoryItem>> getItems() async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return _mockProducts;
    }
    // =========================================================================

    final response = await _connect.get(_baseUrl, headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => InventoryItem.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load items');
    }
  }
}
