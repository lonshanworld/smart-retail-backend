import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

// Represents a single item in a stock-in request
class StockInItem {
  final String productId;
  final int quantity;

  StockInItem({required this.productId, required this.quantity});

  Map<String, dynamic> toJson() => {
    'productId': productId,
    'quantity': quantity,
  };
}

class StockInApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/inventory/stock-in';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Performs a stock-in operation for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/inventory/stock-in`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "shopId": "uuid-shop-1",
  ///     "items": [
  ///       { "productId": "uuid-product-1", "quantity": 50 },
  ///       { "productId": "uuid-product-2", "quantity": 30 }
  ///     ]
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200 or 204
  /// - __Body:__ (Potentially a success message or empty)
  Future<void> performStockIn(String shopId, List<StockInItem> items) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 2));
      // Simulate a successful stock-in
      print('Mock Stock-In for shop $shopId with ${items.length} items.');
      return;
    }

    final body = {
      'shopId': shopId,
      'items': items.map((item) => item.toJson()).toList(),
    };

    final response = await _connect.post(_baseUrl, body, headers: await _getHeaders());

    if (!response.isOk) {
      throw Exception(response.body?['message'] ?? 'Failed to perform stock-in');
    }
  }
}
