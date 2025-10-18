import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class PaginatedStockResponse {
  final List<InventoryItem> items;
  final int totalCount;

  PaginatedStockResponse({required this.items, required this.totalCount});

  factory PaginatedStockResponse.fromJson(Map<String, dynamic> json) {
    return PaginatedStockResponse(
      items: (json['items'] as List).map((i) => InventoryItem.fromJson(i)).toList(),
      totalCount: json['totalItems'],
    );
  }
}

class MerchantStocksApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/stocks';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a combined, paginated list of all inventory items from all shops.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/stocks`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int` (The page number to fetch)
  ///   - `pageSize`: `int` (The number of items per page)
  ///   - `searchTerm`: `String` (Optional search term for item name)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "status": "success",
  ///     "data": {
  ///       "items": [
  ///         {
  ///           "id": "uuid-item-1",
  ///           "name": "Laptop",
  ///           "sku": "LP123",
  ///           "quantity": 50,
  ///           "sellingPrice": 1299.99,
  ///           "originalPrice": 950.00,
  ///           "shopName": "Main Street Branch",
  ///           "shopId": "uuid-shop-1"
  ///         }
  ///       ],
  ///       "totalItems": 1
  ///     }
  ///   }
  ///   ```
  Future<PaginatedStockResponse> getCombinedStocks({int page = 1, int pageSize = 20, String? searchTerm}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 800));
      final items = List.generate(50, (i) => InventoryItem(
        id: 'item-$i',
        name: 'Product Name $i',
        sku: 'SKU-00$i',
        // quantity: (i * 5) % 100,
        sellingPrice: (i+1) * 12.5,
        originalPrice: ((i+1) * 12.5) * 0.7, // Original price is 70% of selling price
        shopName: i % 2 == 0 ? 'Downtown Store' : 'Uptown Mall',
        shopId: 'shop-${i%2}',
        merchantId: 'mock-merchant',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now()
      ));

      return PaginatedStockResponse(items: items, totalCount: items.length);
    }

    final query = {
      'page': page.toString(),
      'pageSize': pageSize.toString(),
      if (searchTerm != null && searchTerm.isNotEmpty) 'searchTerm': searchTerm,
    };

    final response = await _connect.get(_baseUrl, headers: await _getHeaders(), query: query);

    if (response.isOk && response.body['data'] != null) {
      return PaginatedStockResponse.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load combined stock data');
    }
  }
}
