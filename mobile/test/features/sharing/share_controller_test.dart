import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/core/error/failure.dart';
import 'package:pustaka/features/sharing/application/share_controller.dart';
import 'package:pustaka/features/sharing/data/models/share.dart';
import 'package:pustaka/features/sharing/data/share_repository.dart';

class MockShareRepository extends Mock implements ShareRepository {}

DocumentShare _share(String userId, String email) => DocumentShare(
    userId: userId,
    email: email,
    permission: 'viewer',
    createdAt: DateTime(2026));

void main() {
  test('add posts then re-lists; revoke removes', () async {
    final repo = MockShareRepository();
    final shares = <DocumentShare>[];
    when(() => repo.list(any())).thenAnswer((_) async => List.of(shares));
    when(() => repo.add(any(), any())).thenAnswer((_) async {
      final s = _share('u2', 'b@e.com');
      shares.add(s);
      return s;
    });
    when(() => repo.revoke(any(), any()))
        .thenAnswer((_) async => shares.clear());

    final c = ProviderContainer(
        overrides: [shareRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    expect(await c.read(shareControllerProvider('d1').future), isEmpty);
    await c.read(shareControllerProvider('d1').notifier).add('b@e.com');
    expect(c.read(shareControllerProvider('d1')).valueOrNull, hasLength(1));
    await c.read(shareControllerProvider('d1').notifier).revoke('u2');
    expect(c.read(shareControllerProvider('d1')).valueOrNull, isEmpty);
  });

  test('add surfaces the API error for an invalid recipient', () async {
    final repo = MockShareRepository();
    when(() => repo.list(any())).thenAnswer((_) async => <DocumentShare>[]);
    when(() => repo.add(any(), any()))
        .thenThrow(const ApiFailure(1, 'cannot share with this user'));

    final c = ProviderContainer(
        overrides: [shareRepositoryProvider.overrideWithValue(repo)]);
    addTearDown(c.dispose);

    await c.read(shareControllerProvider('d1').future);
    expect(
      () => c.read(shareControllerProvider('d1').notifier).add('ghost@e.com'),
      throwsA(isA<ApiFailure>()),
    );
  });
}
