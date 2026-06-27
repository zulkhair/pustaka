import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_client.dart';
import '../../../core/di/providers.dart';
import '../../library/data/models/document.dart';
import '../../transform/data/models/output.dart';
import 'models/page.dart';

typedef DocDetail = ({Document doc, List<DocPage> pages, List<Output> outputs});

/// Path of a page's full image (served via NetworkImageAuth so the JWT attaches).
String pageImagePath(String docId, int pageNumber) =>
    '/documents/$docId/pages/$pageNumber/image';

class ReaderRepository {
  ReaderRepository(this._client);
  final ApiClient _client;

  Future<DocDetail> fetchDoc(String id) {
    return _client.get<DocDetail>('/documents/$id', parse: (data) {
      final m = data! as Map<String, dynamic>;
      final doc = Document.fromJson(m['document'] as Map<String, dynamic>);
      final pages = (m['pages'] as List<dynamic>? ?? const [])
          .map((e) => DocPage.fromJson(e as Map<String, dynamic>))
          .toList();
      final outputs = (m['outputs'] as List<dynamic>? ?? const [])
          .map((e) => Output.fromJson(e as Map<String, dynamic>))
          .toList();
      return (doc: doc, pages: pages, outputs: outputs);
    });
  }
}

final readerRepositoryProvider = Provider<ReaderRepository>(
    (ref) => ReaderRepository(ref.watch(apiClientProvider)));
