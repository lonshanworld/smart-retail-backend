import 'package:flutter/material.dart'; // For Get.snackbar
import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';
import 'package:smart_retail/app/data/models/admin_paginated_users_response.dart'; // <<< ADDED IMPORT
import 'package:smart_retail/app/data/models/user_selection_item.dart'; // <<< ADDED IMPORT

class AdminUserService extends GetxService {
  final GetConnect _connect = GetConnect(timeout: const Duration(seconds: 30));
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  final String _adminUsersBaseUrl = "${ApiConstants.baseUrl}/admin/users";

  Future<Map<String, String>?> _getAuthHeaders() async {
    final token = _authService.authToken.value;
    if (token == null) {
      print("Auth token is null in AdminUserService");
      return null;
    }
    return {'Authorization': 'Bearer $token', 'Content-Type': 'application/json'};
  }

  /// Fetches a paginated list of users.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/users`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int`
  ///   - `pageSize`: `int`
  ///   - `role`: `string`
  ///   - `is_active`: `bool`
  ///   - `q`: `string` (search term)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A paginated response of user objects)
  Future<AdminPaginatedUsersResponse?> listUsers({
    int page = 1,
    int pageSize = 10,
    String? role,
    bool? isActive,
    String? searchTerm,
  }) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final users = [
        User(id: '1', name: 'Test Merchant', email: 'merchant@test.com', role: 'merchant', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        User(id: '2', name: 'Test Staff', email: 'staff@test.com', role: 'staff', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        User(id: '3', name: 'Inactive User', email: 'inactive@test.com', role: 'merchant', isActive: false, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      ];
      return AdminPaginatedUsersResponse(users: users, currentPage: 1, totalPages: 1, pageSize: 10, totalCount: 3);
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final queryParams = <String, String>{
      'page': page.toString(),
      'pageSize': pageSize.toString(),
    };
    if (role != null && role.isNotEmpty) queryParams['role'] = role;
    if (isActive != null) queryParams['is_active'] = isActive.toString();
    if (searchTerm != null && searchTerm.isNotEmpty) queryParams['q'] = searchTerm;

    final response = await _connect.get(
      _adminUsersBaseUrl,
      query: queryParams,
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      // Assuming response.body['data'] is the Map for AdminPaginatedUsersResponse.fromJson
      return AdminPaginatedUsersResponse.fromJson(response.body['data'] as Map<String, dynamic>);
    } else {
      print('Error listing users: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to fetch users: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
    }
  }

  /// Creates a new user.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/admin/users`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (User data)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created user object)
  Future<User?> createUser(Map<String, dynamic> userData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User.fromJson(userData..['id'] = 'new-user-id');
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.post(
      _adminUsersBaseUrl,
      userData,
      headers: headers,
    );

    if (response.statusCode == 201 && response.body['status'] == 'success') {
      return User.fromJson(response.body['data'] as Map<String, dynamic>);
    } else {
      print('Error creating user: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to create user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
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
  Future<User?> getUserById(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User(id: userId, name: 'Mock User', email: 'mock@test.com', role: 'merchant', createdAt: DateTime.now(), updatedAt: DateTime.now());
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.get(
      '$_adminUsersBaseUrl/$userId',
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return User.fromJson(response.body['data'] as Map<String, dynamic>);
    } else {
      print('Error fetching user $userId: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to fetch user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
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
  Future<User?> updateUser(String userId, Map<String, dynamic> userData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User.fromJson(userData..['id'] = userId);
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.put(
      '$_adminUsersBaseUrl/$userId',
      userData,
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return User.fromJson(response.body['data'] as Map<String, dynamic>);
    } else {
      print('Error updating user $userId: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to update user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
    }
  }

  /// Deactivates a user (soft delete).
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  Future<bool> deleteUser(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return true;
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return false;

    final response = await _connect.delete(
      '$_adminUsersBaseUrl/$userId',
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", response.body?['message'] ?? "User deactivated successfully", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.green, colorText: Colors.white);
      return true;
    } else {
      print('Error deactivating user $userId: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to deactivate user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return false;
    }
  }

  /// Activates a user.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}/activate`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated user object)
  Future<User?> activateUser(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return User(id: userId, name: 'Mock User', email: 'mock@test.com', role: 'merchant', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now());
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.put(
      '$_adminUsersBaseUrl/$userId/activate',
      {}, // Empty body
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", response.body?['message'] ?? "User activated successfully", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.green, colorText: Colors.white);
      return User.fromJson(response.body['data'] as Map<String, dynamic>);
    } else {
      print('Error activating user $userId: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to activate user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
    }
  }

  /// Permanently deletes a user.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/admin/users/{userId}/permanent-delete`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  Future<bool> hardDeleteUser(String userId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return true;
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return false;

    // IMPORTANT: Confirm this is the correct endpoint for permanent deletion on your backend.
    // It might be something like '$userId/force' or with a query parameter like '?permanent=true'.
    // For now, I'm using a common pattern: '$userId/permanent-delete'
    final response = await _connect.delete(
      '$_adminUsersBaseUrl/$userId/permanent-delete', 
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", response.body?['message'] ?? "User permanently deleted", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.orange, colorText: Colors.white);
      return true;
    } else {
      print('Error permanently deleting user $userId: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to permanently delete user: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return false;
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
  Future<List<UserSelectionItem>?> getMerchantsForSelection() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 800));
      return [
        UserSelectionItem(id: 'merchant-1', name: 'SuperMart'),
        UserSelectionItem(id: 'merchant-2', name: 'QuickStop'),
        UserSelectionItem(id: 'merchant-3', name: 'Mega Foods'),
      ];
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.get(
      '$_adminUsersBaseUrl/merchants-for-selection', 
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      List<dynamic> listJson = response.body['data'] as List<dynamic>? ?? [];
      return listJson.map((json) => UserSelectionItem.fromJson(json as Map<String,dynamic>)).toList();
    } else {
      print('Error fetching merchants for selection: ${response.statusCode} - ${response.bodyString}');
      Get.snackbar("Error", "Failed to fetch merchant list: ${response.body?['message'] ?? response.statusText}", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
      return null;
    }
  }
}
