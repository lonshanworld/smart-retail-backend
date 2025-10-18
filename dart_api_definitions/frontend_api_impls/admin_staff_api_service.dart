import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

/// AdminStaffApiService - Service for managing all staff users from the admin perspective.
class AdminStaffApiService extends GetConnect {
  final AppConfig _appConfig = Get.find<AppConfig>();
  final AuthService _authService = Get.find<AuthService>();

  String get _baseUrl => '${ApiConstants.baseUrl}/admin/staff';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }
  
  /// Fetches a paginated list of all staff users across all merchants.
  ///
  /// GET /api/admin/staff
  /// Query Parameters:
  ///   - page: int
  ///   - limit: int
  ///   - sortBy: string
  ///
  /// Expected Success Response (200 OK):
  /// {
  ///   "data": [
  ///     {
  ///       "id": "staff-uuid-1",
  ///       "name": "John Doe",
  ///       "email": "john.doe@merchant.com",
  ///       "role": "staff",
  ///       "isActive": true,
  ///       "merchantId": "merchant-uuid-1",
  ///       "merchantName": "Best Mart",
  ///       "createdAt": "2023-01-10T10:00:00Z"
  ///     },
  ///     // ... other staff users
  ///   ],
  ///   "pagination": { ... }
  /// }
  Future<List<User>> getAllStaff({int page = 1, int limit = 20}) async {
    if (_app_config.isDevelopment) {
      return Future.delayed(const Duration(seconds: 1), () => _mockStaff);
    }
    
    final response = await get(_baseUrl, headers: await _getHeaders());
    if (response.isOk && response.body != null && response.body['data'] != null) {
      return (response.body['data'] as List)
          .map((staffJson) => User.fromJson(staffJson))
          .toList();
    } else {
      throw Exception('Failed to load staff: ${response.statusText}');
    }
  }

  // Mock data for development
  final List<User> _mockStaff = [
    User(
      id: 'mock-staff-1',
      name: 'Carla Staff',
      email: 'carla@merchant-a.com',
      role: 'staff',
      isActive: true,
      merchantId: 'merchant-1',
      merchantName: 'Best Mart',
      createdAt: DateTime.now().subtract(const Duration(days: 20)),
    ),
    User(
      id: 'mock-staff-2',
      name: 'David Employee',
      email: 'david@merchant-b.com',
      role: 'staff',
      isActive: true,
      merchantId: 'merchant-2',
      merchantName: 'Quick Stop',
      createdAt: DateTime.now().subtract(const Duration(days: 15)),
    ),
     User(
      id: 'mock-staff-3',
      name: 'Frank Worker',
      email: 'frank@merchant-a.com',
      role: 'staff',
      isActive: false,
      merchantId: 'merchant-1',
      merchantName: 'Best Mart',
      createdAt: DateTime.now().subtract(const Duration(days: 12)),
    ),
  ];
}
