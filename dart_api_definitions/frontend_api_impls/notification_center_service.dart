import 'package:get/get.dart';
import 'package:smart_retail/app/data/services/notification_api_service.dart';

class NotificationCenterService extends GetxService {
  final NotificationApiService _apiService = Get.find<NotificationApiService>();

  final RxInt unreadCount = 0.obs;

  @override
  void onInit() {
    super.onInit();
    // Fetch the initial count when the service is first initialized
    fetchUnreadCount();
  }

  Future<void> fetchUnreadCount() async {
    try {
      final count = await _apiService.getUnreadCount();
      unreadCount.value = count;
    } catch (e) {
      // Silently fail or log error, as this is often a background task
      print("Failed to fetch unread notification count: $e");
    }
  }

  void decrementUnreadCount() {
    if (unreadCount.value > 0) {
      unreadCount.value--;
    }
  }
}
