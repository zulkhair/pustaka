import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import 'models/template.dart';

class TemplateRepository {
  TemplateRepository(this._client);
  final ApiClient _client;

  Future<List<Template>> fetch() {
    return _client.get<List<Template>>('/templates', parse: (data) {
      final list = data! as List<dynamic>;
      return list
          .map((e) => Template.fromJson(e as Map<String, dynamic>))
          .toList();
    });
  }
}

final templateRepositoryProvider = Provider<TemplateRepository>(
    (ref) => TemplateRepository(ref.watch(apiClientProvider)));
