import 'package:smart_retail/app/data/models/user_model.dart';

class AdminPaginatedUsersResponse {
  final List<User> users;
  final int currentPage;
  final int totalPages;
  final int pageSize;
  final int totalCount;

  AdminPaginatedUsersResponse({
    required this.users,
    required this.currentPage,
    required this.totalPages,
    required this.pageSize,
    required this.totalCount,
  });

  factory AdminPaginatedUsersResponse.fromJson(Map<String, dynamic> json) {
    var usersList = json['users'] as List? ?? [];
    List<User> users = usersList.map((i) => User.fromJson(i as Map<String, dynamic>)).toList();

    return AdminPaginatedUsersResponse(
      users: users,
      currentPage: json['current_page'] as int? ?? 1,
      totalPages: json['total_pages'] as int? ?? 1,
      pageSize: json['page_size'] as int? ?? users.length,
      totalCount: json['total_count'] as int? ?? users.length,
    );
  }

  // Example if your API uses different keys:
  // factory AdminPaginatedUsersResponse.fromJson(Map<String, dynamic> json) {
  //   var dataList = json['data']?['items'] as List? ?? []; // Adjust based on your API
  //   List<User> users = dataList.map((i) => User.fromJson(i as Map<String, dynamic>)).toList();
  //   return AdminPaginatedUsersResponse(
  //     users: users,
  //     currentPage: json['data']?['pagination']?['currentPage'] as int? ?? 1,
  //     totalPages: json['data']?['pagination']?['totalPages'] as int? ?? 1,
  //     pageSize: json['data']?['pagination']?['pageSize'] as int? ?? users.length,
  //     totalCount: json['data']?['pagination']?['totalCount'] as int? ?? users.length,
  //   );
  // }

  Map<String, dynamic> toJson() {
    return {
      'users': users.map((user) => user.toJson()).toList(),
      'current_page': currentPage,
      'total_pages': totalPages,
      'page_size': pageSize,
      'total_count': totalCount,
    };
  }
}
