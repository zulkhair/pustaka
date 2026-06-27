/// Response of POST /documents/:id/pages and the re-run OCR endpoint:
/// {pageNumber, ocrText}.
class PageUploadResult {
  const PageUploadResult({required this.pageNumber, required this.ocrText});

  final int pageNumber;
  final String ocrText;

  factory PageUploadResult.fromJson(Map<String, dynamic> json) {
    return PageUploadResult(
      pageNumber: json['pageNumber'] as int? ?? 0,
      ocrText: json['ocrText'] as String? ?? '',
    );
  }
}
