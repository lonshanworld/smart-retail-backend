import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class ShopProfileApiService extends GetxService {
  final AppConfig _appConfig = Get.find<AppConfig>();
  final AuthService _authService = Get.find<AuthService>();

  /// Fetches the profile of the currently authenticated user.
  /// This is a view-only profile for the shop dashboard.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/profile`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The User object)
  Future<User> getUserProfile() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      // Return the currently logged-in user from the auth service for the mock.
      final currentUser = _authService.user.value;
      if (currentUser != null) {
        return currentUser;
      }
      // Fallback mock user if auth service has no user
      return User(
        id: 'user-mock-123',
        name: 'Mock Staff User',
        email: 'staff.mock@example.com',
        role: 'staff',
        assignedShopId: 'shop-0', // CORRECTED
      );
    }

    // In a real implementation, you would make an API call here.
    // For now, we will rely on the auth service's user data.
    final currentUser = _authService.user.value;
    if (currentUser != null) {
      return currentUser;
    } else {
      throw Exception('User not authenticated');
    }
  }
}
