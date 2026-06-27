import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/library_repository.dart';
import '../data/models/document.dart';

class LibraryController extends AsyncNotifier<LibraryDocs> {
  LibraryRepository get _repo => ref.read(libraryRepositoryProvider);

  @override
  Future<LibraryDocs> build() => _repo.fetch();

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(_repo.fetch);
  }

  Future<Document> createDocument(String title, CaptureMode mode) async {
    final doc = await _repo.createDocument(title, mode);
    final cur =
        state.valueOrNull ?? (owned: <Document>[], shared: <Document>[]);
    state = AsyncData((owned: [doc, ...cur.owned], shared: cur.shared));
    return doc;
  }
}

final libraryControllerProvider =
    AsyncNotifierProvider<LibraryController, LibraryDocs>(
        LibraryController.new);

/// Looks up a document already loaded in the library (owned or shared).
final documentByIdProvider = Provider.family<Document?, String>((ref, id) {
  final docs = ref.watch(libraryControllerProvider).valueOrNull;
  if (docs == null) return null;
  for (final d in [...docs.owned, ...docs.shared]) {
    if (d.id == id) return d;
  }
  return null;
});
