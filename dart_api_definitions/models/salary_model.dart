class Salary {
  final String id;
  final String staffId;
  final double amount;
  final DateTime paymentDate;
  final String? notes;

  Salary({
    required this.id,
    required this.staffId,
    required this.amount,
    required this.paymentDate,
    this.notes,
  });

  factory Salary.fromJson(Map<String, dynamic> json) {
    return Salary(
      id: json['id'] as String,
      staffId: json['staffId'] as String,
      amount: (json['amount'] as num).toDouble(),
      // CORRECTED: Assume the date comes as a standard ISO 8601 string from the API.
      paymentDate: DateTime.parse(json['paymentDate'] as String),
      notes: json['notes'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'staffId': staffId,
      'amount': amount,
      // CORRECTED: Convert DateTime to a string for JSON serialization.
      'paymentDate': paymentDate.toIso8601String(),
      'notes': notes,
    };
  }
}
