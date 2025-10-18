class MasterInventoryItem {
  final String id;
  final String name;
  // Add other relevant fields if needed, e.g., SKU, current_cost_price for prefill

  MasterInventoryItem({required this.id, required this.name});

  factory MasterInventoryItem.fromJson(Map<String, dynamic> json) {
    return MasterInventoryItem(
      id: json['id'] as String,
      name: json['name'] as String,
      // sku: json['sku'] as String?,
    );
  }

  // Override equals and hashCode for DropdownButtonFormField comparison if objects are used directly
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MasterInventoryItem &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;

  // Optional: For easy display in DropdownMenuItem if MasterInventoryItem instances are used as values
  @override
  String toString() {
    return name;
  }
}
