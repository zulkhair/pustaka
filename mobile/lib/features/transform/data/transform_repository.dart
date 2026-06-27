import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import 'models/output.dart';

class TransformRepository {
  TransformRepository(this._client);
  final ApiClient _client;

  Future<Output> run(String docId, String templateId) {
    return _client.post<Output>(
      '/documents/$docId/transform',
      body: {'template_id': templateId},
      parse: (d) => Output.fromJson(d! as Map<String, dynamic>),
    );
  }

  Future<Output> getOutput(String id) {
    return _client.get<Output>(
      '/outputs/$id',
      parse: (d) => Output.fromJson(d! as Map<String, dynamic>),
    );
  }
}

final transformRepositoryProvider = Provider<TransformRepository>(
    (ref) => TransformRepository(ref.watch(apiClientProvider)));
