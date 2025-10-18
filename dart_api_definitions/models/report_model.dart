import 'dart:convert';
import 'package:smart_retail/app/data/models/sale_model.dart'; // ADDED: Import for Sale model

// ADDED: New class for Sales Report Response
class SalesReportResponse {
  final List<Sale> sales;

  SalesReportResponse({required this.sales});

  factory SalesReportResponse.fromJson(Map<String, dynamic> json) {
    return SalesReportResponse(
      sales: (json['sales'] as List).map((i) => Sale.fromJson(i)).toList(),
    );
  }
}


SalesForecastResponse salesForecastResponseFromJson(String str) => SalesForecastResponse.fromJson(json.decode(str));

class SalesForecastResponse {
    final String reportName;
    final DateTime generatedAt;
    final String productName;
    final String shopName;
    final int currentStock;
    final ForecastPeriod forecastPeriod;
    final List<DailyForecast> dailyForecast;
    final AiAnalysis aiAnalysis;

    SalesForecastResponse({
        required this.reportName,
        required this.generatedAt,
        required this.productName,
        required this.shopName,
        required this.currentStock,
        required this.forecastPeriod,
        required this.dailyForecast,
        required this.aiAnalysis,
    });

    factory SalesForecastResponse.fromJson(Map<String, dynamic> json) => SalesForecastResponse(
        reportName: json["reportName"],
        generatedAt: DateTime.parse(json["generatedAt"]),
        productName: json["productName"],
        shopName: json["shopName"],
        currentStock: json["currentStock"],
        forecastPeriod: ForecastPeriod.fromJson(json["forecastPeriod"]),
        dailyForecast: List<DailyForecast>.from(json["dailyForecast"].map((x) => DailyForecast.fromJson(x))),
        aiAnalysis: AiAnalysis.fromJson(json["aiAnalysis"]),
    );
}

class AiAnalysis {
    final String summary;
    final List<String> positiveFactors;
    final List<String> negativeFactors;

    AiAnalysis({
        required this.summary,
        required this.positiveFactors,
        required this.negativeFactors,
    });

    factory AiAnalysis.fromJson(Map<String, dynamic> json) => AiAnalysis(
        summary: json["summary"],
        positiveFactors: List<String>.from(json["positiveFactors"].map((x) => x)),
        negativeFactors: List<String>.from(json["negativeFactors"].map((x) => x)),
    );
}

class DailyForecast {
    final DateTime date;
    final int predictedQuantity;

    DailyForecast({
        required this.date,
        required this.predictedQuantity,
    });

    factory DailyForecast.fromJson(Map<String, dynamic> json) => DailyForecast(
        date: DateTime.parse(json["date"]),
        predictedQuantity: json["predictedQuantity"],
    );
}

class ForecastPeriod {
    final DateTime startDate;
    final DateTime endDate;

    ForecastPeriod({
        required this.startDate,
        required this.endDate,
    });

    factory ForecastPeriod.fromJson(Map<String, dynamic> json) => ForecastPeriod(
        startDate: DateTime.parse(json["startDate"]),
        endDate: DateTime.parse(json["endDate"]),
    );
}
