import 'package:flutter/material.dart'; // For Get.snackbar
import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';
import 'package:smart_retail/app/data/models/admin_paginated_users_response.dart';
import 'package:smart_retail/app/data/models/user_selection_item.dart';

class AdminUserService extends GetxService {
  final GetConnect _connect = GetConnect(timeout: const Duration(seconds: 30));
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  final String _adminBaseUrl = "${ApiConstants.baseUrl}/admin";

  Future<Map<String, String>?> _getAuthHeaders() async {
    final token = _authService.authToken.value;
    if (token == null) {
      debugPrint("Auth token is null in AdminUserService");
      // Optionally, force logout or redirect to login
      return null;
    }
    return {'Authorization': 'Bearer $token', 'Content-Type': 'application/json'};
  }

  /// Fetches a paginated list of users.
  Future<AdminPaginatedUsersResponse?> listUsers({
    int page = 1,
    int pageSize = 10,
    String? role,
    bool? isActive,
    String? searchTerm,
  }) async {
    if (_appConfig.isDevelopment) {
      return _mockListUsers(page: page, pageSize: pageSize, role: role, isActive: isActive, searchTerm: searchTerm);
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
      "$_adminBaseUrl/users",
      query: queryParams,
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return AdminPaginatedUsersResponse.fromJson(response.body['data']);
    } else {
      _handleError(response, "Failed to fetch users");
      return null;
    }
  }

  /// Creates a new user.
  Future<User?> createUser(Map<String, dynamic> userData) async {
     if (_appConfig.isDevelopment) {
        await Future.delayed(const Duration(seconds: 1));
        final newUser = User.fromJson({
            'id': 'dev-user-${DateTime.now().millisecondsSinceEpoch}',
            'createdAt': DateTime.now().toIso8601String(),
            'updatedAt': DateTime.now().toIso8601String(),
            ...userData
        });
        _mockUsers.add(newUser);
        return newUser;
    }

    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.post(
      "$_adminBaseUrl/users",
      userData,
      headers: headers,
    );

    if (response.statusCode == 201 && response.body['status'] == 'success') {
       Get.snackbar("Success", "User created successfully", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.green, colorText: Colors.white);
      return User.fromJson(response.body['data']);
    } else {
      _handleError(response, "Failed to create user");
      return null;
    }
  }

  /// Fetches a single user by their ID.
  Future<User?> getUserById(String userId) async {
    if (_appConfig.isDevelopment) {
        await Future.delayed(const Duration(milliseconds: 500));
        return _mockUsers.firstWhere((u) => u.id == userId, orElse: () => null);
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.get(
      "$_adminBaseUrl/users/$userId",
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return User.fromJson(response.body['data']);
    } else {
      _handleError(response, "Failed to fetch user");
      return null;
    }
  }

  /// Updates an existing user.
  Future<User?> updateUser(String userId, Map<String, dynamic> userData) async {
    if (_appConfig.isDevelopment) {
        await Future.delayed(const Duration(seconds: 1));
        final index = _mockUsers.indexWhere((u) => u.id == userId);
        if (index != -1) {
            final currentUser = _mockUsers[index];
            final updatedUser = User.fromJson(currentUser.toJson()..addAll(userData));
            _mockUsers[index] = updatedUser;
            return updatedUser;
        }
        return null;
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.put(
      "$_adminBaseUrl/users/$userId",
      userData,
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", "User updated successfully", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.green, colorText: Colors.white);
      return User.fromJson(response.body['data']);
    } else {
      _handleError(response, "Failed to update user");
      return null;
    }
  }

  /// Updates a user's active status.
  Future<bool> setUserStatus(String userId, bool isActive) async {
    if (_appConfig.isDevelopment) {
        await Future.delayed(const Duration(seconds: 1));
        final index = _mockUsers.indexWhere((u) => u.id == userId);
        if (index != -1) {
            _mockUsers[index] = _mockUsers[index].copyWith(isActive: isActive);
            return true;
        }
        return false;
    }

    final headers = await _getAuthHeaders();
    if (headers == null) return false;

    final response = await _connect.put(
      "$_adminBaseUrl/users/$userId/status",
      {'is_active': isActive},
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", response.body['message'] ?? "User status updated", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.green, colorText: Colors.white);
      return true;
    } else {
      _handleError(response, "Failed to update user status");
      return false;
    }
  }

  /// Deactivates a user (soft delete).
  Future<bool> deleteUser(String userId) async {
    return await setUserStatus(userId, false);
  }

  /// Activates a user.
  Future<bool> activateUser(String userId) async {
    return await setUserStatus(userId, true);
  }

  /// Permanently deletes a user.
  Future<bool> hardDeleteUser(String userId) async {
     if (_appConfig.isDevelopment) {
        await Future.delayed(const Duration(seconds: 1));
        _mockUsers.removeWhere((u) => u.id == userId);
        return true;
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return false;

    final response = await _connect.delete(
      "$_adminBaseUrl/users/$userId/permanent-delete",
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      Get.snackbar("Success", response.body['message'] ?? "User permanently deleted", snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.orange, colorText: Colors.white);
      return true;
    } else {
      _handleError(response, "Failed to permanently delete user");
      return false;
    }
  }
  
  /// Fetches a list of merchants for selection in a dropdown.
  Future<List<UserSelectionItem>?> getMerchantsForSelection() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 800));
      return _mockMerchantSelection;
    }
    final headers = await _getAuthHeaders();
    if (headers == null) return null;

    final response = await _connect.get(
      "$_adminBaseUrl/users/merchants-for-selection",
      headers: headers,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      List<dynamic> listJson = response.body['data'] as List? ?? [];
      return listJson.map((json) => UserSelectionItem.fromJson(json)).toList();
    } else {
      _handleError(response, "Failed to fetch merchant list");
      return null;
    }
  }
  
  /// Generic error handler
  void _handleError(Response response, String defaultMessage) {
    final message = response.body?['message'] ?? response.statusText ?? defaultMessage;
    debugPrint('API Error: ${response.statusCode} - $message');
    Get.snackbar("Error", message, snackPosition: SnackPosition.BOTTOM, backgroundColor: Colors.red, colorText: Colors.white);
  }

  // --- Mock Data and Methods ---

  final List<User> _mockUsers = [
    User(id: '1', name: 'Test Merchant', email: 'merchant@test.com', role: 'merchant', isActive: true, createdAt: DateTime.now(), updatedAt: DateTime.now()),
    User(id: '2', name: 'Test Staff', email: 'staff@test.com', role: 'staff', isActive: true, createdAt: DateTime.now(), merchantId: '1'),
    User(id: '3', name: 'Inactive User', email: 'inactive@test.com', role: 'merchant', isActive: false, createdAt: DateTime.now()),
    User(id: '4', name: 'Another Merchant', email: 'merchant2@test.com', role: 'merchant', isActive: true, createdAt: DateTime.now()),
  ];

  final List<UserSelectionItem> _mockMerchantSelection = [
      UserSelectionItem(id: '1', name: 'Test Merchant'),
      UserSelectionItem(id: '4', name: 'Another Merchant'),
  ];

  Future<AdminPaginatedUsersResponse?> _mockListUsers({
    int page = 1,
    int pageSize = 10,
    String? role,
    bool? isActive,
    String? searchTerm,
  }) async {
    await Future.delayed(const Duration(seconds: 1));
    
    List<User> filteredUsers = _mockUsers;

    if (role != null && role.isNotEmpty) {
      filteredUsers = filteredUsers.where((u) => u.role == role).toList();
    }
    if (isActive != null) {
      filteredUsers = filteredUsers.where((u) => u.isActive == isActive).toList();
    }
    if (searchTerm != null && searchTerm.isNotEmpty) {
      filteredUsers = filteredUsers.where((u) => u.name.toLowerCase().contains(searchTerm.toLowerCase()) || u.email.toLowerCase().contains(searchTerm.toLowerCase())).toList();
    }
    
    final totalCount = filteredUsers.length;
    final totalPages = (totalCount / pageSize).ceil();
    final startIndex = (page - 1) * pageSize;
    final users = filteredUsers.skip(startIndex).take(pageSize).toList();

    return AdminPaginatedUsersResponse(
      users: users, 
      currentPage: page, 
      totalPages: totalPages, 
      pageSize: pageSize, 
      totalCount: totalCount
    );
  }
}
