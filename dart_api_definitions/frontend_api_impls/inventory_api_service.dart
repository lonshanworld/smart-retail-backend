import 'dart:convert';
import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';
import 'package:smart_retail/app/data/models/supplier_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class InventoryApiService extends GetxService {
  final GetConnect _connect = GetConnect(timeout: const Duration(seconds: 30));
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  Future<String?> _getAuthToken() async {
    return await _authService.getToken();
  }

  final String _inventoryBaseUrl = "${ApiConstants.baseUrl}/merchant/inventory";
  final String _suppliersBaseUrl = "${ApiConstants.baseUrl}/merchant/suppliers";

  // --- Supplier Specific Methods --- //

  /// Fetches a paginated list of suppliers for the merchant.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/suppliers`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int` (The page number to fetch)
  ///   - `pageSize`: `int` (The number of items per page)
  ///   - `name`: `string` (Optional name to filter by)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "status": "success",
  ///     "data": {
  ///       "data": [
  ///         {
  ///           "id": "uuid-supplier-1",
  ///           "merchantId": "uuid-merchant-1",
  ///           "name": "Supplier A",
  ///           "contactName": "John Smith",
  ///           "contactEmail": "john@suppliera.com",
  ///           "contactPhone": "1112223333",
  ///           "address": "123 Supplier Lane",
  ///           "notes": "Notes about Supplier A",
  ///           "createdAt": "2023-01-01T12:00:00Z",
  ///           "updatedAt": "2023-10-01T12:00:00Z"
  ///         }
  ///       ],
  ///       "pagination": {
  ///         "totalItems": 1,
  ///         "currentPage": 1,
  ///         "pageSize": 10,
  ///         "totalPages": 1
  ///       }
  ///     }
  ///   }
  ///   ```
  Future<PaginatedSuppliersResponse?> listMerchantSuppliers(
      {int page = 1, int pageSize = 10, String? nameQuery}) async { // Added nameQuery
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return PaginatedSuppliersResponse(suppliers: [], totalItems: 0, currentPage: 1, pageSize: 10, totalPages: 0);
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot list suppliers.");
      return null;
    }
    final queryParams = <String, String>{
      'page': page.toString(),
      'pageSize': pageSize.toString(),
    };
    if (nameQuery != null && nameQuery.isNotEmpty) {
      queryParams['name'] = nameQuery; // For filtering by name if needed
    }

    final response = await _connect.get(
      _suppliersBaseUrl,
      headers: {'Authorization': 'Bearer $token'},
      query: queryParams,
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      if (response.body['data'] != null) {
        return PaginatedSuppliersResponse.fromJson(response.body['data']);
      }
    }
    print('Error listing suppliers: ${response.statusCode} - ${response.bodyString}');
    return null;
  }

  /// Fetches the details for a single supplier.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/suppliers/{supplierId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The full supplier object)
  Future<Supplier?> getSupplierDetails(String supplierId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Supplier(id: supplierId, merchantId: '1', name: 'Supplier $supplierId', createdAt: DateTime.now(), updatedAt: DateTime.now());
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot get supplier details.");
      return null;
    }
    final response = await _connect.get(
      '$_suppliersBaseUrl/$supplierId',
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      if (response.body['data'] != null) {
        return Supplier.fromJson(response.body['data']);
      }
    }
    print('Error getting supplier details for $supplierId: ${response.statusCode} - ${response.bodyString}');
    return null;
  }

  /// Finds suppliers by name.
  ///
  /// This is a convenience method that uses `listMerchantSuppliers`.
  Future<List<Supplier>> findSuppliersByName(String name) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return [];
    }
    PaginatedSuppliersResponse? paginatedResponse = await listMerchantSuppliers(nameQuery: name, pageSize: 20);
    return paginatedResponse?.suppliers ?? [];
  }

  /// Creates a new supplier.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/suppliers`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "name": "New Supplier",
  ///     "contactName": "Jane Doe",
  ///     "contactEmail": "jane@newsupplier.com"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created supplier object)
  Future<Supplier?> createNewSupplier(Map<String, dynamic> supplierData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Supplier.fromJson(supplierData..['id'] = 'new-supplier-id'..['merchantId'] = '1'..['createdAt'] = DateTime.now().toIso8601String()..['updatedAt'] = DateTime.now().toIso8601String());
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot create supplier.");
      return null;
    }
    if (!supplierData.containsKey('name') || (supplierData['name'] as String).isEmpty) {
        print("Supplier name is required to create a new supplier.");
        return null;
    }
    final response = await _connect.post(
      _suppliersBaseUrl,
      supplierData,
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 201 && response.body['status'] == 'success') {
      if (response.body['data'] != null) {
        return Supplier.fromJson(response.body['data']);
      }
    }
    print('Error creating new supplier: ${response.statusCode} - ${response.bodyString}');
    return null;
  }

  /// Updates an existing supplier.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/suppliers/{supplierId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Fields to be updated)
  ///   ```json
  ///   {
  ///     "contactPhone": "4445556666"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The updated supplier object)
  Future<Supplier?> updateExistingSupplier(String supplierId, Map<String, dynamic> supplierData) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Supplier.fromJson(supplierData..['id'] = supplierId..['merchantId'] = '1'..['createdAt'] = DateTime.now().toIso8601String()..['updatedAt'] = DateTime.now().toIso8601String());
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot update supplier.");
      return null;
    }
    if (supplierData.containsKey('name') && (supplierData['name'] as String).isEmpty) {
        print("Supplier name cannot be empty if provided for update.");
        return null; 
    }
    final response = await _connect.put(
      '$_suppliersBaseUrl/$supplierId',
      supplierData,
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      if (response.body['data'] != null) {
        return Supplier.fromJson(response.body['data']);
      }
    }
    print('Error updating supplier $supplierId: ${response.statusCode} - ${response.bodyString}');
    return null;
  }

  /// Deletes a supplier.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/merchant/suppliers/{supplierId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200 (or 204)
  Future<bool> deleteExistingSupplier(String supplierId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return true;
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot delete supplier.");
      return false;
    }
    final response = await _connect.delete(
      '$_suppliersBaseUrl/$supplierId',
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return true;
    }
    print('Error deleting supplier $supplierId: ${response.statusCode} - ${response.bodyString}');
    return false;
  }

  /// Resolves a supplier name to a supplier ID.
  /// If the supplier exists, its ID is returned. If not, a new supplier is created and its ID is returned.
  Future<String?> _resolveSupplierNameToId(String? supplierName) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return 'mock-supplier-id';
    }
    if (supplierName == null || supplierName.trim().isEmpty) {
      return null;
    }
    try {
      List<Supplier> existingSuppliers = await findSuppliersByName(supplierName);
      Supplier? matchedSupplier = existingSuppliers.firstWhereOrNull(
          (s) => s.name.toLowerCase() == supplierName.toLowerCase());

      if (matchedSupplier != null) {
        return matchedSupplier.id;
      } else {
        print("Supplier '$supplierName' not found, creating new one for product association.");
        Supplier? newSupplier = await createNewSupplier({'name': supplierName});
        return newSupplier?.id;
      }
    } catch (e) {
      print("Error resolving supplier name to ID for '$supplierName': $e");
      return null;
    }
  }

  // --- Inventory Item Methods --- //

  /// Fetches a paginated list of inventory items.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/inventory`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Query Parameters:__
  ///   - `page`: `int`
  ///   - `pageSize`: `int`
  ///   - `name`: `string` (Optional name filter)
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
  ///           "merchantId": "uuid-merchant-1",
  ///           "name": "Laptop",
  ///           "description": "A powerful laptop",
  ///           "sku": "LP123",
  ///           "sellingPrice": 1200.00,
  ///           "originalPrice": 800.00,
  ///           "lowStockThreshold": 10,
  ///           "category": "Electronics",
  ///           "supplier": "Supplier A",
  ///           "isArchived": false,
  ///           "createdAt": "2023-01-10T10:00:00Z",
  ///           "updatedAt": "2023-10-26T10:00:00Z"
  ///         }
  ///       ],
  ///       "totalItems": 1,
  ///       "currentPage": 1,
  ///       "pageSize": 10,
  ///       "totalPages": 1
  ///     }
  ///   }
  ///   ```
  Future<PaginatedInventoryResponse?> listInventoryItems(
      {int page = 1, int pageSize = 10, String? nameFilter}) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      final items = [
        InventoryItem(id: '1', merchantId: '1', name: 'Mock Item 1', sellingPrice: 10.0, originalPrice: 7.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
        InventoryItem(id: '2', merchantId: '1', name: 'Mock Item 2', sellingPrice: 20.0, originalPrice: 14.0, createdAt: DateTime.now(), updatedAt: DateTime.now()),
      ];
      return PaginatedInventoryResponse(items: items, totalItems: 2, currentPage: 1, pageSize: 10, totalPages: 1);
    }
    final token = await _getAuthToken();
    if (token == null) {
      print("Auth token is null, cannot fetch inventory items.");
      return null;
    }
    final queryParameters = <String, String>{
      'page': page.toString(),
      'pageSize': pageSize.toString(),
    };
    if (nameFilter != null && nameFilter.isNotEmpty) {
      queryParameters['name'] = nameFilter;
    }
    final response = await _connect.get(
      _inventoryBaseUrl,
      headers: {'Authorization': 'Bearer $token'},
      query: queryParameters,
    );
    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return PaginatedInventoryResponse.fromJson(response.body['data']);
    } else {
      print('Error fetching inventory: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Creates a new inventory item.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/inventory`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (The `InventoryItem` object serialized for creation)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (Returns the newly created inventory item object)
  Future<InventoryItem?> createInventoryItem(InventoryItem item) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return item.copyWith(id: 'new-item-id');
    }
    final token = await _getAuthToken();
    if (token == null) return null;

    Map<String, dynamic> payload = item.toJsonForCreate();

    if (item.supplier != null && item.supplier!.isNotEmpty) {
      String? supplierId = await _resolveSupplierNameToId(item.supplier!);
      if (supplierId != null) {
        payload['supplierId'] = supplierId;
      }
      payload.remove('supplier');
    } else {
      payload.remove('supplier');
    }

    print("Creating inventory item with payload: ${jsonEncode(payload)}");

    final response = await _connect.post(
      _inventoryBaseUrl,
      payload,
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 201 && response.body['status'] == 'success') {
      return InventoryItem.fromJson(response.body['data']);
    } else {
      print('Error creating inventory item: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Fetches a single inventory item by its ID.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/inventory/{itemId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (The full inventory item object)
  Future<InventoryItem?> getInventoryItemById(String itemId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return InventoryItem(id: itemId, merchantId: '1', name: 'Item $itemId', sellingPrice: 10.0, originalPrice: 7.0, createdAt: DateTime.now(), updatedAt: DateTime.now());
    }
    final token = await _getAuthToken();
    if (token == null) return null;
    final response = await _connect.get(
      '$_inventoryBaseUrl/$itemId',
      headers: {'Authorization': 'Bearer $token'},
    );
    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return InventoryItem.fromJson(response.body['data']);
    } else {
      print('Error fetching item $itemId: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Updates an existing inventory item.
  ///
  /// __Request:__
  /// - __Method:__ PUT
  /// - __Endpoint:__ `/api/v1/merchant/inventory/{itemId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Map of fields to be updated)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (Returns the updated inventory item object)
  Future<InventoryItem?> updateInventoryItem(String itemId, Map<String, dynamic> updates) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      // Use a more realistic mock update
      final updatedItem = InventoryItem(
          id: itemId, 
          merchantId: '1', 
          name: updates['name'] ?? 'Updated Name', 
          sellingPrice: updates['sellingPrice'] ?? 10.0,
          originalPrice: updates['originalPrice'] ?? 7.0,
          createdAt: DateTime.now().subtract(const Duration(days: 1)), 
          updatedAt: DateTime.now()
      );
      return updatedItem;
    }
    final token = await _getAuthToken();
    if (token == null) return null;

    Map<String, dynamic> payload = Map<String, dynamic>.from(updates);

    if (payload.containsKey('supplier') && payload['supplier'] is String) {
      String supplierName = payload['supplier'] as String;
      if (supplierName.isNotEmpty) {
        String? supplierId = await _resolveSupplierNameToId(supplierName);
        if (supplierId != null) {
          payload['supplierId'] = supplierId;
        }
      }
      payload.remove('supplier');
    } else {
       payload.remove('supplier');
    }

    print("Updating inventory item $itemId with payload: ${jsonEncode(payload)}");

    final response = await _connect.put(
      '$_inventoryBaseUrl/$itemId',
      payload,
      headers: {'Authorization': 'Bearer $token'},
    );

    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return InventoryItem.fromJson(response.body['data']);
    } else {
      print('Error updating item $itemId: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Archives an inventory item.
  ///
  /// __Request:__
  /// - __Method:__ PATCH
  /// - __Endpoint:__ `/api/v1/merchant/inventory/{itemId}/archive`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (Returns the updated, archived inventory item object)
  Future<InventoryItem?> archiveInventoryItem(String itemId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return InventoryItem(id: itemId, merchantId: '1', name: 'Item $itemId', sellingPrice: 10.0, originalPrice: 7.0, createdAt: DateTime.now(), updatedAt: DateTime.now(), isArchived: true);
    }
    final token = await _getAuthToken();
    if (token == null) return null;
    final response = await _connect.patch(
      '$_inventoryBaseUrl/$itemId/archive',
      {},
      headers: {'Authorization': 'Bearer $token'},
    );
    if (response.statusCode == 200 && response.body['status'] == 'success') {
      return InventoryItem.fromJson(response.body['data']);
    } else {
      print('Error archiving item $itemId: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Unarchives an inventory item.
  ///
  /// __Request:__
  /// - __Method:__ PATCH
  /// - __Endpoint:__ `/api/v1/merchant/inventory/{itemId}/unarchive`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (Returns the updated, unarchived inventory item object)
  Future<InventoryItem?> unarchiveInventoryItem(String itemId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return InventoryItem(id: itemId, merchantId: '1', name: 'Item $itemId', sellingPrice: 10.0, originalPrice: 7.0, createdAt: DateTime.now(), updatedAt: DateTime.now(), isArchived: false);
    }
    final token = await _getAuthToken();
    if (token == null) return null;
    final response = await _connect.patch(
      '$_inventoryBaseUrl/$itemId/unarchive',
      {},
      headers: {'Authorization': 'Bearer $token'},
    );
     if (response.statusCode == 200 && response.body['status'] == 'success') {
      return InventoryItem.fromJson(response.body['data']);
    } else {
      print('Error unarchiving item $itemId: ${response.statusCode} - ${response.bodyString}');
      return null;
    }
  }

  /// Deletes an inventory item.
  ///
  /// __Request:__
  /// - __Method:__ DELETE
  /// - __Endpoint:__ `/api/v1/merchant/inventory/{itemId}`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200 (or 204)
  Future<bool> deleteInventoryItem(String itemId) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return true;
    }
    final token = await _getAuthToken();
    if (token == null) return false;
    final response = await _connect.delete(
      '$_inventoryBaseUrl/$itemId',
      headers: {'Authorization': 'Bearer $token'},
    );
    if (response.statusCode == 200 && response.body['status'] == 'success') { // Backend uses 200 for successful delete
      return true;
    } else {
      print('Error deleting item $itemId: ${response.statusCode} - ${response.bodyString}');
      return false;
    }
  }
}
