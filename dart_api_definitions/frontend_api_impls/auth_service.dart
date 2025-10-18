import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import 'package:get/get.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/routes/app_pages.dart';

class AuthService extends GetxService {
  static const String _tokenKey = 'auth_token';
  static const String _userKey = 'auth_user';
  static const String _shopKey = 'auth_shop';
  static const String _userRoleKey = 'user_role';
  static const String _userIdKey = 'user_id';
  static const String _shopIdKey = 'shop_id';

  final RxnString authToken = RxnString();
  final Rxn<User> user = Rxn<User>();
  final Rxn<Shop> currentShop = Rxn<Shop>();
  final RxnString userRole = RxnString();
  final RxnString userId = RxnString();
  final RxnString shopId = RxnString();
  final RxString errorMessage = ''.obs;
  final RxBool isLoggingOut = false.obs;

  final _connect = GetConnect(timeout: const Duration(seconds: 30));
  final AppConfig _appConfig = Get.find<AppConfig>();

  @override
  void onInit() {
    super.onInit();
    _loadAuthDataFromStorage();
  }

  void setIsLoggingOut(bool value) {
    isLoggingOut.value = value;
  }

  Future<void> _loadAuthDataFromStorage() async {
    final prefs = await SharedPreferences.getInstance();
    authToken.value = prefs.getString(_tokenKey);

    final userJson = prefs.getString(_userKey);
    if (userJson != null) {
      try {
        user.value = User.fromJson(jsonDecode(userJson));
        userRole.value = user.value?.role;
        userId.value = user.value?.id;
        shopId.value = user.value?.assignedShopId;
      } catch (e) {
        print("AuthService: Failed to decode user from JSON. Error: $e");
        await clearAuthData();
      }
    }

    final shopJson = prefs.getString(_shopKey);
    if (shopJson != null) {
      try {
        currentShop.value = Shop.fromJson(jsonDecode(shopJson));
      } catch (e) {
        print("AuthService: Failed to decode shop from JSON. Error: $e");
        await clearAuthData();
      }
    }
  }

  Future<void> _saveAuthData(String token, User userData, {Shop? shopData}) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_userKey, jsonEncode(userData.toJson()));
    await prefs.setString(_tokenKey, token);

    if (shopData != null) {
      await prefs.setString(_shopKey, jsonEncode(shopData.toJson()));
    } else {
      await prefs.remove(_shopKey);
    }

    authToken.value = token;
    user.value = userData;
    currentShop.value = shopData;
    userRole.value = userData.role;
    userId.value = userData.id;
    shopId.value = userData.assignedShopId;

    print("AuthService: Auth data SAVED. Role: ${userData.role}, Email: ${userData.email}");
  }

  Future<String?> getToken() async => authToken.value;
  Future<String?> getUserId() async => userId.value;
  Future<String?> getUserRole() async => userRole.value;
  Future<String?> getShopId() async => shopId.value;

  bool get isAuthenticated => authToken.value != null;

  Future<void> clearAuthData() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_userKey);
    await prefs.remove(_shopKey);

    authToken.value = null;
    user.value = null;
    currentShop.value = null;
    userRole.value = null;
    userId.value = null;
    shopId.value = null;
    errorMessage.value = '';
    print("AuthService: Auth data CLEARED.");
  }

  Future<void> logout() async {
    await clearAuthData();
    print("AuthService: User logged out, auth data cleared.");
  }

  /// Login for Shop-specific staff users.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/auth/shop-login`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "shopId": "your_shop_id",
  ///     "email": "your_staff_email",
  ///     "password": "your_password"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "accessToken": "your_jwt_token",
  ///     "user": { ...User object... },
  ///     "shop": { ...Shop object... }
  ///   }
  ///   ```
  Future<bool> loginToShop(String shopId, String email, String password) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final mockShopId = dotenv.env['MOCK_SHOP_ID'] ?? '1';
      if (shopId == mockShopId && email == dotenv.env['STAFF_EMAIL'] && password == dotenv.env['STAFF_PASSWORD']) {
        final mockUser = User(id: 'mock_staff_id', name: 'Mock Staff', email: email, role: 'staff', assignedShopId: shopId);
        final mockShop = Shop(id: mockShopId, name: 'Mock Central Shop', merchantId: 'mock-merchant', createdAt: DateTime.now(), updatedAt: DateTime.now());
        await _saveAuthData('mock_staff_token', mockUser, shopData: mockShop);
        return true;
      }
      return false;
    }

    final response = await _connect.post('${ApiConstants.baseUrl}/auth/shop-login', {
      'shopId': shopId,
      'email': email,
      'password': password,
    });

    if (response.statusCode == 200 && response.body['accessToken'] != null) {
      final token = response.body['accessToken'];
      final userData = response.body['user'];
      final shopData = response.body['shop'];
      if (token is String && userData is Map<String, dynamic>) {
        await _saveAuthData(
          token,
          User.fromJson(userData),
          shopData: shopData != null ? Shop.fromJson(shopData) : null,
        );
        return true;
      }
    }
    return false;
  }

  /// General login for Merchant and Admin users.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/auth/login`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "email": "your_email",
  ///     "password": "your_password",
  ///     "userType": "merchant"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "accessToken": "your_jwt_token",
  ///     "user": { ...User object... }
  ///   }
  ///   ```
  Future<bool> login(String email, String password, {String? role, String? onSuccessNavigateTo}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final adminEmail = dotenv.env['ADMIN_EMAIL'];
      final adminPassword = dotenv.env['ADMIN_PASSWORD'];
      final merchantEmail = dotenv.env['MERCHANT_EMAIL'];
      final merchantPassword = dotenv.env['MERCHANT_PASSWORD'];
      final staffEmail = dotenv.env['STAFF_EMAIL'];
      final staffPassword = dotenv.env['STAFF_PASSWORD'];

      User? mockUser;

      if (role == 'admin' && email == adminEmail && password == adminPassword) {
        mockUser = User(id: 'mock-admin-id', name: 'Mock Admin', email: email, role: 'admin');
        await _saveAuthData('mock_admin_token', mockUser);
        Get.offAllNamed(onSuccessNavigateTo ?? Routes.ADMIN_DASHBOARD);
        return true;
      } else if ((role == null || role == 'merchant') && email == merchantEmail && password == merchantPassword) {
        mockUser = User(id: 'mock-merchant-id', name: 'Mock Merchant', email: email, role: 'merchant');
        await _saveAuthData('mock_merchant_token', mockUser);
        Get.offAllNamed(onSuccessNavigateTo ?? Routes.MERCHANT_DASHBOARD);
        return true;
      } else if (role == 'staff' && email == staffEmail && password == staffPassword) {
        mockUser = User(id: 'mock-staff-id', name: 'Mock Staff', email: email, role: 'staff');
        await _saveAuthData('mock_staff_token', mockUser);
        Get.offAllNamed(onSuccessNavigateTo ?? Routes.STAFF_DASHBOARD);
        return true;
      }
      errorMessage.value = 'Invalid mock credentials for the selected role';
      return false;
    }

    errorMessage.value = '';
    final loginUrl = "${ApiConstants.baseUrl}/auth/login";
    final payload = {
      'email': email,
      'password': password,
      'userType': role,
    };

    try {
      final response = await _connect.post(loginUrl, payload);

      if (response.statusCode == 200 && response.body != null) {
        final responseData = response.body;
        final token = responseData['accessToken'];
        final userData = responseData['user'];

        if (token is String && token.isNotEmpty && userData is Map<String, dynamic>) {
          final loggedInUser = User.fromJson(userData);
          await _saveAuthData(token, loggedInUser);

          if (onSuccessNavigateTo != null) {
            Get.offAllNamed(onSuccessNavigateTo);
          } else if (loggedInUser.role == 'admin') {
            Get.offAllNamed(Routes.ADMIN_DASHBOARD);
          } else if (loggedInUser.role == 'merchant') {
            Get.offAllNamed(Routes.MERCHANT_DASHBOARD);
          } else if (loggedInUser.role == 'staff') {
            Get.offAllNamed(Routes.STAFF_DASHBOARD);
          }
          return true;
        }
      }
      String msg = response.body?['message'] ?? 'Login failed';
      errorMessage.value = '$msg (Status: ${response.statusCode})';
      return false;
    } catch (e, stackTrace) {
      print("[AUTH_SERVICE] Login exception: $e\n$stackTrace");
      errorMessage.value = 'An unexpected error occurred during login.';
      return false;
    }
  }
}
