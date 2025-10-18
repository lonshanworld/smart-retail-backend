import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/models/user_profile_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';
import 'package:smart_retail/app/routes/app_pages.dart';
import 'package:smart_retail/app/data/models/admin_paginated_users_response.dart'; // <<< ADDED
import 'package:smart_retail/app/data/models/user_selection_item.dart'; // <<< ADDED

class UserApiService extends GetxService {
  final _connect = GetConnect(timeout: const Duration(seconds: 30));
  late final AuthService _authService;
  final AppConfig _appConfig = Get.find<AppConfig>();

  @override
  void onInit() {
    super.onInit();
    _authService = Get.find<AuthService>();

    _connect.httpClient.addResponseModifier((request, response) async {
      if (response.statusCode == 401) {
        final currentRoute = Get.currentRoute;
        if (currentRoute != Routes.ADMIN_LOGIN &&
            currentRoute != Routes.LOGIN &&
            currentRoute != Routes.SHOP_LOGIN && // Corrected
            !(_authService.isLoggingOut.value)) {
          _authService.setIsLoggingOut(true);
          print("UserApiService: Detected 401 Unauthorized. Logging out and redirecting to login.");
          await _authService.logout();
          Get.offAllNamed(Routes.LOGIN); // Default to merchant login
          Get.snackbar(
            "Session Expired",
            "Your session has expired. Please log in again.",
            snackPosition: SnackPosition.BOTTOM,
            backgroundColor: Colors.orangeAccent,
            colorText: Colors.white,
            duration: const Duration(seconds: 4),
          );
          Future.delayed(const Duration(milliseconds: 500), () => _authService.setIsLoggingOut(false));
        }
      }
      return response;
    });
  }

  Future<Map<String, String>> _getAuthHeaders() async {
    final token = await _authService.getToken();
    if (token == null) {
      print("UserApiService: Auth token is null. API calls will likely result in 401.");
      return {'Content-Type': 'application/json'};
    }
    return {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $token',
    };
  }
  
  /// Fetches the current user's profile for the shop dashboard.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/profile`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///   - `X-Shop-ID: <shopId>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "John Doe",
  ///     "email": "john.doe@example.com",
  ///     "role": "staff",
  ///     "shopName": "Main Street Branch"
  ///   }
  ///   ```
  Future<UserProfile?> getShopUserProfile() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return UserProfile(
        name: 'John Doe (Mock)',
        email: 'john.doe@example.com',
        role: 'Staff',
        shopName: 'Main Street Branch (Mock)',
      );
    }

    final token = await _authService.getToken();
    final shopId = await _authService.getShopId();
    if (token == null || shopId == null) return null;

    final response = await _connect.get(
      '${ApiConstants.baseUrl}/shop/profile',
      headers: {
        'Authorization': 'Bearer $token',
        'X-Shop-ID': shopId,
      },
    );

    if (response.statusCode == 200) {
      return UserProfile.fromJson(response.body);
    }
    return null;
  }


  String get _usersBaseUrl => "${ApiConstants.baseUrl}/admin/users";
  String get _merchantsSelectionUrl => "${ApiConstants.baseUrl}/admin/users/merchants-for-selection";

  /// Fetches a list of users, with an optional filter by role.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/users`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `role`: `string` (Optional, e.g., "merchant", "staff")
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A paginated response of user objects)
  Future<List<User>> getUsers({String? roleFilter}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final users = [
        User(id: '1', name: 'Test Merchant', email: 'merchant@test.com', role: 'merchant', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        User(id: '2', name: 'Test Staff', email: 'staff@test.com', role: 'staff', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        User(id: '3', name: 'Inactive User', email: 'inactive@test.com', role: 'merchant', isActive: false, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      ];
      return users;
    }
    final headers = await _getAuthHeaders();
    if (!headers.containsKey('Authorization')) {
      print("UserApiService (getUsers): Authorization token is missing. Expecting 401 response.");
    }
    String url = _usersBaseUrl;
    final Map<String, dynamic> queryParams = {};
    if (roleFilter != null && roleFilter.isNotEmpty) {
      queryParams['role'] = roleFilter;
    }
    print("UserApiService: Fetching users from $url with params: $queryParams");
    final response = await _connect.get(url, headers: headers, query: queryParams);

    if (response.statusCode == 200 && response.body != null) {
      if (response.body is Map<String, dynamic>) {
        final Map<String, dynamic> responseMap = response.body;
        if (responseMap.containsKey('data') && responseMap['data'] is Map<String, dynamic>) {
          final Map<String, dynamic> paginatedData = responseMap['data'];
          try {
            final AdminPaginatedUsersResponse paginatedResponse = AdminPaginatedUsersResponse.fromJson(paginatedData);
            return paginatedResponse.users; // <<< CORRECTED (was .items)
          } catch (e) {
            print("UserApiService: Error parsing AdminPaginatedUsersResponse from responseMap['data']: $e");
            throw Exception("Failed to load users: Error parsing paginated user data.");
          }
        } else {
          try {
            // This case assumes the entire response.body is the paginated structure
            final AdminPaginatedUsersResponse paginatedResponse = AdminPaginatedUsersResponse.fromJson(responseMap);
            return paginatedResponse.users; // <<< CORRECTED (was .items)
          } catch (e) {
            print("UserApiService: getUsers response.body is a Map but doesn't match expected paginated structures (wrapped or direct): ${response.bodyString}");
            throw Exception("Failed to load users: Unexpected Map response format.");
          }
        }
      } else if (response.body is List) {
        try {
          return (response.body as List).map((item) => User.fromJson(item as Map<String,dynamic>)).toList();
        } catch (e) {
          print("UserApiService: Error parsing direct list of users: $e");
          throw Exception("Failed to load users: Error parsing direct list response.");
        }
      } else {
        print("UserApiService: getUsers response.body is not a Map or List: ${response.bodyString}");
        throw Exception("Failed to load users: Unexpected response type. Expected Map or List.");
      }
    } else if (response.statusCode == 401) {
      print("UserApiService (getUsers): Received 401, interceptor handles logout.");
      throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error fetching users. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to load users. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }

  /// Fetches a single user by their ID.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The full user object)
  Future<User> getUserById(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User(id: userId, name: 'Mock User', email: 'mock@test.com', role: 'merchant', createdAt: DateTime.now(), updatedAt: DateTime.now());
    }
    final headers = await _getAuthHeaders();
    if (!headers.containsKey('Authorization')) {
        print("UserApiService (getUserById): Authorization token is missing. Expecting 401 response.");
    }
    final url = "$_usersBaseUrl/$userId";
    print("UserApiService: Fetching user by ID from $url");
    final response = await _connect.get(url, headers: headers);

    if (response.statusCode == 200 && response.body != null) {
      if (response.body is Map<String, dynamic> && response.body.containsKey('data')) {
        return User.fromJson(response.body['data'] as Map<String, dynamic>);
      } else if (response.body is Map<String, dynamic>){ // Direct user object
        return User.fromJson(response.body as Map<String, dynamic>);
      } else {
         print("UserApiService: getUserById response body is not a Map or does not contain 'data' key: ${response.bodyString}");
        throw Exception("Failed to load user details: Unexpected response format.");
      }
    } else if (response.statusCode == 401) {
        print("UserApiService (getUserById): Received 401, interceptor handles logout.");
        throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error fetching user by ID. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to load user details. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }

  /// Creates a new user.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/admin/users`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "New User",
  ///     "email": "new.user@example.com",
  ///     "password": "password123",
  ///     "role": "merchant"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created user object)
  Future<User> createUser(Map<String, dynamic> userDataWithPassword) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User.fromJson(userDataWithPassword..['id'] = 'new-user-id');
    }
    final headers = await _getAuthHeaders();
     if (!headers.containsKey('Authorization')) {
        print("UserApiService (createUser): Authorization token is missing. Expecting 401 response.");
    }
    print("UserApiService: Creating user at $_usersBaseUrl with payload: $userDataWithPassword");
    final response = await _connect.post(_usersBaseUrl, userDataWithPassword, headers: headers);

    if (response.statusCode == 201 && response.body != null) {
      if (response.body is Map && response.body.containsKey('data')) {
        return User.fromJson(response.body['data']);
      } else { // Direct user object
        return User.fromJson(response.body); 
      }
    } else if (response.statusCode == 401) {
        print("UserApiService (createUser): Received 401, interceptor handles logout.");
        throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error creating user. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to create user. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }

  /// Updates an existing user.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Fields to be updated)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated user object)
  Future<User> updateUser(String userId, Map<String, dynamic> userData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User.fromJson(userData..['id'] = userId);
    }
    final headers = await _getAuthHeaders();
    if (!headers.containsKey('Authorization')) {
        print("UserApiService (updateUser): Authorization token is missing. Expecting 401 response.");
    }
    final url = "$_usersBaseUrl/$userId";
    print("UserApiService: Updating user at $url with payload: $userData");
    final response = await _connect.put(url, userData, headers: headers);

    if (response.statusCode == 200 && response.body != null) {
       if (response.body is Map && response.body.containsKey('data')) {
        return User.fromJson(response.body['data']);
      } else { // Direct user object
        return User.fromJson(response.body);
      }
    } else if (response.statusCode == 401) {
        print("UserApiService (updateUser): Received 401, interceptor handles logout.");
        throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error updating user. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to update user. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }

  /// Deletes a user.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 204 or 200
  Future<void> deleteUser(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return;
    }
    final headers = await _getAuthHeaders();
    if (!headers.containsKey('Authorization')) {
        print("UserApiService (deleteUser): Authorization token is missing. Expecting 401 response.");
    }
    final url = "$_usersBaseUrl/$userId";
    print("UserApiService: Deleting user at $url");
    final response = await _connect.delete(url, headers: headers);

    if (response.statusCode == 204 || response.statusCode == 200) { // 204 No Content is common for delete
      print("UserApiService: User $userId deleted successfully.");
      return;
    } else if (response.statusCode == 401) {
        print("UserApiService (deleteUser): Received 401, interceptor handles logout.");
        throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error deleting user. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to delete user. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }

  /// Fetches a list of merchants for selection in a dropdown.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/users/merchants-for-selection`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of `UserSelectionItem` objects)
  Future<List<UserSelectionItem>> getMerchantsForSelection() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return [];
    }
    final headers = await _getAuthHeaders();
    if (!headers.containsKey('Authorization')) {
        print("UserApiService (getMerchantsForSelection): Authorization token is missing. Expecting 401 response.");
    }
    print("UserApiService: Fetching merchants for selection from $_merchantsSelectionUrl");
    final response = await _connect.get(_merchantsSelectionUrl, headers: headers);

    if (response.statusCode == 200 && response.body != null) {
      if (response.body is Map<String, dynamic>) { 
        final Map<String, dynamic> responseMap = response.body;
        if (responseMap.containsKey('data') && responseMap['data'] is List) {
          return (responseMap['data'] as List).map((item) => UserSelectionItem.fromJson(item as Map<String, dynamic>)).toList();
        } else if (responseMap is List) { // This case should ideally not be hit if backend is consistent
          // This path is problematic if responseMap is a Map. It should be 'else if (response.body is List)'
          // For now, assuming the API is consistent and 'data' key is primary for lists within maps.
          print("UserApiService: getMerchantsForSelection received Map but 'data' is not a List. Trying to parse responseMap as List if it IS a List.");
           if (response.body is List) { // Redundant check, but keeping for clarity if the above structure is hit
            return (response.body as List).map((item) => UserSelectionItem.fromJson(item as Map<String, dynamic>)).toList();
           }
          throw Exception("Failed to load merchants: Unexpected Map format without 'data' as List.");
        } else {
          print("UserApiService: getMerchantsForSelection received Map but 'data' is not a List or the body is not a direct List: ${response.bodyString}");
          throw Exception("Failed to load merchants: Unexpected response format. Expected List or Map with 'data' as List.");
        }
      } else if (response.body is List) { // If the backend sends a direct list
         return (response.body as List).map((item) => UserSelectionItem.fromJson(item as Map<String, dynamic>)).toList();
      } else {
        print("UserApiService: getMerchantsForSelection response.body is not a Map or List: ${response.bodyString}");
        throw Exception("Failed to load merchants: Unexpected response type. Expected Map or List.");
      }
    } else if (response.statusCode == 401) {
        print("UserApiService (getMerchantsForSelection): Received 401, interceptor handles logout.");
        throw Exception("Session expired. Please log in again.");
    } else {
      print("UserApiService: Error fetching merchants for selection. Status: ${response.statusCode}, Body: ${response.bodyString}");
      final errorMessage = response.body?['message'] ?? response.body?['error'] ?? response.statusText ?? "Unknown error";
      throw Exception("Failed to load merchants for selection. Status: ${response.statusCode}, Message: $errorMessage");
    }
  }
}
