import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import 'models/document.dart';

typedef LibraryDocs = ({List<Document> owned, List<Document> shared});

class LibraryRepository {
  LibraryRepository(this._client);
  final ApiClient _client;

  Future<LibraryDocs> fetch() {
    return _client.get<LibraryDocs>('/documents', parse: (data) {
      final m = data! as Map<String, dynamic>;
      final owned = (m['owned'] as List<dynamic>? ?? const [])
          .map((e) => Document.fromJson(e as Map<String, dynamic>)
              .copyWith(isOwner: true))
          .toList();
      final shared = (m['shared'] as List<dynamic>? ?? const [])
          .map((e) => Document.fromJson(e as Map<String, dynamic>))
          .toList();
      return (owned: owned, shared: shared);
    });
  }

  Future<Document> createDocument(String title, CaptureMode mode) {
    return _client.post<Document>(
      '/documents',
      body: {'title': title, 'mode': mode.name},
      parse: (d) =>
          Document.fromJson(d! as Map<String, dynamic>).copyWith(isOwner: true),
    );
  }

  Future<Document> renameDocument(String id, String title) {
    return _client.patch<Document>(
      '/documents/$id',
      body: {'title': title},
      parse: (d) =>
          Document.fromJson(d! as Map<String, dynamic>).copyWith(isOwner: true),
    );
  }

  Future<void> deleteDocument(String id) {
    return _client.delete<void>('/documents/$id', parse: (_) {});
  }

  Future<void> setThumbnail(String id, int page) {
    return _client.patch<void>(
      '/documents/$id/thumbnail',
      body: {'page': page},
      parse: (_) {},
    );
  }
}

final libraryRepositoryProvider = Provider<LibraryRepository>(
    (ref) => LibraryRepository(ref.watch(apiClientProvider)));
