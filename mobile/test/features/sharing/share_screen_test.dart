import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:pustaka/features/sharing/data/models/share.dart';
import 'package:pustaka/features/sharing/data/share_repository.dart';
import 'package:pustaka/features/sharing/presentation/share_screen.dart';

class MockShareRepository extends Mock implements ShareRepository {}

void main() {
  testWidgets('lists shares and Share calls add(email)', (tester) async {
    final repo = MockShareRepository();
    final share = DocumentShare(
        userId: 'u2',
        email: 'b@e.com',
        permission: 'viewer',
        createdAt: DateTime(2026));
    when(() => repo.list(any())).thenAnswer((_) async => [share]);
    when(() => repo.add(any(), any())).thenAnswer((_) async => share);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [shareRepositoryProvider.overrideWithValue(repo)],
        child: const MaterialApp(home: ShareScreen(docId: 'd1')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('b@e.com'), findsOneWidget);
    expect(find.byIcon(Icons.delete_outline), findsOneWidget);

    await tester.enterText(
        find.byKey(const Key('shareEmailField')), 'new@e.com');
    await tester.tap(find.widgetWithText(FilledButton, 'Share'));
    await tester.pumpAndSettle();
    verify(() => repo.add('d1', 'new@e.com')).called(1);
  });
}
