import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/error/failure.dart';
import '../data/models/output.dart';
import '../data/transform_repository.dart';

enum TransformStatus { idle, running, done, failed }

class TransformState {
  const TransformState(
      {this.status = TransformStatus.idle, this.output, this.error});
  final TransformStatus status;
  final Output? output;
  final String? error;
}

class TransformController extends FamilyNotifier<TransformState, String> {
  @override
  TransformState build(String arg) => const TransformState();

  Future<void> run(String templateId) async {
    state = const TransformState(status: TransformStatus.running);
    try {
      final out =
          await ref.read(transformRepositoryProvider).run(arg, templateId);
      state = TransformState(status: TransformStatus.done, output: out);
    } on Failure catch (f) {
      state = TransformState(status: TransformStatus.failed, error: f.message);
    } catch (e) {
      state =
          TransformState(status: TransformStatus.failed, error: e.toString());
    }
  }
}

final transformControllerProvider =
    NotifierProvider.family<TransformController, TransformState, String>(
        TransformController.new);
