import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/api_config.dart';
// Import for Admin Dashboard
import 'package:smart_retail/app/modules/admin/dashboard/models/admin_dashboard_summary_model.dart';
// Import for Merchant Dashboard
import 'package:smart_retail/app/data/models/merchant_dashboard_summary_model.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class AdminDashboardApiService extends GetxService {
  final GetConnect _getConnect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  /// Fetches the summary data for the admin dashboard.
  ///
  /// This method sends a GET request to the `/admin/dashboard/summary` endpoint.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/admin/dashboard/summary`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "total_active_merchants": 15,
  ///     "total_active_staff": 50,
  ///     "total_active_shops": 25,
  ///     "total_products_listed": 1200
  ///   }
  ///   ```
  ///
  /// __Dummy Response Data (for testing):__
  /// ```dart
  /// final dummyAdminSummary = AdminDashboardSummaryModel(
  ///   totalActiveMerchants: 15,
  ///   totalActiveStaff: 50,
  ///   totalActiveShops: 25,
  ///   totalProductsListed: 1200,
  /// );
  /// ```
  Future<AdminDashboardSummaryModel?> getAdminDashboardSummary() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return AdminDashboardSummaryModel(
        totalActiveMerchants: 15,
        totalActiveStaff: 50,
        totalActiveShops: 25,
        totalProductsListed: 1200,
      );
    }
    final token = _authService.authToken.value;
    if (token == null) {
      Get.snackbar('Error', 'Authentication token not found. Please login again.');
      return null;
    }

    final response = await _getConnect.get(
      '${ApiConfig.baseUrl}/admin/dashboard/summary',
      headers: {
        'Authorization': 'Bearer $token',
      },
    );

    if (response.statusCode == 200) {
      if (response.body != null) {
        try {
          if (response.body is Map<String, dynamic>) {
            return AdminDashboardSummaryModel.fromJson(response.body as Map<String, dynamic>);
          } else {
            Get.snackbar('Error', 'Invalid data format received from server.');
            return null;
          }
        } catch (e) {
          Get.snackbar('Error', 'Failed to parse admin dashboard data: ${e.toString()}');
          return null;
        }
      } else {
         Get.snackbar('Error', 'Received empty response from server for admin dashboard summary.');
        return null;
      }
    } else {
      String errorMessage = 'Failed to fetch admin dashboard summary. Status Code: ${response.statusCode}';
      if (response.body != null && response.body['message'] != null) {
        errorMessage = response.body['message'];
      }
      Get.snackbar('Error', errorMessage);
      return null;
    }
  }

  /// Fetches the summary data for the merchant dashboard.
  ///
  /// This method sends a GET request to the `/merchant/dashboard/summary` endpoint.
  /// An optional `shopId` can be provided to filter the summary for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/dashboard/summary`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters (Optional):__
  ///   - `shop_id`: `string` (UUID of the shop)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "total_sales_revenue": { "value": 15000.75 },
  ///     "number_of_transactions": { "value": 300 },
  ///     "average_order_value": { "value": 50.00 },
  ///     "top_selling_products": [
  ///       { "product_id": "uuid-1", "product_name": "Product A", "quantity_sold": 100, "revenue": 5000.00 },
  ///       { "product_id": "uuid-2", "product_name": "Product B", "quantity_sold": 75, "revenue": 3750.50 }
  ///     ]
  ///   }
  ///   ```
  ///
  /// __Dummy Response Data (for testing):__
  /// ```dart
  /// final dummyMerchantSummary = MerchantDashboardSummaryModel(
  ///   totalSalesRevenue: KpiData(value: 15000.75),
  ///   numberOfTransactions: KpiData(value: 300),
  ///   averageOrderValue: KpiData(value: 50.00),
  ///   topSellingProducts: [
  ///     ProductSummaryModel(productId: 'uuid-1', productName: 'Product A', quantitySold: 100, revenue: 5000.00),
  ///     ProductSummaryModel(productId: 'uuid-2', productName: 'Product B', quantitySold: 75, revenue: 3750.50),
  ///   ],
  /// );
  /// ```
  Future<MerchantDashboardSummaryModel?> getMerchantDashboardSummary({String? shopId}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return MerchantDashboardSummaryModel(
        totalSalesRevenue: KpiData(value: 15000.75),
        numberOfTransactions: KpiData(value: 300),
        averageOrderValue: KpiData(value: 50.00),
        topSellingProducts: [
          ProductSummaryModel(productId: 'uuid-1', productName: 'Product A', quantitySold: 100, revenue: 5000.00),
          ProductSummaryModel(productId: 'uuid-2', productName: 'Product B', quantitySold: 75, revenue: 3750.50),
        ],
      );
    }
    final token = _authService.authToken.value;
    if (token == null) {
      Get.snackbar('Error', 'Authentication token not found. Please login again.');
      return null;
    }

    String url = '${ApiConfig.baseUrl}/merchant/dashboard/summary';
    Map<String, String> queryParameters = {};
    if (shopId != null && shopId.isNotEmpty) {
      queryParameters['shop_id'] = shopId;
    }

    final response = await _getConnect.get(
      url,
      headers: {
        'Authorization': 'Bearer $token',
      },
      query: queryParameters.isNotEmpty ? queryParameters : null,
    );

    if (response.statusCode == 200) {
      if (response.body != null) {
        try {
          if (response.body is Map<String, dynamic>) {
            return MerchantDashboardSummaryModel.fromJson(response.body as Map<String, dynamic>);
          } else {
            Get.snackbar('Error', 'Invalid data format received from server for merchant dashboard.');
            return null;
          }
        } catch (e) {
          Get.snackbar('Error', 'Failed to parse merchant dashboard data: ${e.toString()}');
          return null;
        }
      } else {
        Get.snackbar('Error', 'Received empty response from server for merchant dashboard summary.');
        return null;
      }
    } else {
      String errorMessage = 'Failed to fetch merchant dashboard summary. Status Code: ${response.statusCode}';
      if (response.body != null && response.body['message'] != null) {
        errorMessage = response.body['message'];
      }
      Get.snackbar('Error', errorMessage);
      return null;
    }
  }
}
