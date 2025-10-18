
import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';

// Model for the dashboard summary data
class ShopDashboardSummary {
  final double salesToday;
  final int transactionsToday;
  final int lowStockItems;

  ShopDashboardSummary({
    required this.salesToday,
    required this.transactionsToday,
    required this.lowStockItems,
  });

  factory ShopDashboardSummary.fromJson(Map<String, dynamic> json) {
    return ShopDashboardSummary(
      salesToday: (json['salesToday'] as num).toDouble(),
      transactionsToday: (json['transactionsToday'] as num).toInt(),
      lowStockItems: (json['lowStockItems'] as num).toInt(),
    );
  }
}

class ShopDashboardApiService extends GetxService {
  final AppConfig _appConfig = Get.find<AppConfig>();

  /// Fetches the summary data for the shop dashboard.
  ///
  /// This would typically include metrics like sales today, number of transactions,
  /// and count of items that are low on stock for the specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/dashboard/summary`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "salesToday": 1450.75,
  ///     "transactionsToday": 23,
  ///     "lowStockItems": 5
  ///   }
  ///   ```
  Future<ShopDashboardSummary> getDashboardSummary() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      // Return mock data for development environment
      return ShopDashboardSummary(
        salesToday: 1450.75,
        transactionsToday: 23,
        lowStockItems: 5,
      );
    }

    // In a real application, you would make a GetConnect API call here.
    // final response = await _connect.get(...);
    // return ShopDashboardSummary.fromJson(response.body['data']);
    throw UnimplementedError("API call not implemented for production yet.");
  }
}
