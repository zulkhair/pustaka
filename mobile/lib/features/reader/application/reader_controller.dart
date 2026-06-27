import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../library/application/library_controller.dart';
import '../../library/data/models/document.dart';
import '../../transform/data/models/output.dart';
import '../data/models/page.dart';
import '../data/reader_repository.dart';

class ReaderState {
  const ReaderState({
    required this.doc,
    required this.pages,
    required this.outputs,
    required this.isOwner,
  });

  final Document doc;
  final List<DocPage> pages;
  final List<Output> outputs;
  final bool isOwner;
}

class ReaderController extends FamilyAsyncNotifier<ReaderState, String> {
  Future<ReaderState> _load() async {
    final detail = await ref.read(readerRepositoryProvider).fetchDoc(arg);
    // Ownership is the library's source of truth (the detail endpoint doesn't
    // distinguish owner from sharee).
    final isOwner = ref.read(documentByIdProvider(arg))?.isOwner ?? false;
    return ReaderState(
      doc: detail.doc,
      pages: detail.pages,
      outputs: detail.outputs,
      isOwner: isOwner,
    );
  }

  @override
  Future<ReaderState> build(String arg) => _load();

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(_load);
  }
}

final readerControllerProvider =
    AsyncNotifierProvider.family<ReaderController, ReaderState, String>(
        ReaderController.new);
