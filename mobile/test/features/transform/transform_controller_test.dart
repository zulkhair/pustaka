import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/core/error/failure.dart';
import 'package:pustaka/features/library/data/models/document.dart';
import 'package:pustaka/features/transform/application/transform_controller.dart';
import 'package:pustaka/features/transform/data/models/output.dart';
import 'package:pustaka/features/transform/data/transform_repository.dart';

class MockTransformRepository extends Mock implements TransformRepository {}

Output _out() => Output(
      id: 'out1',
      documentId: 'd1',
      templateId: 't1',
      content: '# done',
      model: 'qwen',
      status: DocStatus.done,
      createdAt: DateTime(2026),
    );

void main() {
  test('run transitions running -> done with output', () async {
    final repo = MockTransformRepository();
    when(() => repo.run(any(), any())).thenAnswer((_) async => _out());
    final c = ProviderContainer(
        overrides: [transformRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(transformControllerProvider('d1').notifier).run('t1');
    final s = c.read(transformControllerProvider('d1'));
    expect(s.status, TransformStatus.done);
    expect(s.output!.content, '# done');
  });

  test('run failure sets failed + error', () async {
    final repo = MockTransformRepository();
    when(() => repo.run(any(), any())).thenThrow(const ApiFailure(1, 'no ocr'));
    final c = ProviderContainer(
        overrides: [transformRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(transformControllerProvider('d1').notifier).run('t1');
    final s = c.read(transformControllerProvider('d1'));
    expect(s.status, TransformStatus.failed);
    expect(s.error, 'no ocr');
  });
}
