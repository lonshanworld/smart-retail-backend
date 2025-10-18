import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class PaginatedStaffResponse {
  final List<User> staff;
  final int totalCount;
  final int currentPage;
  final int pageSize;
  final int totalPages;

  PaginatedStaffResponse({
    required this.staff,
    required this.totalCount,
    required this.currentPage,
    required this.pageSize,
    required this.totalPages,
  });

  factory PaginatedStaffResponse.fromJson(Map<String, dynamic> json) {
    return PaginatedStaffResponse(
      staff: (json['users'] as List).map((i) => User.fromJson(i)).toList(),
      totalCount: json['totalCount'],
      currentPage: json['currentPage'],
      pageSize: json['pageSize'],
      totalPages: json['totalPages'],
    );
  }
}

class MerchantStaffApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/staff';
  String get _shopsBaseUrl => '${ApiConstants.baseUrl}/merchant/shops';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a paginated list of staff for the current merchant.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/staff`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int` (The page number to fetch)
  ///   - `pageSize`: `int` (The number of items per page)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "status": "success",
  ///     "data": {
  ///       "users": [
  ///         {
  ///           "id": "uuid-staff-1",
  ///           "name": "John Doe",
  ///           "email": "john.doe@example.com",
  ///           "role": "staff",
  ///           "isActive": true,
  ///           "merchantId": "uuid-merchant-1",
  ///           "createdAt": "2023-01-01T12:00:00Z"
  ///         }
  ///       ],
  ///       "totalCount": 1,
  ///       "currentPage": 1,
  ///       "pageSize": 10,
  ///       "totalPages": 1
  ///     }
  ///   }
  ///   ```
  Future<PaginatedStaffResponse> listStaff({int page = 1, int pageSize = 10}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final mockStaff = List.generate(
        15,
        (index) => User(
          id: 'staff-$index',
          name: 'Staff Member $index',
          email: 'staff$index@shop.com',
          role: 'staff',
          isActive: index % 4 != 0, // Make some inactive
          merchantId: 'mock-merchant-id',
          assignedShopId: (index % 3 == 0) ? 'shop-${index % 3}' : null, // Assign some staff to shops
          createdAt: DateTime.now().subtract(Duration(days: index)),
          updatedAt: DateTime.now(),
        ),
      );
      return PaginatedStaffResponse(
        staff: mockStaff,
        totalCount: mockStaff.length,
        currentPage: 1,
        pageSize: 10,
        totalPages: 2,
      );
    }

    final response = await _connect.get(
      _baseUrl,
      headers: await _getHeaders(),
      query: {'page': page.toString(), 'pageSize': pageSize.toString()},
    );

    if (response.isOk && response.body['data'] != null) {
      return PaginatedStaffResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load staff');
    }
  }

  /// Fetches a list of all shops for the current merchant (for selection).
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/shops`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of `Shop` objects)
  Future<List<Shop>> getShopsForSelection() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      return List.generate(5, (index) => Shop(
        id: 'shop-$index',
        name: 'Shop Branch $index',
        address: '$index Main St',
        merchantId: 'mock-merchant-id',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ));
    }
    final response = await _connect.get(_shopsBaseUrl, headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => Shop.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load shops');
    }
  }

  /// Creates a new staff member for the current merchant.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/staff`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "Jane Doe",
  ///     "email": "jane.doe@example.com",
  ///     "password": "securePassword123",
  ///     "assignedShopId": "uuid-shop-1" // Optional
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created User object for the staff member)
  Future<User> createStaff(Map<String, dynamic> data) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      if (data['email'] == 'fail@test.com') {
        throw Exception('Mock Error: This email is already taken.');
      }
      final newUser = User(
        id: 'new-staff-${DateTime.now().millisecondsSinceEpoch}',
        name: data['name'],
        email: data['email'],
        role: 'staff',
        merchantId: 'mock-merchant-id',
        assignedShopId: data['assignedShopId'],
        isActive: true,
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      );
      return newUser;
    }

    final response = await _connect.post(_baseUrl, data, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create staff member');
    }
  }

  /// Updates an existing staff member.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/staff/{staffId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Fields to update)
  ///   ```json
  ///   {
  ///     "name": "Jane D. Smith", // Optional
  ///     "isActive": false, // Optional
  ///     "assignedShopId": "uuid-shop-2" // Optional
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated User object)
  Future<User> updateStaff(String staffId, Map<String, dynamic> data) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User(
        id: staffId,
        name: data['name'] ?? 'Updated Staff Name',
        email: 'staff@test.com',
        role: 'staff',
        isActive: data['isActive'] ?? true,
        merchantId: 'mock-merchant-id',
        assignedShopId: data['assignedShopId'],
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      );
    }

    final response = await _connect.put('$_baseUrl/$staffId', data, headers: await _getHeaders());

    if (response.isOk && response.body['data'] != null) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to update staff member');
    }
  }

  /// Deletes a staff member.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/merchant/staff/{staffId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200 or 204
  Future<void> deleteStaff(String staffId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      // Simulate success
      return;
    }

    final response = await _connect.delete('$_baseUrl/$staffId', headers: await _getHeaders());

    if (!response.isOk) {
      throw Exception(response.body?['message'] ?? 'Failed to delete staff member');
    }
  }
}
