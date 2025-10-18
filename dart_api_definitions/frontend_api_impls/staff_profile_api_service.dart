import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/staff_profile_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class StaffProfileApiService extends GetxService {
  final AppConfig _appConfig = Get.find<AppConfig>();
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();

  /// Fetches the detailed profile for the currently logged-in staff member.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/profile`
  ///
  /// __Response (Success):__
  /// - __Status:__ 200 OK
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "status": "success",
  ///     "data": {
  ///       "id": "staff_789",
  ///       "name": "Jane Smith",
  ///       "email": "jane.smith@example.com",
  ///       "role": "staff",
  ///       "shopName": "Downtown Branch",
  ///       "salary": 50000.0,
  ///       "payFrequency": "monthly"
  ///     }
  ///   }
  ///   ```
  Future<StaffProfile> getStaffProfile() async {
    if (_appConfig.isDevelopment) {
      // Mock data for development
      await Future.delayed(const Duration(milliseconds: 700));
      return StaffProfile(
        id: 'mock-staff-123',
        name: 'Jane Doe (Staff)',
        email: 'jane.doe@example.com',
        role: 'Staff',
        shopName: 'Main Street Branch',
        salary: 45000.00,
        payFrequency: 'Bi-Weekly',
      );
    }

    final token = await _authService.getToken();
    final response = await _connect.get(
      '${ApiConstants.baseUrl}/staff/profile',
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.isOk && response.body['data'] != null) {
      return StaffProfile.fromJson(response.body['data']);
    } else {
      throw Exception('Failed to load staff profile: ${response.body?['message'] ?? response.statusText}');
    }
  }
}
