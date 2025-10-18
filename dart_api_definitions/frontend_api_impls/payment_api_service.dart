import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/sale_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class PaymentApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/payments';

  Future<Map<String, String>> _getHeaders() async {
    final token = _authService.authToken.value;
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Creates a Stripe Payment Intent for a given set of items.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/payments/create-intent`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///   - `Content-Type: application/json`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "shopId": "uuid-shop-1",
  ///     "items": [
  ///       { "inventoryItemId": "uuid-item-1", "quantitySold": 2 },
  ///       { "inventoryItemId": "uuid-item-2", "quantitySold": 1 }
  ///     ],
  ///     "customerId": "uuid-customer-1" // Optional
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "success": true,
  ///     "data": {
  ///       "clientSecret": "pi_..._secret_..."
  ///     }
  ///   }
  ///   ```
  Future<String> createPaymentIntent(List<SaleItemInput> items, String shopId, {String? customerId}) async {
    // =========================================================================
    // MOCK IMPLEMENTATION
    // =========================================================================
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      // In a real scenario, this would be a unique secret from Stripe
      return 'pi_mock_secret_${DateTime.now().millisecondsSinceEpoch}';
    }
    // =========================================================================

    final payload = {
      'shopId': shopId,
      'items': items.map((item) => item.toJson()).toList(),
      if (customerId != null) 'customerId': customerId,
    };

    final response = await _connect.post(
      '$_baseUrl/create-intent',
      payload,
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return response.body['data']['clientSecret'];
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create payment intent');
    }
  }
}
