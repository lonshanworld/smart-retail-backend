import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class MerchantProfileApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/profile';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches the current merchant's profile.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/profile`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The full user object for the merchant)
  Future<User> getMyProfile() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      final currentUser = _authService.user.value;
      if (currentUser == null || currentUser.role != 'merchant') {
        throw Exception('Mock Error: No merchant user is currently logged in.');
      }
      return currentUser;
    }

    final response = await _connect.get(_baseUrl, headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load profile');
    }
  }

  /// Updates the current merchant's profile (name and/or password).
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/profile`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "New Merchant Name", // Optional
  ///     "password": "new_secure_password" // Optional
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated user object)
  Future<User> updateMyProfile(Map<String, dynamic> updates) async {
    // Ensure no empty values are sent for optional fields
    updates.removeWhere((key, value) => value == null || (value is String && value.isEmpty));
    
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final currentUser = _authService.user.value;
      if (currentUser == null) {
        throw Exception('Mock Error: No merchant user found to update.');
      }
      // Simulate update
      final updatedUser = User.fromJson(currentUser.toJson()..addAll(updates));
      _authService.user.value = updatedUser; // Also update in auth service
      return updatedUser;
    }

    final response = await _connect.put(
      _baseUrl,
      updates,
      headers: await _getHeaders(),
    );

    if (response.isOk && response.body['data'] != null) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to update profile');
    }
  }
}
