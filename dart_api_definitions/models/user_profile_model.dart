class UserProfile {
  final String name;
  final String email;
  final String role;
  final String shopName;

  UserProfile({
    required this.name,
    required this.email,
    required this.role,
    required this.shopName,
  });

  factory UserProfile.fromJson(Map<String, dynamic> json) {
    return UserProfile(
      name: json['name'],
      email: json['email'],
      role: json['role'],
      shopName: json['shopName'],
    );
  }
}
