import 'package:equatable/equatable.dart';
import 'package:smart_retail/app/data/models/user_model.dart';
import 'package:smart_retail/app/data/enums/user_role.dart';

class UserSelectionItem extends Equatable {
  final String id;
  final String name;
  final String? email;
  final String? role; // Display string for role

  const UserSelectionItem({
    required this.id,
    required this.name,
    this.email,
    this.role,
  });

  factory UserSelectionItem.fromUser(User user) {
    return UserSelectionItem(
      id: user.id,
      name: user.name,
      email: user.email,
      role: user.role.toUserRole().toDisplayString(),
    );
  }

  // <<< NEW factory method >>>
  factory UserSelectionItem.fromJson(Map<String, dynamic> json) {
    String? displayRole;
    if (json['role'] != null && json['role'] is String) {
      // Assuming json['role'] is a string like "admin", "merchant"
      displayRole = (json['role'] as String).toUserRole().toDisplayString();
    } else if (json['role_display_name'] != null && json['role_display_name'] is String) {
      // Alternative: if the API directly provides the display name
      displayRole = json['role_display_name'] as String;
    }

    return UserSelectionItem(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String?,
      role: displayRole,
    );
  }

  @override
  List<Object?> get props => [id, name, email, role];

  @override
  String toString() {
    return name;
  }
}
