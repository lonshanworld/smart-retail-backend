import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';

class ShopItemsApiService extends GetxService {
  final AppConfig _appConfig = Get.find<AppConfig>();

  // Mock data for the shop's inventory
  final List<InventoryItem> _mockItems = List.generate(12, (i) => InventoryItem(
    id: 'item-shop-0-$i',
    merchantId: 'mock-merchant',
    name: 'Shop Item $i',
    sku: 'SHP-SKU-00$i',
    sellingPrice: (i + 1) * 5.0,
    originalPrice: ((i + 1) * 5.0) * 0.9,
    shopId: 'shop-0',
    createdAt: DateTime.now(),
    updatedAt: DateTime.now(),
    stock: StockInfo(quantity: 10 + i * 5, shopId: 'shop-0'), // CORRECTED
  ));

  /// Fetches all inventory items for a specific shop.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/shop/items`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of `InventoryItem` objects)
  Future<List<InventoryItem>> getShopItems() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 800));
      return _mockItems;
    }
    // Real API call would go here
    throw UnimplementedError();
  }

  /// Updates the stock quantity for a specific item in a shop.
  /// This is a simple quantity update, not a full stock-in process.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/shop/items/{itemId}/stock`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "quantity": 50
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  Future<void> updateStockQuantity(String itemId, int newQuantity) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(milliseconds: 500));
      final index = _mockItems.indexWhere((item) => item.id == itemId);
      if (index != -1) {
        final currentItem = _mockItems[index];
        final newStock = StockInfo(quantity: newQuantity, shopId: currentItem.stock!.shopId);
        _mockItems[index] = currentItem.copyWith(stock: newStock); // CORRECTED
      } else {
        throw Exception('Item not found in mock data');
      }
      return;
    }
    // Real API call would go here
    throw UnimplementedError();
  }
}
