import '../../../library/data/models/document.dart'
    show DocStatus, docStatusFromString;

/// A generated transform output (backend fullOutputDTO:
/// {id, documentId, templateId, content, model, status, createdAt}).
/// No `format` field — OutputView infers rendering from the content.
class Output {
  const Output({
    required this.id,
    required this.documentId,
    required this.templateId,
    required this.content,
    required this.model,
    required this.status,
    required this.createdAt,
  });

  final String id;
  final String documentId;
  final String templateId;
  final String content;
  final String model;
  final DocStatus status;
  final DateTime createdAt;

  factory Output.fromJson(Map<String, dynamic> json) {
    return Output(
      id: json['id'] as String,
      documentId: json['documentId'] as String? ?? '',
      templateId: json['templateId'] as String? ?? '',
      content: json['content'] as String? ?? '',
      model: json['model'] as String? ?? '',
      status: docStatusFromString(json['status']),
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ??
          DateTime.fromMillisecondsSinceEpoch(0),
    );
  }
}
