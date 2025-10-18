import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/models/admin_dashboard_summary_model.dart'; // New
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class AdminApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/admin';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches the summary data for the admin dashboard.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/dashboard/summary`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "totalMerchants": 10,
  ///     "activeMerchants": 8,
  ///     "totalStaff": 50,
  ///     "activeStaff": 45,
  ///     "totalShops": 20,
  ///     "totalSalesValue": 150000.00,
  ///     "salesToday": 5000.00,
  ///     "transactionsToday": 100
  ///   }
  ///   ```
  Future<AdminDashboardSummary?> getAdminDashboardSummary() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return AdminDashboardSummary(
        totalMerchants: 10,
        activeMerchants: 8,
        totalStaff: 50,
        activeStaff: 45,
        totalShops: 20,
        totalSalesValue: 150000.00,
        salesToday: 5000.00,
        transactionsToday: 100,
      );
    }
    final response = await _connect.get(
      '$_baseUrl/dashboard/summary',
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return AdminDashboardSummary.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to get admin dashboard summary');
    }
  }

  /// Updates a user as an admin.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "Updated Name"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated user object)
  Future<User?> adminUpdateUser(String userId, Map<String, dynamic> updates) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User.fromJson(updates..['id'] = userId..['email'] = 'mock@email.com'..['role'] = 'merchant');
    }
    final response = await _connect.put(
      '$_baseUrl/users/$userId',
      updates,
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to update user');
    }
  }
}
