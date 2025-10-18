import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';

/// AdminAdminsApiService - Service for managing admin users.
///
/// This service handles fetching, creating, updating, and deleting admin users
/// through API calls. It will use mock data for development environments.
class AdminAdminsApiService extends GetConnect {
  final AppConfig _appConfig = Get.find<AppConfig>();
  
  /// Fetches a paginated list of admin users.
  ///
  /// GET /api/admin/admins
  /// Query Parameters:
  ///   - page: int (e.g., 1)
  ///   - limit: int (e.g., 20)
  ///   - sortBy: string (e.g., "createdAt:desc")
  ///
  /// Expected Success Response (200 OK):
  /// {
  ///   "data": [
  ///     {
  ///       "id": "admin-uuid-1",
  ///       "name": "Super Admin",
  ///       "email": "super@admin.com",
  ///       "role": "admin",
  ///       "isActive": true,
  ///       "createdAt": "2023-01-01T12:00:00Z"
  ///     },
  ///     // ... other admin users
  ///   ],
  ///   "pagination": {
  ///     "totalItems": 2,
  ///     "totalPages": 1,
  ///     "currentPage": 1
  ///   }
  /// }
  Future<List<User>> getAdmins({int page = 1, int limit = 20}) async {
    if (_appConfig.isDevelopment) {
      return Future.delayed(const Duration(seconds: 1), () => _mockAdmins);
    }
    
    final response = await get('/api/admin/admins?page=$page&limit=$limit');
    if (response.isOk && response.body != null && response.body['data'] != null) {
      return (response.body['data'] as List)
          .map((adminJson) => User.fromJson(adminJson))
          .toList();
    } else {
      throw Exception('Failed to load admins: ${response.statusText}');
    }
  }

  // Mock data for development
  final List<User> _mockAdmins = [
    User(
      id: 'mock-admin-1',
      name: 'Alice Admin',
      email: 'alice.admin@example.com',
      role: 'admin',
      isActive: true,
      createdAt: DateTime.now().subtract(const Duration(days: 10)),
    ),
    User(
      id: 'mock-admin-2',
      name: 'Bob SuperAdmin',
      email: 'bob.super@example.com',
      role: 'admin',
      isActive: false,
      createdAt: DateTime.now().subtract(const Duration(days: 5)),
    ),
  ];
}
