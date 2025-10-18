import 'package:get/get.dart';
import 'package:smart_retail/app/data/models/inventory_item_model.dart';

class CartItem {
  final InventoryItem product;
  // CORRECTED: Made quantity observable to allow for granular updates in the UI.
  final RxInt quantity;

  CartItem({required this.product, int initialQuantity = 1}) : quantity = initialQuantity.obs;

  // The subtotal is now calculated based on the observable quantity's value.
  double get subtotal => product.sellingPrice * quantity.value;

  /// Increments the quantity of this item in the cart.
  void increment() {
    quantity.value++;
  }

  /// Decrements the quantity of this item in the cart.
  void decrement() {
    if (quantity.value > 0) {
      quantity.value--;
    }
  }
}
