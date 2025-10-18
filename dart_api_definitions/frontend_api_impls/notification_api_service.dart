import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/notification_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class NotificationApiService extends GetxService {
  final GetConnect _connect = GetConnect(); // <<< CORRECTED: Use a new instance
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/notifications';

  Future<Map<String, String>> _getHeaders() async {
    final token = _authService.authToken.value;
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a paginated list of notifications.
  Future<PaginatedNotificationsResponse> getNotifications({int page = 1, int pageSize = 20}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      
      final now = DateTime.now();
      final allMockNotifications = [
        NotificationModel(
          id: '1',
          recipientUserId: 'user1',
          title: 'Low Stock Alert: Wireless Mouse',
          message: 'Your stock for item "Wireless Mouse" (MO456) is running low. Only 5 items remaining in Main Street Branch.',
          type: 'low_stock',
          relatedEntityId: 'item2',
          relatedEntityType: 'InventoryItem',
          isRead: false,
          createdAt: now.subtract(const Duration(hours: 2)),
          updatedAt: now.subtract(const Duration(hours: 2)),
        ),
        NotificationModel(
          id: '2',
          recipientUserId: 'user1',
          title: 'Holiday Promotion Available',
          message: 'A new "Holiday Special" promotion has been added to your account. Apply it at checkout for a 15% discount.',
          type: 'announcement',
          isRead: false,
          createdAt: now.subtract(const Duration(days: 1)),
          updatedAt: now.subtract(const Duration(days: 1)),
        ),
        NotificationModel(
          id: '3',
          recipientUserId: 'user1',
          title: 'Low Stock Alert: Laptop',
          message: 'Your stock for "Laptop" (LP123) is running low. Only 2 items remaining in City Center Outlet.',
          type: 'low_stock',
          isRead: true,
          createdAt: now.subtract(const Duration(days: 2, hours: 5)),
          updatedAt: now.subtract(const Duration(days: 2, hours: 5)),
        ),
        NotificationModel(
          id: '4',
          recipientUserId: 'user1',
          title: 'System Maintenance Scheduled',
          message: 'Scheduled maintenance will occur on Sunday at 2 AM. Brief downtime is expected.',
          type: 'announcement',
          isRead: true,
          createdAt: now.subtract(const Duration(days: 4)),
          updatedAt: now.subtract(const Duration(days: 4)),
        ),
      ];

      // Simulate pagination
      if (page > 1) {
        return PaginatedNotificationsResponse(
          success: true, 
          message: 'success', 
          data: [], // No more items on subsequent pages for this mock
          meta: Meta(totalItems: allMockNotifications.length, currentPage: page, pageSize: pageSize, totalPages: 1)
        );
      }

      return PaginatedNotificationsResponse(
        success: true,
        message: 'success',
        data: allMockNotifications,
        meta: Meta(
          totalItems: allMockNotifications.length,
          currentPage: 1,
          pageSize: pageSize,
          totalPages: 1,
        ),
      );
    }
    final response = await _connect.get(
      _baseUrl,
      headers: await _getHeaders(),
      query: {'page': page.toString(), 'pageSize': pageSize.toString()},
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return PaginatedNotificationsResponse.fromJson(response.body);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load notifications');
    }
  }

  /// Fetches the count of unread notifications.
  Future<int> getUnreadCount() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 300));
      return 2; // Based on the mock data above
    }
    final response = await _connect.get(
      '$_baseUrl/unread-count',
      headers: await _getHeaders(),
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return response.body['data']['count'];
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to get unread count');
    }
  }

  /// Marks a specific notification as read.
  Future<void> markAsRead(String notificationId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      // In mock mode, we don't need to do anything, the controller handles the state.
      return;
    }
    final response = await _connect.patch(
      '$_baseUrl/$notificationId/read', 
      {},
      headers: await _getHeaders(),
    );

    if (response.statusCode != 200 || response.body['success'] != true) {
        throw Exception(response.body?['message'] ?? 'Failed to mark as read');
    }
  }
}
