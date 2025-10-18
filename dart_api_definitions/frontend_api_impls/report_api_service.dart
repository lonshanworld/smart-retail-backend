import 'dart:math';

import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/report_model.dart';
import 'package:smart_retail/app/data/models/sale_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class ReportApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/reports';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a sales report based on specified filters.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/reports/sales`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `startDate`: `string` (ISO 8601 format)
  ///   - `endDate`: `string` (ISO 8601 format)
  ///   - `shopId`: `string` (Optional)
  ///   - `groupBy`: `string` (e.g., 'daily', 'weekly', 'monthly')
  Future<SalesReportResponse> getSalesReport({
    required DateTime startDate,
    required DateTime endDate,
    String? shopId,
    String? groupBy,
  }) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final mockSales = List.generate(20, (index) {
        final date = startDate.add(Duration(days: index % 7));
        return Sale(
          id: 'sale-mock-$index',
          merchantId: 'merchant-mock',
          shopId: shopId ?? 'shop-mock-${index % 3}',
          saleDate: date,
          totalAmount: Random().nextDouble() * 200 + 50,
          items: [], // Simplified for this mock
          paymentType: 'cash',
          paymentStatus: 'succeeded',
          createdAt: date,
          updatedAt: date,
        );
      });
      return SalesReportResponse(sales: mockSales);
    }

    final response = await _connect.get(
      '$_baseUrl/sales',
      headers: await _getHeaders(),
      query: {
        'startDate': startDate.toIso8601String(),
        'endDate': endDate.toIso8601String(),
        if (shopId != null) 'shopId': shopId,
        if (groupBy != null) 'groupBy': groupBy,
      },
    );

    if (response.isOk && response.body['data'] != null) {
      return SalesReportResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load sales report');
    }
  }

  /// Fetches a sales forecast for a specific item in a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/reports/sales-forecast`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `shopId`: `string` (UUID of the shop)
  ///   - `itemId`: `string` (UUID of the inventory item)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The sales forecast response object)
  Future<SalesForecastResponse> getSalesForecast(String shopId, String itemId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return SalesForecastResponse(
        reportName: 'Sales Forecast',
        generatedAt: DateTime.now(),
        productName: 'Mock Product',
        shopName: 'Mock Shop',
        currentStock: 100,
        forecastPeriod: ForecastPeriod(startDate: DateTime.now(), endDate: DateTime.now().add(const Duration(days: 7))),
        dailyForecast: [],
        aiAnalysis: AiAnalysis(summary: 'Mock summary', positiveFactors: [], negativeFactors: []),
      );
    }
    final response = await _connect.get(
      '$_baseUrl/sales-forecast',
      headers: await _getHeaders(),
      query: {'shopId': shopId, 'itemId': itemId},
    );

    if (response.statusCode == 200 && response.body['success'] == true) {
      return SalesForecastResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load sales forecast');
    }
  }
}
