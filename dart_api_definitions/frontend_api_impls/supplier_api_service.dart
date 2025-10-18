import 'package:get/get.dart';
import 'package:smart_retail/app/core/config/app_config.dart';
import 'package:smart_retail/app/data/models/supplier_model.dart';
import 'package:smart_retail/app/data/providers/api_constants.dart';
import 'package:smart_retail/app/data/services/auth_service.dart';

class SupplierApiService extends GetxService {
  final GetConnect _connect = Get.find<GetConnect>();
  final AuthService _authService = Get.find<AuthService>();
  final AppConfig _appConfig = Get.find<AppConfig>();

  String get _baseUrl => '${ApiConstants.baseUrl}/merchant/suppliers';

  Future<Map<String, String>> _getHeaders() async {
    final token = await _authService.getToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  /// Fetches a list of suppliers.
  ///
  /// __Request:__
  /// - __Method:__ GET
  /// - __Endpoint:__ `/api/v1/merchant/suppliers`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__ (A list of supplier objects)
  Future<List<Supplier>> getSuppliers() async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return [
        Supplier(id: '1', merchantId: '1', name: 'Supplier A', contactName: 'John A', contactEmail: 'john.a@supplier.com'),
        Supplier(id: '2', merchantId: '1', name: 'Supplier B', contactName: 'John B', contactEmail: 'john.b@supplier.com'),
      ];
    }

    final response = await _connect.get(_baseUrl, headers: await _getHeaders());

    if (response.isOk && response.body['data'] != null) {
      return (response.body['data'] as List)
          .map((json) => Supplier.fromJson(json))
          .toList();
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to load suppliers');
    }
  }

  /// Creates a new supplier.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/suppliers`
  /// - __Headers:__
  ///   - `Authorization: Bearer <token>`
  /// - __Body (JSON):__ (Supplier data)
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 201
  /// - __Body (JSON):__ (The newly created supplier object)
  Future<Supplier> createSupplier(Map<String, dynamic> data) async {
    if (_appConfig.isDevelopment) {
      await Future.delayed(const Duration(seconds: 1));
      return Supplier.fromJson(data..['id'] = 'new-supplier-id'..['merchantId'] = '1');
    }

    final response = await _connect.post(_baseUrl, data, headers: await _getHeaders());

    if (response.statusCode == 201 && response.body['data'] != null) {
      return Supplier.fromJson(response.body['data']);
    } else {
      throw Exception(response.body?['message'] ?? 'Failed to create supplier');
    }
  }
}
