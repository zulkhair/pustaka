import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import 'models/share.dart';

class ShareRepository {
  ShareRepository(this._client);
  final ApiClient _client;

  Future<List<DocumentShare>> list(String docId) {
    return _client.get<List<DocumentShare>>('/documents/$docId/shares',
        parse: (data) {
      final list = data! as List<dynamic>;
      return list
          .map((e) => DocumentShare.fromJson(e as Map<String, dynamic>))
          .toList();
    });
  }

  Future<DocumentShare> add(String docId, String email) {
    return _client.post<DocumentShare>(
      '/documents/$docId/shares',
      body: {'email': email, 'permission': 'viewer'},
      parse: (d) => DocumentShare.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<void> revoke(String docId, String userId) async {
    await _client.delete<Object?>('/documents/$docId/shares/$userId',
        parse: (_) => null);
  }
}

final shareRepositoryProvider = Provider<ShareRepository>(
    (ref) => ShareRepository(ref.watch(apiClientProvider)));
