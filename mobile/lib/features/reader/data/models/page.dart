import '../../../library/data/models/document.dart'
    show DocStatus, docStatusFromString;

/// One page of a document, as returned inside GET /documents/:id detail
/// (pageDTO: {pageNumber, status, ocrText, ocrStatus, imageUrl?, thumbUrl?}).
class DocPage {
  const DocPage({
    required this.pageNumber,
    required this.status,
    required this.hasImage,
    required this.ocrText,
    required this.ocrStatus,
  });

  final int pageNumber;
  final DocStatus status;
  final bool hasImage; // imageUrl present (photo mode)
  final String ocrText;
  final DocStatus ocrStatus;

  factory DocPage.fromJson(Map<String, dynamic> json) {
    final imageUrl = json['imageUrl'] as String?;
    return DocPage(
      pageNumber: json['pageNumber'] as int? ?? 0,
      status: docStatusFromString(json['status']),
      hasImage: imageUrl != null && imageUrl.isNotEmpty,
      ocrText: json['ocrText'] as String? ?? '',
      ocrStatus: docStatusFromString(json['ocrStatus']),
    );
  }
}
