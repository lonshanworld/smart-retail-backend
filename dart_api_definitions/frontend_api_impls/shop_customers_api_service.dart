import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/shop_customer_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

// CORRECTED: The class name is plural to match what the controller and binding expect.
class ShopCustomersApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/shops';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a list of customers for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shops/{shopId}/customers`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A list of `ShopCustomer` objects.
  // CORRECTED: Method name is now getCustomers to match the controller.
  Future<List<ShopCustomer>> getCustomers(String shopId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return List.generate(8, (i) => ShopCustomer(
        id: 'cust-$i',
        shopId: shopId,
        name: 'Shop $shopId Customer ${i + 1}',
        email: 'customer${i + 1}@example.com',
        phone: '123-456-789$i',
        merchantId: 'merch-1',
        createdAt: DateTime.now().subtract(Duration(days: i * 5)),
        updatedAt: DateTime.now(),
      ));
    }

    final response = await _connect.get('$_baseUrl/$shopId/customers', headers: await _getHeaders());

    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => ShopCustomer.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load customers');
    }
  }

  /// Creates a new customer record for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/shops/{shopId}/customers`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "New Customer",
  ///     "email": "new.customer@example.com",
  ///     "phone": "555-123-4567"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ The newly created `ShopCustomer` object.
  // CORRECTED: Added the createCustomer method that was missing.
  Future<ShopCustomer> createCustomer(String shopId, Map<String, dynamic> customerData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final newCustomer = ShopCustomer.fromJson({
        ...customerData,
        'id': 'cust-new-${DateTime.now().millisecondsSinceEpoch}',
        'shopId': shopId,
        'merchantId': 'merch-1',
        'createdAt': DateTime.now().toIso8601String(),
        'updatedAt': DateTime.now().toIso8601String(),
      });
      return newCustomer;
    }

    final response = await _connect.post('$_baseUrl/$shopId/customers', customerData, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return ShopCustomer.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create customer');
    }
  }
}
