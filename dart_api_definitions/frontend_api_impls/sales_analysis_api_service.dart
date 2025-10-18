import 'package:get/get.dart';

class SalesAnalysisApiService extends GetxService {
  /// Simulates a request to an AI backend for sales analysis.
  ///
  /// In a real application, this service would make an API call to a backend AI service.
  /// For now, we simulate the responses based on keywords in the prompt.
  ///
  /// __Request:__
  /// - __Method:__ POST
  /// - __Endpoint:__ `/api/v1/merchant/ai-analysis` (example)
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "prompt": "What are my best sellers?"
  ///   }
  ///   ```
  ///
  /// __Expected Response (Success):__
  /// - __Status Code:__ 200
  /// - __Body (JSON):__
  ///   ```json
  ///   {
  ///     "success": true,
  ///     "analysis": "Based on sales data..."
  ///   }
  ///   ```
  Future<String> getSalesAnalysis(String prompt) async {
    await Future.delayed(const Duration(seconds: 2)); // Simulate network latency

    final lowerCasePrompt = prompt.toLowerCase();

    if (lowerCasePrompt.contains('best sellers')) {
      return _getBestSellersResponse();
    } else if (lowerCasePrompt.contains('highest sales')) {
      return _getHighestSalesResponse();
    } else if (lowerCasePrompt.contains('peak hours')) {
      return _getPeakHoursResponse();
    }

    return "Sorry, I can't answer that question yet. Try asking about 'best sellers', 'highest sales', or 'peak hours'.";
  }

  String _getBestSellersResponse() {
    return 'Based on sales data from the last 30 days, your best-selling items are:\n\n'
        '1. Product No. 7 (150 units sold)\n'
        '2. Product No. 3 (125 units sold)\n'
        '3. Product No. 11 (98 units sold)';
  }

  String _getHighestSalesResponse() {
    return 'Your shops with the highest sales revenue are:\n\n'
        '1. Shop Branch 0 (\$5,430.50)\n'
        '2. Shop Branch 2 (\$4,890.00)\n'
        '3. Shop Branch 4 (\$4,510.20)';
  }

  String _getPeakHoursResponse() {
    return 'Your busiest sales period across all shops is between 5:00 PM and 7:00 PM on Fridays.';
  }
}
