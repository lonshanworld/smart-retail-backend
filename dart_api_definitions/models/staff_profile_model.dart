class StaffProfile {
  final String id;
  final String name;
  final String email;
  final String role;
  final String? shopName;
  final double salary;
  final String payFrequency;

  StaffProfile({
    required this.id,
    required this.name,
    required this.email,
    required this.role,
    this.shopName,
    required this.salary,
    required this.payFrequency,
  });

  factory StaffProfile.fromJson(Map<String, dynamic> json) {
    return StaffProfile(
      id: json['id'],
      name: json['name'],
      email: json['email'],
      role: json['role'],
      shopName: json['shopName'],
      salary: (json['salary'] as num).toDouble(),
      payFrequency: json['payFrequency'],
    );
  }
}
