import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class MerchantShopsApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  // Manages the mock list locally to ensure data persists across hot reloads.
  final List<Shop> _mockShops = [];

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/shops';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a list of shops for the current merchant.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/shops`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "status": "success",
  ///     "data": [
  ///       {
  ///         "id": "uuid-shop-1",
  ///         "name": "Main Street Branch",
  ///         "address": "123 Main St",
  ///         "merchantId": "uuid-merchant-1",
  ///         "createdAt": "2023-01-01T12:00:00Z",
  ///         "updatedAt": "2023-01-01T12:00:00Z"
  ///       }
  ///     ]
  ///   }
  ///   ```
  Future<List<Shop>> listShops() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      if (_mockShops.isEmpty) {
        _mockShops.addAll(List.generate(
          5,
          (index) => Shop(
            id: 'shop-$index',
            name: 'Shop Branch $index',
            address: '$index Branch Street, City',
            merchantId: 'mock-merchant-id',
            createdAt: DateTime.now().subtract(Duration(days: index * 10)),
            updatedAt: DateTime.now().subtract(Duration(days: index * 10)),
          ),
        ));
      }
      return _mockShops;
    }

    final response = await _connect.get(_baseUrl, headers: await _getHeaders());

    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => Shop.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load shops');
    }
  }

  /// Creates a new shop for the current merchant.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/shops`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "New Outlet",
  ///     "address": "456 Market St"
  ///   }
  ///   ```
  Future<Shop> createShop(String name, String address) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final newShop = Shop(
        id: 'shop-${DateTime.now().millisecondsSinceEpoch}',
        name: name,
        address: address,
        merchantId: 'mock-merchant-id',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      );
      _mockShops.add(newShop);
      return newShop;
    }

    final response = await _connect.post(
      _baseUrl,
      {'name': name, 'address': address},
      headers: await _getHeaders(),
    );

    if (response.isOk && response.body['data'] != null) {
      return Shop.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create shop');
    }
  }

  /// Updates an existing shop.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/shops/{shopId}`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "Updated Outlet Name",
  ///     "address": "789 Commerce Ave"
  ///   }
  ///   ```
  Future<Shop> updateShop(String shopId, String name, String address) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final index = _mockShops.indexWhere((s) => s.id == shopId);
      if (index != -1) {
        final updatedShop = Shop(
          id: shopId,
          name: name,
          address: address,
          merchantId: _mockShops[index].merchantId,
          createdAt: _mockShops[index].createdAt,
          updatedAt: DateTime.now(),
        );
        _mockShops[index] = updatedShop;
        return updatedShop;
      }
      throw Exception('Mock shop not found');
    }

    final response = await _connect.put(
      '$_baseUrl/$shopId',
      {'name': name, 'address': address},
      headers: await _getHeaders(),
    );

    if (response.isOk && response.body['data'] != null) {
      return Shop.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to update shop');
    }
  }

  /// Deletes a shop by its ID.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/merchant/shops/{shopId}`
  Future<void> deleteShop(String shopId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      _mockShops.removeWhere((s) => s.id == shopId);
      return;
    }

    final response = await _connect.delete('$_baseUrl/$shopId', headers: await _getHeaders());

    if (!response.isOk) {
      throw Exception(response.body?['message'] ?? 'Failed to delete shop');
    }
  }
}
