import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/models/share.dart';
import '../data/share_repository.dart';

class ShareController extends FamilyAsyncNotifier<List<DocumentShare>, String> {
  @override
  Future<List<DocumentShare>> build(String arg) =>
      ref.read(shareRepositoryProvider).list(arg);

  /// Adds a viewer share. Re-lists on success. Rethrows the API error (e.g.
  /// unknown/unverified recipient) so the screen can surface the message.
  Future<void> add(String email) async {
    await ref.read(shareRepositoryProvider).add(arg, email);
    state = AsyncData(await ref.read(shareRepositoryProvider).list(arg));
  }

  Future<void> revoke(String userId) async {
    await ref.read(shareRepositoryProvider).revoke(arg, userId);
    state = AsyncData(
        (state.valueOrNull ?? []).where((s) => s.userId != userId).toList());
  }
}

final shareControllerProvider =
    AsyncNotifierProvider.family<ShareController, List<DocumentShare>, String>(
        ShareController.new);
