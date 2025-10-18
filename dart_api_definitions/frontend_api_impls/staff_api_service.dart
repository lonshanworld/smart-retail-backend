import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/models/salary_model.dart';
import 'package:smart_retail/app/data/models/shop_model.dart';
import 'package:smart_retail/app/data/models/staff_dashboard_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class StaffApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/staff';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches the details of the staff member's assigned shop.
  /// The backend uses the token to identify the shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/assigned-shop`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A `Shop` object.
  Future<Shop> getAssignedShop() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final assignedShopId = _authService.user.value?.assignedShopId ?? 'mock-shop-1';
      return Shop(
        id: assignedShopId,
        name: 'Downtown Coffee House',
        merchantId: 'mock-merchant-xyz',
        address: '123 Main Street, Anytown, USA',
        phone: '555-123-4567',
        createdAt: DateTime.now().subtract(const Duration(days: 365)),
        updatedAt: DateTime.now().subtract(const Duration(days: 10)),
      );
    }

    final response = await _connect.get('$_baseUrl/assigned-shop', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return Shop.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to fetch assigned shop');
    }
  }

  /// Fetches the profile of the currently logged-in staff member.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/profile`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A `User` object representing the staff member.
  Future<User> getStaffProfile() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final currentUser = _authService.user.value;
      if (currentUser != null && currentUser.role == 'staff') {
        return currentUser.copyWith(name: 'Mock Staff User', phone: '123-456-7890');
      } else {
        throw Exception('Mock staff user not found or role is incorrect.');
      }
    }

    final response = await _connect.get('$_baseUrl/profile', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return User.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to fetch staff profile');
    }
  }

  /// Fetches the salary history for the currently logged-in staff member.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/salary`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A list of `Salary` objects.
  Future<List<Salary>> getSalaryHistory() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 800));
      return [
        Salary(id: 'sal-1', staffId: _authService.userId.value!, amount: 2200.0, paymentDate: DateTime(2023, 5, 28), notes: 'May Salary'),
        Salary(id: 'sal-2', staffId: _authService.userId.value!, amount: 2200.0, paymentDate: DateTime(2023, 4, 28), notes: 'April Salary'),
        Salary(id: 'sal-3', staffId: _authService.userId.value!, amount: 2150.0, paymentDate: DateTime(2023, 3, 28), notes: 'March Salary (includes OT)'),
      ];
    }
    
    final response = await _connect.get('$_baseUrl/salary', headers: await _getHeaders());
    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List).map((i) => Salary.fromJson(i)).toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to fetch salary history');
    }
  }

  /// Fetches the dashboard summary for the currently logged-in staff member.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/staff/dashboard`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ A `StaffDashboardSummaryResponse` object.
  Future<StaffDashboardSummaryResponse?> getStaffDashboardSummary() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return StaffDashboardSummaryResponse(
        assignedShopName: 'Mock Shop',
        salesToday: 1234.56,
        transactionsToday: 42,
        recentActivities: [
          ActivityItemDTO(
            type: 'sale',
            details: 'Sale #1001 - 2 items',
            timestamp: DateTime.now().subtract(const Duration(minutes: 30)),
            relatedId: 'sale-1001',
          ),
          ActivityItemDTO(
            type: 'inventory',
            details: 'Stock updated for 3 products',
            timestamp: DateTime.now().subtract(const Duration(hours: 1)),
          ),
        ],
      );
    }

    final response = await _connect.get(
      '$_baseUrl/dashboard',
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return StaffDashboardSummaryResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to get staff dashboard summary');
    }
  }
}
