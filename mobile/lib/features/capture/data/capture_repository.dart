import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import 'models/page_upload_result.dart';

class CaptureRepository {
  CaptureRepository(this._client);
  final ApiClient _client;

  Future<PageUploadResult> uploadPage(String docId, Uint8List bytes) {
    return _client.upload<PageUploadResult>(
      '/documents/$docId/pages',
      field: 'file',
      bytes: bytes,
      filename: 'page.jpg',
      parse: (d) => PageUploadResult.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<PageUploadResult> rerunOcr(String docId, int pageNumber) {
    return _client.post<PageUploadResult>(
      '/documents/$docId/pages/$pageNumber/ocr',
      parse: (d) => PageUploadResult.fromJson(d! as Map<String, dynamic>),
    );
  }
}

final captureRepositoryProvider = Provider<CaptureRepository>(
    (ref) => CaptureRepository(ref.watch(apiClientProvider)));
