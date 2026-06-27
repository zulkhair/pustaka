import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pustaka/app.dart';
import 'package:pustaka/core/auth/auth_controller.dart';

import 'package:pustaka/features/auth/presentation/login_screen.dart';
import 'support/fake_token_storage.dart';

void main() {
  testWidgets('app boots and lands on login when no tokens', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [tokenStorageProvider.overrideWithValue(FakeTokenStorage())],
        child: const PustakaApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(LoginScreen), findsOneWidget);
  });
}
