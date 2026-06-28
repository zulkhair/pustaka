enum CaptureMode { photo, text }

enum DocStatus { pending, processing, done, failed }

CaptureMode captureModeFromString(Object? s) =>
    s == 'text' ? CaptureMode.text : CaptureMode.photo;

DocStatus docStatusFromString(Object? s) {
  switch (s) {
    case 'pending':
      return DocStatus.pending;
    case 'processing':
      return DocStatus.processing;
    case 'done':
      return DocStatus.done;
    default:
      return DocStatus.failed; // unknown -> safe default
  }
}

class Document {
  const Document({
    required this.id,
    required this.title,
    required this.mode,
    required this.pageCount,
    required this.status,
    required this.createdAt,
    this.thumbPage = 1,
    this.isOwner = false,
    this.thumbUrl,
  });

  final String id;
  final String title;
  final CaptureMode mode;
  final int pageCount;
  final DocStatus status;
  final DateTime createdAt;

  /// Which scanned page is used as the cover (1-based).
  final int thumbPage;

  /// NOT from JSON — the repository sets it (true for `owned`, false for `shared`).
  final bool isOwner;
  final String? thumbUrl;

  factory Document.fromJson(Map<String, dynamic> json) {
    return Document(
      id: json['id'] as String,
      title: json['title'] as String? ?? '',
      mode: captureModeFromString(json['mode']),
      pageCount: json['pageCount'] as int? ?? 0,
      status: docStatusFromString(json['status']),
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ??
          DateTime.fromMillisecondsSinceEpoch(0),
      thumbPage: json['thumbPage'] as int? ?? 1,
      thumbUrl: json['thumbUrl'] as String?,
    );
  }

  Document copyWith({bool? isOwner}) {
    return Document(
      id: id,
      title: title,
      mode: mode,
      pageCount: pageCount,
      status: status,
      createdAt: createdAt,
      thumbPage: thumbPage,
      isOwner: isOwner ?? this.isOwner,
      thumbUrl: thumbUrl,
    );
  }
}
