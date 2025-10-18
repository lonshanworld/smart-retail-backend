import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/customer_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class CustomerApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/customers';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Searches for customers by name, email, or phone number.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/customers/search`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `query`: `string` (The search term)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "success": true,
  ///     "data": [
  ///       {
  ///         "id": "uuid-customer-1",
  ///         "shopId": "uuid-shop-1",
  ///         "name": "John Doe",
  ///         "phone": "1234567890",
  ///         "email": "john.doe@example.com",
  ///         "createdAt": "2023-01-15T10:00:00Z"
  ///       }
  ///     ]
  ///   }
  ///   ```
  ///
  /// __Dummy Response Data (for testing):__
  /// ```dart
  /// final dummyCustomers = [
  ///   Customer(
  ///     id: 'uuid-customer-1',
  ///     shopId: 'uuid-shop-1',
  ///     name: 'John Doe',
  ///     phone: '1234567890',
  ///     email: 'john.doe@example.com',
  ///     createdAt: DateTime.parse("2023-01-15T10:00:00Z"),
  ///   )
  /// ];
  /// ```
  Future<List<Customer>> searchCustomers(String query, {required String shopId}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      if (query.isEmpty) return [];
      return [
        Customer(
          id: 'uuid-customer-1',
          shopId: shopId, // CORRECTED
          name: 'John Doe',
          phone: '1234567890',
          email: 'john.doe@example.com',
          createdAt: DateTime.parse("2023-01-15T10:00:00Z"),
        )
      ];
    }
    final response = await _connect.get(
      '$_baseUrl/search',
      headers: await _getHeaders(),
      query: {'query': query, 'shopId': shopId}, // CORRECTED: Added shopId to query
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      final List<dynamic> data = response.body['data'];
      return data.map((json) => Customer.fromJson(json)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to search customers');
    }
  }

  /// Creates a new customer for the merchant.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/customers`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///   - `Content-Type: application/json`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "shopId": "uuid-shop-1",
  ///     "name": "Jane Doe",
  ///     "phone": "0987654321",
  ///     "email": "jane.doe@example.com"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "success": true,
  ///     "data": {
  ///       "id": "uuid-customer-2",
  ///       "shopId": "uuid-shop-1",
  ///       "name": "Jane Doe",
  ///       "phone": "0987654321",
  ///       "email": "jane.doe@example.com",
  ///       "createdAt": "2023-10-28T10:00:00Z"
  ///     }
  ///   }
  ///   ```
  ///
  /// __Dummy Response Data (for testing):__
  /// ```dart
  /// final newCustomer = Customer(
  ///   id: 'uuid-customer-2',
  ///   shopId: 'uuid-shop-1',
  ///   name: 'Jane Doe',
  ///   phone: '0987654321',
  ///   email: 'jane.doe@example.com',
  ///   createdAt: DateTime.parse("2023-10-28T10:00:00Z"),
  /// );
  /// ```
  Future<Customer> createCustomer({required String shopId, required String name, String? phone, String? email}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Customer(
        id: 'uuid-customer-2',
        shopId: shopId,
        name: name,
        phone: phone,
        email: email,
        createdAt: DateTime.now(),
      );
    }
    final response = await _connect.post(
      _baseUrl,
      {
        'shopId': shopId, // CORRECTED: Added shopId to body
        'name': name,
        'phone': phone,
        'email': email,
      },
      headers: await _getHeaders(),
    );

    if (response.statusCode == 201 && response.body['success'] == true) {
      return Customer.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create customer');
    }
  }
}
