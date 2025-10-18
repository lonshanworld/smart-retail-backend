import 'dart:convert';

PaginatedNotificationsResponse paginatedNotificationsResponseFromJson(String str) => PaginatedNotificationsResponse.fromJson(json.decode(str));

class PaginatedNotificationsResponse {
    final bool success;
    final String message;
    final List<NotificationModel> data;
    final Meta meta;

    PaginatedNotificationsResponse({
        required this.success,
        required this.message,
        required this.data,
        required this.meta,
    });

    factory PaginatedNotificationsResponse.fromJson(Map<String, dynamic> json) => PaginatedNotificationsResponse(
        success: json["success"],
        message: json["message"],
        data: List<NotificationModel>.from(json["data"].map((x) => NotificationModel.fromJson(x))),
        meta: Meta.fromJson(json["meta"]),
    );
}

class NotificationModel {
    final String id;
    final String recipientUserId;
    final String title;
    final String message;
    final String type;
    final String? relatedEntityId;
    final String? relatedEntityType;
    final bool isRead;
    final DateTime createdAt;
    final DateTime updatedAt;

    NotificationModel({
        required this.id,
        required this.recipientUserId,
        required this.title,
        required this.message,
        required this.type,
        this.relatedEntityId,
        this.relatedEntityType,
        required this.isRead,
        required this.createdAt,
        required this.updatedAt,
    });

    factory NotificationModel.fromJson(Map<String, dynamic> json) => NotificationModel(
        id: json["id"],
        recipientUserId: json["recipientUserId"],
        title: json["title"],
        message: json["message"],
        type: json["type"],
        relatedEntityId: json["relatedEntityId"],
        relatedEntityType: json["relatedEntityType"],
        isRead: json["isRead"],
        createdAt: DateTime.parse(json["createdAt"]),
        updatedAt: DateTime.parse(json["updatedAt"]),
    );
    
    NotificationModel copyWith({bool? isRead}) {
        return NotificationModel(
            id: id,
            recipientUserId: recipientUserId,
            title: title,
            message: message,
            type: type,
            relatedEntityId: relatedEntityId,
            relatedEntityType: relatedEntityType,
            isRead: isRead ?? this.isRead,
            createdAt: createdAt,
            updatedAt: updatedAt,
        );
    }
}

class Meta {
    final int totalItems;
    final int currentPage;
    final int pageSize;
    final int totalPages;

    Meta({
        required this.totalItems,
        required this.currentPage,
        required this.pageSize,
        required this.totalPages,
    });

    factory Meta.fromJson(Map<String, dynamic> json) => Meta(
        totalItems: json["totalItems"],
        currentPage: json["currentPage"],
        pageSize: json["pageSize"],
        totalPages: json["totalPages"],
    );
}
